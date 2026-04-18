package jobqueue

import "time"

// ── Job types ─────────────────────────────────────────────────────────────────

// Job represents a persisted job returned by the API.
type Job struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Payload      map[string]any `json:"payload"`
	Priority     int            `json:"priority"`
	Status       string         `json:"status"`
	Attempts     int            `json:"attempts"`
	MaxAttempts  int            `json:"max_attempts"`
	QueueName    string         `json:"queue_name"`
	ScheduledAt  time.Time      `json:"scheduled_at"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	WorkerID     *string        `json:"worker_id,omitempty"`
	ErrorMessage *string        `json:"error_message,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
}

// EnqueueRequest is the payload for submitting a new job.
type EnqueueRequest struct {
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	MaxAttempts int            `json:"max_attempts,omitempty"`
	QueueName   string         `json:"queue_name,omitempty"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
}

// ListJobsParams filters for the list-jobs endpoint.
type ListJobsParams struct {
	Status  string // pending | running | completed | failed | dead | cancelled
	Type    string
	Queue   string
	Limit   int
	Offset  int
}

// ── Stats ─────────────────────────────────────────────────────────────────────

// Stats is the response from GET /api/v1/stats.
type Stats struct {
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

// ── Webhooks ──────────────────────────────────────────────────────────────────

// Webhook is a registered delivery endpoint.
type Webhook struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateWebhookRequest is the payload for registering a new webhook.
type CreateWebhookRequest struct {
	URL     string   `json:"url"`
	Secret  string   `json:"secret,omitempty"`
	Events  []string `json:"events,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// ── DLQ ───────────────────────────────────────────────────────────────────────

// DLQEntry is an item from the dead-letter queue.
type DLQEntry struct {
	ID                string         `json:"id"`
	Type              string         `json:"type"`
	Payload           map[string]any `json:"payload"`
	Priority          int            `json:"priority"`
	QueueName         string         `json:"queue_name"`
	MaxAttempts       int            `json:"max_attempts"`
	TotalAttempts     int            `json:"total_attempts"`
	LastError         *string        `json:"last_error,omitempty"`
	DiedAt            time.Time      `json:"died_at"`
	OriginalCreatedAt time.Time      `json:"original_created_at"`
	Requeued          bool           `json:"requeued"`
}

// ── Pagination ────────────────────────────────────────────────────────────────

// Page wraps a list response with pagination metadata.
type Page[T any] struct {
	Items      []T
	TotalCount int64
	Limit      int
	Offset     int
	HasMore    bool
}

// ── Internal API envelope ─────────────────────────────────────────────────────

type apiResponse[T any] struct {
	Data  T      `json:"data"`
	Error string `json:"error"`
	Meta  struct {
		RequestID  string `json:"request_id"`
		TotalCount *int64 `json:"total_count,omitempty"`
		Limit      *int   `json:"limit,omitempty"`
		Offset     *int   `json:"offset,omitempty"`
		HasMore    *bool  `json:"has_more,omitempty"`
	} `json:"meta"`
}
