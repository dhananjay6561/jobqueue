// Package handler — job.go implements all job-related HTTP handlers:
//   - POST   /api/v1/jobs          EnqueueJob
//   - GET    /api/v1/jobs          ListJobs
//   - GET    /api/v1/jobs/:id      GetJob
//   - DELETE /api/v1/jobs/:id      CancelJob
//   - POST   /api/v1/jobs/:id/retry RetryJob
//   - GET    /api/v1/dlq           ListDLQ
//   - POST   /api/v1/dlq/:id/requeue RequeueDLQ
//   - GET    /api/v1/stats         GetStats
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// JobHandler handles all job-related HTTP endpoints. Dependencies are injected
// via the constructor and stored as unexported fields so each method is a pure
// function of its inputs.
type JobHandler struct {
	store           store.JobStorer
	broker          queue.Broker
	publisher       queue.EventPublisher
	defaultMaxAttempts int
}

// NewJobHandler constructs a JobHandler with its required dependencies.
func NewJobHandler(
	jobStore store.JobStorer,
	broker queue.Broker,
	publisher queue.EventPublisher,
	defaultMaxAttempts int,
) *JobHandler {
	return &JobHandler{
		store:           jobStore,
		broker:          broker,
		publisher:       publisher,
		defaultMaxAttempts: defaultMaxAttempts,
	}
}

// EnqueueJob handles POST /api/v1/jobs.
//
// Request body: queue.EnqueueRequest (JSON)
// Response: 201 Created with the new job object.
func (h *JobHandler) EnqueueJob(w http.ResponseWriter, r *http.Request) {
	var req queue.EnqueueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Type == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "field 'type' is required")
		return
	}

	// Apply defaults for optional fields.
	if req.Priority < queue.PriorityMin || req.Priority > queue.PriorityMax {
		req.Priority = queue.PriorityDefault
	}
	if req.MaxAttempts <= 0 {
		req.MaxAttempts = h.defaultMaxAttempts
	}
	if req.QueueName == "" {
		req.QueueName = queue.DefaultQueueName
	}

	scheduledAt := time.Now()
	if req.ScheduledAt != nil && req.ScheduledAt.After(scheduledAt) {
		scheduledAt = *req.ScheduledAt
	}

	if len(req.Payload) == 0 {
		req.Payload = []byte("{}")
	}

	job := &queue.Job{
		ID:          uuid.New(),
		Type:        req.Type,
		Payload:     req.Payload,
		Priority:    req.Priority,
		MaxAttempts: req.MaxAttempts,
		QueueName:   req.QueueName,
		ScheduledAt: scheduledAt,
	}
	if key := middleware.APIKeyFromContext(r.Context()); key != nil {
		job.APIKeyID = &key.ID
	}

	createdJob, err := h.store.CreateJob(r.Context(), job)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to persist job: "+err.Error())
		return
	}

	// Push the job ID into the broker (Redis).
	if err := h.broker.Enqueue(r.Context(), createdJob); err != nil {
		// The job is persisted but not queued. Log and surface the error; the
		// operator can re-queue via POST /api/v1/jobs/:id/retry.
		writeError(w, r, http.StatusInternalServerError, "job persisted but broker enqueue failed: "+err.Error())
		return
	}

	// Broadcast the enqueue event and bump the Prometheus counter.
	queue.CounterJobsEnqueued.Add(1)
	if h.publisher != nil {
		h.publisher.Publish(queue.Event{
			Type:    queue.EventJobEnqueued,
			JobID:   createdJob.ID.String(),
			JobType: createdJob.Type,
			Payload: map[string]any{"job": createdJob},
		})
	}

	writeJSON(w, r, http.StatusCreated, createdJob)
}

// EnqueueJobBatch handles POST /api/v1/jobs/batch.
// Accepts an array of EnqueueRequest objects and creates all jobs atomically.
// Returns 201 with the array of created jobs, or 400/422 on validation failure.
func (h *JobHandler) EnqueueJobBatch(w http.ResponseWriter, r *http.Request) {
	var reqs []queue.EnqueueRequest
	if err := decodeJSON(r, &reqs); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(reqs) == 0 {
		writeError(w, r, http.StatusUnprocessableEntity, "batch must contain at least one job")
		return
	}
	if len(reqs) > 500 {
		writeError(w, r, http.StatusUnprocessableEntity, "batch size exceeds maximum of 500")
		return
	}

	key := middleware.APIKeyFromContext(r.Context())
	now := time.Now()

	jobs := make([]*queue.Job, 0, len(reqs))
	for i, req := range reqs {
		if req.Type == "" {
			writeError(w, r, http.StatusUnprocessableEntity, fmt.Sprintf("job[%d]: field 'type' is required", i))
			return
		}
		if req.Priority < queue.PriorityMin || req.Priority > queue.PriorityMax {
			req.Priority = queue.PriorityDefault
		}
		if req.MaxAttempts <= 0 {
			req.MaxAttempts = h.defaultMaxAttempts
		}
		if req.QueueName == "" {
			req.QueueName = queue.DefaultQueueName
		}
		scheduledAt := now
		if req.ScheduledAt != nil && req.ScheduledAt.After(now) {
			scheduledAt = *req.ScheduledAt
		}
		if len(req.Payload) == 0 {
			req.Payload = []byte("{}")
		}

		job := &queue.Job{
			ID:          uuid.New(),
			Type:        req.Type,
			Payload:     req.Payload,
			Priority:    req.Priority,
			MaxAttempts: req.MaxAttempts,
			QueueName:   req.QueueName,
			ScheduledAt: scheduledAt,
		}
		if key != nil {
			job.APIKeyID = &key.ID
		}
		jobs = append(jobs, job)
	}

	created, err := h.store.CreateJobBatch(r.Context(), jobs)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to persist batch: "+err.Error())
		return
	}

	for _, job := range created {
		if err := h.broker.Enqueue(r.Context(), job); err != nil {
			// Non-fatal: job is persisted, operator can retry individually.
			continue
		}
		queue.CounterJobsEnqueued.Add(1)
		if h.publisher != nil {
			h.publisher.Publish(queue.Event{
				Type:    queue.EventJobEnqueued,
				JobID:   job.ID.String(),
				JobType: job.Type,
				Payload: map[string]any{"job": job},
			})
		}
	}

	writeJSON(w, r, http.StatusCreated, created)
}

// ListJobs handles GET /api/v1/jobs.
//
// Query parameters:
//   - status    filter by job status
//   - type      filter by job type
//   - queue     filter by queue name
//   - limit     page size (default 20, max 100)
//   - offset    page offset (default 0)
func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	filter := store.JobFilter{
		Status:    queryParamString(r, "status"),
		Type:      queryParamString(r, "type"),
		QueueName: queryParamString(r, "queue"),
		Limit:     queryParamInt(r, "limit", 20),
		Offset:    queryParamInt(r, "offset", 0),
	}
	if key := middleware.APIKeyFromContext(r.Context()); key != nil {
		filter.APIKeyID = &key.ID
	}

	page, err := h.store.ListJobs(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	writePaginated(w, r, page.Items, page.TotalCount, page.Limit, page.Offset)
}

// GetJob handles GET /api/v1/jobs/:id.
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get job: "+err.Error())
		return
	}

	if !jobBelongsToKey(r, job) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, r, http.StatusOK, job)
}

// GetJobResult handles GET /api/v1/jobs/:id/result.
// Returns the JSON result stored by the handler on job completion.
// Returns 404 if the job doesn't exist, 204 if no result was stored.
func (h *JobHandler) GetJobResult(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get job: "+err.Error())
		return
	}
	if !jobBelongsToKey(r, job) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}

	result, err := h.store.GetJobResult(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get job result: "+err.Error())
		return
	}

	if len(result) == 0 || string(result) == "null" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

// CancelJob handles DELETE /api/v1/jobs/:id.
// Only pending jobs can be cancelled. Returns 409 Conflict for other states.
func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Remove from Redis first to prevent a worker from picking it up.
	job, err := h.store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get job: "+err.Error())
		return
	}

	if !jobBelongsToKey(r, job) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}

	if job.Status != queue.StatusPending {
		writeError(w, r, http.StatusConflict, "only pending jobs can be cancelled")
		return
	}

	// Best-effort removal from broker; the DB update is authoritative.
	_ = h.broker.Remove(r.Context(), job.QueueName, job.ID.String())

	if err := h.store.CancelJob(r.Context(), id); errors.Is(err, store.ErrConflict) {
		writeError(w, r, http.StatusConflict, "job cannot be cancelled in its current state")
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to cancel job: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]string{"id": id.String(), "status": "cancelled"})
}

// RetryJob handles POST /api/v1/jobs/:id/retry.
// Re-queues a failed or dead job by resetting its status to pending.
func (h *JobHandler) RetryJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := h.store.GetJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get job: "+err.Error())
		return
	}
	if !jobBelongsToKey(r, existing) {
		writeError(w, r, http.StatusNotFound, "job not found")
		return
	}

	job, err := h.store.ResetJobForRetry(r.Context(), id)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, r, http.StatusConflict, "job must be in failed or dead state to retry")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to reset job: "+err.Error())
		return
	}

	if err := h.broker.Enqueue(r.Context(), job); err != nil {
		writeError(w, r, http.StatusInternalServerError, "job reset but broker enqueue failed: "+err.Error())
		return
	}

	if h.publisher != nil {
		h.publisher.Publish(queue.Event{
			Type:    queue.EventJobEnqueued,
			JobID:   job.ID.String(),
			JobType: job.Type,
			Payload: map[string]any{"job": job, "retried": true},
		})
	}

	writeJSON(w, r, http.StatusOK, job)
}

// ListDLQ handles GET /api/v1/dlq.
//
// Query parameters:
//   - include_requeued  "true" to include requeued entries (default false)
//   - limit             page size (default 20, max 100)
//   - offset            page offset (default 0)
func (h *JobHandler) ListDLQ(w http.ResponseWriter, r *http.Request) {
	includeRequeued := queryParamString(r, "include_requeued") == "true"

	filter := store.DLQFilter{
		IncludeRequeued: includeRequeued,
		Limit:           queryParamInt(r, "limit", 20),
		Offset:          queryParamInt(r, "offset", 0),
	}
	if key := middleware.APIKeyFromContext(r.Context()); key != nil {
		filter.APIKeyID = &key.ID
	}

	page, err := h.store.ListDLQ(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list DLQ: "+err.Error())
		return
	}

	writePaginated(w, r, page.Items, page.TotalCount, page.Limit, page.Offset)
}

// RequeueDLQJob handles POST /api/v1/dlq/:id/requeue.
// Creates a new job from the DLQ entry and marks the entry as requeued.
func (h *JobHandler) RequeueDLQJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.store.GetDLQEntry(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "DLQ entry not found")
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get DLQ entry: "+err.Error())
		return
	}

	if entry.Requeued {
		writeError(w, r, http.StatusConflict, "DLQ entry has already been requeued")
		return
	}

	// Create a fresh job from the DLQ entry, preserving API key ownership.
	newJob := &queue.Job{
		ID:          uuid.New(),
		Type:        entry.Type,
		Payload:     entry.Payload,
		Priority:    entry.Priority,
		MaxAttempts: entry.MaxAttempts,
		QueueName:   entry.QueueName,
		ScheduledAt: time.Now(),
		APIKeyID:    entry.APIKeyID,
	}

	createdJob, err := h.store.CreateJob(r.Context(), newJob)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create requeued job: "+err.Error())
		return
	}

	if err := h.broker.Enqueue(r.Context(), createdJob); err != nil {
		writeError(w, r, http.StatusInternalServerError, "job created but broker enqueue failed: "+err.Error())
		return
	}

	if err := h.store.MarkDLQRequeued(r.Context(), id, createdJob.ID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to mark DLQ entry as requeued: "+err.Error())
		return
	}

	if h.publisher != nil {
		h.publisher.Publish(queue.Event{
			Type:    queue.EventJobEnqueued,
			JobID:   createdJob.ID.String(),
			JobType: createdJob.Type,
			Payload: map[string]any{"job": createdJob, "from_dlq": true, "original_id": id},
		})
	}

	writeJSON(w, r, http.StatusCreated, map[string]any{
		"new_job":    createdJob,
		"dlq_entry_id": id,
	})
}

// GetStats handles GET /api/v1/stats.
// Returns job counts per status, jobs/min throughput, DLQ size, and active workers.
func (h *JobHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to get stats: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, stats)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// parseUUID extracts and parses a UUID URL parameter from the request context.
func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, errors.New("invalid UUID: " + raw)
	}
	return id, nil
}

// jobBelongsToKey returns false when a DB-backed API key is present in context
// but the job was created by a different key. Open/static auth always returns true.
func jobBelongsToKey(r *http.Request, job *queue.Job) bool {
	key := middleware.APIKeyFromContext(r.Context())
	if key == nil {
		return true // open or static auth — no scoping
	}
	if job.APIKeyID == nil {
		return true // job predates key scoping
	}
	return *job.APIKeyID == key.ID
}
