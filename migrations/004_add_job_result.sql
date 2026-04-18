-- Add a result column so handlers can store structured output.
-- Consumers can then poll GET /api/v1/jobs/:id/result to retrieve it.
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS result JSONB DEFAULT NULL;
