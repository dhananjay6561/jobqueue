-- Add TTL support: track when a job expires and provide a bulk-delete function.
-- expires_at is nullable; NULL means the job never auto-expires.
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ DEFAULT NULL;

ALTER TABLE dead_letter_jobs
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ DEFAULT NULL;

-- Index so the cleanup query can find expired rows cheaply.
CREATE INDEX IF NOT EXISTS idx_jobs_expires_at
    ON jobs (expires_at) WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_dlq_expires_at
    ON dead_letter_jobs (expires_at) WHERE expires_at IS NOT NULL;
