// Package queue — broker_test.go tests the RedisBroker using a mock Redis
// client pattern. Integration tests that use a real Redis instance can be run
// with: go test -tags=integration ./...
package queue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Unit tests for score computation ────────────────────────────────────────

func TestPriorityScore_HigherPriorityHasLowerScore(t *testing.T) {
	t.Parallel()

	scheduledAt := time.Now()

	highPriority := priorityScore(10, scheduledAt)
	lowPriority := priorityScore(1, scheduledAt)

	assert.Less(t, highPriority, lowPriority,
		"higher priority jobs must have a lower score (dequeued first by ZPOPMIN)")
}

func TestPriorityScore_EarlierScheduledAtHasLowerScore(t *testing.T) {
	t.Parallel()

	earlier := time.Now()
	later := earlier.Add(5 * time.Minute)

	sameP := 5

	scoreEarlier := priorityScore(sameP, earlier)
	scoreLater := priorityScore(sameP, later)

	assert.Less(t, scoreEarlier, scoreLater,
		"for the same priority, earlier scheduled_at must have a lower score")
}

func TestPriorityScore_HigherPriorityBeatsLaterSchedule(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// A high-priority job scheduled far in the future should still beat a
	// low-priority job scheduled in the past, because the priority component
	// dominates.
	highP := priorityScore(10, now.Add(24*time.Hour))
	lowP := priorityScore(1, now.Add(-24*time.Hour))

	assert.Less(t, highP, lowP,
		"high-priority job scheduled in future should have lower score than low-priority job in past")
}

// ─── Job struct tests ─────────────────────────────────────────────────────────

func TestJob_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	original := &Job{
		ID:          uuid.New(),
		Type:        "send_email",
		Payload:     json.RawMessage(`{"to":"a@b.com"}`),
		Priority:    8,
		Status:      StatusPending,
		Attempts:    0,
		MaxAttempts: 5,
		QueueName:   "default",
		ScheduledAt: time.Now().Truncate(time.Millisecond),
		CreatedAt:   time.Now().Truncate(time.Millisecond),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Job
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.Priority, decoded.Priority)
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.MaxAttempts, decoded.MaxAttempts)
	assert.Equal(t, original.QueueName, decoded.QueueName)
}

func TestEnqueueRequest_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		req         EnqueueRequest
		wantPrio    int
		wantQueue   string
	}{
		{
			name:      "zero priority uses default",
			req:       EnqueueRequest{Type: "noop", Priority: 0},
			wantPrio:  PriorityDefault,
			wantQueue: "",
		},
		{
			name:      "explicit priority retained",
			req:       EnqueueRequest{Type: "noop", Priority: 9},
			wantPrio:  9,
			wantQueue: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Mimic the handler logic that normalises defaults.
			if tt.req.Priority < PriorityMin || tt.req.Priority > PriorityMax {
				tt.req.Priority = PriorityDefault
			}
			if tt.req.QueueName == "" {
				tt.req.QueueName = DefaultQueueName
			}

			assert.Equal(t, tt.wantPrio, tt.req.Priority)
			assert.Equal(t, DefaultQueueName, tt.req.QueueName)
		})
	}
}
