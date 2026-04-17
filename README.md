# jobqueue — Distributed Job Queue System

[![CI](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/ci.yml)
[![Docker](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml/badge.svg)](https://github.com/dhananjay6561/jobqueue/actions/workflows/docker.yml)

A production-grade distributed background job processing system written in Go.
Enqueue jobs via HTTP, process them concurrently with a worker pool, and monitor
everything in real time over WebSocket.

Inspired by how Shopify, GitHub, and Stripe handle async workloads internally.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          HTTP Clients                           │
│               (curl, dashboard, internal services)              │
└────────────────────────────┬────────────────────────────────────┘
                             │  REST API  /  WebSocket
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Chi HTTP Router                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  Job Handler │  │Worker Handler│  │  WebSocket Handler   │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
└─────────┼─────────────────┼─────────────────────┼──────────────┘
          │                 │                      │
          ▼                 ▼                      ▼
┌──────────────────┐  ┌──────────────┐  ┌──────────────────────┐
│  PostgreSQL Store│  │  store.DB    │  │      WS Hub          │
│  (source of      │  │  (workers,   │  │  (broadcasts events  │
│   truth / audit) │  │   stats)     │  │   to all clients)    │
└──────────────────┘  └──────────────┘  └──────────┬───────────┘
          ▲                                         │
          │                                         │ Events
┌─────────┴───────────────────────────────────────────────────┐
│                     Worker Pool                             │
│   Worker-0   Worker-1   Worker-2   Worker-3   Worker-4      │
│      │           │          │          │          │         │
│      └───────────┴──────────┴──────────┴──────────┘         │
│                            │                                │
│                  Dequeue job ID (ZPopMin)                   │
│                            │                                │
└────────────────────────────┼────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                        Redis Broker                         │
│  queue:default  (sorted set, score = priority + sched_at)  │
│  queue:critical                                             │
│  queue:bulk                                                 │
│  queue:delayed  (jobs scheduled in the future)             │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

| Tool            | Version   |
|-----------------|-----------|
| Docker          | 24+       |
| Docker Compose  | v2        |
| Go (for dev)    | 1.26.2+   |

---

## Quick Start

```bash
# 1. Clone and enter the project
git clone https://github.com/dhananjay6561/jobqueue && cd jobqueue

# 2. Copy the environment template
cp .env.example .env

# 3. Start everything (Postgres + Redis + App)
docker compose up --build -d

# 4. Verify the server is healthy
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok","checks":{"postgres":"healthy","redis":"healthy"},"uptime":"3s"}
```

---

## API Reference

All endpoints return:
```json
{ "data": <result|null>, "error": <message|null>, "meta": { "request_id": "..." } }
```

List endpoints additionally include:
```json
"meta": { "total_count": 42, "limit": 20, "offset": 0, "has_more": true }
```

### Enqueue a job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "send_email",
    "payload": {"to": "user@example.com", "subject": "Hello"},
    "priority": 8,
    "max_attempts": 3,
    "queue_name": "default"
  }'
```

### Schedule a job for the future

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "send_notification",
    "payload": {"user_id": 42},
    "priority": 5,
    "scheduled_at": "2026-04-18T09:00:00Z"
  }'
```

### List jobs

```bash
# All jobs (paginated)
curl "http://localhost:8080/api/v1/jobs?limit=10&offset=0"

# Filter by status
curl "http://localhost:8080/api/v1/jobs?status=failed"

# Filter by type and queue
curl "http://localhost:8080/api/v1/jobs?type=send_email&queue=default"
```

### Get a single job

```bash
curl http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000
```

### Cancel a pending job

```bash
curl -X DELETE http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000
```

### Manually retry a failed job

```bash
curl -X POST http://localhost:8080/api/v1/jobs/550e8400-e29b-41d4-a716-446655440000/retry
```

### List the dead-letter queue

```bash
curl "http://localhost:8080/api/v1/dlq?limit=20"

# Include already-requeued entries
curl "http://localhost:8080/api/v1/dlq?include_requeued=true"
```

### Requeue a DLQ entry

```bash
curl -X POST http://localhost:8080/api/v1/dlq/550e8400-e29b-41d4-a716-446655440000/requeue
```

### List workers

```bash
# Active workers only (default)
curl http://localhost:8080/api/v1/workers

# Include stopped workers
curl "http://localhost:8080/api/v1/workers?active_only=false"
```

### Queue stats

```bash
curl http://localhost:8080/api/v1/stats
```

### Health check

```bash
curl http://localhost:8080/health
```

### Metrics

```bash
curl http://localhost:8080/metrics
```

### WebSocket (real-time events)

```bash
# Using websocat (brew install websocat)
websocat ws://localhost:8080/ws
```

Event types pushed over WebSocket:

| Event | When |
|---|---|
| `job.enqueued` | New job accepted via API |
| `job.started` | Worker claims a job |
| `job.completed` | Handler returns nil |
| `job.failed` | Handler returns error (retryable) |
| `job.dead` | Job exhausted all retries → DLQ |
| `worker.heartbeat` | Each worker's periodic heartbeat |
| `stats.update` | Aggregate stats snapshot (every 5s) |

---

## How the Worker Pool Works

1. **Startup** — `WORKER_COUNT` goroutines are spawned at process start. Each
   registers itself in the `workers` table with status `active`.

2. **Dequeue loop** — Each worker calls `ZPOPMIN` on the Redis sorted set for
   each configured queue name. This is atomic; two workers can never claim the
   same job.

3. **Priority** — The sorted-set score encodes both priority and scheduled_at:
   ```
   score = -(priority × 1e15) + scheduledAt.UnixNano
   ```
   The `1e15` multiplier ensures a higher-priority job always sorts before a
   lower-priority one regardless of how far in the future it is scheduled
   (dominates up to ~285 years of time difference).

4. **Delayed jobs** — Jobs with `scheduled_at` in the future are stored in a
   separate `queue:delayed` sorted set with routing metadata encoded in the
   member string (`jobID|queueName|priority|scheduledNano`). A background
   promoter goroutine moves them to the active queue when their time arrives,
   using a Lua script for atomic promotion.

5. **Processing** — After dequeuing a job ID, the worker hydrates the full job
   from PostgreSQL, marks it `running`, and dispatches to the registered handler
   function for that job type.

6. **Heartbeat** — A sub-goroutine per worker updates `last_seen` in PostgreSQL
   every `WORKER_HEARTBEAT_INTERVAL`. The dashboard can detect stale workers by
   checking for rows where `last_seen < NOW() - 3 × heartbeat_interval`.

7. **Graceful shutdown** — On SIGTERM, the HTTP server stops accepting requests,
   the stats goroutine drains, and then the worker pool context is cancelled.
   Each worker finishes its current job before exiting. `Pool.Shutdown()` blocks
   until all workers return or `WORKER_SHUTDOWN_TIMEOUT` elapses.

---

## How Retry + DLQ Works

```
Handler returns error
        │
        ▼
attempts < max_attempts?
    ├── YES → MarkJobFailed in DB
    │          └── Re-enqueue with exponential backoff:
    │               delay = base_delay × 2^(attempt-1), capped at max_delay
    │               e.g. attempt 1: 5s, 2: 10s, 3: 20s, 4: 40s, 5: 80s
    │
    └── NO  → MarkJobDead in DB
               InsertDLQ in dead_letter_jobs table
               Broadcast job.dead event over WebSocket
```

DLQ entries are never deleted. They can be re-enqueued via
`POST /api/v1/dlq/:id/requeue`, which creates a fresh job with reset attempt
counters and links back to the original DLQ entry.

---

## CI / CD

Two GitHub Actions workflows run on every push and pull request to `master`:

### CI (`ci.yml`)

| Job | What it does |
|---|---|
| **Lint** | `golangci-lint` — style, correctness, and static analysis |
| **Vet** | `go vet` — catches suspicious constructs |
| **Test** | `go test -race` — full test suite with race detector + coverage upload |
| **Security** | `govulncheck` (known CVEs) + `gosec` (SARIF artifact) |

### Docker (`docker.yml`)

Builds and pushes a multi-stage distroless image to GitHub Container Registry
(`ghcr.io/dhananjay6561/jobqueue`) on every push to `master` and on version
tags (`v*`).

```bash
docker pull ghcr.io/dhananjay6561/jobqueue:master
```

---

## Environment Variables

| Variable                   | Default          | Description                          |
|----------------------------|------------------|--------------------------------------|
| `SERVER_HOST`              | `0.0.0.0`        | HTTP bind interface                  |
| `SERVER_PORT`              | `8080`           | HTTP port                            |
| `SERVER_READ_TIMEOUT`      | `15s`            | HTTP read timeout                    |
| `SERVER_WRITE_TIMEOUT`     | `30s`            | HTTP write timeout                   |
| `SERVER_IDLE_TIMEOUT`      | `120s`           | HTTP keep-alive idle timeout         |
| `RATE_LIMIT_RPS`           | `100`            | Max requests/second per IP           |
| `RATE_LIMIT_BURST`         | `20`             | Token bucket burst size              |
| `DATABASE_DSN`             | **required**     | PostgreSQL connection string         |
| `DB_MAX_CONNS`             | `20`             | Max pool connections                 |
| `DB_MIN_CONNS`             | `2`              | Min idle connections                 |
| `DB_MAX_CONN_LIFETIME`     | `30m`            | Max connection age                   |
| `DB_MAX_CONN_IDLE_TIME`    | `5m`             | Max idle connection age              |
| `REDIS_ADDR`               | `localhost:6379` | Redis address                        |
| `REDIS_PASSWORD`           | `""`             | Redis AUTH password                  |
| `REDIS_DB`                 | `0`              | Redis logical DB index               |
| `REDIS_DIAL_TIMEOUT`       | `5s`             | Redis dial timeout                   |
| `REDIS_READ_TIMEOUT`       | `3s`             | Redis read timeout                   |
| `REDIS_WRITE_TIMEOUT`      | `3s`             | Redis write timeout                  |
| `WORKER_COUNT`             | `5`              | Concurrent worker goroutines         |
| `WORKER_HEARTBEAT_INTERVAL`| `10s`            | Heartbeat update frequency           |
| `WORKER_POLL_INTERVAL`     | `1s`             | Empty-queue poll backoff             |
| `WORKER_SHUTDOWN_TIMEOUT`  | `30s`            | Graceful shutdown deadline           |
| `RETRY_BASE_DELAY`         | `5s`             | Initial retry backoff                |
| `RETRY_MAX_DELAY`          | `1h`             | Max retry backoff                    |
| `RETRY_DEFAULT_MAX_ATTEMPTS`| `5`             | Global default max_attempts          |

---

## Local Development (without Docker)

```bash
# Start dependencies only
docker compose up postgres redis -d

# Copy and edit .env
cp .env.example .env

# Download modules
go mod download

# Run with race detector
make run-race

# Run tests
make test-race

# Generate coverage report
make test-cover

# Run linter
make lint
```

---

## Project Structure

```
.
├── cmd/server/          # Entry point — wires all components together
├── internal/
│   ├── api/
│   │   ├── handler/     # HTTP request handlers (jobs, workers, WS)
│   │   ├── middleware/  # CORS, logger, rate limiter
│   │   └── router.go    # Route definitions
│   ├── config/          # Environment variable parsing
│   ├── queue/           # Broker (Redis), worker pool, retry logic, events
│   ├── store/           # PostgreSQL CRUD (jobs, DLQ, workers, stats)
│   └── ws/              # WebSocket hub and client pump goroutines
├── migrations/          # SQL migration files (run at startup)
├── frontend/            # React dashboard (Vite + Tailwind + Zustand)
├── .github/workflows/   # CI and Docker workflows
├── docker-compose.yml
├── Dockerfile           # Multi-stage distroless build
└── Makefile
```

---

## Future Improvements

- **Prometheus metrics** — export `promhttp.Handler()` with counters for
  `jobs_enqueued_total`, `jobs_completed_total`, `job_duration_seconds`.
- **Cron-scheduled jobs** — add a `schedule` field (cron expression) and a
  promoter that re-enqueues on schedule.
- **Job dependencies** — allow a job to declare prerequisite job IDs and only
  become eligible once all predecessors complete.
- **Multi-tenant queues** — namespace queues by tenant ID with per-tenant rate
  limits.
- **Job result storage** — store handler return values in the payload column
  for caller retrieval.
- **Dead-letter expiry** — automatically purge DLQ entries older than a
  configurable retention period.
- **gRPC streaming** — replace WebSocket events with a gRPC server-stream
  endpoint for internal service consumers.
