// Package queue — worker_pool.go implements the concurrent worker pool.
//
// Design decisions:
//   - Each worker runs in its own goroutine with a bounded lifetime tied to
//     the pool's context. When the context is cancelled (graceful shutdown),
//     each worker finishes its current job then exits.
//   - Workers poll Redis for the next job ID, then hydrate the full job from
//     PostgreSQL before processing. This two-phase approach keeps Redis lean.
//   - A heartbeat goroutine per worker updates last_seen in PostgreSQL every
//     HeartbeatInterval so the health dashboard can detect stale workers.
//   - All errors are logged and fed to the retry/DLQ pipeline rather than
//     propagating to the caller — worker goroutines must not crash the process.
//   - A sync.WaitGroup is used to coordinate graceful shutdown; the pool's
//     Shutdown method blocks until all workers have returned.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/dj/jobqueue/internal/config"
)

// Handler is a function that processes a single job. Returning a non-nil error
// marks the job as failed; a nil return marks it as completed.
// The handler may optionally return a JSON-serialisable result as the second
// value — pass nil to store no result.
type Handler func(ctx context.Context, job *Job) (result any, err error)

// EventPublisher is called by the pool on every significant state change so
// connected WebSocket clients can receive real-time updates.
type EventPublisher interface {
	Publish(event Event)
}

// JobStore is a subset of store.JobStorer used by the worker pool.
// Using a focused interface rather than the full store interface keeps the
// dependency surface minimal and tests simple.
type JobStore interface {
	GetJob(ctx context.Context, id uuid.UUID) (*Job, error)
	MarkJobStarted(ctx context.Context, id uuid.UUID, workerID string) (*Job, error)
	MarkJobCompleted(ctx context.Context, id uuid.UUID, result json.RawMessage) (*Job, error)
	MarkJobFailed(ctx context.Context, id uuid.UUID, errMsg string) (*Job, error)
	MarkJobDead(ctx context.Context, id uuid.UUID, errMsg string) error
	InsertDLQ(ctx context.Context, job *Job) error
	UpsertWorker(ctx context.Context, id string) error
	UpdateWorkerHeartbeat(ctx context.Context, workerID string, currentJobID *uuid.UUID) error
	UpdateWorkerStats(ctx context.Context, workerID string, processed, failed int64) error
	MarkWorkerStopped(ctx context.Context, workerID string) error
}

// Pool manages a fixed set of worker goroutines that consume jobs from Redis
// and process them against registered handlers.
type Pool struct {
	cfg       config.WorkerConfig
	retryCfg  config.RetryConfig
	broker    Broker
	store     JobStore
	publisher EventPublisher
	handlers  map[string]Handler
	logger    zerolog.Logger

	wg       sync.WaitGroup
	cancelFn context.CancelFunc

	// queues is the list of queue names this pool consumes from.
	queues []string
}

// PoolConfig bundles all dependencies for NewPool.
type PoolConfig struct {
	Config    config.WorkerConfig
	Retry     config.RetryConfig
	Broker    Broker
	Store     JobStore
	Publisher EventPublisher
	Queues    []string
}

// NewPool creates a Pool. Handlers must be registered before calling Start.
func NewPool(cfg PoolConfig) *Pool {
	queues := cfg.Queues
	if len(queues) == 0 {
		queues = []string{DefaultQueueName}
	}

	return &Pool{
		cfg:       cfg.Config,
		retryCfg:  cfg.Retry,
		broker:    cfg.Broker,
		store:     cfg.Store,
		publisher: cfg.Publisher,
		handlers:  make(map[string]Handler),
		logger:    zerolog.New(os.Stdout).With().Str("component", "worker_pool").Logger(),
		queues:    queues,
	}
}

// Register associates a Handler with a job type name. Calling Register after
// Start is a programming error and will have no effect on running workers.
func (p *Pool) Register(jobType string, handler Handler) {
	p.handlers[jobType] = handler
}

// Start launches cfg.Count worker goroutines and a delayed-job promoter.
// The supplied context controls the lifetime of the entire pool.
// Callers should call Shutdown to wait for graceful termination.
func (p *Pool) Start(ctx context.Context) {
	poolCtx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	for workerIndex := 0; workerIndex < p.cfg.Count; workerIndex++ {
		workerID := buildWorkerID(workerIndex)

		p.wg.Add(1)
		go func(id string) {
			defer p.wg.Done()
			p.runWorker(poolCtx, id)
		}(workerID)
	}

	// Background goroutine that promotes delayed jobs into the active queue.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.runPromoter(poolCtx)
	}()
}

// Shutdown signals all workers to stop and waits for them to finish their
// current job. It blocks until either all workers have returned or the supplied
// timeout elapses.
func (p *Pool) Shutdown() {
	if p.cancelFn != nil {
		p.cancelFn()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info().Msg("all workers stopped cleanly")
	case <-time.After(p.cfg.ShutdownTimeout):
		p.logger.Warn().
			Dur("timeout", p.cfg.ShutdownTimeout).
			Msg("shutdown timeout reached; some workers may still be running")
	}
}

// ─── Worker goroutine ─────────────────────────────────────────────────────────

// runWorker is the main loop for a single worker goroutine. It runs until ctx
// is cancelled, polling Redis for the next job and processing it.
func (p *Pool) runWorker(ctx context.Context, workerID string) {
	log := p.logger.With().Str("worker_id", workerID).Logger()

	// Register the worker in PostgreSQL.
	if err := p.store.UpsertWorker(context.Background(), workerID); err != nil {
		log.Error().Err(err).Msg("failed to register worker")
	}

	// Publish worker heartbeat event.
	if p.publisher != nil {
		p.publisher.Publish(Event{
			Type:     EventWorkerHeartbeat,
			WorkerID: workerID,
			Payload:  map[string]any{"status": "started"},
		})
	}

	// Start the heartbeat sub-goroutine.
	heartbeatDone := make(chan struct{})
	var currentJobID atomic.Pointer[uuid.UUID]

	go func() {
		defer close(heartbeatDone)
		p.runHeartbeat(ctx, workerID, &currentJobID)
	}()

	var processed, failed int64

	defer func() {
		// Best-effort: mark worker as stopped in the database.
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := p.store.UpdateWorkerStats(stopCtx, workerID, processed, failed); err != nil {
			log.Error().Err(err).Msg("failed to update final worker stats")
		}
		if err := p.store.MarkWorkerStopped(stopCtx, workerID); err != nil {
			log.Error().Err(err).Msg("failed to mark worker stopped")
		}

		<-heartbeatDone
		log.Info().Int64("processed", processed).Int64("failed", failed).Msg("worker stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		jobID, err := p.dequeueNextJob(ctx)
		if err != nil {
			log.Error().Err(err).Msg("dequeue error")
			p.sleepOrCancel(ctx, p.cfg.PollInterval)
			continue
		}

		if jobID == "" {
			// Queue was empty; back off before polling again.
			p.sleepOrCancel(ctx, p.cfg.PollInterval)
			continue
		}

		id, err := uuid.Parse(jobID)
		if err != nil {
			log.Error().Str("raw_id", jobID).Err(err).Msg("invalid job id from queue")
			continue
		}

		// Record what job this worker is processing.
		currentJobID.Store(&id)

		if p.processJob(ctx, workerID, id, log) {
			failed++
		}

		currentJobID.Store(nil)

		processed++
	}
}

// dequeueNextJob tries each registered queue name in order and returns the
// first job ID found. Returns ("", nil) when all queues are empty.
func (p *Pool) dequeueNextJob(ctx context.Context) (string, error) {
	for _, queueName := range p.queues {
		jobID, err := p.broker.Dequeue(ctx, queueName)
		if err != nil {
			return "", fmt.Errorf("dequeue from %s: %w", queueName, err)
		}
		if jobID != "" {
			return jobID, nil
		}
	}
	return "", nil
}

// processJob hydrates the full job from PostgreSQL, dispatches it to the
// registered handler, and handles success/failure/retry/DLQ transitions.
// Returns true if the job failed (handler error or no handler), false on success.
func (p *Pool) processJob(ctx context.Context, workerID string, id uuid.UUID, log zerolog.Logger) bool {
	log = log.With().Str("job_id", id.String()).Logger()

	// Verify the job exists in PostgreSQL before claiming it.
	if _, err := p.store.GetJob(ctx, id); err != nil {
		log.Error().Err(err).Msg("failed to hydrate job from store")
		return false
	}

	// Mark the job as running.
	job, err := p.store.MarkJobStarted(ctx, id, workerID)
	if err != nil {
		log.Error().Err(err).Msg("failed to mark job as started")
		return false
	}

	p.publishEvent(EventJobStarted, job, workerID)
	log.Info().Str("type", job.Type).Msg("job started")

	// Find the handler.
	handler, ok := p.handlers[job.Type]
	if !ok {
		errMsg := fmt.Sprintf("no handler registered for job type %q", job.Type)
		log.Error().Msg(errMsg)
		p.handleJobFailure(ctx, job, workerID, errMsg, log)
		return true
	}

	// Execute the handler with a per-job context so it can be cancelled on
	// pool shutdown without affecting other workers.
	handlerResult, handlerErr := handler(ctx, job)

	if handlerErr == nil {
		// Success path — marshal the optional handler result.
		var resultJSON json.RawMessage
		if handlerResult != nil {
			if b, merr := json.Marshal(handlerResult); merr == nil {
				resultJSON = b
			}
		}
		if _, err := p.store.MarkJobCompleted(ctx, id, resultJSON); err != nil {
			log.Error().Err(err).Msg("failed to mark job completed")
			return false
		}
		CounterJobsCompleted.Add(1)
		p.publishEvent(EventJobCompleted, job, workerID)
		log.Info().Str("type", job.Type).Msg("job completed")
		return false
	}

	// Failure path.
	log.Warn().Err(handlerErr).Str("type", job.Type).Int("attempts", job.Attempts).Msg("job failed")
	p.handleJobFailure(ctx, job, workerID, handlerErr.Error(), log)
	return true
}

// handleJobFailure decides whether to retry or move to DLQ based on the
// attempt count and max_attempts policy.
func (p *Pool) handleJobFailure(ctx context.Context, job *Job, workerID, errMsg string, log zerolog.Logger) {
	retryPolicy := RetryPolicy{
		BaseDelay: p.retryCfg.BaseDelay,
		MaxDelay:  p.retryCfg.MaxDelay,
	}

	if ShouldRetry(job.Attempts, job.MaxAttempts, p.retryCfg.DefaultMaxAttempts) {
		// Mark failed in DB so the state is consistent.
		failedJob, err := p.store.MarkJobFailed(ctx, job.ID, errMsg)
		if err != nil {
			log.Error().Err(err).Msg("failed to mark job as failed")
			return
		}

		// Re-enqueue with exponential backoff.
		failedJob.ScheduledAt = retryPolicy.ScheduledRetryAt(failedJob.Attempts)

		if err := p.broker.Enqueue(ctx, failedJob); err != nil {
			log.Error().Err(err).Msg("failed to re-enqueue job for retry")
			return
		}

		CounterJobsFailed.Add(1)
		p.publishEvent(EventJobFailed, failedJob, workerID)
		log.Info().
			Int("attempts", failedJob.Attempts).
			Time("retry_at", failedJob.ScheduledAt).
			Msg("job scheduled for retry")
		return
	}

	// Exhausted retries — move to DLQ.
	if err := p.store.MarkJobDead(ctx, job.ID, errMsg); err != nil {
		log.Error().Err(err).Msg("failed to mark job as dead")
		return
	}

	job.ErrorMessage = &errMsg
	if err := p.store.InsertDLQ(ctx, job); err != nil {
		log.Error().Err(err).Msg("failed to insert job into DLQ")
		return
	}

	CounterJobsDead.Add(1)
	p.publishEvent(EventJobDead, job, workerID)
	log.Warn().Str("job_id", job.ID.String()).Msg("job moved to dead-letter queue")
}

// ─── Heartbeat goroutine ──────────────────────────────────────────────────────

// runHeartbeat periodically updates the worker's last_seen timestamp and
// current_job_id in PostgreSQL until ctx is cancelled.
func (p *Pool) runHeartbeat(ctx context.Context, workerID string, currentJobID *atomic.Pointer[uuid.UUID]) {
	ticker := time.NewTicker(p.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobID := currentJobID.Load()

			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := p.store.UpdateWorkerHeartbeat(updateCtx, workerID, jobID)
			cancel()

			if err != nil {
				p.logger.Error().
					Str("worker_id", workerID).
					Err(err).
					Msg("heartbeat update failed")
			}

			if p.publisher != nil {
				p.publisher.Publish(Event{
					Type:     EventWorkerHeartbeat,
					WorkerID: workerID,
					Payload:  map[string]any{"status": "alive", "current_job_id": jobID},
				})
			}
		}
	}
}

// ─── Delayed promoter ─────────────────────────────────────────────────────────

// runPromoter periodically moves due delayed jobs into their active queues.
func (p *Pool) runPromoter(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			promoted, err := p.broker.PromoteDelayed(ctx)
			if err != nil {
				p.logger.Error().Err(err).Msg("delayed job promotion failed")
				continue
			}
			if promoted > 0 {
				p.logger.Info().Int64("count", promoted).Msg("promoted delayed jobs")
			}
		}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// publishEvent is a nil-safe wrapper around p.publisher.Publish.
func (p *Pool) publishEvent(eventType EventType, job *Job, workerID string) {
	if p.publisher == nil {
		return
	}
	p.publisher.Publish(Event{
		Type:     eventType,
		JobID:    job.ID.String(),
		JobType:  job.Type,
		WorkerID: workerID,
		Payload:  map[string]any{"job": job},
	})
}

// sleepOrCancel sleeps for the given duration or returns early when ctx is cancelled.
func (p *Pool) sleepOrCancel(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// buildWorkerID creates a unique worker identifier from the hostname and a
// worker index. Falls back to "unknown-host" if os.Hostname fails.
func buildWorkerID(index int) string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s-worker-%d", host, index)
}
