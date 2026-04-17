// Package middleware — cors.go configures cross-origin resource sharing.
// In development the CORS policy is permissive; in production callers should
// restrict AllowedOrigins to the actual dashboard domain.
package middleware

import (
	"net/http"

	chiCORS "github.com/go-chi/cors"
)

// CORS returns a Chi-compatible CORS middleware configured for a job-queue
// dashboard. Adjust AllowedOrigins before deploying to production.
func CORS(next http.Handler) http.Handler {
	cors := chiCORS.New(chiCORS.Options{
		// AllowedOrigins: restrict to your dashboard domain in production.
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-Request-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
		},
		AllowCredentials: false,
		MaxAge:           300,
	})
	return cors.Handler(next)
}
