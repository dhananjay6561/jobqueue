-- Add 'cancelled' to job_status enum so manually cancelled jobs
-- are stored with a distinct status instead of reusing 'failed'.
-- NOTE: The UPDATE backfill must run in a separate transaction after
-- the enum value is committed, so it is intentionally omitted here.
DO $$ BEGIN
    ALTER TYPE job_status ADD VALUE IF NOT EXISTS 'cancelled';
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
