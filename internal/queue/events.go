// Package queue — events.go defines the event types broadcast over WebSocket.
// Every significant job lifecycle transition and worker heartbeat produces
// an Event that is published to the WebSocket hub for connected clients.
package queue

// EventType is a string tag identifying the kind of event.
type EventType string

const (
	// EventJobEnqueued fires when a new job is accepted via the API.
	EventJobEnqueued EventType = "job.enqueued"

	// EventJobStarted fires when a worker claims a job.
	EventJobStarted EventType = "job.started"

	// EventJobCompleted fires when a job handler returns nil.
	EventJobCompleted EventType = "job.completed"

	// EventJobFailed fires when a job handler returns an error (retryable).
	EventJobFailed EventType = "job.failed"

	// EventJobDead fires when a job exhausts all retries and enters the DLQ.
	EventJobDead EventType = "job.dead"

	// EventWorkerHeartbeat fires on each worker heartbeat tick.
	EventWorkerHeartbeat EventType = "worker.heartbeat"

	// EventStatsUpdate fires on the periodic stats broadcast (every 5 seconds).
	EventStatsUpdate EventType = "stats.update"
)

// Event is the payload broadcast to all connected WebSocket clients.
type Event struct {
	// Type identifies the event kind.
	Type EventType `json:"type"`

	// JobID is the string UUID of the related job (empty for worker-only events).
	JobID string `json:"job_id,omitempty"`

	// JobType is the handler type of the related job.
	JobType string `json:"job_type,omitempty"`

	// WorkerID is the identifier of the worker that triggered the event.
	WorkerID string `json:"worker_id,omitempty"`

	// Payload carries the full event data (varies by event type).
	Payload any `json:"payload,omitempty"`

	// Timestamp is the Unix millisecond timestamp of the event.
	Timestamp int64 `json:"ts"`
}
