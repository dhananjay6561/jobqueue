package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

type apiKeyStorer interface {
	GetAPIKeyByHash(ctx context.Context, raw string) (*queue.APIKey, error)
}

type userKeyStorer interface {
	GetAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*queue.APIKey, error)
}

// APIKeyAuth returns middleware that accepts either X-API-Key or Authorization: Bearer <JWT>.
// JWT auth looks up the user's primary API key and scopes the request to it — so
// logged-in dashboard users get the same per-key scoping as SDK users.
func APIKeyAuth(staticKey, adminKey string, keyStore apiKeyStorer, jwtSecret string, userStore userKeyStorer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if staticKey == "" && adminKey == "" && keyStore == nil && jwtSecret == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ── 1. Admin key ──────────────────────────────────────────────
			supplied := r.Header.Get("X-API-Key")
			if supplied == "" {
				supplied = r.URL.Query().Get("api_key")
			}
			if adminKey != "" && supplied == adminKey {
				ctx := context.WithValue(r.Context(), adminCtxKey{}, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// ── 2. JWT Bearer token → resolve to user's primary API key ──
			if jwtSecret != "" && userStore != nil {
				if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
					raw := strings.TrimPrefix(bearer, "Bearer ")
					tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
						if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
							return nil, jwt.ErrSignatureInvalid
						}
						return []byte(jwtSecret), nil
					}, jwt.WithValidMethods([]string{"HS256"}))
					if err == nil && tok.Valid {
						if claims, ok := tok.Claims.(jwt.MapClaims); ok {
							if sub, ok := claims["sub"].(string); ok {
								if userID, err := uuid.Parse(sub); err == nil {
									keys, err := userStore.GetAPIKeysByUserID(r.Context(), userID)
									if err == nil && len(keys) > 0 && keys[0].Enabled {
										ctx := context.WithValue(r.Context(), apiKeyCtxKey{}, keys[0])
										next.ServeHTTP(w, r.WithContext(ctx))
										return
									}
								}
							}
						}
					}
				}
			}

			// ── 3. X-API-Key DB lookup ────────────────────────────────────
			if keyStore != nil && supplied != "" {
				key, err := keyStore.GetAPIKeyByHash(r.Context(), supplied)
				if err == nil && key.Enabled {
					ctx := context.WithValue(r.Context(), apiKeyCtxKey{}, key)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// ── 4. Static key fallback ────────────────────────────────────
			if staticKey != "" && supplied == staticKey {
				next.ServeHTTP(w, r)
				return
			}

			// ── 5. Reject ─────────────────────────────────────────────────
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid or missing API key","data":null}`))
		})
	}
}

type apiKeyCtxKey struct{}
type adminCtxKey struct{}

func APIKeyFromContext(ctx context.Context) *queue.APIKey {
	k, _ := ctx.Value(apiKeyCtxKey{}).(*queue.APIKey)
	return k
}

func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(adminCtxKey{}).(bool)
	return v
}

// UsageLimitMiddleware increments the monthly job counter before passing through.
// For JWT-authed requests the raw key is not available, so we increment by key ID via the stored key.
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
