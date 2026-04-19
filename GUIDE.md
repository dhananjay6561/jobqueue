# Complete Beginner's Guide to JobQueue

This guide assumes you have never heard of a job queue before. By the end you will understand what it is, why you need it, how to set it up, and how to use every feature — from the dashboard to the API to the SDKs.

---

## Part 1 — Understanding the Problem

### What is a job queue and why do you need one?

Imagine you have a web app. A user clicks "Send Invoice". Your server has to:

1. Generate a PDF
2. Upload it to S3
3. Send it via email
4. Log it in your database

Steps 1–3 are slow (1–5 seconds combined). If you do all this inside the HTTP request, the user sits staring at a spinner for 5 seconds. Worse, if step 3 fails (email provider is down), the whole request fails and the user gets an error.

A **job queue** solves this by separating "accepting the work" from "doing the work":

```
User clicks "Send Invoice"
        │
        ▼
Your server creates a job: { type: "send_invoice", payload: { invoice_id: 42 } }
        │
        ▼
Returns to user instantly: "Invoice queued! You'll receive it shortly."
        │
        ▼ (separately, in the background)
A worker picks up the job, runs steps 1–3, retries if any fail
```

The user gets a response in milliseconds. The heavy work happens in the background, reliably, with automatic retries.

### Why not just use a background goroutine / thread?

Background goroutines are lost when your server restarts. If you deploy a new version while 50 jobs are in-flight, they all disappear. A job queue persists jobs in a database so nothing is ever lost, even across restarts, crashes, or deployments.

### What JobQueue gives you specifically

- **HTTP API** — any language can enqueue jobs with a simple POST request
- **PostgreSQL** — every job is saved to a database; nothing is lost on restart
- **Redis** — fast sorted-set queue so workers atomically claim jobs (no double-processing)
- **Worker pool** — configurable number of goroutines process jobs in parallel
- **Automatic retries** — failed jobs are retried with exponential backoff
- **Dead-letter queue (DLQ)** — jobs that exhaust all retries are preserved for manual inspection
- **Cron** — recurring jobs on a schedule, no external cron daemon needed
- **Real-time dashboard** — watch everything happen live in a browser
- **Multi-tenant** — one deployment serves many teams, each isolated by API key
- **SDKs** — Go, Node.js, and Python clients so you don't have to write raw HTTP

---

## Part 2 — Setting Up Locally

### What you need

| Tool | Install | Purpose |
|---|---|---|
| Docker Desktop | [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop/) | Runs Postgres, Redis, and the app |
| Docker Compose v2 | Included in Docker Desktop | Orchestrates the services |

That's it. You do not need Go, Node.js, or Python to run JobQueue — everything runs inside Docker containers.

### Step 1 — Clone the repository

```bash
git clone https://github.com/dhananjay6561/jobqueue
cd jobqueue
```

### Step 2 — Start everything

```bash
./run.sh
```

This script:
1. Builds the Docker image (compiles Go + React frontend)
2. Starts PostgreSQL
3. Starts Redis
4. Starts the JobQueue server
5. Runs all database migrations automatically
6. Waits until the health check passes

You should see:
```
==> Server is UP!
{"status":"ok","checks":{"postgres":"healthy","redis":"healthy"},"uptime":"3s"}
---------------------------------------------
  Dashboard : http://localhost:8080
  Health    : http://localhost:8080/health
---------------------------------------------
```

If you see errors, see the **Troubleshooting** section at the bottom.

### Step 3 — Open the dashboard

Go to **http://localhost:8080** in your browser.

You will see a login screen. Click **"Continue with demo account"** — this automatically logs you in with the built-in demo user and takes you straight to the dashboard.

---

## Part 3 — The Dashboard, Page by Page

### Dashboard (Home)

This is your mission control. Every number and chart updates in real time.

**Stats Bar** (top row of cards):
- **Total** — all jobs ever created in your account
- **Pending** — waiting in the queue to be picked up
- **Running** — being processed by a worker right now
- **Completed** — successfully finished
- **Failed** — failed but will be retried
- **Dead** — exhausted all retries, sitting in the DLQ
- **Workers** — goroutines actively processing jobs
- **JPM** — jobs completed in the last 60 seconds

**Throughput Chart** — rolling line chart of completed vs failed jobs. A widening gap between the lines means errors are accumulating.

**Queue Depth Gauge** — jobs currently sitting in Redis waiting to be picked up. If this grows faster than it drains, you need more workers (`WORKER_COUNT` env var).

**Live Events Feed** — a real-time terminal. Every single job lifecycle event appears here the instant it happens, streamed over WebSocket. If the feed is silent, check the connection indicator — it should say "Connected".

**Recent Jobs table** — the 10 most recent jobs. Click any row to open a full detail drawer with payload, error message, attempt count, and timestamps.

---

### Jobs Page

This is where you create and manage jobs.

**Enqueue a single job:**
1. Click **+ Enqueue Job**
2. Fill in the form:
   - **Job Type** — the name of the handler that will process it (e.g. `send_email`, `resize_image`). This is how the worker knows what function to run.
   - **Queue** — `default` for normal work, `critical` for high-urgency tasks that jump ahead, `bulk` for low-priority batch work
   - **Priority** — 1–10 slider. Priority 10 always dequeues before priority 1, regardless of when it was submitted
   - **Max Attempts** — how many times to try before giving up and sending to DLQ
   - **Schedule for** — leave blank to run immediately, or pick a future datetime
   - **TTL seconds** — optional; auto-delete the job record N seconds after it completes or fails (useful for keeping the database clean)
   - **Payload** — the JSON data your handler will receive. For a real email handler this might be `{"to": "user@example.com", "subject": "Your invoice", "template": "invoice_v2"}`

3. Click **Enqueue**. Watch the Live Events Feed on the Dashboard — you'll see `job.enqueued` → `job.started` → `job.completed` all within a second.

**Batch enqueue:**
Click **Batch** and paste a JSON array. All jobs in the array are inserted in a single database transaction — either all succeed or none do:
```json
[
  { "type": "send_email", "payload": { "to": "alice@example.com" } },
  { "type": "send_email", "payload": { "to": "bob@example.com" } },
  { "type": "resize_image", "payload": { "url": "https://example.com/photo.jpg", "width": 800 } }
]
```

**Filtering:**
Use the status dropdown and type filter to find specific jobs. Useful when debugging failures.

**Job statuses explained:**

| Status | Meaning |
|---|---|
| `pending` | In the queue, waiting for a worker |
| `running` | A worker is processing it right now |
| `completed` | Handler finished successfully |
| `failed` | Handler errored — will be retried after a delay |
| `dead` | Exhausted all retries — check the DLQ |
| `cancelled` | Manually cancelled before it ran |

---

### Workers Page

Shows every goroutine in the worker pool:

- **Status** — `active` means currently processing a job; `idle` means waiting for work
- **Current Job** — the job type being processed right now (only when active)
- **Jobs Processed** — how many jobs this worker has completed since the server started
- **Jobs Failed** — how many jobs this worker has failed
- **Last Seen** — the heartbeat timestamp. Workers ping every 10 seconds. If this is old, the worker may have crashed.

The number of workers is controlled by the `WORKER_COUNT` environment variable (default: 5). On the free Render tier you'd use 2 to stay within the 512 MB RAM limit.

---

### Dead Letter Queue (DLQ)

When a job fails more times than its `max_attempts` allows, it lands here. Nothing in the DLQ is ever automatically deleted (unless you set `expires_at` when you created the job).

**What to do with a DLQ entry:**

1. Click the entry to see the full error message and payload
2. Figure out why it failed (bug in handler? bad payload? external service was down?)
3. Fix the underlying issue
4. Click **Requeue** — this creates a brand-new job with reset attempt counters, linked back to this DLQ entry
5. The DLQ entry is marked `requeued` so you have a record

---

### Cron Schedules

Recurring jobs on a schedule. JobQueue runs a background promoter every 30 seconds that checks which schedules are due and enqueues them.

**Create a cron schedule:**
1. Click **+ New Schedule**
2. Fill in:
   - **Name** — human-readable label (e.g. "Daily database cleanup")
   - **Job Type** — handler name (same as when enqueuing one-off jobs)
   - **Cron Expression** — 5-field expression (minute hour day-of-month month day-of-week)
   - **Queue / Priority** — same as single jobs
   - **Payload** — JSON passed to the handler every time it fires

**Cron expression examples:**

| Expression | Meaning |
|---|---|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour at :00 |
| `0 9 * * *` | Every day at 9:00 AM |
| `0 9 * * 1` | Every Monday at 9:00 AM |
| `*/15 * * * *` | Every 15 minutes |
| `0 2 1 * *` | First of every month at 2:00 AM |

**Enable/disable toggle** — pause a schedule without losing its configuration. Useful for maintenance windows.

---

### Billing Page

Shows your current plan and usage:

- **Tier** — Free / Pro / Business
- **Jobs this month** — usage bar showing how close you are to the monthly limit
- **Reset date** — when the counter resets to zero
- **Upgrade** — click to go through Stripe Checkout (requires Stripe to be configured on the server)
- **Your API key** — shows the prefix of your key for reference. The full key was shown once at registration.

---

## Part 4 — Using the API Directly

The dashboard is great for exploration, but your application will use the HTTP API directly.

### Getting your API key

When you register, your full API key is displayed **once** and never shown again. Copy it immediately and store it somewhere secure (environment variable in your app, a secrets manager, etc.).

If you lose it, you'll need to create a new one (contact the admin or use the admin key to create a new one via `POST /api/v1/keys`).

### Making your first API call

```bash
# Replace qly_abc123 with your actual key
export JOBQUEUE_KEY="qly_abc123..."
export JOBQUEUE_URL="http://localhost:8080"

# Check the server is healthy
curl $JOBQUEUE_URL/health

# Enqueue a job
curl -X POST $JOBQUEUE_URL/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $JOBQUEUE_KEY" \
  -d '{
    "type": "send_email",
    "payload": { "to": "user@example.com", "subject": "Hello" }
  }'
```

The response will look like:
```json
{
  "data": {
    "id": "a1b2c3d4-...",
    "type": "send_email",
    "status": "pending",
    "priority": 5,
    "attempts": 0,
    "max_attempts": 5,
    "queue_name": "default",
    "created_at": "2026-04-19T10:00:00Z"
  },
  "error": null,
  "meta": { "request_id": "abc/xyz-000001" }
}
```

Save the `id`. You can use it to check the job's status:

```bash
curl -H "X-API-Key: $JOBQUEUE_KEY" \
  $JOBQUEUE_URL/api/v1/jobs/a1b2c3d4-...
```

### Practical examples

**Send 1000 notification emails in one request:**
```bash
# Build the batch array in your app, then:
curl -X POST $JOBQUEUE_URL/api/v1/jobs/batch \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $JOBQUEUE_KEY" \
  -d '[
    {"type":"send_notification","payload":{"user_id":1}},
    {"type":"send_notification","payload":{"user_id":2}},
    ...up to 500 per request
  ]'
```

**Schedule a welcome email 10 minutes in the future:**
```bash
SCHEDULE=$(date -u -v+10M '+%Y-%m-%dT%H:%M:%SZ')  # macOS
# or: SCHEDULE=$(date -u -d '+10 minutes' '+%Y-%m-%dT%H:%M:%SZ')  # Linux

curl -X POST $JOBQUEUE_URL/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $JOBQUEUE_KEY" \
  -d "{
    \"type\": \"send_email\",
    \"payload\": {\"template\": \"welcome\"},
    \"scheduled_at\": \"$SCHEDULE\"
  }"
```

**Tag jobs and filter later:**
```bash
# Enqueue with tags
curl -X POST $JOBQUEUE_URL/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $JOBQUEUE_KEY" \
  -d '{
    "type": "generate_report",
    "payload": {"report_id": 99},
    "tags": {"customer": "acme", "region": "eu", "triggered_by": "dashboard"}
  }'

# Later, find all EU jobs for ACME
curl -H "X-API-Key: $JOBQUEUE_KEY" \
  "$JOBQUEUE_URL/api/v1/jobs?tags=customer:acme,region:eu"
```

**Check your usage:**
```bash
curl -H "X-API-Key: $JOBQUEUE_KEY" $JOBQUEUE_URL/api/v1/usage
```
```json
{
  "data": {
    "tier": "free",
    "jobs_used": 247,
    "jobs_limit": 1000,
    "usage_percent": 24.7,
    "limit_reached": false,
    "reset_at": "2026-05-01T00:00:00Z"
  }
}
```

---

## Part 5 — Integrating with Your App via SDKs

### Go SDK

```go
package main

import (
    "context"
    "fmt"
    "log"

    jobqueue "github.com/dhananjay6561/jobqueue/sdk/go"
)

func main() {
    ctx := context.Background()

    // Create a client
    client := jobqueue.New(
        "http://localhost:8080",
        jobqueue.WithAPIKey("qly_your_key_here"),
    )

    // Enqueue a job
    job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
        Type:        "send_email",
        Payload:     map[string]any{"to": "user@example.com"},
        Priority:    8,
        MaxAttempts: 3,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Job enqueued:", job.ID)

    // Batch enqueue
    jobs, err := client.EnqueueBatch(ctx, []jobqueue.EnqueueRequest{
        {Type: "resize_image", Payload: map[string]any{"url": "https://..."}},
        {Type: "send_email",   Payload: map[string]any{"to": "..."}},
    })
    fmt.Println("Batch enqueued:", len(jobs), "jobs")

    // Get job status
    j, _ := client.GetJob(ctx, job.ID.String())
    fmt.Println("Status:", j.Status)

    // Get stored result (after job completes)
    result, _ := client.GetJobResult(ctx, job.ID.String())
    fmt.Println("Result:", string(result))

    // Create a cron schedule
    _, err = client.CreateCron(ctx, jobqueue.CreateCronRequest{
        Name:           "daily-cleanup",
        JobType:        "cleanup_storage",
        CronExpression: "0 2 * * *",
        Payload:        map[string]any{"target": "tmp"},
    })
}
```

### Node.js SDK

```js
import { JobQueueClient } from '@jobqueue/client'

const jq = new JobQueueClient('http://localhost:8080', {
  apiKey: 'qly_your_key_here',
})

// Enqueue a single job
const job = await jq.enqueue({
  type: 'send_email',
  payload: { to: 'user@example.com', subject: 'Welcome!' },
  priority: 7,
  maxAttempts: 3,
})
console.log('Enqueued:', job.id)

// Batch enqueue
const jobs = await jq.enqueueBatch([
  { type: 'resize_image', payload: { url: 'https://...' } },
  { type: 'send_email',   payload: { to: 'user@example.com' } },
])
console.log('Batch:', jobs.length, 'jobs')

// Get job
const status = await jq.getJob(job.id)
console.log('Status:', status.status)

// List completed jobs with cursor pagination
let cursor = ''
do {
  const page = await jq.listJobsCursor({ status: 'completed', cursor, limit: 50 })
  page.items.forEach(j => console.log(j.id, j.type))
  cursor = page.next_cursor
} while (page.has_more)

// Cron schedule
await jq.createCron({
  name: 'hourly-sync',
  job_type: 'sync_data',
  cron_expression: '0 * * * *',
  payload: { source: 'crm' },
})

// Webhooks
await jq.createWebhook({
  url: 'https://yourapp.com/hooks/jobs',
  secret: 'your-secret',
  events: ['job.completed', 'job.dead'],
})
```

### Python SDK

```python
from jobqueue_client import JobQueueClient, AsyncJobQueueClient

# ── Sync usage ────────────────────────────────────────────────────────────────

with JobQueueClient("http://localhost:8080", api_key="qly_your_key_here") as jq:

    # Enqueue a job
    job = jq.enqueue(
        type="send_email",
        payload={"to": "user@example.com"},
        priority=8,
        max_attempts=3,
        ttl_seconds=3600,
    )
    print("Enqueued:", job["id"])

    # Batch enqueue
    jobs = jq.enqueue_batch([
        {"type": "resize_image", "payload": {"url": "https://..."}},
        {"type": "send_email",   "payload": {"to": "user@example.com"}},
    ])
    print("Batch:", len(jobs), "jobs")

    # Get status
    status = jq.get_job(job["id"])
    print("Status:", status["status"])

    # Stats
    stats = jq.get_stats()
    print("Total jobs:", stats["total_jobs"])

    # Cron
    jq.create_cron(
        name="daily-cleanup",
        job_type="cleanup_storage",
        cron_expression="0 2 * * *",
        payload={"target": "tmp"},
    )


# ── Async usage ───────────────────────────────────────────────────────────────

import asyncio
from jobqueue_client import AsyncJobQueueClient

async def main():
    async with AsyncJobQueueClient("http://localhost:8080", api_key="qly_...") as jq:
        job = await jq.enqueue(type="generate_report", payload={"id": 1})
        print("Async enqueued:", job["id"])

asyncio.run(main())
```

---

## Part 6 — Receiving Webhooks

Instead of polling for job status, you can receive push notifications when jobs complete, fail, or die.

### Step 1 — Create a webhook

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $JOBQUEUE_KEY" \
  -d '{
    "url": "https://yourapp.com/hooks/jobqueue",
    "secret": "pick-a-random-secret-string",
    "events": ["job.completed", "job.failed", "job.dead"]
  }'
```

### Step 2 — Receive and verify in your app

JobQueue signs every webhook payload with an HMAC-SHA256 signature using your secret. Always verify it:

```js
// Node.js / Express
import crypto from 'crypto'
import express from 'express'

const app = express()
app.use(express.raw({ type: 'application/json' }))  // important: raw body

app.post('/hooks/jobqueue', (req, res) => {
  // Verify signature
  const expected = 'sha256=' + crypto
    .createHmac('sha256', 'pick-a-random-secret-string')
    .update(req.body)
    .digest('hex')

  if (req.headers['x-webhook-signature'] !== expected) {
    return res.status(401).send('Bad signature')
  }

  const event = JSON.parse(req.body)
  console.log('Event type:', event.type)

  switch (event.type) {
    case 'job.completed':
      console.log('Job done:', event.job_id, 'took', event.duration_ms, 'ms')
      break
    case 'job.dead':
      console.log('Job died:', event.job_id, 'error:', event.error)
      // alert your on-call team, write to Slack, etc.
      break
  }

  res.status(200).send('ok')
})
```

```python
# Python / FastAPI
from fastapi import FastAPI, Request, HTTPException
import hashlib, hmac

app = FastAPI()
SECRET = "pick-a-random-secret-string"

@app.post("/hooks/jobqueue")
async def webhook(request: Request):
    body = await request.body()

    expected = "sha256=" + hmac.new(
        SECRET.encode(), body, hashlib.sha256
    ).hexdigest()

    if request.headers.get("x-webhook-signature") != expected:
        raise HTTPException(status_code=401, detail="Bad signature")

    event = await request.json()
    print(f"Event: {event['type']}, job: {event.get('job_id')}")
    return {"ok": True}
```

---

## Part 7 — Monitoring with Prometheus

If you run Prometheus, point it at JobQueue's `/metrics` endpoint:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: jobqueue
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

Key metrics:

| Metric | What it tells you |
|---|---|
| `jobqueue_jobs_enqueued_total` | Total jobs ever submitted |
| `jobqueue_jobs_completed_total` | Total jobs completed successfully |
| `jobqueue_jobs_failed_total` | Total jobs that errored (including retried) |
| `jobqueue_jobs_dead_total` | Total jobs that exhausted retries |
| `jobqueue_queue_depth` | Jobs currently waiting in Redis |
| `jobqueue_active_workers` | Workers currently processing jobs |
| `jobqueue_ws_clients` | Connected dashboard clients |
| `jobqueue_job_duration_seconds` | Histogram of processing time by job type |

**Useful alerts:**

```yaml
# Alert when DLQ is growing (jobs dying faster than you're requeuing them)
- alert: JobQueueDLQGrowing
  expr: increase(jobqueue_jobs_dead_total[5m]) > 5
  annotations:
    summary: "More than 5 jobs died in the last 5 minutes"

# Alert when queue is backing up
- alert: JobQueueDepthHigh
  expr: jobqueue_queue_depth > 1000
  annotations:
    summary: "Queue depth over 1000 — workers may be too slow"
```

---

## Part 8 — Deploying to Production (Render)

This repo includes a `render.yaml` that sets up the full stack on Render with one click.

### Step 1 — Fork the repository

Fork `https://github.com/dhananjay6561/jobqueue` to your own GitHub account.

### Step 2 — Create a Render account

Go to [render.com](https://render.com) and sign up. Connect your GitHub account.

### Step 3 — Set up Redis (Upstash)

JobQueue needs Redis. The free option is [Upstash](https://upstash.com):

1. Create a free Upstash account
2. Create a new Redis database
3. Copy the **Endpoint** (looks like `your-db.upstash.io:6379`) and **Password**

### Step 4 — Deploy on Render

1. In Render, click **New** → **Blueprint**
2. Connect your forked repository
3. Render reads `render.yaml` and creates the web service + managed Postgres automatically
4. Click **Apply**

### Step 5 — Set environment variables

In the Render dashboard, go to your service → **Environment**. Add these:

| Variable | Value |
|---|---|
| `JWT_SECRET` | A long random string — run `openssl rand -hex 32` to generate one |
| `BASE_URL` | Your Render URL, e.g. `https://jobqueue.onrender.com` |
| `REDIS_ADDR` | Your Upstash endpoint, e.g. `your-db.upstash.io:6379` |
| `REDIS_PASSWORD` | Your Upstash password |

Then click **Save Changes** and trigger a manual deploy.

### Step 6 — Verify

Visit `https://your-app.onrender.com/health`. You should see:
```json
{"status":"ok","checks":{"postgres":"healthy","redis":"healthy"}}
```

Open the dashboard, log in with the demo account or register, and you're live.

> **Note:** The free Render plan spins down after 15 minutes of inactivity and takes ~30 seconds to cold-start. Upgrade to a paid plan for always-on availability.

---

## Part 9 — Admin Mode

Set `ADMIN_KEY` to a secret value. Any request authenticated with this key bypasses per-key scoping and sees all jobs across all API keys — useful for operators debugging issues.

```bash
ADMIN_KEY=ops-secret-abc123

# See every job from every user
curl -H "X-API-Key: ops-secret-abc123" http://localhost:8080/api/v1/jobs

# Global stats
curl -H "X-API-Key: ops-secret-abc123" http://localhost:8080/api/v1/stats

# Create a new API key for a user
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ops-secret-abc123" \
  -d '{"name": "alice-production", "tier": "pro"}'
```

Keep the admin key secret — it bypasses all data isolation.

---

## Part 10 — Troubleshooting

### The server won't start

**Check the logs:**
```bash
docker compose logs app
```

Common causes:
- `DATABASE_DSN is required` — you haven't set the database connection string. Locally this is handled by `docker-compose.yml`. In production, set the env var.
- `failed to connect to redis` — Redis isn't running or the address is wrong. Check `REDIS_ADDR` and `REDIS_PASSWORD`.
- `database migration failed` — a migration crashed. Check the specific error. Usually a type or table already exists from a partial previous run. The server will retry on the next start.

### I can't log in

- Make sure you registered first (click Register, not Sign In)
- The demo account (`demo@jobqueue.dev` / `demo1234`) is seeded automatically on first startup
- If you changed `JWT_SECRET`, existing tokens are invalidated — log out and log back in

### API calls return 401

- Check you're passing `X-API-Key: <your-full-key>` (not just the prefix)
- The raw key is only shown once at registration. If you lost it, create a new one via the admin key or register a new account.
- Check your monthly usage — `GET /api/v1/usage`. A 429 (not 401) means you hit the limit.

### Jobs are stuck in `pending`

- Workers are running: check the Workers page. If all workers show as `stopped` or the page is empty, the worker pool may not have started.
- Check `docker compose logs app` for worker-related errors.
- If you see `queue:default is empty` in logs repeatedly, there may be a Redis connectivity issue.

### Jobs keep failing

- Click the job in the Jobs page → detail drawer → read the `error_message` field. This is the exact error your handler returned.
- Common cause: the job `type` doesn't match any registered handler. The `send_email` and other demo handlers are registered in `cmd/server/main.go`. Add your own there.

### The Live Events feed shows "Disconnected"

- The WebSocket connects to the same host/port as the page. If you're behind a reverse proxy or load balancer, make sure WebSocket upgrade requests are passed through.
- On Render: WebSocket is supported on all paid plans. On the free plan it should work but may disconnect on cold start.

### `run.sh` says "Server did not respond after 40s"

The image build takes time on first run (compiling Go + Node.js). Try:
```bash
docker compose logs app   # see what's happening
```
If Postgres or Redis aren't healthy yet, wait and try again. If the app itself is crashing, read the error.

---

## Quick Reference Card

```bash
# Start / stop locally
./run.sh
docker compose down

# Enqueue a job
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <key>" \
  -d '{"type":"noop","payload":{}}'

# Check job status
curl -H "X-API-Key: <key>" http://localhost:8080/api/v1/jobs/<id>

# List recent jobs
curl -H "X-API-Key: <key>" http://localhost:8080/api/v1/jobs?limit=10

# Check usage
curl -H "X-API-Key: <key>" http://localhost:8080/api/v1/usage

# Health check
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics

# Real-time events (requires websocat: brew install websocat)
websocat ws://localhost:8080/ws

# View logs
docker compose logs -f app

# Restart cleanly (wipes data)
docker compose down -v && ./run.sh
```
