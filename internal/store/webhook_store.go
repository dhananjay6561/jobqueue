package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/queue"
)

// CreateWebhook inserts a new webhook and returns the persisted row.
func (db *DB) CreateWebhook(ctx context.Context, url, secret string, events []string, enabled bool) (*queue.Webhook, error) {
	row := db.pool.QueryRow(ctx, queryInsertWebhook, url, secret, events, enabled)
	return scanWebhook(row)
}

// ListWebhooks returns all registered webhooks.
func (db *DB) ListWebhooks(ctx context.Context) ([]*queue.Webhook, error) {
	return queryWebhooks(ctx, db, queryListWebhooks)
}

// ListEnabledWebhooks returns only enabled webhooks — used by the dispatcher.
func (db *DB) ListEnabledWebhooks(ctx context.Context) ([]*queue.Webhook, error) {
	return queryWebhooks(ctx, db, queryListEnabledWebhooks)
}

// DeleteWebhook removes a webhook by ID.
func (db *DB) DeleteWebhook(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx, queryDeleteWebhook, id)
	if err != nil {
		return fmt.Errorf("delete webhook %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func queryWebhooks(ctx context.Context, db *DB, q string) ([]*queue.Webhook, error) {
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var hooks []*queue.Webhook
	for rows.Next() {
		h, err := scanWebhook(rows)
		if err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

type webhookScanner interface {
	Scan(dest ...any) error
}

func scanWebhook(row webhookScanner) (*queue.Webhook, error) {
	h := &queue.Webhook{}
	err := row.Scan(&h.ID, &h.URL, &h.Secret, &h.Events, &h.Enabled, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return h, nil
}
