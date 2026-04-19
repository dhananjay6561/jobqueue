BEGIN;

INSERT INTO users (email, password_hash)
VALUES ('demo@jobqueue.dev', '$2a$10$IxBNyQyFPIvC9w1VAFNjGeySN7W8alfoe.YqvDoIulXA.ziB1ug0u')
ON CONFLICT (email) DO NOTHING;

INSERT INTO api_keys (name, key_hash, key_prefix, tier, jobs_limit, user_id)
SELECT
    'default',
    encode(sha256('demo-api-key-jobqueue'::bytea), 'hex'),
    'demo-api',
    'free',
    1000,
    id
FROM users WHERE email = 'demo@jobqueue.dev'
ON CONFLICT DO NOTHING;

COMMIT;
