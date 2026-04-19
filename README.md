# JobQueue

[![CI](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml)
[![Docker](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml)

A production-grade distributed background job processing system. Enqueue tasks via HTTP, process them concurrently with a worker pool, monitor everything in real time, and scale across teams with built-in multi-tenancy.

**New to job queues?** Read the [complete beginner guide →](GUIDE.md)

---

## What It Does

When your web app needs to do something slow — send an email, resize an image, generate a report — you don't want the user waiting. JobQueue lets you hand that work off instantly and process it in the background, reliably, with retries, scheduling, and full observability.

```
Your app                    JobQueue
────────────────            ──────────────────────────────────
POST /api/v1/jobs ────────► Saved to PostgreSQL
                            Pushed into Redis queue
                ◄────────── Returns job ID (instant)

                            Worker picks up the job
                            Executes it
                            Retries on failure (exponential backoff)
                            Moves to DLQ after max retries
```

---

## Features

| Category | What's included |
|---|---|
| **Jobs** | Enqueue, batch (up to 500), get, list, cancel, retry, store result |
| **Scheduling** | `scheduled_at` for future execution, 5-field cron schedules |
| **Queues** | `default`, `critical`, `bulk` — priority-ordered |
| **Retries** | Exponential backoff, configurable `max_attempts` per job |
| **Dead-letter queue** | Failed jobs preserved forever, one-click requeue |
| **TTL** | Per-job auto-expiry, bulk purge endpoint, hourly background cleanup |
| **Tags** | Arbitrary `key:value` metadata, filterable on list endpoints |
| **Pagination** | Offset-based and cursor-based (stable across live inserts) |
| **Auth** | Register/login with JWT sessions + DB-backed API keys with tiers |
| **Multi-tenancy** | Each API key sees only its own jobs |
| **Admin mode** | `ADMIN_KEY` bypasses scoping for global visibility |
| **Rate limiting** | Per-key token bucket with `X-RateLimit-*` headers |
| **Usage metering** | Monthly job counter, 429 on limit, upgrade via Stripe |
| **Webhooks** | HMAC-signed HTTP callbacks on job lifecycle events |
| **Prometheus** | `/metrics` — counters, gauges, duration histograms |
| **WebSocket** | Real-time event stream to the dashboard |
| **Dashboard** | React SPA — jobs, workers, DLQ, cron, live events |
| **Billing** | Stripe Checkout integration for Pro/Business tier upgrades |
| **SDKs** | Go, Node.js (ESM + CJS), Python (sync + async) |
| **OpenAPI** | Full 3.1 spec at `openapi.yaml` |

---

## Quick Start

**Prerequisites:** Docker and Docker Compose v2.

```bash
git clone https://github.com/dhananjay6561/jobqueue
cd jobqueue
./run.sh
```

That's it. The script builds the image, starts Postgres + Redis + the app, and waits until everything is healthy.

Open **http://localhost:8080** — you'll be prompted to create an account or use the demo credentials (`demo@jobqueue.dev` / `demo1234`).

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       HTTP Clients                           │
│          (browser, curl, SDKs, internal services)            │
└───────────────────────────┬──────────────────────────────────┘
                            │  REST API  /  WebSocket
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                      Chi HTTP Router                         │
│  Auth middleware (JWT / API key / admin) → Rate limiter      │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────┐  │
│  │Job Handler │ │Cron Handler│ │Key Handler │ │WS Handler│  │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └────┬─────┘  │
└────────┼──────────────┼──────────────┼───────────────┼───────┘
         ▼              ▼              ▼               ▼
┌─────────────────┐                            ┌──────────────┐
│ PostgreSQL Store│                            │   WS Hub     │
│ (source of truth│                            │(broadcasts to│
│  + audit log)   │                            │ all clients) │
└────────┬────────┘                            └──────────────┘
         ▲
┌────────┴────────────────────────────────────────────────────┐
│                       Worker Pool                           │
│   Worker-0  Worker-1  Worker-2  ...                         │
└────────────────────────┬────────────────────────────────────┘
                         ▼
┌────────────────────────────────────────────────────────────┐
│                     Redis Broker                           │
│  queue:default   (sorted set — score = priority + sched)  │
│  queue:critical                                            │
│  queue:bulk                                                │
│  queue:delayed   (future-scheduled jobs)                   │
└────────────────────────────────────────────────────────────┘
```

---

## API Reference

### Authentication

All `/api/v1/*` endpoints require authentication. You can authenticate with either:

- **JWT session** (recommended for dashboard users): log in at `/auth/login`, then the frontend sends `Authorization: Bearer <token>` automatically.
- **API key** (recommended for programmatic access): pass `X-API-Key: <your-key>` on every request.

```bash
# Using API key
curl -H 'X-API-Key: qly_abc123...' http://localhost:8080/api/v1/jobs
```

### Response envelope

Every endpoint returns:
```json
{ "data": <result or null>, "error": <message or null>, "meta": { "request_id": "..." } }
```

List endpoints add pagination to `meta`:
```json
"meta": { "total_count": 42, "limit": 20, "offset": 0, "has_more": true }
```

---

### Auth Endpoints

```bash
# Register (creates account + free API key)
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email": "you@example.com", "password": "yourpassword"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "you@example.com", "password": "yourpassword"}'
```

---

### Jobs

```bash
# Enqueue a job
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{
    "type": "send_email",
    "payload": {"to": "user@example.com"},
    "priority": 8,
    "queue_name": "default",
    "max_attempts": 3,
    "ttl_seconds": 86400,
    "tags": {"env": "prod", "user_id": "42"}
  }'

# Batch enqueue (up to 500, single transaction)
curl -X POST http://localhost:8080/api/v1/jobs/batch \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '[
    {"type":"resize_image","payload":{"url":"..."}},
    {"type":"send_email","payload":{"to":"..."}}
  ]'

# Schedule for the future
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{"type":"send_notification","payload":{"user_id":42},"scheduled_at":"2026-12-01T09:00:00Z"}'

# List jobs (offset pagination)
curl -H 'X-API-Key: <key>' \
  "http://localhost:8080/api/v1/jobs?status=failed&limit=20&offset=0"

# List jobs (cursor pagination — stable across concurrent inserts)
curl -H 'X-API-Key: <key>' \
  "http://localhost:8080/api/v1/jobs/cursor?limit=20"
curl -H 'X-API-Key: <key>' \
  "http://localhost:8080/api/v1/jobs/cursor?cursor=<next_cursor>"

# Filter by tags
curl -H 'X-API-Key: <key>' \
  "http://localhost:8080/api/v1/jobs?tags=env:prod,user_id:42"

# Get a job
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/jobs/<id>

# Get stored result
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/jobs/<id>/result

# Cancel a pending job
curl -X DELETE -H 'X-API-Key: <key>' http://localhost:8080/api/v1/jobs/<id>

# Retry a failed job
curl -X POST -H 'X-API-Key: <key>' http://localhost:8080/api/v1/jobs/<id>/retry

# Bulk-delete terminal jobs older than a timestamp
curl -X DELETE -H 'X-API-Key: <key>' \
  "http://localhost:8080/api/v1/jobs?before=2026-01-01T00:00:00Z"
```

**Job fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | Handler name your workers are registered for |
| `payload` | object | no | Arbitrary JSON passed to the handler |
| `priority` | int (1–10) | no | Higher = dequeued first. Default: 5 |
| `queue_name` | string | no | `default`, `critical`, or `bulk`. Default: `default` |
| `max_attempts` | int | no | Retry ceiling. Default: 5 |
| `scheduled_at` | RFC3339 | no | Future execution time |
| `ttl_seconds` | int | no | Auto-delete N seconds after reaching terminal state |
| `tags` | object | no | `{"key": "value"}` metadata for filtering |

---

### Dead-Letter Queue

```bash
# List failed jobs
curl -H 'X-API-Key: <key>' "http://localhost:8080/api/v1/dlq?limit=20"

# Include already-requeued entries
curl -H 'X-API-Key: <key>' "http://localhost:8080/api/v1/dlq?include_requeued=true"

# Requeue a dead job (creates a fresh job, links back to this entry)
curl -X POST -H 'X-API-Key: <key>' http://localhost:8080/api/v1/dlq/<id>/requeue
```

---

### Cron Schedules

```bash
# Create a recurring schedule
curl -X POST http://localhost:8080/api/v1/cron \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{
    "name": "daily-cleanup",
    "job_type": "cleanup_storage",
    "cron_expression": "0 2 * * *",
    "payload": {"target": "tmp"},
    "queue_name": "bulk",
    "priority": 3
  }'

# Pause a schedule
curl -X PATCH http://localhost:8080/api/v1/cron/<id> \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{"enabled": false}'

# Change the schedule
curl -X PATCH http://localhost:8080/api/v1/cron/<id> \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{"cron_expression": "*/30 * * * *"}'

# List / delete
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/cron
curl -X DELETE -H 'X-API-Key: <key>' http://localhost:8080/api/v1/cron/<id>
```

**Cron expression reference:**

| Expression | Meaning |
|---|---|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour (on the hour) |
| `0 9 * * *` | Daily at 09:00 |
| `0 9 * * 1` | Every Monday at 09:00 |
| `*/15 * * * *` | Every 15 minutes |
| `0 2 1 * *` | 1st of every month at 02:00 |

---

### Webhooks

```bash
# Register a webhook endpoint
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: <key>' \
  -d '{
    "url": "https://yourapp.com/hooks/jobqueue",
    "secret": "your-signing-secret",
    "events": ["job.completed", "job.failed", "job.dead"]
  }'

# List / delete
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/webhooks
curl -X DELETE -H 'X-API-Key: <key>' http://localhost:8080/api/v1/webhooks/<id>
```

**Verify the signature in your receiver:**
```js
// Node.js
const sig = crypto.createHmac('sha256', 'your-signing-secret')
  .update(rawBody).digest('hex')
if (`sha256=${sig}` !== req.headers['x-webhook-signature']) {
  return res.status(401).send('Bad signature')
}
```

Supported events: `job.enqueued` `job.started` `job.completed` `job.failed` `job.dead`

---

### Stats, Workers, Health

```bash
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/stats    # job counts by status
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/workers  # worker pool status
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/usage    # monthly usage vs limit
curl http://localhost:8080/health                                  # postgres + redis health
curl http://localhost:8080/metrics                                 # Prometheus text format
```

---

### WebSocket

Connect to `ws://localhost:8080/ws` to receive real-time events:

```bash
websocat ws://localhost:8080/ws
```

| Event | Triggered when |
|---|---|
| `job.enqueued` | New job accepted |
| `job.started` | Worker claims a job |
| `job.completed` | Handler returns successfully |
| `job.failed` | Handler errors (retryable) |
| `job.dead` | All retries exhausted → DLQ |
| `worker.heartbeat` | Worker periodic ping (every 10s) |
| `stats.update` | Aggregate snapshot (every 5s) |

---

## SDKs

### Go

```go
import jobqueue "github.com/dhananjay6561/jobqueue/sdk/go"

client := jobqueue.New("http://localhost:8080", jobqueue.WithAPIKey("qly_..."))

// Enqueue
job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
    Type:    "send_email",
    Payload: map[string]any{"to": "user@example.com"},
    Priority: 8,
})

// Poll result
for job.Status != "completed" && job.Status != "dead" {
    time.Sleep(500 * time.Millisecond)
    job, _ = client.GetJob(ctx, job.ID.String())
}
result, _ := client.GetJobResult(ctx, job.ID.String())
```

### Node.js

```js
import { JobQueueClient } from '@jobqueue/client'

const jq = new JobQueueClient('http://localhost:8080', { apiKey: 'qly_...' })

// Single job
const job = await jq.enqueue({ type: 'send_email', payload: { to: 'user@example.com' } })

// Batch
const jobs = await jq.enqueueBatch([
  { type: 'resize_image', payload: { url: '...' } },
  { type: 'send_email',   payload: { to: '...'  } },
])

// Cursor-paginated list
let cursor = ''
do {
  const page = await jq.listJobsCursor({ status: 'completed', cursor, limit: 50 })
  console.log(page.items)
  cursor = page.next_cursor
} while (page.has_more)
```

### Python

```python
from jobqueue_client import JobQueueClient, AsyncJobQueueClient

# Sync
with JobQueueClient("http://localhost:8080", api_key="qly_...") as jq:
    job = jq.enqueue(type="send_email", payload={"to": "user@example.com"})
    batch = jq.enqueue_batch([
        {"type": "resize_image", "payload": {"url": "..."}},
        {"type": "send_email",   "payload": {"to": "..."}},
    ])
    stats = jq.get_stats()

# Async
async with AsyncJobQueueClient("http://localhost:8080", api_key="qly_...") as jq:
    job = await jq.enqueue(type="generate_report", payload={"id": 1})
```

---

## How Priority Works

Jobs are stored in Redis sorted sets where the score encodes both priority and schedule time:

```
score = -(priority × 1e15) + scheduledAt.UnixNano
```

The `1e15` multiplier means a priority-10 job always dequeues before a priority-1 job, no matter when it was scheduled (it dominates ~285 years of nanosecond timestamps). Within the same priority, earlier `scheduled_at` wins.

---

## How Retries Work

```
Handler returns error
        │
        ▼
attempts < max_attempts?
    ├── YES → status = failed, re-enqueue after backoff:
    │          delay = base_delay × 2^(attempt-1)  (capped at max_delay)
    │          e.g. 5s → 10s → 20s → 40s → 80s → ...
    │
    └── NO  → status = dead → inserted into dead_letter_jobs
                               WebSocket broadcasts job.dead
```

DLQ entries are preserved indefinitely unless `expires_at` is set. Requeue via `POST /api/v1/dlq/:id/requeue` — creates a fresh job with reset counters, marks the DLQ entry as `requeued`.

---

## Tiers and Billing

| Tier | Monthly jobs | Price |
|---|---|---|
| Free | 1,000 | $0 |
| Pro | 100,000 | $29/mo |
| Business | Unlimited | $99/mo |

Register at `/auth/register` to get a free tier API key. Upgrade via the Billing page in the dashboard (Stripe required — set `STRIPE_SECRET_KEY` and related env vars).

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | `change-me-in-production` | Signs user session tokens — **set this in prod** |
| `BASE_URL` | `http://localhost:8080` | Public URL — used for Stripe redirect URLs |
| `API_KEY` | `""` | Static API key (leave empty to use DB-backed keys) |
| `ADMIN_KEY` | `""` | Operator key that bypasses per-key scoping |
| `SERVER_HOST` | `0.0.0.0` | HTTP bind interface |
| `SERVER_PORT` | `8080` | HTTP listen port (`PORT` also accepted for PaaS) |
| `SERVER_READ_TIMEOUT` | `15s` | |
| `SERVER_WRITE_TIMEOUT` | `30s` | |
| `SERVER_IDLE_TIMEOUT` | `120s` | |
| `RATE_LIMIT_RPS` | `100` | Requests/sec per key (or per IP) |
| `RATE_LIMIT_BURST` | `20` | Token bucket burst size |
| `DATABASE_DSN` | **required** | PostgreSQL connection string |
| `DB_MAX_CONNS` | `20` | Max pool connections |
| `DB_MIN_CONNS` | `2` | Min idle connections |
| `DB_MAX_CONN_LIFETIME` | `30m` | |
| `DB_MAX_CONN_IDLE_TIME` | `5m` | |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `""` | Redis auth password |
| `REDIS_DB` | `0` | Redis database index |
| `REDIS_TLS` | `false` | `true` for Upstash/TLS-only Redis |
| `REDIS_DIAL_TIMEOUT` | `5s` | |
| `REDIS_READ_TIMEOUT` | `3s` | |
| `REDIS_WRITE_TIMEOUT` | `3s` | |
| `WORKER_COUNT` | `5` | Concurrent worker goroutines |
| `WORKER_HEARTBEAT_INTERVAL` | `10s` | |
| `WORKER_POLL_INTERVAL` | `1s` | Empty-queue poll interval |
| `WORKER_SHUTDOWN_TIMEOUT` | `30s` | Graceful drain deadline |
| `RETRY_BASE_DELAY` | `5s` | Initial backoff delay |
| `RETRY_MAX_DELAY` | `1h` | Backoff cap |
| `RETRY_DEFAULT_MAX_ATTEMPTS` | `5` | Global retry ceiling |
| `STRIPE_SECRET_KEY` | `""` | Stripe API key (billing optional) |
| `STRIPE_WEBHOOK_SECRET` | `""` | Stripe webhook signing secret |
| `STRIPE_PRO_PRICE_ID` | `""` | Stripe Price ID for Pro tier |
| `STRIPE_BUSINESS_PRICE_ID` | `""` | Stripe Price ID for Business tier |

---

## Local Development

```bash
# Start only the infrastructure (Postgres + Redis)
docker compose up postgres redis -d

# Run the server with race detector
make run-race

# Run tests
make test-race

# Coverage report
make test-cover

# Lint
make lint
```

---

## Project Structure

```
.
├── cmd/server/            # Main entry point — wires all components
├── internal/
│   ├── api/
│   │   ├── handler/       # HTTP handlers (jobs, cron, auth, billing, keys, workers, WS)
│   │   ├── middleware/    # Auth (JWT + API key), CORS, logger, rate limiter
│   │   ├── metrics.go     # Prometheus handler
│   │   └── router.go      # All route definitions
│   ├── config/            # Environment variable parsing
│   ├── queue/             # Redis broker, worker pool, retry logic, cron promoter, events
│   ├── store/             # PostgreSQL CRUD — jobs, DLQ, workers, webhooks, keys, cron, users
│   └── ws/                # WebSocket hub and client connection management
├── migrations/            # SQL migration files — run automatically at startup
├── frontend/              # React dashboard (Vite + Tailwind + Zustand + TanStack Query)
├── sdk/
│   ├── go/                # Go client SDK
│   ├── node/              # Node.js SDK (ESM + CJS + TypeScript types)
│   └── python/            # Python SDK (sync + async via httpx)
├── openapi.yaml           # OpenAPI 3.1 specification
├── render.yaml            # One-click Render deployment config
├── docker-compose.yml     # Local development stack
├── Dockerfile             # Multi-stage distroless production image
└── Makefile               # Dev shortcuts
```

---

## CI / CD

### CI (`ci.yml`)

| Job | What it runs |
|---|---|
| Lint | `golangci-lint` |
| Vet | `go vet ./...` |
| Test | `go test -race` + coverage artifact |
| Security | `govulncheck` + `gosec` SARIF upload |

### Docker (`docker.yml`)

Multi-stage distroless image pushed to GHCR on every push to `master` and on `v*` tags:

```bash
docker pull ghcr.io/dhananjay6561/jobqueue:master
```
