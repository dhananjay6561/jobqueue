-- Cron schedules: recurring job templates enqueued automatically on a schedule.
CREATE TABLE IF NOT EXISTS cron_schedules (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL UNIQUE,
    job_type        TEXT        NOT NULL,
    payload         JSONB       NOT NULL DEFAULT '{}',
    queue_name      TEXT        NOT NULL DEFAULT 'default',
    priority        INT         NOT NULL DEFAULT 5,
    max_attempts    INT         NOT NULL DEFAULT 3,
    cron_expression TEXT        NOT NULL,
    enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cron_due
    ON cron_schedules (next_run_at)
    WHERE enabled = TRUE;
