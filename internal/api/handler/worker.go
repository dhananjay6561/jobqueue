// Package handler — worker.go implements the worker-registry HTTP handler.
//
//   - GET /api/v1/workers  → list active workers and their current state
package handler

import (
	"net/http"

	"github.com/dj/jobqueue/internal/store"
)

// WorkerHandler handles worker-registry HTTP endpoints.
type WorkerHandler struct {
	store store.JobStorer
}

// NewWorkerHandler constructs a WorkerHandler with its required store.
func NewWorkerHandler(jobStore store.JobStorer) *WorkerHandler {
	return &WorkerHandler{store: jobStore}
}

// ListWorkers handles GET /api/v1/workers.
//
// Query parameters:
//   - active_only  "false" to include stopped workers (default true)
func (h *WorkerHandler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	activeOnly := queryParamString(r, "active_only") != "false"

	workers, err := h.store.ListWorkers(r.Context(), activeOnly)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list workers: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, workers)
}
