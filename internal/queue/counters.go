package queue

import "sync/atomic"

// Package-level atomic counters incremented by the worker pool.
// The Prometheus metrics handler in the api package reads these on each scrape.
var (
	CounterJobsEnqueued  atomic.Int64
	CounterJobsCompleted atomic.Int64
	CounterJobsFailed    atomic.Int64
	CounterJobsDead      atomic.Int64
)
