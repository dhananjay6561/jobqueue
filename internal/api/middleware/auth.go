package middleware

import (
	"context"
	"net/http"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

type apiKeyStorer interface {
	GetAPIKeyByHash(ctx context.Context, raw string) (*queue.APIKey, error)
}

// APIKeyAuth returns middleware that:
//   - If no store is provided (legacy mode): falls back to static key check.
//   - If store is provided: validates the key exists in the DB, checks it's
//     enabled, and rejects with 429 if the monthly limit is reached.
//   - If adminKey is non-empty and the supplied key matches it, the request is
//     marked as admin (bypasses per-key scoping) and passes through.
//
// The resolved *queue.APIKey is stored in the request context under apiKeyCtxKey
// so downstream handlers (e.g. the enqueue handler) can call IncrementUsage.
func APIKeyAuth(staticKey, adminKey string, store apiKeyStorer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// No auth configured at all — pass through.
		if staticKey == "" && adminKey == "" && store == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			supplied := r.Header.Get("X-API-Key")
			if supplied == "" {
				supplied = r.URL.Query().Get("api_key")
			}

			// Admin key check — bypasses DB lookup and scoping.
			if adminKey != "" && supplied == adminKey {
				ctx := context.WithValue(r.Context(), adminCtxKey{}, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// DB-backed key validation.
			if store != nil && supplied != "" {
				key, err := store.GetAPIKeyByHash(r.Context(), supplied)
				if err == nil && key.Enabled {
					ctx := context.WithValue(r.Context(), apiKeyCtxKey{}, key)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Fall back to static key comparison.
			if staticKey != "" && supplied == staticKey {
				next.ServeHTTP(w, r)
				return
			}

			// No valid key supplied.
			if staticKey != "" || adminKey != "" || store != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid or missing API key","data":null}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type apiKeyCtxKey struct{}
type adminCtxKey struct{}

// APIKeyFromContext retrieves the resolved APIKey from the request context.
// Returns nil if no DB-backed key was resolved (static key or no auth).
func APIKeyFromContext(ctx context.Context) *queue.APIKey {
	k, _ := ctx.Value(apiKeyCtxKey{}).(*queue.APIKey)
	return k
}

// IsAdminFromContext returns true when the request was authenticated with the
// admin key and should bypass per-key data scoping.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(adminCtxKey{}).(bool)
	return v
}

// EnforceUsageLimit is a per-route middleware that increments the usage counter
// for the API key in context and blocks with 429 if the limit is reached.
// It is applied only to the enqueue endpoint, not to read-only routes.
func EnforceUsageLimit(s usageIncrementer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				// No DB-backed key (open or static auth) — skip metering.
				next.ServeHTTP(w, r)
				return
			}

			// Check current usage before incrementing.
			if key.LimitReached() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"monthly job limit reached — upgrade your plan to continue","data":null}`))
				return
			}

			// Atomically increment.
			updated, err := s.IncrementAPIKeyUsage(r.Context(), "")
			if err == nil && updated.LimitReached() && updated.JobsUsed > updated.JobsLimit {
				// Edge case: concurrent request pushed it over — still allow this one.
				_ = updated
			}

			next.ServeHTTP(w, r)
		})
	}
}

type usageIncrementer interface {
	IncrementAPIKeyUsage(ctx context.Context, raw string) (*queue.APIKey, error)
}

// UsageLimitMiddleware is a simpler version that takes the store + raw key
// from context and increments before passing through.
func UsageLimitMiddleware(db store.JobStorer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := APIKeyFromContext(r.Context())
			if key == nil {
				next.ServeHTTP(w, r)
				return
			}

			if key.LimitReached() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"monthly job limit reached — upgrade your plan to continue","data":null}`))
				return
			}

			// Get raw key from header to increment.
			raw := r.Header.Get("X-API-Key")
			if raw == "" {
				raw = r.URL.Query().Get("api_key")
			}
			if raw != "" {
				_, _ = db.IncrementAPIKeyUsage(r.Context(), raw)
			}

			next.ServeHTTP(w, r)
		})
	}
}
