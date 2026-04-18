package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dj/jobqueue/internal/queue"
)

// CreateAPIKey hashes the raw key, persists it, and returns the stored record.
// The raw key is only returned at creation time — it is never stored.
func (db *DB) CreateAPIKey(ctx context.Context, name string, tier queue.APIKeyTier) (*queue.APIKey, string, error) {
	raw := generateAPIKey()
	hash := hashKey(raw)
	prefix := raw[:8]
	limit := queue.TierLimits[tier]

	row := db.pool.QueryRow(ctx, queryInsertAPIKey, name, hash, prefix, string(tier), limit)
	key, err := scanAPIKey(row)
	if err != nil {
		return nil, "", fmt.Errorf("create api key: %w", err)
	}
	return key, raw, nil
}

// GetAPIKeyByHash looks up a key by its SHA-256 hash.
func (db *DB) GetAPIKeyByHash(ctx context.Context, raw string) (*queue.APIKey, error) {
	hash := hashKey(raw)
	row := db.pool.QueryRow(ctx, queryGetAPIKeyByHash, hash)
	key, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return key, nil
}

// ListAPIKeys returns all API keys (hashes never exposed).
func (db *DB) ListAPIKeys(ctx context.Context) ([]*queue.APIKey, error) {
	rows, err := db.pool.Query(ctx, queryListAPIKeys)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var keys []*queue.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// IncrementAPIKeyUsage atomically increments jobs_used (resetting if month rolled over).
// Returns the updated key so callers can check LimitReached().
func (db *DB) IncrementAPIKeyUsage(ctx context.Context, raw string) (*queue.APIKey, error) {
	hash := hashKey(raw)
	row := db.pool.QueryRow(ctx, queryIncrementAPIKeyUsage, hash)
	key, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("increment api key usage: %w", err)
	}
	return key, nil
}

// DeleteAPIKey removes a key by its UUID.
func (db *DB) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx, queryDeleteAPIKey, id)
	if err != nil {
		return fmt.Errorf("delete api key %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func generateAPIKey() string {
	id := uuid.New()
	return "qly_" + hex.EncodeToString(id[:])
}

type apikeyScanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row apikeyScanner) (*queue.APIKey, error) {
	k := &queue.APIKey{}
	err := row.Scan(
		&k.ID, &k.Name, &k.KeyPrefix, &k.Tier,
		&k.JobsUsed, &k.JobsLimit, &k.ResetAt, &k.Enabled, &k.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return k, nil
}
