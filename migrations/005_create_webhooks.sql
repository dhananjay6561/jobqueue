-- Webhooks table: stores registered HTTP endpoints that receive job event POSTs.
CREATE TABLE IF NOT EXISTS webhooks (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL DEFAULT '',
    events      TEXT[]      NOT NULL DEFAULT ARRAY['job.completed','job.failed','job.dead'],
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_enabled ON webhooks (enabled);
