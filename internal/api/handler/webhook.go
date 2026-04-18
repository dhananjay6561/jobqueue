package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// WebhookStorer is the store interface used by the webhook handler.
type WebhookStorer interface {
	CreateWebhook(ctx context.Context, url, secret string, events []string, enabled bool) (*queue.Webhook, error)
	ListWebhooks(ctx context.Context) ([]*queue.Webhook, error)
	DeleteWebhook(ctx context.Context, id uuid.UUID) error
}

// WebhookHandler handles CRUD for registered webhooks.
type WebhookHandler struct {
	store WebhookStorer
}

// NewWebhookHandler creates a WebhookHandler.
func NewWebhookHandler(s WebhookStorer) *WebhookHandler {
	return &WebhookHandler{store: s}
}

var defaultEvents = []string{"job.completed", "job.failed", "job.dead"}

// CreateWebhook handles POST /api/v1/webhooks.
func (h *WebhookHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL     string   `json:"url"`
		Secret  string   `json:"secret"`
		Events  []string `json:"events"`
		Enabled *bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.URL == "" {
		writeError(w, r, http.StatusBadRequest, "url is required")
		return
	}
	events := body.Events
	if len(events) == 0 {
		events = defaultEvents
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	hook, err := h.store.CreateWebhook(r.Context(), body.URL, body.Secret, events, enabled)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create webhook: "+err.Error())
		return
	}
	writeJSON(w, r, http.StatusCreated, hook)
}

// ListWebhooks handles GET /api/v1/webhooks.
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := h.store.ListWebhooks(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list webhooks: "+err.Error())
		return
	}
	if hooks == nil {
		hooks = []*queue.Webhook{}
	}
	writeJSON(w, r, http.StatusOK, hooks)
}

// DeleteWebhook handles DELETE /api/v1/webhooks/:id.
func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid webhook id")
		return
	}
	if err := h.store.DeleteWebhook(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "webhook not found")
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to delete webhook: "+err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"id": rawID, "status": "deleted"})
}
