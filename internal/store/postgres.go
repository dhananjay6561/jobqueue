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

// RunMigrations executes all migration files in order, skipping any that have
// already been applied. Applied migrations are tracked in schema_migrations.
func (db *DB) RunMigrations(ctx context.Context, migrationDir string) error {
	// Ensure tracking table exists before anything else.
	_, err := db.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Bootstrap: if schema_migrations is empty but the jobs table already exists,
	// the DB was set up before migration tracking was introduced. Mark all
	// migration files as applied without re-running them so we don't crash on
	// non-idempotent statements (e.g. CREATE TYPE without IF NOT EXISTS).
	var tracked int
	_ = db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&tracked)
	if tracked == 0 {
		var jobsExists bool
		_ = db.pool.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM information_schema.tables WHERE table_name='jobs' AND table_schema='public'
		)`).Scan(&jobsExists)
		if jobsExists {
			db.logger.Info().Msg("bootstrapping schema_migrations — marking pre-existing migrations as applied")
			entries, _ := os.ReadDir(migrationDir)
			for _, e := range entries {
				if !e.IsDir() {
					_, _ = db.pool.Exec(ctx,
						`INSERT INTO schema_migrations (name) VALUES ($1) ON CONFLICT DO NOTHING`, e.Name())
				}
			}
		}
	}

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

		// Skip already-applied migrations.
		var applied bool
		_ = db.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&applied)
		if applied {
			db.logger.Debug().Str("migration", name).Msg("skipping already-applied migration")
			continue
		}

		sql, err := os.ReadFile(path) // #nosec G304 -- path is validated against absDir above
		if err != nil {
			return fmt.Errorf("read migration %q: %w", path, err)
		}

		db.logger.Info().Str("migration", name).Msg("running migration")

		if _, err := db.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("execute migration %q: %w", path, err)
		}

		if _, err := db.pool.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record migration %q: %w", path, err)
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
