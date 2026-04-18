package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// APIKeyStorer is the store interface used by the API key handler.
type APIKeyStorer interface {
	CreateAPIKey(ctx context.Context, name string, tier queue.APIKeyTier) (*queue.APIKey, string, error)
	ListAPIKeys(ctx context.Context) ([]*queue.APIKey, error)
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error
}

// APIKeyHandler handles CRUD for API keys.
type APIKeyHandler struct {
	store APIKeyStorer
}

// NewAPIKeyHandler creates an APIKeyHandler.
func NewAPIKeyHandler(s APIKeyStorer) *APIKeyHandler {
	return &APIKeyHandler{store: s}
}

// CreateAPIKey handles POST /api/v1/keys.
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required")
		return
	}

	tier := queue.APIKeyTier(body.Tier)
	switch tier {
	case queue.TierFree, queue.TierPro, queue.TierBusiness:
	default:
		tier = queue.TierFree
	}

	key, rawKey, err := h.store.CreateAPIKey(r.Context(), body.Name, tier)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to create API key: "+err.Error())
		return
	}

	// Return raw key only once — it is never stored and cannot be recovered.
	writeJSON(w, r, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key":        rawKey, // shown once only
		"key_prefix": key.KeyPrefix,
		"tier":       key.Tier,
		"jobs_limit": key.JobsLimit,
		"reset_at":   key.ResetAt,
		"created_at": key.CreatedAt,
		"warning":    "Save this key — it will not be shown again.",
	})
}

// ListAPIKeys handles GET /api/v1/keys.
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListAPIKeys(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to list API keys: "+err.Error())
		return
	}
	if keys == nil {
		keys = []*queue.APIKey{}
	}
	writeJSON(w, r, http.StatusOK, keys)
}

// DeleteAPIKey handles DELETE /api/v1/keys/:id.
func (h *APIKeyHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid key id")
		return
	}
	if err := h.store.DeleteAPIKey(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "API key not found")
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to delete API key: "+err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetUsage handles GET /api/v1/usage — returns the current key's usage.
func (h *APIKeyHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	key := middleware.APIKeyFromContext(r.Context())
	if key == nil {
		writeError(w, r, http.StatusUnauthorized, "API key required to check usage")
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"tier":           key.Tier,
		"jobs_used":      key.JobsUsed,
		"jobs_limit":     key.JobsLimit,
		"usage_percent":  key.UsagePercent(),
		"limit_reached":  key.LimitReached(),
		"reset_at":       key.ResetAt,
	})
}
