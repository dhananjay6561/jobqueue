-- Composite index to support cursor-based pagination on jobs.
-- Queries page forward using (created_at DESC, id DESC) so results are
-- stable even as new rows are inserted between pages.
CREATE INDEX IF NOT EXISTS idx_jobs_created_at_id
    ON jobs (created_at DESC, id DESC);
