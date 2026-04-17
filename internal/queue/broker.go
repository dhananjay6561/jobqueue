// Package queue — broker.go implements the Redis-backed job broker.
//
// Architecture overview:
//   - Each queue is a Redis sorted set keyed as "queue:{name}".
//   - The score is computed as:  -(priority * 1e12) + unix_nano_scheduled_at
//     This means higher-priority jobs sort first (lowest score wins in ZPOPMIN)
//     and, within the same priority, jobs scheduled earlier are dequeued first.
//   - A separate sorted set "queue:delayed" holds jobs whose scheduled_at is in
//     the future; a background promoter moves them to the active queue when due.
//   - Job IDs (UUIDs) are the members in every sorted set.
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisQueuePrefix is the key prefix for all active queue sorted sets.
const redisQueuePrefix = "queue:"

// redisDelayedKey is the sorted set holding future-scheduled jobs.
// Score = Unix timestamp (seconds) at which the job becomes eligible.
const redisDelayedKey = "queue:delayed"

// redisJobIDKey returns the sorted-set key for the given queue name.
func redisJobIDKey(queueName string) string {
	return redisQueuePrefix + queueName
}

// priorityScore computes the Redis sorted-set score for a job.
// Lower score = dequeued first by ZPOPMIN.
//
//	score = -(priority × 1_000_000_000_000_000) + scheduledAt.UnixNano
//
// The 1e15 multiplier ensures the priority band (up to 10×1e15 = 1e16 ns ≈ 285 years)
// dominates any realistic scheduled_at difference, so a higher-priority job
// always sorts before a lower-priority job regardless of schedule time.
// Within the same priority, earlier scheduled_at yields a lower score.
func priorityScore(priority int, scheduledAt time.Time) float64 {
	return float64(-priority)*1e15 + float64(scheduledAt.UnixNano())
}

// Broker defines the interface for the Redis-backed queue operations.
// All methods accept a context for deadline/cancellation propagation.
type Broker interface {
	// Enqueue adds a job id to the appropriate queue (or delayed set).
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue atomically pops the highest-priority eligible job id from the
	// given queue. Returns ("", nil) when the queue is empty.
	Dequeue(ctx context.Context, queueName string) (string, error)

	// Remove removes a job id from the queue (used when cancelling a pending job).
	Remove(ctx context.Context, queueName, jobID string) error

	// PromoteDelayed moves jobs whose scheduled_at has passed from the delayed
	// set into the active queue.
	PromoteDelayed(ctx context.Context) (int64, error)

	// QueueDepth returns the number of items in the named active queue.
	QueueDepth(ctx context.Context, queueName string) (int64, error)

	// DelayedDepth returns the number of items waiting in the delayed set.
	DelayedDepth(ctx context.Context) (int64, error)

	// Ping verifies connectivity to Redis.
	Ping(ctx context.Context) error
}

// RedisBroker implements Broker using Redis sorted sets via go-redis.
type RedisBroker struct {
	client *redis.Client
}

// Compile-time proof that *RedisBroker satisfies Broker.
var _ Broker = (*RedisBroker)(nil)

// NewRedisBroker creates a RedisBroker and verifies the connection.
func NewRedisBroker(ctx context.Context, client *redis.Client) (*RedisBroker, error) {
	b := &RedisBroker{client: client}
	if err := b.Ping(ctx); err != nil {
		return nil, fmt.Errorf("redis broker ping: %w", err)
	}
	return b, nil
}

// Enqueue adds the job's ID to the Redis sorted set for its queue.
// If scheduled_at is in the future the job goes to the delayed set;
// otherwise it goes directly to the active queue.
func (b *RedisBroker) Enqueue(ctx context.Context, job *Job) error {
	now := time.Now()

	if job.ScheduledAt.After(now) {
		// Encode routing metadata into the member so PromoteDelayed can reconstruct
		// the queue name, priority, and scheduled time without a DB round-trip.
		// Format: "jobID|queueName|priority|scheduledNano"
		member := fmt.Sprintf("%s|%s|%d|%d",
			job.ID.String(),
			job.QueueName,
			job.Priority,
			job.ScheduledAt.UnixNano(),
		)
		err := b.client.ZAdd(ctx, redisDelayedKey, redis.Z{
			Score:  float64(job.ScheduledAt.Unix()),
			Member: member,
		}).Err()
		if err != nil {
			return fmt.Errorf("enqueue delayed job %s: %w", job.ID, err)
		}
		return nil
	}

	key := redisJobIDKey(job.QueueName)
	score := priorityScore(job.Priority, job.ScheduledAt)

	err := b.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: job.ID.String(),
	}).Err()
	if err != nil {
		return fmt.Errorf("enqueue job %s into %s: %w", job.ID, key, err)
	}
	return nil
}

// Dequeue atomically pops the member with the lowest score (highest priority,
// earliest scheduled_at) from the named queue. Returns an empty string and nil
// error when the queue is empty.
func (b *RedisBroker) Dequeue(ctx context.Context, queueName string) (string, error) {
	key := redisJobIDKey(queueName)

	results, err := b.client.ZPopMin(ctx, key, 1).Result()
	if err != nil {
		return "", fmt.Errorf("dequeue from %s: %w", key, err)
	}
	if len(results) == 0 {
		return "", nil
	}

	jobID, ok := results[0].Member.(string)
	if !ok {
		return "", fmt.Errorf("unexpected member type in queue %s", key)
	}

	return jobID, nil
}

// Remove deletes a specific job id from the named active queue.
// Used when a pending job is cancelled via the API.
func (b *RedisBroker) Remove(ctx context.Context, queueName, jobID string) error {
	key := redisJobIDKey(queueName)
	if err := b.client.ZRem(ctx, key, jobID).Err(); err != nil {
		return fmt.Errorf("remove job %s from %s: %w", jobID, key, err)
	}
	return nil
}

// PromoteDelayed scans the delayed sorted set for jobs whose scheduled_at has
// passed, removes them from the delayed set, and adds them to their active queue.
// It returns the number of jobs promoted.
func (b *RedisBroker) PromoteDelayed(ctx context.Context) (int64, error) {
	now := float64(time.Now().Unix())

	// Atomically get all eligible entries using a Lua script to avoid races.
	promoteScript := redis.NewScript(`
		local delayed = KEYS[1]
		local now     = tonumber(ARGV[1])

		-- Fetch all members whose score (scheduled_at unix) <= now
		local entries = redis.call("ZRANGEBYSCORE", delayed, "-inf", now, "WITHSCORES")
		if #entries == 0 then
			return 0
		end

		local count = 0
		for i = 1, #entries, 2 do
			-- entries[i]   = job_id:queueName encoded as "jobUUID|queueName|priority|scheduledNano"
			-- We stored the routing data in the member string to avoid a DB lookup here.
			local member = entries[i]
			local parts  = {}
			for part in string.gmatch(member, "([^|]+)") do
				table.insert(parts, part)
			end
			-- parts: {jobID, queueName, priority, scheduledNano}
			local jobID       = parts[1]
			local queueName   = parts[2] or "default"
			local priority    = tonumber(parts[3]) or 5
			local scheduledNs = tonumber(parts[4]) or 0

			-- Score formula mirrors Go's priorityScore()
			local score = (-priority * 1e15) + scheduledNs

			redis.call("ZADD", "queue:" .. queueName, score, jobID)
			redis.call("ZREM", delayed, member)
			count = count + 1
		end
		return count
	`)

	result, err := promoteScript.Run(ctx, b.client, []string{redisDelayedKey}, now).Int64()
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("promote delayed jobs: %w", err)
	}
	return result, nil
}

// QueueDepth returns the number of items currently in the named active queue.
func (b *RedisBroker) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	n, err := b.client.ZCard(ctx, redisJobIDKey(queueName)).Result()
	if err != nil {
		return 0, fmt.Errorf("queue depth for %s: %w", queueName, err)
	}
	return n, nil
}

// DelayedDepth returns the number of items in the delayed set.
func (b *RedisBroker) DelayedDepth(ctx context.Context) (int64, error) {
	n, err := b.client.ZCard(ctx, redisDelayedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("delayed queue depth: %w", err)
	}
	return n, nil
}

// Ping verifies the Redis connection is alive.
func (b *RedisBroker) Ping(ctx context.Context) error {
	if err := b.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}
