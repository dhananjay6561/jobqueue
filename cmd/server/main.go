// Command server is the entry point for the jobqueue HTTP server and worker pool.
//
// Startup sequence:
//  1. Load .env file (if present) then parse all configuration from environment
//  2. Connect to PostgreSQL and run pending migrations
//  3. Connect to Redis and create the broker
//  4. Start the WebSocket hub
//  5. Build and start the worker pool
//  6. Start the HTTP server
//  7. Wait for SIGINT or SIGTERM
//  8. Gracefully shut down: HTTP server → worker pool → database → redis
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dj/jobqueue/internal/api"
	"github.com/dj/jobqueue/internal/config"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
	"github.com/dj/jobqueue/internal/ws"
)

func main() {
	// ── Structured logging setup ───────────────────────────────────────────────
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Caller().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// ── Load .env (optional; does not override existing env vars) ─────────────
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Msg("could not load .env file")
	}

	// ── Parse configuration ────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}

	log.Info().
		Str("addr", cfg.Server.Addr()).
		Int("workers", cfg.Worker.Count).
		Msg("starting jobqueue server")

	// ── Connect to PostgreSQL ──────────────────────────────────────────────────
	ctx := context.Background()

	dbStore, err := store.New(ctx, cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer dbStore.Close()

	// Run migrations on startup.
	if err := dbStore.RunMigrations(ctx, "migrations"); err != nil {
		log.Fatal().Err(err).Msg("database migration failed")
	}

	log.Info().Msg("database connected and migrations applied")

	// ── Connect to Redis ───────────────────────────────────────────────────────
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close redis client")
		}
	}()

	broker, err := queue.NewRedisBroker(ctx, redisClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}

	log.Info().Str("addr", cfg.Redis.Addr).Msg("redis connected")

	// ── WebSocket hub ──────────────────────────────────────────────────────────
	hub := ws.NewHub()
	go hub.Run()

	// ── Stats broadcast ticker (every 5 seconds) ───────────────────────────────
	statsCtx, statsCancel := context.WithCancel(ctx)
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		runStatsBroadcast(statsCtx, dbStore, hub)
	}()

	// ── Worker pool ────────────────────────────────────────────────────────────
	workerPool := queue.NewPool(queue.PoolConfig{
		Config:    cfg.Worker,
		Retry:     cfg.Retry,
		Broker:    broker,
		Store:     dbStore,
		Publisher: hub,
		Queues:    []string{queue.DefaultQueueName, "critical", "bulk"},
	})

	// Register handlers for all job types exposed in the dashboard.
	// These simulate realistic work with a short sleep so the "running" state
	// is visible in the UI before the job completes.
	simulatedHandlers := []string{
		"noop",
		"send_email",
		"send_notification",
		"generate_report",
		"resize_image",
		"sync_data",
		"process_payment",
		"export_csv",
		"cleanup_storage",
	}
	for _, jobType := range simulatedHandlers {
		jt := jobType
		workerPool.Register(jt, func(_ context.Context, _ *queue.Job) error {
			time.Sleep(500 * time.Millisecond)
			return nil
		})
	}

	workerPool.Start(ctx)
	log.Info().Int("count", cfg.Worker.Count).Msg("worker pool started")

	// ── HTTP server ────────────────────────────────────────────────────────────
	router := api.NewRouter(api.RouterConfig{
		Store:              dbStore,
		Broker:             broker,
		Hub:                hub,
		Publisher:          hub,
		DefaultMaxAttempts: cfg.Retry.DefaultMaxAttempts,
		RateLimitRPS:       cfg.Server.RateLimit,
		RateLimitBurst:     cfg.Server.RateBurst,
		StaticDir:          "/frontend/dist",
	})

	httpServer := &http.Server{
		Addr:         cfg.Server.Addr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start HTTP server in background.
	serverErrors := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.Server.Addr()).Msg("HTTP server listening")
		serverErrors <- httpServer.ListenAndServe()
	}()

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}

	// 1. Stop accepting new HTTP requests.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info().Msg("shutting down HTTP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	// 2. Stop the stats broadcast goroutine.
	statsCancel()
	<-statsDone

	// 3. Stop the worker pool (waits for in-flight jobs to finish).
	log.Info().Msg("stopping worker pool")
	workerPool.Shutdown()

	log.Info().Msg("shutdown complete")
}

// runStatsBroadcast sends a stats.update WebSocket event every 5 seconds.
// It stops when ctx is cancelled.
func runStatsBroadcast(ctx context.Context, jobStore store.JobStorer, hub *ws.Hub) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			statsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			stats, err := jobStore.GetStats(statsCtx)
			cancel()

			if err != nil {
				log.Error().Err(err).Msg("stats broadcast: failed to get stats")
				continue
			}

			hub.Publish(queue.Event{
				Type:    queue.EventStatsUpdate,
				Payload: stats,
			})
		}
	}
}
