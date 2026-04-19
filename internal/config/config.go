// Package config loads and validates all application configuration from environment
// variables at startup. A single Config struct is passed by value through the
// dependency tree; no global state is used.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the root configuration struct. It is populated once at startup
// from environment variables and then passed to every subsystem.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Worker   WorkerConfig
	Retry    RetryConfig
}

// ServerConfig holds HTTP server tuning parameters.
type ServerConfig struct {
	// Host is the interface the HTTP server binds to (e.g. "0.0.0.0").
	Host string
	// Port is the TCP port the HTTP server listens on.
	Port int
	// ReadTimeout caps the time allowed to read an entire request.
	ReadTimeout time.Duration
	// WriteTimeout caps the time allowed to write a response.
	WriteTimeout time.Duration
	// IdleTimeout caps keep-alive idle time.
	IdleTimeout time.Duration
	// RateLimit is the maximum number of requests per second per IP.
	RateLimit int
	// RateBurst is the burst size for the token bucket rate limiter.
	RateBurst int
	// APIKey is the shared secret for API authentication. When non-empty,
	// all /api/v1/* requests must include it as X-API-Key header or
	// ?api_key= query param. If empty, authentication is disabled.
	APIKey string
	// AdminKey is a separate secret that bypasses per-key scoping so operators
	// can inspect all jobs globally. Set ADMIN_KEY env var to enable.
	AdminKey string
	// JWTSecret signs and verifies user session tokens.
	JWTSecret string
	// BaseURL is the public-facing URL used for Stripe redirect URLs.
	BaseURL string
	// StripeSecretKey is the Stripe API secret key.
	StripeSecretKey string
	// StripeWebhookSecret is the signing secret for Stripe webhook events.
	StripeWebhookSecret string
	// StripeProPriceID is the Stripe Price ID for the Pro tier.
	StripeProPriceID string
	// StripeBusinessPriceID is the Stripe Price ID for the Business tier.
	StripeBusinessPriceID string
}

// DatabaseConfig holds PostgreSQL connection parameters.
type DatabaseConfig struct {
	// DSN is the full PostgreSQL connection string.
	DSN string
	// MaxConns is the maximum number of connections in the pool.
	MaxConns int
	// MinConns is the minimum number of idle connections in the pool.
	MinConns int
	// MaxConnLifetime is the maximum connection age before it is closed.
	MaxConnLifetime time.Duration
	// MaxConnIdleTime is the maximum idle time before a connection is closed.
	MaxConnIdleTime time.Duration
}

// RedisConfig holds Redis connection parameters.
type RedisConfig struct {
	// Addr is the Redis server address (host:port).
	Addr string
	// Password is the Redis AUTH password (empty string disables auth).
	Password string
	// DB is the Redis logical database index.
	DB int
	// TLS enables TLS for the Redis connection (required by Upstash).
	TLS bool
	// DialTimeout is the timeout for establishing a Redis connection.
	DialTimeout time.Duration
	// ReadTimeout is the timeout for reading a Redis response.
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for writing a Redis command.
	WriteTimeout time.Duration
}

// WorkerConfig controls the worker pool behaviour.
type WorkerConfig struct {
	// Count is the number of concurrent worker goroutines.
	Count int
	// HeartbeatInterval is how often each worker updates its last_seen timestamp.
	HeartbeatInterval time.Duration
	// PollInterval is how often workers poll Redis when the queue is empty.
	PollInterval time.Duration
	// ShutdownTimeout is the maximum time to wait for workers to drain on shutdown.
	ShutdownTimeout time.Duration
}

// RetryConfig controls the exponential-backoff retry behaviour.
type RetryConfig struct {
	// BaseDelay is the initial delay applied on the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the computed exponential delay.
	MaxDelay time.Duration
	// DefaultMaxAttempts is used when a job does not specify its own limit.
	DefaultMaxAttempts int
}

// Load reads all configuration from environment variables, applies defaults for
// optional fields, and returns an error if any required variable is missing or
// cannot be parsed.
func Load() (Config, error) {
	var cfg Config
	var err error

	// --- Server ---
	cfg.Server.Host = envString("SERVER_HOST", "0.0.0.0")
	cfg.Server.APIKey = os.Getenv("API_KEY")
	cfg.Server.AdminKey = os.Getenv("ADMIN_KEY")
	cfg.Server.JWTSecret = envString("JWT_SECRET", "change-me-in-production")
	cfg.Server.BaseURL = envString("BASE_URL", "http://localhost:8080")
	cfg.Server.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	cfg.Server.StripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	cfg.Server.StripeProPriceID = os.Getenv("STRIPE_PRO_PRICE_ID")
	cfg.Server.StripeBusinessPriceID = os.Getenv("STRIPE_BUSINESS_PRICE_ID")
	// Render (and many PaaS) inject PORT; fall back to SERVER_PORT then 8080.
	if os.Getenv("SERVER_PORT") == "" && os.Getenv("PORT") != "" {
		os.Setenv("SERVER_PORT", os.Getenv("PORT"))
	}
	if cfg.Server.Port, err = envInt("SERVER_PORT", 8080); err != nil {
		return cfg, fmt.Errorf("SERVER_PORT: %w", err)
	}
	if cfg.Server.ReadTimeout, err = envDuration("SERVER_READ_TIMEOUT", 15*time.Second); err != nil {
		return cfg, fmt.Errorf("SERVER_READ_TIMEOUT: %w", err)
	}
	if cfg.Server.WriteTimeout, err = envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second); err != nil {
		return cfg, fmt.Errorf("SERVER_WRITE_TIMEOUT: %w", err)
	}
	if cfg.Server.IdleTimeout, err = envDuration("SERVER_IDLE_TIMEOUT", 120*time.Second); err != nil {
		return cfg, fmt.Errorf("SERVER_IDLE_TIMEOUT: %w", err)
	}
	if cfg.Server.RateLimit, err = envInt("RATE_LIMIT_RPS", 100); err != nil {
		return cfg, fmt.Errorf("RATE_LIMIT_RPS: %w", err)
	}
	if cfg.Server.RateBurst, err = envInt("RATE_LIMIT_BURST", 20); err != nil {
		return cfg, fmt.Errorf("RATE_LIMIT_BURST: %w", err)
	}

	// --- Database ---
	cfg.Database.DSN = os.Getenv("DATABASE_DSN")
	if cfg.Database.DSN == "" {
		return cfg, fmt.Errorf("DATABASE_DSN is required")
	}
	if cfg.Database.MaxConns, err = envInt("DB_MAX_CONNS", 20); err != nil {
		return cfg, fmt.Errorf("DB_MAX_CONNS: %w", err)
	}
	if cfg.Database.MinConns, err = envInt("DB_MIN_CONNS", 2); err != nil {
		return cfg, fmt.Errorf("DB_MIN_CONNS: %w", err)
	}
	if cfg.Database.MaxConnLifetime, err = envDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute); err != nil {
		return cfg, fmt.Errorf("DB_MAX_CONN_LIFETIME: %w", err)
	}
	if cfg.Database.MaxConnIdleTime, err = envDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute); err != nil {
		return cfg, fmt.Errorf("DB_MAX_CONN_IDLE_TIME: %w", err)
	}

	// --- Redis ---
	cfg.Redis.Addr = envString("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Redis.TLS = os.Getenv("REDIS_TLS") == "true"
	if cfg.Redis.DB, err = envInt("REDIS_DB", 0); err != nil {
		return cfg, fmt.Errorf("REDIS_DB: %w", err)
	}
	if cfg.Redis.DialTimeout, err = envDuration("REDIS_DIAL_TIMEOUT", 5*time.Second); err != nil {
		return cfg, fmt.Errorf("REDIS_DIAL_TIMEOUT: %w", err)
	}
	if cfg.Redis.ReadTimeout, err = envDuration("REDIS_READ_TIMEOUT", 3*time.Second); err != nil {
		return cfg, fmt.Errorf("REDIS_READ_TIMEOUT: %w", err)
	}
	if cfg.Redis.WriteTimeout, err = envDuration("REDIS_WRITE_TIMEOUT", 3*time.Second); err != nil {
		return cfg, fmt.Errorf("REDIS_WRITE_TIMEOUT: %w", err)
	}

	// --- Worker ---
	if cfg.Worker.Count, err = envInt("WORKER_COUNT", 5); err != nil {
		return cfg, fmt.Errorf("WORKER_COUNT: %w", err)
	}
	if cfg.Worker.HeartbeatInterval, err = envDuration("WORKER_HEARTBEAT_INTERVAL", 10*time.Second); err != nil {
		return cfg, fmt.Errorf("WORKER_HEARTBEAT_INTERVAL: %w", err)
	}
	if cfg.Worker.PollInterval, err = envDuration("WORKER_POLL_INTERVAL", 1*time.Second); err != nil {
		return cfg, fmt.Errorf("WORKER_POLL_INTERVAL: %w", err)
	}
	if cfg.Worker.ShutdownTimeout, err = envDuration("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second); err != nil {
		return cfg, fmt.Errorf("WORKER_SHUTDOWN_TIMEOUT: %w", err)
	}

	// --- Retry ---
	if cfg.Retry.BaseDelay, err = envDuration("RETRY_BASE_DELAY", 5*time.Second); err != nil {
		return cfg, fmt.Errorf("RETRY_BASE_DELAY: %w", err)
	}
	if cfg.Retry.MaxDelay, err = envDuration("RETRY_MAX_DELAY", 1*time.Hour); err != nil {
		return cfg, fmt.Errorf("RETRY_MAX_DELAY: %w", err)
	}
	if cfg.Retry.DefaultMaxAttempts, err = envInt("RETRY_DEFAULT_MAX_ATTEMPTS", 5); err != nil {
		return cfg, fmt.Errorf("RETRY_DEFAULT_MAX_ATTEMPTS: %w", err)
	}

	return cfg, nil
}

// Addr returns the full host:port listen address for the HTTP server.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// envString returns the value of the named environment variable, or defaultVal
// if the variable is not set.
func envString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// envInt parses the named environment variable as a base-10 integer.
// If the variable is not set, defaultVal is returned.
func envInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", v, err)
	}
	return n, nil
}

// envDuration parses the named environment variable as a time.Duration string
// (e.g. "30s", "5m"). If the variable is not set, defaultVal is returned.
func envDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", v, err)
	}
	return d, nil
}
