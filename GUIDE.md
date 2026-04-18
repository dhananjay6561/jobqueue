# JobQueue — What It Is, How It Works, and How to Use It

## What is this?

When you build a web app, some tasks are too slow or too risky to run while the user is waiting — sending emails, resizing images, generating reports, calling slow third-party APIs. You don't want the user's HTTP request to hang while all that happens.

A **job queue** solves this. Instead of doing the work immediately, your app drops a "job" into a queue and instantly returns a response. Background workers pick those jobs up and execute them independently, reliably, and concurrently.

This project is a **production-grade distributed job queue** built in Go:

- **PostgreSQL** — source of truth, full audit history of every job
- **Redis** — broker; workers atomically pull job IDs from sorted sets (no two workers ever claim the same job)
- **Worker pool** — configurable number of Go goroutines processing jobs in parallel
- **WebSocket** — push real-time events to the dashboard the instant anything happens
- **Multi-tenant** — every API key sees only its own jobs; one deployment serves many teams
- **React dashboard** — observe and control everything without writing curl commands

---

## The Flow — What Actually Happens

```
Your app                         JobQueue
──────────────────               ────────────────────────────────────────────

POST /api/v1/jobs ─────────────► Saved to PostgreSQL  (status: pending)
                                  Job ID pushed into Redis sorted set
                                  WebSocket broadcasts: job.enqueued
                  ◄───────────── Returns job ID immediately

                                  Worker dequeues the job ID from Redis
                                  Job marked running in PostgreSQL
                                  WebSocket broadcasts: job.started

                                  Handler function executes the work

                                  ┌── success ──► status = completed
                                  │               result stored in DB
                                  │               job.completed broadcast
                                  │
                                  └── failure ──► retried with backoff
                                                  (up to max_attempts times)
                                                  ──► if exhausted:
                                                      status = dead
                                                      moved to DLQ
                                                      job.dead broadcast
```

---

## The Dashboard — Page by Page

### Dashboard (Home)

Open **http://localhost:8080**

**Stats Bar** — live counters: Total, Pending, Running, Completed, Failed, Dead, Workers, JPM (jobs/minute).

**Throughput Chart** — rolling chart of completed vs failed jobs. Updates every 5 seconds.

**Queue Depth Gauge** — jobs currently waiting in Redis. If this climbs faster than it drains, add more workers.

**Live Events Feed** — real-time terminal streaming over WebSocket. Every job lifecycle event appears here instantly.

**Recent Jobs table** — last few jobs, clickable for full details.

---

### Jobs Page

**Enqueue a job** — click `+ Enqueue Job`. Fill in:
- **Job Type** — handler name (`send_email`, `resize_image`, etc.)
- **Queue** — `default`, `critical`, or `bulk`
- **Priority** — 1–10 slider; higher jumps the queue
- **Max Attempts** — retries before DLQ
- **Schedule for** — optional datetime for future execution
- **TTL seconds** — optional auto-expire after N seconds in a terminal state
- **Payload** — JSON passed to the handler

**Batch enqueue** — click `Batch`. Paste a JSON array of job objects; all enqueued atomically in one transaction.

**Filter bar** — filter by status, type, or queue.

**Click any row** — side drawer with full detail: ID, payload, error, timestamps, attempt count.

---

### Workers Page

All worker goroutines running inside the Go process:
- **Status** — `active` (processing right now) or `idle`
- **Current Job** — job type currently being processed
- **Jobs Processed** — cumulative completions since startup
- **Last Seen** — heartbeat timestamp; stale means the worker may have crashed

---

### Dead Letter Queue

Jobs that exhausted all retry attempts land here. Nothing is ever auto-deleted (unless you set `expires_at`).

**Requeue** — creates a fresh job with reset counters, links back to the DLQ entry. The entry is marked `requeued` so you know it was actioned.

---

### Cron Schedules

Recurring jobs driven by 5-field cron expressions. No external cron daemon needed.

- **Enable/disable toggle** — pause a schedule without deleting it
- **Create** — fill in name, job type, and cron expression
- **Delete** — removes the schedule; in-flight jobs are unaffected

---

## API Key Setup

By default the server runs in open mode (no auth). To enable multi-tenant access with per-key isolation:

```bash
# 1. Set a database-backed key store (already on when DATABASE_DSN is set)
# 2. Create your first key via the API
curl -X POST http://localhost:8080/api/v1/keys \
  -H 'Content-Type: application/json' \
  -d '{"name": "my-service", "tier": "free"}'

# Response includes the raw key (shown once only — save it):
# { "data": { "key": "sk_live_...", "api_key": { "id": "...", "tier": "free", ... } } }

# 3. Use the key on every request
curl -H 'X-API-Key: sk_live_...' http://localhost:8080/api/v1/jobs

# 4. Check usage
curl -H 'X-API-Key: sk_live_...' http://localhost:8080/api/v1/usage
```

**Tiers:**

| Tier | Monthly job limit |
|---|---|
| `free` | 1,000 |
| `pro` | 100,000 |
| `business` | Unlimited |

When the limit is hit, `POST /api/v1/jobs` returns `429 Too Many Requests`.

**Admin key** — set `ADMIN_KEY` to a separate secret. That key bypasses per-key scoping and sees all jobs globally. Useful for operators:

```bash
ADMIN_KEY=ops-secret

curl -H 'X-API-Key: ops-secret' http://localhost:8080/api/v1/stats
# → global stats across all API keys
```

**Rate limit headers** — every response includes:
```
X-RateLimit-Limit: 20
X-RateLimit-Remaining: 18
X-RateLimit-Reset: 0
```

---

## Hands-On Demo

### Step 1 — Confirm workers are alive

Open the dashboard. The Live Events feed shows `[worker.heartbeat]` events every 10 seconds — one per worker.

### Step 2 — Enqueue your first job

Go to **Jobs → Enqueue Job**:
```
Type:         noop
Queue:        default
Priority:     5
Max Attempts: 3
Payload:      { "hello": "world" }
```

Switch to the Dashboard. Three events appear in under a second:
```
[job.enqueued]   noop job abc123 enqueued
[job.started]    worker-2 claimed abc123
[job.completed]  abc123 completed in 1ms
```

### Step 3 — Watch priority in action

Enqueue 5 jobs at priority 2, then immediately one at priority 9. The priority-9 job completes first — Redis score encoding guarantees it.

### Step 4 — Watch a job fail and retry

Enqueue a `send_email` job with `Max Attempts: 3`. The Live Feed shows:
```
[job.enqueued] → [job.started] → [job.failed] (attempt 1/3, backoff 5s)
             → [job.started] → [job.failed] (attempt 2/3, backoff 10s)
             → [job.started] → [job.failed] (attempt 3/3, backoff 20s)
             → [job.dead]    moved to DLQ
```

### Step 5 — Rescue from the DLQ

Go to **Dead Letter Queue**, find the entry, click **Requeue**. A fresh job is created and the cycle begins again.

### Step 6 — Schedule a job for the future

In the enqueue modal, set **Schedule for** to a time a few minutes from now. The job sits in `pending` until then — the Redis delayed promoter moves it to the active queue at the right moment.

### Step 7 — Batch enqueue

Click **Batch** on the Jobs page, paste:
```json
[
  { "type": "send_email", "payload": { "to": "a@example.com" } },
  { "type": "resize_image", "payload": { "url": "https://..." } },
  { "type": "generate_report", "payload": { "report_id": 99 } }
]
```
All three are inserted in a single transaction and immediately available to workers.

### Step 8 — Tag jobs and filter

Enqueue with tags:
```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{"type":"send_email","payload":{},"tags":{"user_id":"42","region":"eu"}}'

# Filter later
curl "http://localhost:8080/api/v1/jobs?tags=region:eu"
```

---

## SDK Quick Start

### Go

```go
import jobqueue "github.com/dhananjay6561/jobqueue/sdk/go"

client := jobqueue.New("http://localhost:8080", jobqueue.WithAPIKey("sk_..."))

// Enqueue
job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
    Type:    "generate_report",
    Payload: map[string]any{"report_id": 42},
    Priority: 7,
})

// Poll for completion and fetch result
for job.Status != "completed" && job.Status != "dead" {
    time.Sleep(500 * time.Millisecond)
    job, _ = client.GetJob(ctx, job.ID.String())
}
result, _ := client.GetJobResult(ctx, job.ID.String())

// Cron
_, err = client.CreateCron(ctx, "/cron schedule", ...)
```

### Node.js

```js
import { JobQueueClient } from '@jobqueue/client'

const jq = new JobQueueClient('http://localhost:8080', { apiKey: 'sk_...' })

// Single job
const job = await jq.enqueue({ type: 'send_email', payload: { to: 'x@example.com' } })

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

// Cron
await jq.createCron({ name: 'daily', job_type: 'cleanup', cron_expression: '0 2 * * *' })
await jq.patchCron(id, { enabled: false })
```

### Python

```python
from jobqueue_client import JobQueueClient

with JobQueueClient("http://localhost:8080", api_key="sk_...") as jq:
    job = jq.enqueue(type="send_email", payload={"to": "user@example.com"}, ttl_seconds=3600)
    batch = jq.enqueue_batch([
        {"type": "resize_image", "payload": {"url": "..."}},
        {"type": "send_email",   "payload": {"to": "..."}},
    ])
    stats = jq.get_stats()
    jq.create_cron(name="daily", job_type="cleanup", cron_expression="0 2 * * *")

# Async
import asyncio
from jobqueue_client import AsyncJobQueueClient

async def main():
    async with AsyncJobQueueClient("http://localhost:8080", api_key="sk_...") as jq:
        job = await jq.enqueue(type="generate_report", payload={"id": 1})

asyncio.run(main())
```

---

## Advanced Features

### Job TTL and Auto-Cleanup

Set a TTL on individual jobs so they auto-expire after a terminal state:

```bash
# Job expires 1 hour after it completes/fails
curl -X POST http://localhost:8080/api/v1/jobs \
  -d '{"type":"noop","payload":{},"ttl_seconds":3600}'

# Bulk-delete all terminal jobs created before a timestamp
curl -X DELETE "http://localhost:8080/api/v1/jobs?before=2026-01-01T00:00:00Z"
```

The server also runs a background goroutine that purges expired jobs every hour automatically — no external cron needed.

---

### Webhooks

Receive signed HTTP POSTs on job lifecycle events:

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H 'Content-Type: application/json' \
  -d '{
    "url": "https://myapp.com/hooks/jobqueue",
    "secret": "my-signing-secret",
    "events": ["job.completed", "job.failed", "job.dead"]
  }'
```

**Verify the signature** (Node.js):
```js
const sig = crypto.createHmac('sha256', 'my-signing-secret')
  .update(rawBody).digest('hex')
if (`sha256=${sig}` !== req.headers['x-webhook-signature']) {
  return res.status(401).send('invalid signature')
}
```

Supported events: `job.enqueued`, `job.started`, `job.completed`, `job.failed`, `job.dead`.

---

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics | grep "^jobqueue_"
```

```
jobqueue_jobs_enqueued_total 42
jobqueue_jobs_completed_total 38
jobqueue_jobs_failed_total 3
jobqueue_jobs_dead_total 1
jobqueue_queue_depth 4
jobqueue_active_workers 5
jobqueue_ws_clients 2
jobqueue_job_duration_seconds_bucket{job_type="send_email",le="0.5"} 12
```

**Prometheus scrape config:**
```yaml
scrape_configs:
  - job_name: jobqueue
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

---

### Job Result Storage

Handlers can return a value. It's stored as JSON in PostgreSQL and retrievable async:

```bash
# Enqueue
curl -X POST http://localhost:8080/api/v1/jobs \
  -d '{"type":"generate_report","payload":{"report_id":42}}'

# Fetch result after completion
curl http://localhost:8080/api/v1/jobs/<id>/result
# → {"report_url":"https://...","rows":1234}
```

Returns `204 No Content` if the job stored no result. Returns `404` if not found.

**In your handler (Go):**
```go
pool.Register("generate_report", func(ctx context.Context, job *queue.Job) (any, error) {
    report := buildReport(job.Payload)
    return map[string]any{"report_url": report.URL, "rows": report.RowCount}, nil
})
```

---

### Cursor Pagination

The standard `GET /api/v1/jobs` uses offset pagination which can skip or duplicate rows when new jobs are inserted between pages. Use the cursor endpoint for stable pagination:

```bash
# First page
curl "http://localhost:8080/api/v1/jobs/cursor?limit=50&status=completed"
# → { "items": [...], "next_cursor": "eyJ...", "has_more": true }

# Next page
curl "http://localhost:8080/api/v1/jobs/cursor?limit=50&cursor=eyJ..."
```

The cursor encodes `(created_at, id)` so pages are fully stable even as new jobs arrive.

---

## Why This Matters

Same pattern used at scale by:
- **Shopify** — millions of background jobs/day (order confirmations, inventory, webhooks)
- **GitHub** — CI dispatch, notification emails, webhook fan-out
- **Stripe** — async payment processing, fraud checks, email receipts

Core ideas this demonstrates:
- **Atomic dequeue via ZPOPMIN** — no two workers ever claim the same job
- **PostgreSQL as audit log** — Redis can be flushed; Postgres never loses history
- **Exponential backoff** — transient failures don't permanently kill jobs
- **DLQ as safety net** — nothing silently disappears; everything recoverable
- **Multi-tenant by default** — one deployment, many isolated teams
- **Graceful shutdown** — in-flight jobs always finish before the process exits
