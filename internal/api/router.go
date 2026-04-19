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
	UserStore          store.UserStorer
	Broker             queue.Broker
	Hub                *ws.Hub
	Publisher          queue.EventPublisher
	DefaultMaxAttempts int
	RateLimitRPS       int
	RateLimitBurst     int
	StaticDir          string // path to built frontend; empty = no UI served
	APIKey             string // when non-empty, /api/v1/* requires X-API-Key
	AdminKey           string // when non-empty, requests with this key bypass scoping
	JWTSecret             string
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripeProPriceID      string
	StripeBusinessPriceID string
	BaseURL               string
	SMTPHost              string
	SMTPPort              string
	SMTPUser              string
	SMTPPass              string
	SMTPFrom              string
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
	// Stricter rate limiter for auth endpoints: 5 req/min per IP.
	authRateLimiter := appMiddleware.NewRateLimiter(1, 5)

	// ── Instantiate handlers ──────────────────────────────────────────────────
	jobHandler := handler.NewJobHandler(
		cfg.Store,
		cfg.Broker,
		cfg.Publisher,
		cfg.DefaultMaxAttempts,
	)
	workerHandler := handler.NewWorkerHandler(cfg.Store)
	wsHandler := handler.NewWSHandler(cfg.Hub)
	webhookHandler := handler.NewWebhookHandler(cfg.Store)
	cronHandler := handler.NewCronHandler(cfg.Store)
	apiKeyHandler := handler.NewAPIKeyHandler(cfg.Store)
	authHandler := handler.NewAuthHandler(
		cfg.UserStore, cfg.JWTSecret, cfg.BaseURL,
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom,
	)
	billingHandler := handler.NewBillingHandler(
		cfg.UserStore,
		cfg.StripeSecretKey,
		cfg.StripeWebhookSecret,
		cfg.StripeProPriceID,
		cfg.StripeBusinessPriceID,
		cfg.BaseURL,
	)

	// ── Routes ────────────────────────────────────────────────────────────────

	// Health and metrics (no version prefix — infrastructure consumers expect
	// these at a stable path).
	r.Get("/health", healthHandler(cfg.Store, cfg.Broker))
	r.Get("/metrics", prometheusHandler(cfg.Store, cfg.Hub))

	// WebSocket upgrade endpoint.
	r.Get("/ws", wsHandler.ServeWS)

	// Auth — public endpoints with strict rate limiting.
	r.With(authRateLimiter.Middleware).Post("/auth/register", authHandler.Register)
	r.With(authRateLimiter.Middleware).Post("/auth/login", authHandler.Login)
	r.With(authRateLimiter.Middleware).Post("/auth/forgot-password", authHandler.ForgotPassword)
	r.With(authRateLimiter.Middleware).Post("/auth/reset-password", authHandler.ResetPassword)

	// Stripe webhook — uses its own signature-based auth.
	r.Post("/webhooks/stripe", billingHandler.StripeWebhook)

	// Portal — JWT-protected user account routes.
	r.Route("/portal", func(r chi.Router) {
		r.Use(appMiddleware.JWTAuth(cfg.JWTSecret))
		r.Get("/usage", authHandler.GetUsage)
		r.Post("/checkout", billingHandler.CreateCheckout)
		r.Post("/customer-portal", billingHandler.CustomerPortal)
		r.Post("/regenerate-key", authHandler.RegenerateKey)
	})

	// Versioned REST API — optionally gated by API key.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appMiddleware.APIKeyAuth(cfg.APIKey, cfg.AdminKey, cfg.Store, cfg.JWTSecret, cfg.UserStore))
		// Jobs — enqueue is metered, reads are free
		r.With(appMiddleware.UsageLimitMiddleware(cfg.Store)).Post("/jobs", jobHandler.EnqueueJob)
		r.With(appMiddleware.UsageLimitMiddleware(cfg.Store)).Post("/jobs/batch", jobHandler.EnqueueJobBatch)
		r.Get("/jobs", jobHandler.ListJobs)
		r.Get("/jobs/cursor", jobHandler.ListJobsCursor)
		r.Delete("/jobs", jobHandler.PurgeJobs)
		r.Get("/jobs/{id}", jobHandler.GetJob)
		r.Get("/jobs/{id}/result", jobHandler.GetJobResult)
		r.Delete("/jobs/{id}", jobHandler.CancelJob)
		r.Post("/jobs/{id}/retry", jobHandler.RetryJob)

		// Dead-letter queue
		r.Get("/dlq", jobHandler.ListDLQ)
		r.Post("/dlq/{id}/requeue", jobHandler.RequeueDLQJob)

		// Workers
		r.Get("/workers", workerHandler.ListWorkers)

		// Stats
		r.Get("/stats", jobHandler.GetStats)

		// Webhooks
		r.Get("/webhooks", webhookHandler.ListWebhooks)
		r.Post("/webhooks", webhookHandler.CreateWebhook)
		r.Delete("/webhooks/{id}", webhookHandler.DeleteWebhook)

		// Cron schedules
		r.Get("/cron", cronHandler.ListCronSchedules)
		r.Post("/cron", cronHandler.CreateCronSchedule)
		r.Patch("/cron/{id}", cronHandler.PatchCronSchedule)
		r.Delete("/cron/{id}", cronHandler.DeleteCronSchedule)

		// API key management
		r.Get("/keys", apiKeyHandler.ListAPIKeys)
		r.Post("/keys", apiKeyHandler.CreateAPIKey)
		r.Delete("/keys/{id}", apiKeyHandler.DeleteAPIKey)
		r.Get("/usage", apiKeyHandler.GetUsage)
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

