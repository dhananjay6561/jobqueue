// Package store implements the PostgreSQL-backed persistence layer.
// All database interactions use raw SQL via pgx/v5 — no ORM is used.
// Every exported function accepts a context.Context as its first parameter
// so callers can propagate deadlines and cancellation signals.
package store

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/dj/jobqueue/internal/config"
)

// DB wraps a pgxpool.Pool and exposes the application-level store operations.
// All methods are safe for concurrent use by multiple goroutines.
type DB struct {
	pool   *pgxpool.Pool
	logger zerolog.Logger
}

// Compile-time proof that *DB satisfies both store interfaces used by handlers.
var _ JobStorer = (*DB)(nil)

// New creates and validates a connection pool using the supplied config.
// It runs a connectivity check before returning so callers know the database
// is reachable at startup.
func New(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}

	poolCfg.MaxConns = clampInt32(cfg.MaxConns)
	poolCfg.MinConns = clampInt32(cfg.MinConns)
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	db := &DB{
		pool:   pool,
		logger: zerolog.New(os.Stdout).With().Str("component", "store").Logger(),
	}

	if err := db.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initial database ping: %w", err)
	}

	return db, nil
}

// Ping sends a lightweight query to verify the database is reachable.
func (db *DB) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	return nil
}

// Close gracefully terminates all pool connections. It should be called once
// during application shutdown after all in-flight operations have completed.
func (db *DB) Close() {
	db.pool.Close()
}

// RunMigrations executes all migration files in order. In production this is
// typically handled by a migration tool (goose, migrate), but for local
// development and testing it is convenient to run them inline at startup.
func (db *DB) RunMigrations(ctx context.Context, migrationDir string) error {
	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf("read migration directory %q: %w", migrationDir, err)
	}

	absDir, err := filepath.Abs(migrationDir)
	if err != nil {
		return fmt.Errorf("resolve migration directory %q: %w", migrationDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		path := filepath.Join(absDir, name)

		// Ensure the resolved path stays within the migration directory.
		if !strings.HasPrefix(path, absDir+string(filepath.Separator)) {
			return fmt.Errorf("migration file %q escapes migration directory", name)
		}

		sql, err := os.ReadFile(path) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read migration %q: %w", path, err)
		}

		db.logger.Info().Str("migration", name).Msg("running migration")

		if _, err := db.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("execute migration %q: %w", path, err)
		}
	}

	return nil
}

// Pool exposes the underlying pgxpool.Pool for advanced use-cases such as
// transactions initiated at a higher level.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// clampInt32 converts n to int32, clamping to [0, math.MaxInt32] to prevent overflow.
func clampInt32(n int) int32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n) //nolint:gosec
}
