# JobQueue — What It Is, How It Works, and How to Use It

## What is this?

When you build a web app, some tasks are too slow or too risky to run while the user is waiting — sending emails, resizing images, generating reports, calling slow third-party APIs. You don't want the user's HTTP request to hang for 10 seconds while all that happens.

A **job queue** solves this. Instead of doing the work immediately, your app drops a "job" (a small message describing the work to be done) into a queue and instantly returns a response to the user. Meanwhile, a pool of background workers picks up those jobs one by one and actually executes them — independently, reliably, and concurrently.

This project is a **production-grade distributed job queue** built in Go. It uses:

- **PostgreSQL** as the source of truth — every job and its full history is stored here
- **Redis** as the broker — workers atomically pull job IDs from sorted sets, ensuring no two workers ever process the same job
- **5 concurrent Go workers** processing jobs in parallel
- **WebSocket** to push real-time events to the dashboard the moment anything happens
- **React dashboard** (what you're looking at) to observe and control everything

---

## The Flow — What Actually Happens

```
You (or your app)                    JobQueue System
─────────────────                    ───────────────────────────────────────────

POST /api/v1/jobs  ──────────────►  Saved to PostgreSQL (status: pending)
                                     Job ID pushed into Redis sorted set
                                     WebSocket broadcasts: job.enqueued
                   ◄──────────────  Returns job ID immediately

                                     Worker picks up job from Redis
                                     Job marked running in PostgreSQL
                                     WebSocket broadcasts: job.started

                                     Handler executes the work

                                     ┌── success ──► job.completed
                                     │
                                     └── failure ──► retried with backoff
                                                     (up to max_attempts)
                                                     ──► if exhausted: job.dead
                                                         moved to Dead Letter Queue
```

---

## The Dashboard — Page by Page

### Dashboard (Home)

Open **http://localhost:8080**

This is your mission control. Here's what you're looking at:

**Stats Bar (top row)** — live counters for:
- `Total` — every job ever submitted
- `Pending` — waiting in queue, not yet picked up
- `Running` — actively being processed by a worker right now
- `Completed` — finished successfully
- `Failed` — failed at least once but still has retry attempts left
- `Dead` — exhausted all retries, sitting in the Dead Letter Queue
- `Workers` — how many workers are currently active
- `JPM` — jobs processed per minute (throughput)

**Throughput Chart** — a live rolling chart showing completed vs failed jobs over time. Each data point updates every 5 seconds. A healthy system shows a green-dominant chart.

**Queue Depth Gauge** — shows how many jobs are currently waiting in Redis to be picked up. If this number keeps climbing, your workers can't keep up with the incoming rate.

**Live Events Feed** — a real-time terminal-style feed streamed over WebSocket. Every time something happens in the system, a line appears here instantly:
- `[job.enqueued]` — a new job was submitted
- `[job.started]` — a worker claimed a job
- `[job.completed]` — a job finished successfully
- `[job.failed]` — a job failed (will be retried)
- `[job.dead]` — a job died after exhausting all retries
- `[worker.heartbeat]` — workers checking in every 10s to say they're alive

**Recent Jobs table** — the last few jobs, clickable to see full details.

---

### Jobs Page

Click **Jobs** in the sidebar.

This is the full job registry. Every job ever submitted lives here.

**Enqueue a Job** — click the `+ Enqueue Job` button (top right). Fill in:
- **Job Type** — the kind of work (e.g. `send_email`, `noop`, `send_notification`). Workers have registered handlers for these types.
- **Queue** — `default`, `critical`, or `bulk`. Critical jobs are always processed before default, which are processed before bulk (priority encoding via Redis score).
- **Priority** — slider from 1–10. Higher priority jobs jump ahead of lower ones within the same queue.
- **Max Attempts** — how many times the system should retry before giving up and moving to DLQ.
- **Payload** — a JSON object passed to the handler. Put anything your handler needs here.

**Filter bar** — filter by status (Pending / Running / Completed / Failed / Dead / Cancelled), by job type, or by queue.

**Click any row** — opens a side drawer with the full job detail: ID, status history, payload, error message if failed, timestamps, attempt count.

---

### Workers Page

Click **Workers** in the sidebar.

Shows all 5 worker goroutines that are running inside the Go process. For each worker you can see:

- **Status** — `active` (processing a job right now), `idle` (waiting for work)
- **Current Job** — the job type it's currently working on
- **Jobs Processed** — how many jobs this worker has completed since startup
- **Last Seen** — the last heartbeat timestamp. If this goes stale (more than ~30s), the worker has likely crashed.

Workers are started automatically when the server boots. You don't start or stop them manually.

---

### Dead Letter Queue (DLQ)

Click **Dead Letter Queue** in the sidebar.

When a job fails and has no retry attempts left, it lands here. Nothing is ever auto-deleted — this is your audit trail for everything that went wrong.

For each DLQ entry you can see:
- The original job type and payload
- How many attempts were made
- The last error message that caused the final failure

**Requeue button** — click it to create a brand new job from this DLQ entry, with fresh retry counters. The new job goes back into the queue and workers will pick it up again.

---

## Try It Now — Hands-On Demo

Make sure the server is running (`./run.sh`), then follow these steps.

### Step 1 — Watch the Live Feed wake up

Go to the **Dashboard**. The Live Events feed should show `[worker.heartbeat]` events arriving every 10 seconds — one per worker. This confirms all 5 workers are alive and connected.

### Step 2 — Enqueue your first job

Go to **Jobs → Enqueue Job**.

Fill in:
```
Type:         noop
Queue:        default
Priority:     5
Max Attempts: 3
Payload:      { "hello": "world" }
```

Click **Enqueue Job**.

Switch immediately to the **Dashboard**. In the Live Events feed you'll see three lines appear within milliseconds:

```
[job.enqueued]   noop job abc123 enqueued on default (priority 5)
[job.started]    worker-2 picked up job abc123 (noop)
[job.completed]  job abc123 (noop) completed in 1ms
```

The `noop` handler does nothing and returns immediately, so it completes in under a millisecond. The Stats Bar `Completed` counter ticks up by 1.

### Step 3 — Enqueue a higher-priority job

Go back to **Jobs → Enqueue Job**, enqueue 5 jobs with priority 2, then quickly enqueue one more with priority 9.

Go to **Jobs** and sort by status. You'll see the priority-9 job completed before the priority-2 jobs — Redis sorted-set scoring ensures higher priority jobs are always dequeued first.

### Step 4 — Watch a job fail and retry

The `send_email` job type has a handler registered but your server doesn't have a real email service wired up, so it will fail.

Go to **Jobs → Enqueue Job**:
```
Type:         send_email
Queue:        default
Priority:     5
Max Attempts: 3
Payload:      { "to": "test@example.com", "subject": "Hello" }
```

Watch the Live Feed:
```
[job.enqueued]   send_email job enqueued
[job.started]    worker picked it up
[job.failed]     attempt 1/3 — error: ...
[job.started]    worker retried (after 5s backoff)
[job.failed]     attempt 2/3 — error: ...
[job.started]    worker retried (after 10s backoff)
[job.failed]     attempt 3/3 — error: ...
[job.dead]       send_email moved to Dead Letter Queue
```

Backoff doubles each attempt: 5s → 10s → 20s. After the 3rd failure the job is dead.

### Step 5 — Rescue from the Dead Letter Queue

Go to **Dead Letter Queue**. You'll see the `send_email` job sitting there with its last error message.

Click **Requeue**. A fresh job is created and you'll see `[job.enqueued]` appear in the Live Feed. The process starts again.

### Step 6 — Cancel a pending job

Enqueue a job with `Max Attempts: 1` and then immediately go to **Jobs** and click on it. In the detail drawer, hit **Cancel**. The status changes to `cancelled` and no worker will process it.

---

## Why this matters

This is the same pattern used by:
- **Shopify** — processes millions of background jobs per day (order confirmations, inventory updates, webhook deliveries)
- **GitHub** — runs CI job dispatching, notification emails, and webhook fan-out
- **Stripe** — async payment processing, fraud checks, email receipts

The key ideas this system demonstrates:
- **Atomic dequeue via Redis ZPOPMIN** — no two workers ever claim the same job
- **PostgreSQL as audit log** — Redis can be flushed; Postgres never loses history
- **Exponential backoff** — transient failures (network hiccup, rate limit) don't permanently kill jobs
- **DLQ as safety net** — nothing silently disappears; everything that fails is visible and recoverable
- **Graceful shutdown** — in-flight jobs always finish before the process exits
