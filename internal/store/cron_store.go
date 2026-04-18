package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/queue"
)

// CreateCronSchedule inserts a new cron schedule and returns the persisted row.
func (db *DB) CreateCronSchedule(ctx context.Context, s *queue.CronSchedule) (*queue.CronSchedule, error) {
	row := db.pool.QueryRow(ctx, queryInsertCron,
		s.Name, s.JobType, s.Payload, s.QueueName,
		s.Priority, s.MaxAttempts, s.CronExpression, s.Enabled, s.NextRunAt,
	)
	return scanCron(row)
}

// ListCronSchedules returns all cron schedules ordered by creation time.
func (db *DB) ListCronSchedules(ctx context.Context) ([]*queue.CronSchedule, error) {
	return queryCrons(ctx, db, queryListCron)
}

// ListDueCronSchedules returns enabled schedules whose next_run_at <= now.
func (db *DB) ListDueCronSchedules(ctx context.Context, now time.Time) ([]*queue.CronSchedule, error) {
	rows, err := db.pool.Query(ctx, queryListDueCron, now)
	if err != nil {
		return nil, fmt.Errorf("list due crons: %w", err)
	}
	defer rows.Close()
	var out []*queue.CronSchedule
	for rows.Next() {
		s, err := scanCron(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateCronRun sets last_run_at and next_run_at after a successful dispatch.
func (db *DB) UpdateCronRun(ctx context.Context, id uuid.UUID, lastRun, nextRun time.Time) error {
	_, err := db.pool.Exec(ctx, queryUpdateCronRun, lastRun, nextRun, id)
	return err
}

// PatchCronSchedule updates only non-nil fields on a cron schedule.
func (db *DB) PatchCronSchedule(ctx context.Context, id uuid.UUID, enabled *bool, cronExpr *string, payload []byte, nextRunAt *time.Time) (*queue.CronSchedule, error) {
	row := db.pool.QueryRow(ctx, queryPatchCron, enabled, cronExpr, payload, nextRunAt, id)
	s, err := scanCron(row)
	if err != nil {
		return nil, fmt.Errorf("patch cron %s: %w", id, err)
	}
	return s, nil
}

// DeleteCronSchedule removes a schedule by ID.
func (db *DB) DeleteCronSchedule(ctx context.Context, id uuid.UUID) error {
	tag, err := db.pool.Exec(ctx, queryDeleteCron, id)
	if err != nil {
		return fmt.Errorf("delete cron %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func queryCrons(ctx context.Context, db *DB, q string) ([]*queue.CronSchedule, error) {
	rows, err := db.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query crons: %w", err)
	}
	defer rows.Close()
	var out []*queue.CronSchedule
	for rows.Next() {
		s, err := scanCron(rows)
		if err != nil {
			return nil, fmt.Errorf("scan cron: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

type cronScanner interface {
	Scan(dest ...any) error
}

func scanCron(row cronScanner) (*queue.CronSchedule, error) {
	s := &queue.CronSchedule{}
	var payload []byte
	err := row.Scan(
		&s.ID, &s.Name, &s.JobType, &payload, &s.QueueName,
		&s.Priority, &s.MaxAttempts, &s.CronExpression,
		&s.Enabled, &s.LastRunAt, &s.NextRunAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		s.Payload = json.RawMessage(payload)
	}
	return s, nil
}
