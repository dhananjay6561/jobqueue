package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dj/jobqueue/internal/api/middleware"
	"github.com/dj/jobqueue/internal/queue"
	"github.com/google/uuid"
)

// stubKeyStore returns the configured key on any lookup.
type stubKeyStore struct {
	key *queue.APIKey
	err error
}

func (s *stubKeyStore) GetAPIKeyByHash(_ context.Context, _ string) (*queue.APIKey, error) {
	return s.key, s.err
}

func okHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAPIKeyAuth_NoAuth_PassThrough(t *testing.T) {
	t.Parallel()
	mw := middleware.APIKeyAuth("", "", nil)(okHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_StaticKey_Valid(t *testing.T) {
	t.Parallel()
	mw := middleware.APIKeyAuth("secret", "", nil)(okHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_StaticKey_Invalid(t *testing.T) {
	t.Parallel()
	mw := middleware.APIKeyAuth("secret", "", nil)(okHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_AdminKey_SetsContext(t *testing.T) {
	t.Parallel()
	var gotAdmin bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAdmin = middleware.IsAdminFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.APIKeyAuth("", "adminpass", nil)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "adminpass")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !gotAdmin {
		t.Error("expected IsAdminFromContext to be true")
	}
}

func TestAPIKeyAuth_DBKey_SetsContext(t *testing.T) {
	t.Parallel()
	keyID := uuid.New()
	store := &stubKeyStore{key: &queue.APIKey{ID: keyID, Enabled: true}}

	var gotKey *queue.APIKey
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = middleware.APIKeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := middleware.APIKeyAuth("", "", store)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "any-value")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotKey == nil || gotKey.ID != keyID {
		t.Error("expected API key in context")
	}
}
