// Package handler contains all HTTP request handlers.
// Every handler returns a consistent JSON envelope:
//
//	{ "data": <result or null>, "error": <message or null>, "meta": { ... } }
//
// This package also provides shared helpers for encoding responses and
// decoding/validating request bodies.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"
)

// response is the universal JSON response envelope returned by every handler.
type response struct {
	Data  any    `json:"data"`
	Error *string `json:"error"`
	Meta  meta   `json:"meta"`
}

// meta carries request-level metadata included in every response.
type meta struct {
	RequestID string `json:"request_id,omitempty"`
}

// paginatedMeta extends meta with pagination fields for list endpoints.
type paginatedMeta struct {
	meta
	TotalCount int64 `json:"total_count"`
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	HasMore    bool  `json:"has_more"`
}

// writeJSON serialises v as JSON with the given status code.
// It always sets Content-Type: application/json.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	reqID := middleware.GetReqID(r.Context())

	env := response{
		Data:  data,
		Error: nil,
		Meta:  meta{RequestID: reqID},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(env); err != nil {
		// At this point the status line has already been sent, so we can only log.
		http.Error(w, `{"error":"internal encoding error","data":null}`, http.StatusInternalServerError)
	}
}

// writePaginated writes a paginated list response.
func writePaginated(w http.ResponseWriter, r *http.Request, data any, totalCount int64, limit, offset int) {
	reqID := middleware.GetReqID(r.Context())

	hasMore := int64(offset+limit) < totalCount

	type paginatedResponse struct {
		Data  any           `json:"data"`
		Error *string       `json:"error"`
		Meta  paginatedMeta `json:"meta"`
	}

	env := paginatedResponse{
		Data:  data,
		Error: nil,
		Meta: paginatedMeta{
			meta:       meta{RequestID: reqID},
			TotalCount: totalCount,
			Limit:      limit,
			Offset:     offset,
			HasMore:    hasMore,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(env); err != nil {
		http.Error(w, `{"error":"internal encoding error","data":null}`, http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response with the given status code.
func writeError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	reqID := middleware.GetReqID(r.Context())

	env := response{
		Data:  nil,
		Error: &msg,
		Meta:  meta{RequestID: reqID},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(env)
}

// decodeJSON decodes the request body into dst and returns an error if the
// body is malformed or missing required fields.
func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// queryParamInt reads a URL query parameter as an integer. Returns defaultVal
// if the parameter is absent or cannot be parsed.
func queryParamInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return n
}

// queryParamString reads a URL query parameter as a string. Returns "" if absent.
func queryParamString(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}
