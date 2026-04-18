package api

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
	"github.com/dj/jobqueue/internal/ws"
)

// jobqueueMetrics holds all Prometheus instruments for the system.
type jobqueueMetrics struct {
	// Gauges (point-in-time snapshots refreshed on each scrape)
	queueDepth    prometheus.Gauge
	activeWorkers prometheus.Gauge
	wsClients     prometheus.Gauge

	// Histogram for per-job processing duration in seconds
	jobDuration *prometheus.HistogramVec
}

var prom = func() *jobqueueMetrics {
	ns := "jobqueue"

	// Counter functions read the atomic values from the queue package so there
	// is no double-counting and no circular import.
	promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: ns, Name: "jobs_enqueued_total",
		Help: "Total number of jobs submitted via the API.",
	}, func() float64 { return float64(queue.CounterJobsEnqueued.Load()) })

	promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: ns, Name: "jobs_completed_total",
		Help: "Total number of jobs completed successfully.",
	}, func() float64 { return float64(queue.CounterJobsCompleted.Load()) })

	promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: ns, Name: "jobs_failed_total",
		Help: "Total number of job handler failures (retryable).",
	}, func() float64 { return float64(queue.CounterJobsFailed.Load()) })

	promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: ns, Name: "jobs_dead_total",
		Help: "Total number of jobs that exhausted retries and entered the DLQ.",
	}, func() float64 { return float64(queue.CounterJobsDead.Load()) })

	return &jobqueueMetrics{
		queueDepth: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Name: "queue_depth",
			Help: "Current number of pending + running jobs.",
		}),
		activeWorkers: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Name: "active_workers",
			Help: "Number of currently active worker goroutines.",
		}),
		wsClients: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: ns, Name: "ws_clients",
			Help: "Number of connected WebSocket clients.",
		}),
		jobDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns, Name: "job_duration_seconds",
			Help:    "Job processing duration in seconds, by job type.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job_type"}),
	}
}()

// prometheusHandler returns an http.HandlerFunc that refreshes gauge snapshots
// from the DB then delegates to the standard promhttp handler.
func prometheusHandler(jobStore store.JobStorer, hub *ws.Hub) http.HandlerFunc {
	h := promhttp.Handler()
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if stats, err := jobStore.GetStats(ctx); err == nil {
			prom.queueDepth.Set(float64(stats.QueueDepth))
			prom.activeWorkers.Set(float64(stats.ActiveWorkers))
		}
		prom.wsClients.Set(float64(hub.ClientCount()))

		h.ServeHTTP(w, r)
	}
}
