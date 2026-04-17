// Package queue — retry_test.go contains table-driven unit tests for the
// exponential backoff retry policy.
package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryPolicy_NextDelay(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		BaseDelay: 5 * time.Second,
		MaxDelay:  1 * time.Hour,
	}

	tests := []struct {
		name     string
		attempt  int
		wantMin  time.Duration // inclusive lower bound
		wantMax  time.Duration // inclusive upper bound
	}{
		{
			name:    "attempt 1 returns BaseDelay",
			attempt: 1,
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "attempt 2 doubles",
			attempt: 2,
			wantMin: 10 * time.Second,
			wantMax: 10 * time.Second,
		},
		{
			name:    "attempt 3 quadruples",
			attempt: 3,
			wantMin: 20 * time.Second,
			wantMax: 20 * time.Second,
		},
		{
			name:    "attempt 4",
			attempt: 4,
			wantMin: 40 * time.Second,
			wantMax: 40 * time.Second,
		},
		{
			name:    "attempt 5",
			attempt: 5,
			wantMin: 80 * time.Second,
			wantMax: 80 * time.Second,
		},
		{
			name:    "large attempt is capped at MaxDelay",
			attempt: 1000,
			wantMin: 1 * time.Hour,
			wantMax: 1 * time.Hour,
		},
		{
			name:    "zero attempt treated as 1",
			attempt: 0,
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "negative attempt treated as 1",
			attempt: -5,
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := policy.NextDelay(tt.attempt)
			assert.GreaterOrEqual(t, got, tt.wantMin, "delay should be >= wantMin")
			assert.LessOrEqual(t, got, tt.wantMax, "delay should be <= wantMax")
		})
	}
}

func TestRetryPolicy_MaxDelayWithSmallBase(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		BaseDelay: 1 * time.Second,
		MaxDelay:  10 * time.Second,
	}

	// After enough doublings, the cap kicks in.
	delay := policy.NextDelay(100)
	assert.Equal(t, 10*time.Second, delay, "delay should be capped at MaxDelay")
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	const defaultMax = 5

	tests := []struct {
		name               string
		attempts           int
		maxAttempts        int
		defaultMaxAttempts int
		want               bool
	}{
		{
			name:               "first attempt of five should retry",
			attempts:           1,
			maxAttempts:        5,
			defaultMaxAttempts: defaultMax,
			want:               true,
		},
		{
			name:               "at limit should not retry",
			attempts:           5,
			maxAttempts:        5,
			defaultMaxAttempts: defaultMax,
			want:               false,
		},
		{
			name:               "beyond limit should not retry",
			attempts:           10,
			maxAttempts:        5,
			defaultMaxAttempts: defaultMax,
			want:               false,
		},
		{
			name:               "zero maxAttempts falls back to default",
			attempts:           3,
			maxAttempts:        0,
			defaultMaxAttempts: defaultMax,
			want:               true,
		},
		{
			name:               "zero maxAttempts at default limit should not retry",
			attempts:           5,
			maxAttempts:        0,
			defaultMaxAttempts: defaultMax,
			want:               false,
		},
		{
			name:               "negative maxAttempts falls back to default",
			attempts:           1,
			maxAttempts:        -1,
			defaultMaxAttempts: defaultMax,
			want:               true,
		},
		{
			name:               "zero attempts should always retry",
			attempts:           0,
			maxAttempts:        3,
			defaultMaxAttempts: defaultMax,
			want:               true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ShouldRetry(tt.attempts, tt.maxAttempts, tt.defaultMaxAttempts)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRetryPolicy_ScheduledRetryAt(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		BaseDelay: 5 * time.Second,
		MaxDelay:  1 * time.Hour,
	}

	before := time.Now()
	retryAt := policy.ScheduledRetryAt(1)
	after := time.Now()

	expectedDelay := 5 * time.Second
	lowerBound := before.Add(expectedDelay)
	upperBound := after.Add(expectedDelay)

	assert.True(t, retryAt.After(lowerBound) || retryAt.Equal(lowerBound),
		"retryAt should be at least BaseDelay from now")
	assert.True(t, retryAt.Before(upperBound) || retryAt.Equal(upperBound),
		"retryAt should not be more than BaseDelay + epsilon from now")
}
