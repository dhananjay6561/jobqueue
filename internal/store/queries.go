// Package store — queries.go contains all raw SQL statements used by the store
// layer. Centralising queries here makes it easy to review the full SQL surface
// of the application in one place and keeps handler/store code readable.
package store

// All SQL constants are named after the operation they perform.
// Positional parameters ($1, $2, …) map to the argument order in the calling
// function.

const (
	// queryInsertJob inserts a new job row and returns its persisted state.
	// Parameters: $1=id, $2=type, $3=payload, $4=priority, $5=max_attempts,
	//             $6=queue_name, $7=scheduled_at, $8=api_key_id, $9=expires_at.
	queryInsertJob = `
		INSERT INTO jobs (
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, api_key_id, expires_at
		) VALUES (
			$1, $2, $3, $4, 'pending', 0, $5, $6, $7, NOW(), $8, $9
		)
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at`

	// queryGetJobByID fetches a single job by its UUID primary key.
	queryGetJobByID = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at
		FROM jobs
		WHERE id = $1`

	// queryGetJobResult fetches only the result column for a completed job.
	queryGetJobResult = `
		SELECT result FROM jobs WHERE id = $1`

	// queryListJobs fetches a page of jobs with optional status, type,
	// queue_name, and api_key_id filters. Results are ordered by created_at DESC.
	// Parameters: $1=status (nullable), $2=type (nullable), $3=queue (nullable),
	//             $4=api_key_id (nullable UUID), $5=limit, $6=offset.
	queryListJobs = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at
		FROM jobs
		WHERE
			($1::job_status IS NULL OR status = $1::job_status)
			AND ($2::text IS NULL OR type = $2)
			AND ($3::text IS NULL OR queue_name = $3)
			AND ($4::uuid IS NULL OR api_key_id = $4::uuid)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6`

	// queryCountJobs returns the total count matching the same filters as
	// queryListJobs. Used to compute pagination metadata.
	queryCountJobs = `
		SELECT COUNT(*)
		FROM jobs
		WHERE
			($1::job_status IS NULL OR status = $1::job_status)
			AND ($2::text IS NULL OR type = $2)
			AND ($3::text IS NULL OR queue_name = $3)
			AND ($4::uuid IS NULL OR api_key_id = $4::uuid)`

	// queryUpdateJobStatus transitions a job to a new status and records
	// started_at / completed_at / error_message as appropriate.
	// Parameters: $1=new_status, $2=started_at, $3=completed_at,
	//             $4=error_message, $5=worker_id, $6=attempts increment, $7=id.
	queryUpdateJobStarted = `
		UPDATE jobs
		SET
			status     = 'running',
			started_at = NOW(),
			worker_id  = $1,
			attempts   = attempts + 1
		WHERE id = $2
		  AND status = 'pending'
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at`

	// queryMarkJobCompleted transitions a running job to completed and stores result.
	queryMarkJobCompleted = `
		UPDATE jobs
		SET
			status       = 'completed',
			completed_at = NOW(),
			result       = $2
		WHERE id = $1
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at`

	// queryMarkJobFailed transitions a running job to failed and records the error.
	queryMarkJobFailed = `
		UPDATE jobs
		SET
			status        = 'failed',
			completed_at  = NOW(),
			error_message = $1
		WHERE id = $2
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at`

	// queryMarkJobDead transitions a failed job to dead (moves to DLQ table).
	queryMarkJobDead = `
		UPDATE jobs
		SET
			status        = 'dead',
			completed_at  = NOW(),
			error_message = $1
		WHERE id = $2`

	// queryCancelJob sets a pending job to cancelled.
	queryCancelJob = `
		UPDATE jobs
		SET
			status        = 'cancelled',
			completed_at  = NOW(),
			error_message = 'cancelled by user'
		WHERE id = $1 AND status = 'pending'
		RETURNING id`

	// queryResetJobForRetry sets a failed job back to pending so it can be
	// re-enqueued. Used by the manual retry endpoint.
	queryResetJobForRetry = `
		UPDATE jobs
		SET
			status        = 'pending',
			error_message = NULL,
			completed_at  = NULL
		WHERE id = $1 AND status IN ('failed', 'dead')
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at`

	// --- Dead-Letter Queue ---

	// queryInsertDLQ records a dead job in the dead_letter_jobs table.
	// Parameters: $1...$9 as before, $10=api_key_id, $11=expires_at.
	queryInsertDLQ = `
		INSERT INTO dead_letter_jobs (
			id, type, payload, priority, queue_name, max_attempts,
			original_created_at, last_error, total_attempts, api_key_id, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO NOTHING`

	// queryListDLQ fetches DLQ entries ordered by most recently dead.
	// Parameters: $1=include_requeued (bool), $2=api_key_id (nullable UUID),
	//             $3=limit, $4=offset.
	queryListDLQ = `
		SELECT
			id, type, payload, priority, queue_name, max_attempts,
			died_at, original_created_at, last_error, total_attempts,
			requeued, requeued_at, new_job_id, api_key_id, expires_at
		FROM dead_letter_jobs
		WHERE ($1 = TRUE OR requeued = FALSE)
		  AND ($2::uuid IS NULL OR api_key_id = $2::uuid)
		ORDER BY died_at DESC
		LIMIT $3 OFFSET $4`

	// queryCountDLQ counts DLQ entries for pagination.
	// Parameters: $1=include_requeued (bool), $2=api_key_id (nullable UUID).
	queryCountDLQ = `
		SELECT COUNT(*) FROM dead_letter_jobs
		WHERE ($1 = TRUE OR requeued = FALSE)
		  AND ($2::uuid IS NULL OR api_key_id = $2::uuid)`

	// queryMarkDLQRequeued marks a DLQ entry as requeued when a new job is created.
	queryMarkDLQRequeued = `
		UPDATE dead_letter_jobs
		SET requeued = TRUE, requeued_at = NOW(), new_job_id = $1
		WHERE id = $2`

	// queryGetDLQEntry fetches a single DLQ entry by id.
	queryGetDLQEntry = `
		SELECT
			id, type, payload, priority, queue_name, max_attempts,
			died_at, original_created_at, last_error, total_attempts,
			requeued, requeued_at, new_job_id, api_key_id, expires_at
		FROM dead_letter_jobs
		WHERE id = $1`

	// queryPurgeExpiredJobs deletes terminal jobs whose expires_at has passed.
	// Returns the count of deleted rows.
	queryPurgeExpiredJobs = `
		DELETE FROM jobs
		WHERE expires_at IS NOT NULL
		  AND expires_at <= NOW()
		  AND status IN ('completed', 'failed', 'dead', 'cancelled')`

	// queryPurgeExpiredDLQ deletes DLQ entries whose expires_at has passed.
	queryPurgeExpiredDLQ = `
		DELETE FROM dead_letter_jobs
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()`

	// queryPurgeJobsBefore bulk-deletes terminal jobs older than a given time.
	// Used by DELETE /api/v1/jobs?before=<timestamp>.
	// Parameters: $1=before (timestamptz), $2=api_key_id (nullable UUID).
	queryPurgeJobsBefore = `
		DELETE FROM jobs
		WHERE created_at < $1
		  AND status IN ('completed', 'failed', 'dead', 'cancelled')
		  AND ($2::uuid IS NULL OR api_key_id = $2::uuid)`

	// --- Workers ---

	// queryUpsertWorker inserts or updates the worker heartbeat row.
	queryUpsertWorker = `
		INSERT INTO workers (id, status, started_at, last_seen)
		VALUES ($1, 'active', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET status    = 'active',
		    last_seen = NOW()`

	// queryUpdateWorkerHeartbeat updates the heartbeat timestamp and current job.
	queryUpdateWorkerHeartbeat = `
		UPDATE workers
		SET last_seen      = NOW(),
		    current_job_id = $1
		WHERE id = $2`

	// queryUpdateWorkerStats increments the processed/failed counters.
	queryUpdateWorkerStats = `
		UPDATE workers
		SET jobs_processed  = jobs_processed + $1,
		    jobs_failed     = jobs_failed + $2,
		    current_job_id  = NULL
		WHERE id = $3`

	// queryMarkWorkerStopped sets a worker status to stopped on graceful shutdown.
	queryMarkWorkerStopped = `
		UPDATE workers
		SET status = 'stopped', last_seen = NOW()
		WHERE id = $1`

	// queryListWorkers fetches all workers (optionally filtered to active only).
	queryListWorkers = `
		SELECT id, status, jobs_processed, jobs_failed, current_job_id,
		       started_at, last_seen
		FROM workers
		WHERE ($1 = FALSE OR status = 'active')
		ORDER BY started_at DESC`

	// --- Stats ---

	// queryJobStats returns the count of jobs in each status.
	queryJobStats = `
		SELECT status, COUNT(*) as count
		FROM jobs
		GROUP BY status`

	// queryJobsPerMinute returns jobs completed in the last 60 seconds.
	queryJobsPerMinute = `
		SELECT COUNT(*) FROM jobs
		WHERE status = 'completed' AND completed_at >= NOW() - INTERVAL '1 minute'`

	// queryDLQCount returns the total number of un-requeued DLQ entries.
	queryDLQCount = `
		SELECT COUNT(*) FROM dead_letter_jobs WHERE requeued = FALSE`

	// queryActiveWorkerCount returns the number of currently active workers.
	queryActiveWorkerCount = `
		SELECT COUNT(*) FROM workers WHERE status = 'active'`

	// --- Webhooks ---

	queryInsertWebhook = `
		INSERT INTO webhooks (url, secret, events, enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING id, url, secret, events, enabled, created_at, updated_at`

	queryListWebhooks = `
		SELECT id, url, secret, events, enabled, created_at, updated_at
		FROM webhooks ORDER BY created_at DESC`

	queryListEnabledWebhooks = `
		SELECT id, url, secret, events, enabled, created_at, updated_at
		FROM webhooks WHERE enabled = TRUE`

	queryDeleteWebhook = `
		DELETE FROM webhooks WHERE id = $1`

	// --- Cron schedules ---

	queryInsertCron = `
		INSERT INTO cron_schedules
			(name, job_type, payload, queue_name, priority, max_attempts, cron_expression, enabled, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, job_type, payload, queue_name, priority, max_attempts,
		          cron_expression, enabled, last_run_at, next_run_at, created_at`

	queryListCron = `
		SELECT id, name, job_type, payload, queue_name, priority, max_attempts,
		       cron_expression, enabled, last_run_at, next_run_at, created_at
		FROM cron_schedules ORDER BY created_at DESC`

	queryListDueCron = `
		SELECT id, name, job_type, payload, queue_name, priority, max_attempts,
		       cron_expression, enabled, last_run_at, next_run_at, created_at
		FROM cron_schedules
		WHERE enabled = TRUE AND next_run_at <= $1
		ORDER BY next_run_at ASC`

	queryUpdateCronRun = `
		UPDATE cron_schedules
		SET last_run_at = $1, next_run_at = $2
		WHERE id = $3`

	queryDeleteCron = `
		DELETE FROM cron_schedules WHERE id = $1`

	// queryPatchCron updates only the provided fields (enabled, cron_expression,
	// payload). NULL parameters leave the column unchanged.
	// Parameters: $1=enabled (nullable bool), $2=cron_expression (nullable text),
	//             $3=payload (nullable jsonb), $4=next_run_at (nullable timestamptz),
	//             $5=id.
	queryPatchCron = `
		UPDATE cron_schedules
		SET
			enabled         = COALESCE($1, enabled),
			cron_expression = COALESCE($2, cron_expression),
			payload         = COALESCE($3, payload),
			next_run_at     = COALESCE($4, next_run_at)
		WHERE id = $5
		RETURNING id, name, job_type, payload, queue_name, priority, max_attempts,
		          cron_expression, enabled, last_run_at, next_run_at, created_at`

	// queryListJobsCursor is a keyset-pagination variant of queryListJobs.
	// The cursor is an opaque (created_at, id) pair encoded as two params.
	// Parameters: $1=status, $2=type, $3=queue, $4=api_key_id,
	//             $5=cursor_created_at (nullable timestamptz),
	//             $6=cursor_id (nullable uuid), $7=limit.
	queryListJobsCursor = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at
		FROM jobs
		WHERE
			($1::job_status IS NULL OR status = $1::job_status)
			AND ($2::text IS NULL OR type = $2)
			AND ($3::text IS NULL OR queue_name = $3)
			AND ($4::uuid IS NULL OR api_key_id = $4::uuid)
			AND ($5::timestamptz IS NULL OR (created_at, id) < ($5::timestamptz, $6::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $7`

	// queryGetJobsByIDs fetches multiple jobs by their UUIDs (used after batch insert).
	queryGetJobsByIDs = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message, result, api_key_id, expires_at
		FROM jobs
		WHERE id = ANY($1)
		ORDER BY created_at ASC`

	// --- API keys ---

	queryInsertAPIKey = `
		INSERT INTO api_keys (name, key_hash, key_prefix, tier, jobs_limit)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, key_prefix, tier, jobs_used, jobs_limit, reset_at, enabled, created_at`

	queryGetAPIKeyByHash = `
		SELECT id, name, key_prefix, tier, jobs_used, jobs_limit, reset_at, enabled, created_at
		FROM api_keys WHERE key_hash = $1`

	queryListAPIKeys = `
		SELECT id, name, key_prefix, tier, jobs_used, jobs_limit, reset_at, enabled, created_at
		FROM api_keys ORDER BY created_at DESC`

	// Atomically increment jobs_used and return the updated row.
	// Resets counter if reset_at has passed before incrementing.
	queryIncrementAPIKeyUsage = `
		UPDATE api_keys
		SET
			jobs_used = CASE WHEN NOW() >= reset_at THEN 1 ELSE jobs_used + 1 END,
			reset_at  = CASE WHEN NOW() >= reset_at
			                 THEN date_trunc('month', NOW()) + INTERVAL '1 month'
			                 ELSE reset_at END
		WHERE key_hash = $1
		RETURNING id, name, key_prefix, tier, jobs_used, jobs_limit, reset_at, enabled, created_at`

	queryDeleteAPIKey = `
		DELETE FROM api_keys WHERE id = $1`
)
