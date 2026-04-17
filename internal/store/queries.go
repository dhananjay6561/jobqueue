// Package store — queries.go contains all raw SQL statements used by the store
// layer. Centralising queries here makes it easy to review the full SQL surface
// of the application in one place and keeps handler/store code readable.
package store

// All SQL constants are named after the operation they perform.
// Positional parameters ($1, $2, …) map to the argument order in the calling
// function.

const (
	// queryInsertJob inserts a new job row and returns its persisted state.
	queryInsertJob = `
		INSERT INTO jobs (
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at
		) VALUES (
			$1, $2, $3, $4, 'pending', 0, $5, $6, $7, NOW()
		)
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message`

	// queryGetJobByID fetches a single job by its UUID primary key.
	queryGetJobByID = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message
		FROM jobs
		WHERE id = $1`

	// queryListJobs fetches a page of jobs with optional status, type, and
	// queue_name filters. Results are ordered by created_at DESC.
	// Parameters: $1=status (nullable), $2=type (nullable), $3=queue (nullable),
	//             $4=limit, $5=offset.
	queryListJobs = `
		SELECT
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message
		FROM jobs
		WHERE
			($1::job_status IS NULL OR status = $1::job_status)
			AND ($2::text IS NULL OR type = $2)
			AND ($3::text IS NULL OR queue_name = $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`

	// queryCountJobs returns the total count matching the same filters as
	// queryListJobs. Used to compute pagination metadata.
	queryCountJobs = `
		SELECT COUNT(*)
		FROM jobs
		WHERE
			($1::job_status IS NULL OR status = $1::job_status)
			AND ($2::text IS NULL OR type = $2)
			AND ($3::text IS NULL OR queue_name = $3)`

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
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message`

	// queryMarkJobCompleted transitions a running job to completed.
	queryMarkJobCompleted = `
		UPDATE jobs
		SET
			status       = 'completed',
			completed_at = NOW()
		WHERE id = $1
		RETURNING
			id, type, payload, priority, status, attempts, max_attempts,
			queue_name, scheduled_at, created_at, started_at, completed_at,
			worker_id, error_message`

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
			worker_id, error_message`

	// queryMarkJobDead transitions a failed job to dead (moves to DLQ table).
	queryMarkJobDead = `
		UPDATE jobs
		SET
			status        = 'dead',
			completed_at  = NOW(),
			error_message = $1
		WHERE id = $2`

	// queryCancelJob sets a pending job to failed (as a manual cancellation).
	queryCancelJob = `
		UPDATE jobs
		SET
			status        = 'failed',
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
			worker_id, error_message`

	// --- Dead-Letter Queue ---

	// queryInsertDLQ records a dead job in the dead_letter_jobs table.
	queryInsertDLQ = `
		INSERT INTO dead_letter_jobs (
			id, type, payload, priority, queue_name, max_attempts,
			original_created_at, last_error, total_attempts
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO NOTHING`

	// queryListDLQ fetches DLQ entries ordered by most recently dead.
	// Parameters: $1=include_requeued (bool), $2=limit, $3=offset.
	queryListDLQ = `
		SELECT
			id, type, payload, priority, queue_name, max_attempts,
			died_at, original_created_at, last_error, total_attempts,
			requeued, requeued_at, new_job_id
		FROM dead_letter_jobs
		WHERE ($1 = TRUE OR requeued = FALSE)
		ORDER BY died_at DESC
		LIMIT $2 OFFSET $3`

	// queryCountDLQ counts DLQ entries for pagination.
	queryCountDLQ = `
		SELECT COUNT(*) FROM dead_letter_jobs WHERE ($1 = TRUE OR requeued = FALSE)`

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
			requeued, requeued_at, new_job_id
		FROM dead_letter_jobs
		WHERE id = $1`

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
)
