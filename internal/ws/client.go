// Package ws — client.go manages a single WebSocket connection lifecycle.
//
// Each client has two goroutines:
//   - readPump: reads frames from the browser to handle pings/pongs and detect
//     disconnection. Messages from the client are currently ignored (read-only
//     dashboard) but the loop is required so gorilla/websocket can call its
//     internal close handler.
//   - writePump: drains the outbound channel and forwards messages to the wire.
//     A ticker sends WebSocket ping frames to keep the connection alive through
//     proxies that close idle connections.
package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Connection tuning constants. These are conservative defaults appropriate for
// a dashboard client over a LAN or low-latency internet connection.
const (
	// writeWait is the maximum duration allowed to write a single message.
	writeWait = 10 * time.Second

	// pongWait is the maximum duration to wait for a pong response after sending ping.
	pongWait = 60 * time.Second

	// pingPeriod is how often the server sends a ping frame. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize is the maximum inbound message size in bytes.
	maxMessageSize = 1024
)

// upgrader configures the WebSocket handshake. CheckOrigin is permissive here;
// production deployments should restrict this to known origins.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client represents a single WebSocket connection. It is created by the HTTP
// upgrade handler and registered with the Hub.
type Client struct {
	hub *Hub

	// conn is the underlying WebSocket connection.
	conn *websocket.Conn

	// send is a buffered channel of outbound JSON messages.
	send chan []byte

	// remoteAddr is the client's IP:port for logging.
	remoteAddr string
}

// NewClient upgrades an HTTP connection to WebSocket, registers the client
// with the hub, and starts both pump goroutines. It returns immediately after
// launching the goroutines.
func NewClient(hub *Hub, w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	client := &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, outboundBufferSize),
		remoteAddr: r.RemoteAddr,
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()

	return nil
}

// readPump drains inbound frames and handles close/ping/pong. It signals the
// hub to unregister the client and closes the connection on any error.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Error().Err(err).Msg("ws: set initial read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		// We do not act on inbound messages; we only drain the read side so
		// the WebSocket library can process control frames.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Error().Err(err).Str("remote", c.remoteAddr).Msg("ws: unexpected close")
			}
			return
		}
	}
}

// writePump drains the outbound channel and writes each message to the wire.
// It also sends periodic ping frames to detect dead connections.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Error().Err(err).Msg("ws: set write deadline")
				return
			}

			if !ok {
				// The hub closed the channel; send a clean close frame.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			writer, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := writer.Write(message); err != nil {
				return
			}

			// Flush any additional messages queued since we started writing.
			pending := len(c.send)
			for i := 0; i < pending; i++ {
				if _, err := writer.Write([]byte("\n")); err != nil {
					return
				}
				if _, err := writer.Write(<-c.send); err != nil {
					return
				}
			}

			if err := writer.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
