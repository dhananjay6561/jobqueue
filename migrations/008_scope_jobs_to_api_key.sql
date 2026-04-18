-- Scope jobs to API keys so each customer only sees their own data.
-- Nullable so existing jobs and keyless (local/dev) mode still work.
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS api_key_id UUID DEFAULT NULL
        REFERENCES api_keys(id) ON DELETE SET NULL;

ALTER TABLE dead_letter_jobs
    ADD COLUMN IF NOT EXISTS api_key_id UUID DEFAULT NULL
        REFERENCES api_keys(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_api_key_id
    ON jobs (api_key_id) WHERE api_key_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_dlq_api_key_id
    ON dead_letter_jobs (api_key_id) WHERE api_key_id IS NOT NULL;
