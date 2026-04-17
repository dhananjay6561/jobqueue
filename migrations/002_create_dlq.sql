-- Migration 002: Create the dead-letter queue (DLQ) table and workers registry.
--
-- dead_letter_jobs mirrors the jobs table but stores jobs that have exhausted
-- all retry attempts. They can be manually re-enqueued via the API.
--
-- workers stores the registry of active worker instances for observability.

BEGIN;

CREATE TABLE IF NOT EXISTS dead_letter_jobs (
    -- id matches the original job id for full traceability.
    id              UUID            PRIMARY KEY,
    type            TEXT            NOT NULL,
    payload         JSONB           NOT NULL DEFAULT '{}',
    priority        SMALLINT        NOT NULL DEFAULT 5,
    queue_name      TEXT            NOT NULL DEFAULT 'default',
    max_attempts    INTEGER         NOT NULL DEFAULT 5,

    -- died_at is when the job was moved to the DLQ.
    died_at         TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- original_created_at preserves when the job was first created.
    original_created_at TIMESTAMPTZ NOT NULL,

    -- last_error is the final error message from the last failed attempt.
    last_error      TEXT,

    -- total_attempts is the total number of processing attempts made.
    total_attempts  INTEGER         NOT NULL DEFAULT 0,

    -- requeued indicates whether this DLQ entry has been manually re-enqueued.
    requeued        BOOLEAN         NOT NULL DEFAULT FALSE,

    -- requeued_at is when the job was re-enqueued (NULL if never requeued).
    requeued_at     TIMESTAMPTZ,

    -- new_job_id is the id of the new job created when this entry was requeued.
    new_job_id      UUID
);

-- Index for listing DLQ jobs ordered by death time (most recent first).
CREATE INDEX IF NOT EXISTS idx_dlq_died_at ON dead_letter_jobs (died_at DESC);

-- Index for filtering non-requeued entries (the common case for the DLQ list).
CREATE INDEX IF NOT EXISTS idx_dlq_requeued ON dead_letter_jobs (requeued) WHERE requeued = FALSE;

-- workers table tracks active worker instances. Each worker row is updated
-- periodically via heartbeat; rows that have not been updated within
-- 3 × heartbeat_interval are considered stale/dead.
CREATE TABLE IF NOT EXISTS workers (
    -- id is set by the worker process itself (typically hostname + PID).
    id              TEXT            PRIMARY KEY,

    -- status is 'active' while the worker is running, 'stopped' after graceful shutdown.
    status          TEXT            NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'stopped')),

    -- jobs_processed is a running counter of successfully completed jobs.
    jobs_processed  BIGINT          NOT NULL DEFAULT 0,

    -- jobs_failed is a running counter of permanently failed jobs.
    jobs_failed     BIGINT          NOT NULL DEFAULT 0,

    -- current_job_id is the id of the job the worker is currently processing
    -- (NULL when idle).
    current_job_id  UUID,

    -- started_at is when this worker process started.
    started_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- last_seen is updated by the heartbeat goroutine every heartbeat_interval.
    last_seen       TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Index for the common query: "list all active workers".
CREATE INDEX IF NOT EXISTS idx_workers_status ON workers (status);

-- Index to efficiently find stale workers.
CREATE INDEX IF NOT EXISTS idx_workers_last_seen ON workers (last_seen DESC);

COMMIT;
