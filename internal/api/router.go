// Package api wires together all HTTP routes, middleware, and handlers into a
// single http.Handler that can be passed to an http.Server.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"

	"github.com/dj/jobqueue/internal/api/handler"
	appMiddleware "github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
	"github.com/dj/jobqueue/internal/ws"
)

// RouterConfig bundles all dependencies required to build the router.
type RouterConfig struct {
	Store              store.JobStorer
	Broker             queue.Broker
	Hub                *ws.Hub
	Publisher          queue.EventPublisher
	DefaultMaxAttempts int
	RateLimitRPS       int
	RateLimitBurst     int
	StaticDir          string // path to built frontend; empty = no UI served
	APIKey             string // when non-empty, /api/v1/* requires X-API-Key
}

// NewRouter constructs and returns the application's HTTP router.
// It registers all API routes, attaches middleware, and wires handlers.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware stack (applied to every route) ──────────────────────
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(appMiddleware.Logger)
	r.Use(appMiddleware.CORS)
	r.Use(chiMiddleware.Recoverer)

	rateLimiter := appMiddleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	r.Use(rateLimiter.Middleware)

	// ── Instantiate handlers ──────────────────────────────────────────────────
	jobHandler := handler.NewJobHandler(
		cfg.Store,
		cfg.Broker,
		cfg.Publisher,
		cfg.DefaultMaxAttempts,
	)
	workerHandler := handler.NewWorkerHandler(cfg.Store)
	wsHandler := handler.NewWSHandler(cfg.Hub)

	// ── Routes ────────────────────────────────────────────────────────────────

	// Health and metrics (no version prefix — infrastructure consumers expect
	// these at a stable path).
	r.Get("/health", healthHandler(cfg.Store, cfg.Broker))
	r.Get("/metrics", metricsHandler(cfg.Store, cfg.Hub))

	// WebSocket upgrade endpoint.
	r.Get("/ws", wsHandler.ServeWS)

	// Versioned REST API — optionally gated by API key.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appMiddleware.APIKeyAuth(cfg.APIKey))
		// Jobs
		r.Post("/jobs", jobHandler.EnqueueJob)
		r.Get("/jobs", jobHandler.ListJobs)
		r.Get("/jobs/{id}", jobHandler.GetJob)
		r.Delete("/jobs/{id}", jobHandler.CancelJob)
		r.Post("/jobs/{id}/retry", jobHandler.RetryJob)

		// Dead-letter queue
		r.Get("/dlq", jobHandler.ListDLQ)
		r.Post("/dlq/{id}/requeue", jobHandler.RequeueDLQJob)

		// Workers
		r.Get("/workers", workerHandler.ListWorkers)

		// Stats
		r.Get("/stats", jobHandler.GetStats)
	})

	// Serve the React SPA if a static dir is configured.
	if cfg.StaticDir != "" {
		fs := http.FileServer(http.Dir(cfg.StaticDir))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// If the requested file exists, serve it; otherwise fall back to
			// index.html so the React router handles the path.
			path := cfg.StaticDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, cfg.StaticDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
	}

	return r
}

// ─── Infrastructure handlers ──────────────────────────────────────────────────

// healthResponse is the JSON payload returned by GET /health.
type healthResponse struct {
	Status   string            `json:"status"`
	Checks   map[string]string `json:"checks"`
	Uptime   string            `json:"uptime"`
}

// startTime is set once at package init so /health can report process uptime.
var startTime = time.Now()

// healthHandler returns an http.HandlerFunc that checks Redis and Postgres
// connectivity. It returns 200 when both are healthy, 503 otherwise.
func healthHandler(jobStore store.JobStorer, broker queue.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := make(map[string]string)
		healthy := true

		// Check Postgres.
		dbCtx, dbCancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer dbCancel()
		if err := jobStore.Ping(dbCtx); err != nil {
			checks["postgres"] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			checks["postgres"] = "healthy"
		}

		// Check Redis.
		redisCtx, redisCancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer redisCancel()
		if err := broker.Ping(redisCtx); err != nil {
			checks["redis"] = "unhealthy: " + err.Error()
			healthy = false
		} else {
			checks["redis"] = "healthy"
		}

		status := "ok"
		httpStatus := http.StatusOK
		if !healthy {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}

		resp := healthResponse{
			Status: status,
			Checks: checks,
			Uptime: time.Since(startTime).Round(time.Second).String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// metricsResponse is the JSON payload returned by GET /metrics.
type metricsResponse struct {
	QueueDepth    int64   `json:"queue_depth"`
	ActiveWorkers int     `json:"active_workers"`
	WSClients     int     `json:"ws_clients"`
	JobsPerMinute int64   `json:"jobs_per_minute"`
	DLQCount      int64   `json:"dlq_count"`
	FailedRate    float64 `json:"failed_rate"`
}

// metricsHandler returns a lightweight metrics snapshot without external
// instrumentation libraries.
func metricsHandler(jobStore store.JobStorer, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		stats, err := jobStore.GetStats(ctx)
		if err != nil {
			http.Error(w, `{"error":"failed to get metrics"}`, http.StatusInternalServerError)
			return
		}

		workers, err := jobStore.ListWorkers(ctx, true)
		if err != nil {
			http.Error(w, `{"error":"failed to list workers"}`, http.StatusInternalServerError)
			return
		}

		resp := metricsResponse{
			QueueDepth:    stats.QueueDepth,
			ActiveWorkers: len(workers),
			WSClients:     hub.ClientCount(),
			JobsPerMinute: stats.JobsPerMinute,
			DLQCount:      stats.DLQCount,
			FailedRate:    stats.FailedRate,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
