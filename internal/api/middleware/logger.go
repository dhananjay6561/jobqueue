// Package middleware contains HTTP middleware functions for the Chi router.
// Each middleware is a pure function that wraps an http.Handler and adds
// orthogonal behaviour (logging, rate-limiting, CORS).
package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger returns a zerolog-based request logging middleware. It logs:
//   - HTTP method, path, status code, latency
//   - Request ID (injected by the RequestID middleware upstream)
//   - Remote address and user agent
//
// The middleware uses the global zerolog logger so the log level is controlled
// by the application's zerolog configuration.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := middleware.GetReqID(r.Context())

		// Wrap the ResponseWriter so we can capture the status code.
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			latency := time.Since(start)
			status := wrapped.Status()

			var event *zerolog.Event
			switch {
			case status >= 500:
				event = log.Error()
			case status >= 400:
				event = log.Warn()
			default:
				event = log.Info()
			}

			event.
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Int("status", status).
				Int("bytes", wrapped.BytesWritten()).
				Dur("latency_ms", latency).
				Msg("request")
		}()

		next.ServeHTTP(wrapped, r)
	})
}
