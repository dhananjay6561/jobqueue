// Package store — job_store.go implements all CRUD operations for jobs, the
// dead-letter queue, and worker registry. The JobStorer interface is declared
// here so consumers can depend on the abstraction and tests can inject a mock.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/dj/jobqueue/internal/queue"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when an operation cannot be applied to the current
// resource state (e.g. cancelling a job that is already running).
var ErrConflict = errors.New("conflict")

// JobFilter defines the optional filters for the ListJobs query.
type JobFilter struct {
	Status    string
	Type      string
	QueueName string
	Limit     int
	Offset    int
}

// DLQFilter defines the optional filters for the ListDLQ query.
type DLQFilter struct {
	IncludeRequeued bool
	Limit           int
	Offset          int
}

// Page wraps a result slice with pagination metadata.
type Page[T any] struct {
	Items      []T
	TotalCount int64
	Limit      int
	Offset     int
}

// JobStats holds aggregated queue metrics returned by the stats endpoint and
// broadcast over WebSocket. Field names use snake_case json tags to match the
// React frontend's QueueStats interface exactly.
type JobStats struct {
	TotalJobs     int64   `json:"total_jobs"`
	Pending       int64   `json:"pending"`
	Running       int64   `json:"running"`
	Completed     int64   `json:"completed"`
	Failed        int64   `json:"failed"`
	Dead          int64   `json:"dead"`
	Cancelled     int64   `json:"cancelled"`
	ActiveWorkers int64   `json:"active_workers"`
	JobsPerMinute int64   `json:"jobs_per_minute"`
	FailedRate    float64 `json:"failed_rate"`
	QueueDepth    int64   `json:"queue_depth"`
	DLQCount      int64   `json:"dlq_count"`
}

// JobStorer defines all persistence operations needed by the API and worker pool.
// Using an interface enables mock injection in unit tests.
type JobStorer interface {
	// Job lifecycle
	CreateJob(ctx context.Context, job *queue.Job) (*queue.Job, error)
	GetJob(ctx context.Context, id uuid.UUID) (*queue.Job, error)
	ListJobs(ctx context.Context, filter JobFilter) (Page[*queue.Job], error)
	MarkJobStarted(ctx context.Context, id uuid.UUID, workerID string) (*queue.Job, error)
	MarkJobCompleted(ctx context.Context, id uuid.UUID, result json.RawMessage) (*queue.Job, error)
	GetJobResult(ctx context.Context, id uuid.UUID) (json.RawMessage, error)
	MarkJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (*queue.Job, error)
	MarkJobDead(ctx context.Context, id uuid.UUID, errMsg string) error
	CancelJob(ctx context.Context, id uuid.UUID) error
	ResetJobForRetry(ctx context.Context, id uuid.UUID) (*queue.Job, error)

	// Dead-letter queue
	InsertDLQ(ctx context.Context, job *queue.Job) error
	ListDLQ(ctx context.Context, filter DLQFilter) (Page[*queue.DLQEntry], error)
	GetDLQEntry(ctx context.Context, id uuid.UUID) (*queue.DLQEntry, error)
	MarkDLQRequeued(ctx context.Context, dlqID, newJobID uuid.UUID) error

	// Worker registry
	UpsertWorker(ctx context.Context, id string) error
	UpdateWorkerHeartbeat(ctx context.Context, workerID string, currentJobID *uuid.UUID) error
	UpdateWorkerStats(ctx context.Context, workerID string, processed, failed int64) error
	MarkWorkerStopped(ctx context.Context, workerID string) error
	ListWorkers(ctx context.Context, activeOnly bool) ([]*queue.WorkerInfo, error)

	// Statistics
	GetStats(ctx context.Context) (JobStats, error)

	// Webhooks
	CreateWebhook(ctx context.Context, url, secret string, events []string, enabled bool) (*queue.Webhook, error)
	ListWebhooks(ctx context.Context) ([]*queue.Webhook, error)
	ListEnabledWebhooks(ctx context.Context) ([]*queue.Webhook, error)
	DeleteWebhook(ctx context.Context, id uuid.UUID) error

	// API keys
	CreateAPIKey(ctx context.Context, name string, tier queue.APIKeyTier) (*queue.APIKey, string, error)
	GetAPIKeyByHash(ctx context.Context, raw string) (*queue.APIKey, error)
	ListAPIKeys(ctx context.Context) ([]*queue.APIKey, error)
	IncrementAPIKeyUsage(ctx context.Context, raw string) (*queue.APIKey, error)
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error

	// Cron schedules
	CreateCronSchedule(ctx context.Context, s *queue.CronSchedule) (*queue.CronSchedule, error)
	ListCronSchedules(ctx context.Context) ([]*queue.CronSchedule, error)
	ListDueCronSchedules(ctx context.Context, now time.Time) ([]*queue.CronSchedule, error)
	UpdateCronRun(ctx context.Context, id uuid.UUID, lastRun, nextRun time.Time) error
	DeleteCronSchedule(ctx context.Context, id uuid.UUID) error

	// Infrastructure
	Ping(ctx context.Context) error
}

// Compile-time guarantee that *DB satisfies the JobStorer interface.
var _ JobStorer = (*DB)(nil)

// ─── Job lifecycle ────────────────────────────────────────────────────────────

// CreateJob inserts a new job into the jobs table and returns the persisted row.
func (db *DB) CreateJob(ctx context.Context, job *queue.Job) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryInsertJob,
		job.ID,
		job.Type,
		job.Payload,
		job.Priority,
		job.MaxAttempts,
		job.QueueName,
		job.ScheduledAt,
	)

	result, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return result, nil
}

// GetJob fetches a single job by UUID. Returns ErrNotFound if it does not exist.
func (db *DB) GetJob(ctx context.Context, id uuid.UUID) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryGetJobByID, id)
	job, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", id, err)
	}
	return job, nil
}

// ListJobs returns a filtered, paginated list of jobs.
func (db *DB) ListJobs(ctx context.Context, filter JobFilter) (Page[*queue.Job], error) {
	// Normalise limit/offset defaults.
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Convert empty strings to nil for nullable SQL params.
	statusParam := nullableString(filter.Status)
	typeParam := nullableString(filter.Type)
	queueParam := nullableString(filter.QueueName)

	// Fetch total count for pagination metadata.
	var totalCount int64
	countRow := db.pool.QueryRow(ctx, queryCountJobs, statusParam, typeParam, queueParam)
	if err := countRow.Scan(&totalCount); err != nil {
		return Page[*queue.Job]{}, fmt.Errorf("count jobs: %w", err)
	}

	rows, err := db.pool.Query(ctx, queryListJobs,
		statusParam, typeParam, queueParam, filter.Limit, filter.Offset)
	if err != nil {
		return Page[*queue.Job]{}, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]*queue.Job, 0)
	for rows.Next() {
		job, err := scanJobFromRows(rows)
		if err != nil {
			return Page[*queue.Job]{}, fmt.Errorf("scan job row: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return Page[*queue.Job]{}, fmt.Errorf("iterate job rows: %w", err)
	}

	return Page[*queue.Job]{
		Items:      jobs,
		TotalCount: totalCount,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

// MarkJobStarted transitions a job to 'running' and records the worker_id.
func (db *DB) MarkJobStarted(ctx context.Context, id uuid.UUID, workerID string) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryUpdateJobStarted, workerID, id)
	job, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mark job started %s: %w", id, err)
	}
	return job, nil
}

// MarkJobCompleted transitions a running job to 'completed' and stores result.
// result may be nil if the handler produced no output.
func (db *DB) MarkJobCompleted(ctx context.Context, id uuid.UUID, result json.RawMessage) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryMarkJobCompleted, id, result)
	job, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mark job completed %s: %w", id, err)
	}
	return job, nil
}

// GetJobResult returns the stored result JSON for a completed job.
// Returns ErrNotFound if the job doesn't exist.
func (db *DB) GetJobResult(ctx context.Context, id uuid.UUID) (json.RawMessage, error) {
	var result json.RawMessage
	err := db.pool.QueryRow(ctx, queryGetJobResult, id).Scan(&result)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job result %s: %w", id, err)
	}
	return result, nil
}

// MarkJobFailed transitions a running job to 'failed' and stores the error message.
func (db *DB) MarkJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryMarkJobFailed, errMsg, id)
	job, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mark job failed %s: %w", id, err)
	}
	return job, nil
}

// MarkJobDead transitions a job to the 'dead' terminal state.
// The caller is responsible for also calling InsertDLQ to record the DLQ entry.
func (db *DB) MarkJobDead(ctx context.Context, id uuid.UUID, errMsg string) error {
	tag, err := db.pool.Exec(ctx, queryMarkJobDead, errMsg, id)
	if err != nil {
		return fmt.Errorf("mark job dead %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CancelJob transitions a pending job to 'failed' with message "cancelled by user".
// Returns ErrConflict if the job is not in the pending state.
func (db *DB) CancelJob(ctx context.Context, id uuid.UUID) error {
	var cancelledID uuid.UUID
	err := db.pool.QueryRow(ctx, queryCancelJob, id).Scan(&cancelledID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the job does not exist or it is not pending.
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("cancel job %s: %w", id, err)
	}
	return nil
}

// ResetJobForRetry sets a failed or dead job back to pending so it can be
// re-enqueued. Returns ErrConflict if the job is not in a retryable state.
func (db *DB) ResetJobForRetry(ctx context.Context, id uuid.UUID) (*queue.Job, error) {
	row := db.pool.QueryRow(ctx, queryResetJobForRetry, id)
	job, err := scanJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, fmt.Errorf("reset job for retry %s: %w", id, err)
	}
	return job, nil
}

// ─── Dead-Letter Queue ────────────────────────────────────────────────────────

// InsertDLQ records a dead job in the dead_letter_jobs table.
func (db *DB) InsertDLQ(ctx context.Context, job *queue.Job) error {
	_, err := db.pool.Exec(ctx, queryInsertDLQ,
		job.ID,
		job.Type,
		job.Payload,
		job.Priority,
		job.QueueName,
		job.MaxAttempts,
		job.CreatedAt,
		job.ErrorMessage,
		job.Attempts,
	)
	if err != nil {
		return fmt.Errorf("insert DLQ entry %s: %w", job.ID, err)
	}
	return nil
}

// ListDLQ returns a filtered, paginated list of DLQ entries.
func (db *DB) ListDLQ(ctx context.Context, filter DLQFilter) (Page[*queue.DLQEntry], error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	var totalCount int64
	if err := db.pool.QueryRow(ctx, queryCountDLQ, filter.IncludeRequeued).Scan(&totalCount); err != nil {
		return Page[*queue.DLQEntry]{}, fmt.Errorf("count DLQ entries: %w", err)
	}

	rows, err := db.pool.Query(ctx, queryListDLQ, filter.IncludeRequeued, filter.Limit, filter.Offset)
	if err != nil {
		return Page[*queue.DLQEntry]{}, fmt.Errorf("list DLQ: %w", err)
	}
	defer rows.Close()

	entries := make([]*queue.DLQEntry, 0)
	for rows.Next() {
		entry, err := scanDLQEntry(rows)
		if err != nil {
			return Page[*queue.DLQEntry]{}, fmt.Errorf("scan DLQ row: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return Page[*queue.DLQEntry]{}, fmt.Errorf("iterate DLQ rows: %w", err)
	}

	return Page[*queue.DLQEntry]{
		Items:      entries,
		TotalCount: totalCount,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

// GetDLQEntry fetches a single DLQ entry by UUID.
func (db *DB) GetDLQEntry(ctx context.Context, id uuid.UUID) (*queue.DLQEntry, error) {
	row := db.pool.QueryRow(ctx, queryGetDLQEntry, id)
	entry, err := scanDLQEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get DLQ entry %s: %w", id, err)
	}
	return entry, nil
}

// MarkDLQRequeued marks a DLQ entry as requeued with the new job's id.
func (db *DB) MarkDLQRequeued(ctx context.Context, dlqID, newJobID uuid.UUID) error {
	tag, err := db.pool.Exec(ctx, queryMarkDLQRequeued, newJobID, dlqID)
	if err != nil {
		return fmt.Errorf("mark DLQ requeued %s: %w", dlqID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ─── Worker registry ──────────────────────────────────────────────────────────

// UpsertWorker inserts or refreshes the heartbeat row for a worker.
func (db *DB) UpsertWorker(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, queryUpsertWorker, id)
	if err != nil {
		return fmt.Errorf("upsert worker %s: %w", id, err)
	}
	return nil
}

// UpdateWorkerHeartbeat updates last_seen and the current job id for a worker.
func (db *DB) UpdateWorkerHeartbeat(ctx context.Context, workerID string, currentJobID *uuid.UUID) error {
	_, err := db.pool.Exec(ctx, queryUpdateWorkerHeartbeat, currentJobID, workerID)
	if err != nil {
		return fmt.Errorf("update worker heartbeat %s: %w", workerID, err)
	}
	return nil
}

// UpdateWorkerStats increments the processed/failed counters for a worker.
func (db *DB) UpdateWorkerStats(ctx context.Context, workerID string, processed, failed int64) error {
	_, err := db.pool.Exec(ctx, queryUpdateWorkerStats, processed, failed, workerID)
	if err != nil {
		return fmt.Errorf("update worker stats %s: %w", workerID, err)
	}
	return nil
}

// MarkWorkerStopped sets the worker status to 'stopped' on graceful shutdown.
func (db *DB) MarkWorkerStopped(ctx context.Context, workerID string) error {
	_, err := db.pool.Exec(ctx, queryMarkWorkerStopped, workerID)
	if err != nil {
		return fmt.Errorf("mark worker stopped %s: %w", workerID, err)
	}
	return nil
}

// ListWorkers returns all (or only active) workers ordered by start time.
func (db *DB) ListWorkers(ctx context.Context, activeOnly bool) ([]*queue.WorkerInfo, error) {
	rows, err := db.pool.Query(ctx, queryListWorkers, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list workers: %w", err)
	}
	defer rows.Close()

	workers := make([]*queue.WorkerInfo, 0)
	for rows.Next() {
		w := &queue.WorkerInfo{}
		if err := rows.Scan(
			&w.ID,
			&w.Status,
			&w.JobsProcessed,
			&w.JobsFailed,
			&w.CurrentJobID,
			&w.StartedAt,
			&w.LastSeen,
		); err != nil {
			return nil, fmt.Errorf("scan worker row: %w", err)
		}
		workers = append(workers, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worker rows: %w", err)
	}

	return workers, nil
}

// ─── Statistics ───────────────────────────────────────────────────────────────

// GetStats returns aggregated queue metrics for the stats endpoint.
func (db *DB) GetStats(ctx context.Context) (JobStats, error) {
	var stats JobStats

	// Count per status — populate flat fields directly.
	rows, err := db.pool.Query(ctx, queryJobStats)
	if err != nil {
		return stats, fmt.Errorf("query job stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return stats, fmt.Errorf("scan job stat row: %w", err)
		}
		stats.TotalJobs += count
		switch status {
		case "pending":
			stats.Pending = count
		case "running":
			stats.Running = count
		case "completed":
			stats.Completed = count
		case "failed":
			stats.Failed = count
		case "dead":
			stats.Dead = count
		case "cancelled":
			stats.Cancelled = count
		}
	}
	if err := rows.Err(); err != nil {
		return stats, fmt.Errorf("iterate job stat rows: %w", err)
	}

	stats.QueueDepth = stats.Pending + stats.Running
	if stats.TotalJobs > 0 {
		stats.FailedRate = float64(stats.Failed+stats.Dead) / float64(stats.TotalJobs)
	}

	// Jobs completed in the last minute.
	if err := db.pool.QueryRow(ctx, queryJobsPerMinute).Scan(&stats.JobsPerMinute); err != nil {
		return stats, fmt.Errorf("query jobs per minute: %w", err)
	}

	// Total un-requeued DLQ entries.
	if err := db.pool.QueryRow(ctx, queryDLQCount).Scan(&stats.DLQCount); err != nil {
		return stats, fmt.Errorf("query DLQ count: %w", err)
	}

	// Count of currently active workers.
	if err := db.pool.QueryRow(ctx, queryActiveWorkerCount).Scan(&stats.ActiveWorkers); err != nil {
		return stats, fmt.Errorf("query active worker count: %w", err)
	}

	return stats, nil
}

// ─── Scanner helpers ──────────────────────────────────────────────────────────

// pgxScanner is satisfied by both pgx.Row and pgx.Rows, allowing a single
// scan helper to work for QueryRow and Query results.
type pgxScanner interface {
	Scan(dest ...any) error
}

// scanJob maps a database row to a queue.Job struct.
func scanJob(row pgxScanner) (*queue.Job, error) {
	job := &queue.Job{}
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Payload,
		&job.Priority,
		&job.Status,
		&job.Attempts,
		&job.MaxAttempts,
		&job.QueueName,
		&job.ScheduledAt,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
		&job.WorkerID,
		&job.ErrorMessage,
		&job.Result,
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}

// scanJobFromRows maps a pgx.Rows row to a queue.Job struct.
func scanJobFromRows(rows pgx.Rows) (*queue.Job, error) {
	return scanJob(rows)
}

// scanDLQEntry maps a database row to a queue.DLQEntry struct.
func scanDLQEntry(row pgxScanner) (*queue.DLQEntry, error) {
	entry := &queue.DLQEntry{}
	err := row.Scan(
		&entry.ID,
		&entry.Type,
		&entry.Payload,
		&entry.Priority,
		&entry.QueueName,
		&entry.MaxAttempts,
		&entry.DiedAt,
		&entry.OriginalCreatedAt,
		&entry.LastError,
		&entry.TotalAttempts,
		&entry.Requeued,
		&entry.RequeuedAt,
		&entry.NewJobID,
	)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// nullableString converts an empty string to nil for nullable SQL parameters.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullableTime converts a zero time.Time to nil for nullable SQL parameters.
func nullableTime(t time.Time) *time.Time { //nolint:unused
	if t.IsZero() {
		return nil
	}
	return &t
}
