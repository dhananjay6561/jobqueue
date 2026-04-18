-- Add a tags column to jobs for arbitrary key-value metadata.
-- Stored as jsonb so individual tag keys are indexable via GIN.
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '{}' NOT NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_tags ON jobs USING gin (tags);
