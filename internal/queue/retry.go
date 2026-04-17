// Package queue — retry.go implements the exponential-backoff retry policy
// and the logic that decides whether to re-queue a failed job or move it to
// the dead-letter queue.
package queue

import (
	"math"
	"time"
)

// RetryPolicy encapsulates all parameters that govern retry behaviour.
type RetryPolicy struct {
	// BaseDelay is the initial backoff duration applied after the first failure.
	BaseDelay time.Duration

	// MaxDelay caps the computed backoff to prevent multi-hour waits.
	MaxDelay time.Duration
}

// NextDelay computes the exponential backoff for the given attempt number.
//
// Formula:  delay = BaseDelay × 2^(attempt-1)
//
// The result is capped at MaxDelay so workers are not stalled indefinitely.
// attempt is 1-based: the first retry uses attempt=1, yielding BaseDelay × 1.
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}

	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(p.BaseDelay) * multiplier)

	if delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}

// ShouldRetry returns true when the job has not yet exhausted its retry budget.
// A job's maxAttempts of 0 falls back to defaultMaxAttempts from config.
func ShouldRetry(attempts, maxAttempts, defaultMaxAttempts int) bool {
	limit := maxAttempts
	if limit <= 0 {
		limit = defaultMaxAttempts
	}
	return attempts < limit
}

// ScheduledRetryAt returns the time at which the next retry should become
// eligible for dequeue.
func (p RetryPolicy) ScheduledRetryAt(attempt int) time.Time {
	return time.Now().Add(p.NextDelay(attempt))
}
