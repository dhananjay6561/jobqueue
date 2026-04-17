// Package handler — ws.go handles the WebSocket upgrade endpoint.
// GET /ws upgrades the HTTP connection to WebSocket so the client can receive
// real-time job events pushed by the worker pool and stats ticker.
package handler

import (
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/dj/jobqueue/internal/ws"
)

// WSHandler upgrades HTTP connections to WebSocket and registers them with
// the hub for event broadcasting.
type WSHandler struct {
	hub *ws.Hub
}

// NewWSHandler constructs a WSHandler with the shared WebSocket hub.
func NewWSHandler(hub *ws.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// ServeWS handles GET /ws — upgrades to WebSocket and parks the goroutines.
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	if err := ws.NewClient(h.hub, w, r); err != nil {
		log.Error().Err(err).Str("remote", r.RemoteAddr).Msg("ws: upgrade failed")
		return
	}
}
