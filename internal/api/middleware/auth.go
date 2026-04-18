package middleware

import (
	"net/http"
)

// APIKeyAuth returns a middleware that enforces X-API-Key authentication.
// If key is empty the middleware is a no-op and all requests pass through.
// Clients may supply the key via the X-API-Key header or ?api_key= query param.
func APIKeyAuth(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			supplied := r.Header.Get("X-API-Key")
			if supplied == "" {
				supplied = r.URL.Query().Get("api_key")
			}
			if supplied != key {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"invalid or missing API key","data":null}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
