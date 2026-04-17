// Package ws — hub_test.go tests the WebSocket hub's event broadcasting
// and client registration logic without requiring a real WebSocket connection.
package ws

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dj/jobqueue/internal/queue"
)

// testClient is a simplified stand-in for a real WebSocket client used in
// hub unit tests. It has the same send channel structure.
type testClient struct {
	hub        *Hub
	send       chan []byte
	remoteAddr string
}

// registerTestClient inserts a fake client directly into the hub's register
// channel and blocks until the hub processes it. It returns both the testClient
// wrapper and the actual *Client pointer so callers can use it for unregistration.
func registerTestClient(hub *Hub) (*testClient, *Client) {
	tc := &testClient{
		hub:        hub,
		send:       make(chan []byte, outboundBufferSize),
		remoteAddr: "test-client",
	}
	// We bypass ws.Client and insert a Client-shaped struct directly.
	client := &Client{
		hub:        hub,
		send:       tc.send,
		remoteAddr: tc.remoteAddr,
	}
	hub.register <- client
	// Give the hub's goroutine time to process the registration.
	time.Sleep(5 * time.Millisecond)
	return tc, client
}

func startHub(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		// Nothing explicit needed; hub.Run exits when the process ends.
	})
	return hub
}

func TestHub_ClientCount(t *testing.T) {
	hub := startHub(t)

	assert.Equal(t, 0, hub.ClientCount(), "no clients at start")

	_, client := registerTestClient(hub)
	assert.Equal(t, 1, hub.ClientCount(), "one client after register")

	// Unregister using the exact same pointer that was registered.
	hub.unregister <- client
	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount(), "zero clients after unregister")
}

func TestHub_Publish_SingleClient(t *testing.T) {
	hub := startHub(t)

	tc, _ := registerTestClient(hub)

	hub.Publish(queue.Event{
		Type:    queue.EventJobEnqueued,
		JobID:   "test-job-id",
		JobType: "send_email",
	})

	// Wait for the broadcast to reach the client.
	select {
	case msg := <-tc.send:
		var event queue.Event
		require.NoError(t, json.Unmarshal(msg, &event))
		assert.Equal(t, queue.EventJobEnqueued, event.Type)
		assert.Equal(t, "test-job-id", event.JobID)
		assert.NotZero(t, event.Timestamp, "timestamp should be set by Publish")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast message")
	}
}

func TestHub_Publish_MultipleClients(t *testing.T) {
	hub := startHub(t)

	const clientCount = 5
	clients := make([]*testClient, clientCount)
	for i := range clients {
		clients[i], _ = registerTestClient(hub)
	}

	hub.Publish(queue.Event{Type: queue.EventStatsUpdate, Payload: "test"})

	var wg sync.WaitGroup
	for _, tc := range clients {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case msg := <-tc.send:
				var event queue.Event
				if err := json.Unmarshal(msg, &event); err != nil {
					t.Errorf("unmarshal event: %v", err)
					return
				}
				assert.Equal(t, queue.EventStatsUpdate, event.Type)
			case <-time.After(200 * time.Millisecond):
				t.Errorf("client did not receive broadcast within timeout")
			}
		}()
	}
	wg.Wait()
}

func TestHub_Publish_SetsTimestamp(t *testing.T) {
	hub := startHub(t)
	tc, _ := registerTestClient(hub)

	before := time.Now().UnixMilli()
	hub.Publish(queue.Event{Type: queue.EventWorkerHeartbeat})
	after := time.Now().UnixMilli()

	select {
	case msg := <-tc.send:
		var event queue.Event
		require.NoError(t, json.Unmarshal(msg, &event))
		assert.GreaterOrEqual(t, event.Timestamp, before)
		assert.LessOrEqual(t, event.Timestamp, after)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out")
	}
}
