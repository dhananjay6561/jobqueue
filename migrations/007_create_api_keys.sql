-- API keys table: stores hashed keys with tier, usage counters, and limits.
-- The raw key is never stored — only a SHA-256 hash.
CREATE TYPE api_key_tier AS ENUM ('free', 'pro', 'business');

CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT            NOT NULL,
    key_hash    TEXT            NOT NULL UNIQUE,   -- SHA-256(raw_key)
    key_prefix  TEXT            NOT NULL,          -- first 8 chars for display
    tier        api_key_tier    NOT NULL DEFAULT 'free',
    jobs_used   BIGINT          NOT NULL DEFAULT 0,
    jobs_limit  BIGINT          NOT NULL DEFAULT 1000, -- -1 = unlimited
    reset_at    TIMESTAMPTZ     NOT NULL DEFAULT (date_trunc('month', NOW()) + INTERVAL '1 month'),
    enabled     BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys (key_hash);
