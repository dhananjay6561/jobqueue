package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// CronStorer is the store interface used by the cron handler.
type CronStorer interface {
	CreateCronSchedule(ctx context.Context, s *queue.CronSchedule) (*queue.CronSchedule, error)
	ListCronSchedules(ctx context.Context) ([]*queue.CronSchedule, error)
	DeleteCronSchedule(ctx context.Context, id uuid.UUID) error
}

// CronHandler handles CRUD for cron schedules.
type CronHandler struct {
	store CronStorer
}

// NewCronHandler creates a CronHandler.
func NewCronHandler(s CronStorer) *CronHandler {
	return &CronHandler{store: s}
}

type createCronRequest struct {
	Name           string          `json:"name"`
	JobType        string          `json:"job_type"`
	Payload        json.RawMessage `json:"payload"`
	QueueName      string          `json:"queue_name"`
	Priority       int             `json:"priority"`
	MaxAttempts    int             `json:"max_attempts"`
	CronExpression string          `json:"cron_expression"`
	Enabled        *bool           `json:"enabled"`
}

// CreateCronSchedule handles POST /api/v1/cron.
func (h *CronHandler) CreateCronSchedule(w http.ResponseWriter, r *http.Request) {
	var body createCronRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}
	if body.JobType == "" {
		writeError(w, r, http.StatusBadRequest, "job_type is required")
		return
	}
	if body.CronExpression == "" {
		writeError(w, r, http.StatusBadRequest, "cron_expression is required")
		return
	}

	// Validate and compute first next_run_at.
	expr, err := queue.ParseCron(body.CronExpression)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid cron_expression: "+err.Error())
		return
	}
	nextRun, err := expr.NextAfter(time.Now().UTC())
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "cron_expression produces no future run times")
		return
	}

	// Apply defaults.
	if body.QueueName == "" {
		body.QueueName = "default"
	}
	if body.Priority == 0 {
		body.Priority = 5
	}
	if body.MaxAttempts == 0 {
		body.MaxAttempts = 3
	}
	if body.Payload == nil {
		body.Payload = json.RawMessage(`{}`)
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	sched := &queue.CronSchedule{
		Name:           body.Name,
		JobType:        body.JobType,
		Payload:        body.Payload,
		QueueName:      body.QueueName,
		Priority:       body.Priority,
		MaxAttempts:    body.MaxAttempts,
		CronExpression: body.CronExpression,
		Enabled:        enabled,
		NextRunAt:      nextRun,
	}

	created, err := h.store.CreateCronSchedule(r.Context(), sched)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create cron schedule: "+err.Error())
		return
	}
	writeJSON(w, r, http.StatusCreated, created)
}

// ListCronSchedules handles GET /api/v1/cron.
func (h *CronHandler) ListCronSchedules(w http.ResponseWriter, r *http.Request) {
	scheds, err := h.store.ListCronSchedules(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list cron schedules: "+err.Error())
		return
	}
	if scheds == nil {
		scheds = []*queue.CronSchedule{}
	}
	writeJSON(w, r, http.StatusOK, scheds)
}

// DeleteCronSchedule handles DELETE /api/v1/cron/:id.
func (h *CronHandler) DeleteCronSchedule(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid cron schedule id")
		return
	}
	if err := h.store.DeleteCronSchedule(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "cron schedule not found")
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to delete cron schedule: "+err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"id": rawID, "status": "deleted"})
}
