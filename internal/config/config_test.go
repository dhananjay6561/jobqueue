// Package config — config_test.go validates the configuration loading logic,
// including defaults, required fields, and type parsing.
package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setEnv sets the given env vars and returns a cleanup function that restores
// their previous values (or unsets them if they were not set before).
func setEnv(t *testing.T, pairs ...string) {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatal("setEnv requires an even number of arguments (key, value pairs)")
	}

	for i := 0; i < len(pairs); i += 2 {
		key, val := pairs[i], pairs[i+1]
		prev, hadPrev := os.LookupEnv(key)

		if err := os.Setenv(key, val); err != nil {
			t.Fatalf("setenv %s: %v", key, err)
		}

		t.Cleanup(func() {
			if hadPrev {
				_ = os.Setenv(key, prev)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestLoad_MissingDatabaseDSN(t *testing.T) {
	_ = os.Unsetenv("DATABASE_DSN")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_DSN")
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, "DATABASE_DSN", "postgres://test:test@localhost/test")
	// Unset optional vars to exercise defaults.
	optional := []string{
		"SERVER_HOST", "SERVER_PORT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT",
		"SERVER_IDLE_TIMEOUT", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST",
		"DB_MAX_CONNS", "DB_MIN_CONNS", "DB_MAX_CONN_LIFETIME", "DB_MAX_CONN_IDLE_TIME",
		"REDIS_ADDR", "REDIS_DB", "REDIS_DIAL_TIMEOUT", "REDIS_READ_TIMEOUT", "REDIS_WRITE_TIMEOUT",
		"WORKER_COUNT", "WORKER_HEARTBEAT_INTERVAL", "WORKER_POLL_INTERVAL", "WORKER_SHUTDOWN_TIMEOUT",
		"RETRY_BASE_DELAY", "RETRY_MAX_DELAY", "RETRY_DEFAULT_MAX_ATTEMPTS",
	}
	for _, key := range optional {
		_ = os.Unsetenv(key)
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 15*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 100, cfg.Server.RateLimit)
	assert.Equal(t, 20, cfg.Server.RateBurst)

	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, 0, cfg.Redis.DB)

	assert.Equal(t, 5, cfg.Worker.Count)
	assert.Equal(t, 10*time.Second, cfg.Worker.HeartbeatInterval)
	assert.Equal(t, 1*time.Second, cfg.Worker.PollInterval)
	assert.Equal(t, 30*time.Second, cfg.Worker.ShutdownTimeout)

	assert.Equal(t, 5*time.Second, cfg.Retry.BaseDelay)
	assert.Equal(t, 1*time.Hour, cfg.Retry.MaxDelay)
	assert.Equal(t, 5, cfg.Retry.DefaultMaxAttempts)
}

func TestLoad_CustomValues(t *testing.T) {
	setEnv(t,
		"DATABASE_DSN", "postgres://prod:pass@db/mydb?sslmode=require",
		"SERVER_PORT", "9090",
		"SERVER_READ_TIMEOUT", "30s",
		"WORKER_COUNT", "10",
		"RETRY_BASE_DELAY", "2s",
		"RETRY_MAX_DELAY", "30m",
		"RETRY_DEFAULT_MAX_ATTEMPTS", "3",
	)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 10, cfg.Worker.Count)
	assert.Equal(t, 2*time.Second, cfg.Retry.BaseDelay)
	assert.Equal(t, 30*time.Minute, cfg.Retry.MaxDelay)
	assert.Equal(t, 3, cfg.Retry.DefaultMaxAttempts)
}

func TestLoad_InvalidPort(t *testing.T) {
	setEnv(t, "DATABASE_DSN", "postgres://test/test", "SERVER_PORT", "not-a-number")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_PORT")
}

func TestLoad_InvalidDuration(t *testing.T) {
	setEnv(t, "DATABASE_DSN", "postgres://test/test", "SERVER_READ_TIMEOUT", "not-a-duration")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SERVER_READ_TIMEOUT")
}

func TestServerConfig_Addr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  ServerConfig
		want string
	}{
		{
			name: "default",
			cfg:  ServerConfig{Host: "0.0.0.0", Port: 8080},
			want: "0.0.0.0:8080",
		},
		{
			name: "localhost high port",
			cfg:  ServerConfig{Host: "127.0.0.1", Port: 9999},
			want: "127.0.0.1:9999",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cfg.Addr())
		})
	}
}
