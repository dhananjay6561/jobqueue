// Package ws implements the WebSocket hub and client types for real-time
// event broadcasting. The hub maintains the set of connected clients and
// fans out every published event to all of them.
//
// Architecture:
//   - A single Hub goroutine serialises all register/unregister/broadcast
//     operations, removing the need for a mutex around the client set.
//   - Each Client has a buffered outbound channel. If the buffer is full
//     (the client is too slow to read), the client is forcibly disconnected
//     to prevent the hub from blocking.
//   - The hub implements queue.EventPublisher so the worker pool can call
//     Publish without importing the ws package (dependency inversion).
package ws

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dj/jobqueue/internal/queue"
)

// outboundBufferSize is the maximum number of events queued per client before
// the client is considered unresponsive and disconnected.
const outboundBufferSize = 256

// Hub is the central message broker for WebSocket clients. It is safe for
// concurrent use — all mutable state changes go through the run loop channel.
type Hub struct {
	// clients is the set of currently registered clients.
	clients map[*Client]struct{}

	// mu protects clients for the stats snapshot (read-only) exposed to the
	// HTTP stats handler.
	mu sync.RWMutex

	// register queues a client for addition to the hub.
	register chan *Client

	// unregister queues a client for removal from the hub.
	unregister chan *Client

	// broadcast queues outbound messages to all clients.
	broadcast chan []byte

	logger zerolog.Logger
}

// Compile-time proof that *Hub satisfies the EventPublisher interface.
var _ queue.EventPublisher = (*Hub)(nil)

// NewHub allocates and returns a Hub. Call Run in a separate goroutine to
// start the event loop.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client, 16),
		unregister: make(chan *Client, 16),
		broadcast:  make(chan []byte, 512),
		logger:     zerolog.New(os.Stdout).With().Str("component", "ws_hub").Logger(),
	}
}

// Run starts the hub's select loop. It must be called exactly once in a
// dedicated goroutine. It returns when ctx (or the parent context) is done.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			h.logger.Debug().Str("remote", client.remoteAddr).Msg("client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debug().Str("remote", client.remoteAddr).Msg("client disconnected")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer is full — drop the client rather than
					// blocking the hub.
					h.mu.RUnlock()
					h.mu.Lock()
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
						h.logger.Warn().Str("remote", client.remoteAddr).Msg("dropped slow client")
					}
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Publish serialises an Event to JSON and queues it for broadcast. It is
// safe to call from any goroutine and is non-blocking — if the broadcast
// channel is full the event is dropped with a warning log.
func (h *Hub) Publish(event queue.Event) {
	event.Timestamp = time.Now().UnixMilli()

	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error().Err(err).Str("event_type", string(event.Type)).Msg("failed to marshal event")
		return
	}

	select {
	case h.broadcast <- data:
	default:
		h.logger.Warn().Str("event_type", string(event.Type)).Msg("broadcast channel full; event dropped")
	}
}

// ClientCount returns the number of currently connected WebSocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
