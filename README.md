# jobqueue — Distributed Job Queue System

[![CI](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml)
[![Docker](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml)

A production-grade distributed background job processing system written in Go.
Enqueue jobs via HTTP, process them concurrently with a worker pool, and monitor
everything in real time over WebSocket. Multi-tenant out of the box — every API
key sees only its own jobs.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       HTTP Clients                           │
│          (curl, dashboard, SDKs, internal services)          │
└───────────────────────────┬──────────────────────────────────┘
                            │  REST API  /  WebSocket
                            ▼
┌──────────────────────────────────────────────────────────────┐
│                      Chi HTTP Router                         │
│  Auth middleware (API key / admin) → Rate limiter (per key)  │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌──────────┐  │
│  │Job Handler │ │Cron Handler│ │Key Handler │ │WS Handler│  │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └────┬─────┘  │
└────────┼──────────────┼──────────────┼───────────────┼───────┘
         │              │              │               │
         ▼              ▼              ▼               ▼
┌─────────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐
│ PostgreSQL Store│ │Cron Store│ │Key Store │ │   WS Hub     │
│ (source of truth│ │          │ │          │ │(broadcasts to│
│  + audit log)   │ └──────────┘ └──────────┘ │ all clients) │
└────────┬────────┘                            └──────────────┘
         ▲
         │
┌────────┴────────────────────────────────────────────────────┐
│                       Worker Pool                           │
│   Worker-0  Worker-1  Worker-2  Worker-3  Worker-4          │
│                       │                                     │
│             Dequeue job ID (ZPOPMIN)                        │
└────────────────────────┼────────────────────────────────────┘
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

## Features

| Category | What's included |
|---|---|
| **Jobs** | Enqueue, batch enqueue (up to 500), get, list, cancel, retry, get result |
| **Scheduling** | `scheduled_at` for future execution, 5-field cron schedules |
| **Queues** | `default`, `critical`, `bulk` — priority-ordered via Redis scores |
| **Retries** | Exponential backoff per job, configurable `max_attempts` |
| **Dead-letter** | DLQ with manual or policy-based requeue |
| **TTL** | Per-job `expires_at` / `ttl_seconds`; bulk purge endpoint; hourly auto-purge |
| **Tags** | Arbitrary `map[string]string` metadata on jobs, filterable via `?tags=k:v` |
| **Pagination** | Offset-based + cursor-based (stable across live inserts) |
| **Auth** | DB-backed API keys with tiers (free/pro/business), static key, or open |
| **Multi-tenancy** | Jobs scoped to API key — each key sees only its own data |
| **Admin mode** | `ADMIN_KEY` bypasses scoping for operator-level visibility |
| **Rate limiting** | Per-key (or per-IP fallback) token bucket with `X-RateLimit-*` headers |
| **Usage metering** | Monthly job counter per key, 429 when limit hit, `GET /api/v1/usage` |
| **Webhooks** | HMAC-signed HTTP POST on job lifecycle events |
| **Prometheus** | `GET /metrics` — counters, gauges, `job_duration_seconds` histogram |
| **WebSocket** | Real-time event stream to all connected dashboard clients |
| **Dashboard** | React SPA — jobs, workers, DLQ, cron schedules, live events feed |
| **SDKs** | Go, Node.js (ESM + CJS), Python (sync + async) |
| **OpenAPI** | `openapi.yaml` — full 3.1 spec for every endpoint |

---

## Prerequisites

| Tool           | Version |
|----------------|---------|
| Docker         | 24+     |
| Docker Compose | v2      |
| Go (dev only)  | 1.22+   |

---

## Quick Start

```bash
git clone https://github.com/dhananjay6561/jobqueue && cd jobqueue
cp .env.example .env
docker compose up --build -d
curl http://localhost:8080/health
```

```json
{"status":"ok","checks":{"postgres":"healthy","redis":"healthy"},"uptime":"3s"}
```

Open **http://localhost:8080** for the dashboard.

---

## API Reference

All endpoints return:
```json
{ "data": <result|null>, "error": <message|null>, "meta": { "request_id": "..." } }
```

List endpoints include:
```json
"meta": { "total_count": 42, "limit": 20, "offset": 0, "has_more": true }
```

Authentication: pass `X-API-Key: <key>` on every request when auth is enabled.

---

### Jobs

```bash
# Enqueue
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "send_email",
    "payload": {"to": "user@example.com"},
    "priority": 8,
    "queue_name": "default",
    "max_attempts": 3,
    "ttl_seconds": 86400,
    "tags": {"env": "prod", "user_id": "42"}
  }'

# Batch enqueue (up to 500 jobs atomically)
curl -X POST http://localhost:8080/api/v1/jobs/batch \
  -H 'Content-Type: application/json' \
  -d '[{"type":"resize_image","payload":{"url":"..."}},{"type":"send_email","payload":{"to":"..."}}]'

# Schedule for the future
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{"type":"send_notification","payload":{"user_id":42},"scheduled_at":"2026-05-01T09:00:00Z"}'

# List (offset pagination)
curl "http://localhost:8080/api/v1/jobs?status=failed&limit=20&offset=0"

# List (cursor pagination — stable across concurrent inserts)
curl "http://localhost:8080/api/v1/jobs/cursor?limit=20"
curl "http://localhost:8080/api/v1/jobs/cursor?cursor=<next_cursor_from_previous_response>"

# Filter by tags
curl "http://localhost:8080/api/v1/jobs?tags=env:prod,user_id:42"

# Get / cancel / retry
curl http://localhost:8080/api/v1/jobs/<id>
curl -X DELETE http://localhost:8080/api/v1/jobs/<id>
curl -X POST http://localhost:8080/api/v1/jobs/<id>/retry

# Get stored result
curl http://localhost:8080/api/v1/jobs/<id>/result

# Bulk-delete terminal jobs older than a timestamp
curl -X DELETE "http://localhost:8080/api/v1/jobs?before=2026-01-01T00:00:00Z"
```

---

### Dead-Letter Queue

```bash
curl "http://localhost:8080/api/v1/dlq?limit=20"
curl "http://localhost:8080/api/v1/dlq?include_requeued=true"
curl -X POST http://localhost:8080/api/v1/dlq/<id>/requeue
```

---

### Cron Schedules

```bash
# Create
curl -X POST http://localhost:8080/api/v1/cron \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "daily-cleanup",
    "job_type": "cleanup_storage",
    "cron_expression": "0 2 * * *",
    "payload": {"target": "tmp"},
    "queue_name": "bulk",
    "priority": 3
  }'

# Enable / disable / update expression
curl -X PATCH http://localhost:8080/api/v1/cron/<id> \
  -H 'Content-Type: application/json' \
  -d '{"enabled": false}'

curl -X PATCH http://localhost:8080/api/v1/cron/<id> \
  -H 'Content-Type: application/json' \
  -d '{"cron_expression": "*/30 * * * *"}'

# List / delete
curl http://localhost:8080/api/v1/cron
curl -X DELETE http://localhost:8080/api/v1/cron/<id>
```

**Expression reference:**

| Expression | Meaning |
|---|---|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour |
| `0 9 * * *` | Daily at 09:00 |
| `0 9 * * 1` | Every Monday at 09:00 |
| `*/15 * * * *` | Every 15 minutes |
| `0 2 1 * *` | 1st of month at 02:00 |

---

### API Key Management

```bash
# Create a key (raw key returned once — store it)
curl -X POST http://localhost:8080/api/v1/keys \
  -H 'Content-Type: application/json' \
  -d '{"name": "my-service", "tier": "pro"}'

# List keys (prefixes only, never hashes)
curl http://localhost:8080/api/v1/keys

# Delete a key
curl -X DELETE http://localhost:8080/api/v1/keys/<id>

# Check your own usage
curl -H 'X-API-Key: <key>' http://localhost:8080/api/v1/usage
```

**Tiers:**

| Tier | Monthly limit |
|---|---|
| `free` | 1,000 jobs |
| `pro` | 100,000 jobs |
| `business` | Unlimited |

---

### Stats, Workers, Health, Metrics

```bash
curl http://localhost:8080/api/v1/stats    # scoped to calling API key
curl http://localhost:8080/api/v1/workers
curl http://localhost:8080/health
curl http://localhost:8080/metrics         # Prometheus text format
```

---

### WebSocket

```bash
websocat ws://localhost:8080/ws
```

| Event | Trigger |
|---|---|
| `job.enqueued` | New job accepted |
| `job.started` | Worker claims a job |
| `job.completed` | Handler returns nil |
| `job.failed` | Handler errors (retryable) |
| `job.dead` | All retries exhausted → DLQ |
| `worker.heartbeat` | Worker periodic ping |
| `stats.update` | Aggregate snapshot every 5s |

---

## SDKs

### Go

```go
import jobqueue "github.com/dhananjay6561/jobqueue/sdk/go"

client := jobqueue.New("http://localhost:8080", jobqueue.WithAPIKey("sk_..."))
job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
    Type:    "send_email",
    Payload: map[string]any{"to": "user@example.com"},
})
```

### Node.js

```js
import { JobQueueClient } from '@jobqueue/client'

const client = new JobQueueClient('http://localhost:8080', { apiKey: 'sk_...' })
const job = await client.enqueue({ type: 'send_email', payload: { to: 'user@example.com' } })
const batch = await client.enqueueBatch([
  { type: 'resize_image', payload: { url: '...' } },
  { type: 'send_email', payload: { to: '...' } },
])
```

### Python

```python
from jobqueue_client import JobQueueClient, AsyncJobQueueClient

# Sync
with JobQueueClient("http://localhost:8080", api_key="sk_...") as client:
    job = client.enqueue(type="send_email", payload={"to": "user@example.com"})
    stats = client.get_stats()

# Async
async with AsyncJobQueueClient("http://localhost:8080", api_key="sk_...") as client:
    job = await client.enqueue(type="send_email", payload={"to": "user@example.com"})
```

---

## How Priority Works

The Redis sorted-set score encodes both priority and schedule time:

```
score = -(priority × 1e15) + scheduledAt.UnixNano
```

The `1e15` multiplier means a higher-priority job always dequeues before a lower-priority one, regardless of when it was scheduled (dominates ~285 years of time difference). Within the same priority, earlier `scheduled_at` wins.

---

## How Retry + DLQ Works

```
Handler returns error
        │
        ▼
attempts < max_attempts?
    ├── YES → status = failed, re-enqueue with backoff:
    │          delay = base_delay × 2^(attempt-1), capped at max_delay
    │          e.g. 5s → 10s → 20s → 40s → 80s
    │
    └── NO  → status = dead, insert into dead_letter_jobs
               broadcast job.dead over WebSocket
```

DLQ entries are never auto-deleted (unless `expires_at` is set).
Requeue via `POST /api/v1/dlq/:id/requeue` — creates a fresh job, resets counters, links back to original entry.

---

## Admin Mode

Set `ADMIN_KEY` to a separate secret. Requests authenticated with this key bypass per-key data scoping and see all jobs globally — useful for operators and debugging.

```bash
ADMIN_KEY=ops-secret-key

curl -H 'X-API-Key: ops-secret-key' http://localhost:8080/api/v1/jobs   # all jobs
curl -H 'X-API-Key: ops-secret-key' http://localhost:8080/api/v1/stats  # global stats
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `API_KEY` | `""` | Static API key (leave empty for open access) |
| `ADMIN_KEY` | `""` | Operator key that bypasses per-key scoping |
| `SERVER_HOST` | `0.0.0.0` | HTTP bind interface |
| `SERVER_PORT` | `8080` | HTTP port (`PORT` also accepted for PaaS) |
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
| `REDIS_ADDR` | `localhost:6379` | |
| `REDIS_PASSWORD` | `""` | |
| `REDIS_DB` | `0` | |
| `REDIS_TLS` | `false` | Set `true` for Upstash / TLS-only Redis |
| `REDIS_DIAL_TIMEOUT` | `5s` | |
| `REDIS_READ_TIMEOUT` | `3s` | |
| `REDIS_WRITE_TIMEOUT` | `3s` | |
| `WORKER_COUNT` | `5` | Concurrent worker goroutines |
| `WORKER_HEARTBEAT_INTERVAL` | `10s` | |
| `WORKER_POLL_INTERVAL` | `1s` | Empty-queue backoff |
| `WORKER_SHUTDOWN_TIMEOUT` | `30s` | Graceful drain deadline |
| `RETRY_BASE_DELAY` | `5s` | Initial backoff |
| `RETRY_MAX_DELAY` | `1h` | Backoff cap |
| `RETRY_DEFAULT_MAX_ATTEMPTS` | `5` | Global default |

---

## CI / CD

### CI (`ci.yml`)

| Job | What |
|---|---|
| Lint | `golangci-lint` |
| Vet | `go vet` |
| Test | `go test -race` + coverage upload |
| Security | `govulncheck` + `gosec` SARIF |

### Docker (`docker.yml`)

Multi-stage distroless image pushed to GHCR on every push to `master` and on `v*` tags.

```bash
docker pull ghcr.io/dhananjay6561/jobqueue:master
```

---

## Project Structure

```
.
├── cmd/server/            # Entry point — wires all components
├── internal/
│   ├── api/
│   │   ├── handler/       # HTTP handlers (jobs, cron, keys, workers, WS)
│   │   ├── middleware/    # Auth, CORS, logger, rate limiter
│   │   ├── metrics.go     # Prometheus handler
│   │   └── router.go      # Route definitions
│   ├── config/            # Env var parsing
│   ├── queue/             # Broker (Redis), worker pool, retry, cron, events
│   ├── store/             # PostgreSQL CRUD (jobs, DLQ, workers, webhooks, keys, cron)
│   └── ws/                # WebSocket hub and client pumps
├── migrations/            # SQL migration files (001–011, run at startup)
├── frontend/              # React dashboard (Vite + Tailwind + Zustand + TanStack Query)
├── sdk/
│   ├── go/                # Go client SDK
│   ├── node/              # Node.js client SDK (ESM + CJS + types)
│   └── python/            # Python client SDK (sync + async, requires httpx)
├── openapi.yaml           # OpenAPI 3.1 specification
├── render.yaml            # One-click Render deployment
├── docker-compose.yml
├── Dockerfile
└── Makefile
```

---

## Local Development

```bash
# Start dependencies only
docker compose up postgres redis -d

cp .env.example .env
go mod download

make run-race      # run with race detector
make test-race     # full test suite with race detector
make test-cover    # coverage report
make lint          # golangci-lint
```
