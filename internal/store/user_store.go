package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dj/jobqueue/internal/queue"
)

// PasswordResetToken is a one-time token for resetting a user's password.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt string
	Used      bool
}

// UserStorer defines persistence operations for the user/auth layer.
type UserStorer interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*queue.User, error)
	GetUserByEmail(ctx context.Context, email string) (*queue.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*queue.User, error)
	UpdateStripeCustomerID(ctx context.Context, userID uuid.UUID, customerID string) error
	CreateAPIKeyForUser(ctx context.Context, name string, tier queue.APIKeyTier, userID uuid.UUID) (*queue.APIKey, string, error)
	GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*queue.APIKey, error)
	UpdateAPIKeyTierBySubscription(ctx context.Context, subscriptionID string, tier queue.APIKeyTier) error
	SetAPIKeyStripeSubscription(ctx context.Context, keyID uuid.UUID, subscriptionID string) error
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error
	CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, tokenHash string) error
	RegenerateAPIKey(ctx context.Context, userID uuid.UUID) (*queue.APIKey, string, error)
}

var _ UserStorer = (*DB)(nil)

func (db *DB) CreateUser(ctx context.Context, email, passwordHash string) (*queue.User, error) {
	row := db.pool.QueryRow(ctx, queryInsertUser, email, passwordHash)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*queue.User, error) {
	row := db.pool.QueryRow(ctx, queryGetUserByEmail, email)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*queue.User, error) {
	row := db.pool.QueryRow(ctx, queryGetUserByID, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (db *DB) UpdateStripeCustomerID(ctx context.Context, userID uuid.UUID, customerID string) error {
	_, err := db.pool.Exec(ctx, queryUpdateStripeCustomerID, customerID, userID)
	return err
}

func (db *DB) CreateAPIKeyForUser(ctx context.Context, name string, tier queue.APIKeyTier, userID uuid.UUID) (*queue.APIKey, string, error) {
	raw := generateAPIKey()
	hash := hashKey(raw)
	prefix := raw[:8]
	limit := queue.TierLimits[tier]
	row := db.pool.QueryRow(ctx, queryInsertAPIKeyForUser, name, hash, prefix, string(tier), limit, userID)
	key, err := scanAPIKey(row)
	if err != nil {
		return nil, "", fmt.Errorf("create api key for user: %w", err)
	}
	return key, raw, nil
}

func (db *DB) GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error) {
	rows, err := db.pool.Query(ctx, queryGetAPIKeysByUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("get api keys by user: %w", err)
	}
	defer rows.Close()
	var keys []*queue.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (db *DB) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*queue.APIKey, error) {
	row := db.pool.QueryRow(ctx, queryGetAPIKeyByID, id)
	k, err := scanAPIKey(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key by id: %w", err)
	}
	return k, nil
}

func (db *DB) UpdateAPIKeyTierBySubscription(ctx context.Context, subscriptionID string, tier queue.APIKeyTier) error {
	limit := queue.TierLimits[tier]
	_, err := db.pool.Exec(ctx, queryUpdateAPIKeyTierBySubscription, string(tier), limit, subscriptionID)
	return err
}

func (db *DB) SetAPIKeyStripeSubscription(ctx context.Context, keyID uuid.UUID, subscriptionID string) error {
	_, err := db.pool.Exec(ctx, querySetAPIKeyStripeSubscription, subscriptionID, keyID)
	return err
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(row userScanner) (*queue.User, error) {
	u := &queue.User{}
	var stripeCustomerID *string
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &stripeCustomerID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if stripeCustomerID != nil {
		u.StripeCustomerID = *stripeCustomerID
	}
	return u, nil
}

func (db *DB) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := db.pool.Exec(ctx, queryUpdatePasswordHash, hash, userID)
	return err
}

func (db *DB) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	_, err := db.pool.Exec(ctx, queryInsertResetToken, userID, tokenHash)
	return err
}

func (db *DB) GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	t := &PasswordResetToken{}
	err := db.pool.QueryRow(ctx, queryGetResetToken, tokenHash).Scan(
		&t.ID, &t.UserID, &t.ExpiresAt, &t.Used,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (db *DB) MarkResetTokenUsed(ctx context.Context, tokenHash string) error {
	_, err := db.pool.Exec(ctx, queryMarkResetTokenUsed, tokenHash)
	return err
}

func (db *DB) RegenerateAPIKey(ctx context.Context, userID uuid.UUID) (*queue.APIKey, string, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, queryDeleteUserAPIKeys, userID); err != nil {
		return nil, "", fmt.Errorf("delete old keys: %w", err)
	}

	raw := generateAPIKey()
	hash := hashKey(raw)
	prefix := raw[:8]
	limit := queue.TierLimits[queue.TierFree]

	row := tx.QueryRow(ctx, queryInsertAPIKeyForUser, "default", hash, prefix, string(queue.TierFree), limit, userID)
	key, err := scanAPIKey(row)
	if err != nil {
		return nil, "", fmt.Errorf("insert key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}
	return key, raw, nil
}
