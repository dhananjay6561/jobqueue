package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// CronJobStore is the subset of store.DB the cron promoter needs.
type CronJobStore interface {
	ListDueCronSchedules(ctx context.Context, now time.Time) ([]*CronSchedule, error)
	UpdateCronRun(ctx context.Context, id uuid.UUID, lastRun, nextRun time.Time) error
	CreateJob(ctx context.Context, job *Job) (*Job, error)
}

// CronPromoter ticks on an interval, finds due cron schedules, and enqueues
// a job for each. It is safe to run concurrently with other promoters as long
// as UpdateCronRun is called atomically after each dispatch.
type CronPromoter struct {
	store    CronJobStore
	broker   Broker
	interval time.Duration
}

// NewCronPromoter creates a CronPromoter that ticks every interval.
func NewCronPromoter(store CronJobStore, broker Broker, interval time.Duration) *CronPromoter {
	return &CronPromoter{store: store, broker: broker, interval: interval}
}

// Run starts the promotion loop. It blocks until ctx is cancelled.
func (p *CronPromoter) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run once immediately on startup to catch any missed schedules.
	p.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *CronPromoter) tick(ctx context.Context) {
	now := time.Now().UTC()

	due, err := p.store.ListDueCronSchedules(ctx, now)
	if err != nil {
		log.Error().Err(err).Msg("cron promoter: list due schedules")
		return
	}

	for _, sched := range due {
		p.dispatch(ctx, sched, now)
	}
}

func (p *CronPromoter) dispatch(ctx context.Context, sched *CronSchedule, now time.Time) {
	// Build and persist the job.
	job := &Job{
		ID:          mustNewUUID(),
		Type:        sched.JobType,
		Payload:     sched.Payload,
		Priority:    sched.Priority,
		Status:      StatusPending,
		MaxAttempts: sched.MaxAttempts,
		QueueName:   sched.QueueName,
		ScheduledAt: now,
	}

	created, err := p.store.CreateJob(ctx, job)
	if err != nil {
		log.Error().Err(err).Str("cron", sched.Name).Msg("cron promoter: create job")
		return
	}

	// Enqueue into Redis.
	if err := p.broker.Enqueue(ctx, created); err != nil {
		log.Error().Err(err).Str("cron", sched.Name).Msg("cron promoter: broker enqueue")
		return
	}

	// Compute next run.
	expr, err := ParseCron(sched.CronExpression)
	if err != nil {
		log.Error().Err(err).Str("cron", sched.Name).Msg("cron promoter: parse expression")
		return
	}
	nextRun, err := expr.NextAfter(now)
	if err != nil {
		log.Error().Err(err).Str("cron", sched.Name).Msg("cron promoter: compute next run")
		return
	}

	if err := p.store.UpdateCronRun(ctx, sched.ID, now, nextRun); err != nil {
		log.Error().Err(err).Str("cron", sched.Name).Msg("cron promoter: update run timestamps")
		return
	}

	log.Info().
		Str("cron", sched.Name).
		Str("job_id", created.ID.String()).
		Time("next_run", nextRun).
		Msg("cron: dispatched job")
}

func mustNewUUID() uuid.UUID {
	id, err := uuid.NewRandom()
	if err != nil {
		panic("uuid: " + err.Error())
	}
	return id
}
