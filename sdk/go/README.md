# jobqueue-go — Go SDK

Official Go client for the [JobQueue](https://github.com/dhananjay6561/jobqueue) distributed job queue system.

## Install

```bash
go get github.com/dhananjay6561/jobqueue-go
```

## Quick start

```go
import jobqueue "github.com/dhananjay6561/jobqueue-go"

// No auth (dev/local)
client := jobqueue.New("http://localhost:8080")

// With API key auth
client := jobqueue.New("https://your-jobqueue.com",
    jobqueue.WithAPIKey("your-secret-key"),
)
```

## Enqueue a job

```go
job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
    Type:        "send_email",
    Payload:     map[string]any{"to": "user@example.com", "subject": "Hello"},
    Priority:    8,
    MaxAttempts: 3,
    QueueName:   "default",
})
// job.ID, job.Status == "pending"
```

## Schedule a job for the future

```go
at := time.Now().Add(10 * time.Minute)
job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
    Type:        "generate_report",
    Payload:     map[string]any{"report_id": 42},
    ScheduledAt: &at,
})
```

## Poll for completion and fetch result

```go
for {
    job, err := client.GetJob(ctx, job.ID)
    if job.Status == "completed" {
        result, _ := client.GetJobResult(ctx, job.ID)
        fmt.Println(result)
        break
    }
    if job.Status == "dead" || job.Status == "cancelled" {
        fmt.Println("job failed:", job.ErrorMessage)
        break
    }
    time.Sleep(500 * time.Millisecond)
}
```

## List jobs with filters

```go
page, err := client.ListJobs(ctx, jobqueue.ListJobsParams{
    Status: "failed",
    Limit:  20,
    Offset: 0,
})
for _, j := range page.Items {
    fmt.Println(j.ID, j.Type, j.Status)
}
```

## Cancel a pending job

```go
err := client.CancelJob(ctx, jobID)
```

## Retry a failed job

```go
job, err := client.RetryJob(ctx, jobID)
```

## Queue stats

```go
stats, err := client.GetStats(ctx)
fmt.Printf("pending=%d running=%d completed=%d jpm=%d\n",
    stats.Pending, stats.Running, stats.Completed, stats.JobsPerMinute)
```

## Dead-letter queue

```go
// List DLQ entries
page, err := client.ListDLQ(ctx, 20, 0)

// Requeue a failed entry
err = client.RequeueDLQ(ctx, dlqEntryID)
```

## Webhooks

```go
// Register a webhook
hook, err := client.CreateWebhook(ctx, jobqueue.CreateWebhookRequest{
    URL:    "https://myapp.com/hooks/jobqueue",
    Secret: "my-hmac-secret",
    Events: []string{"job.completed", "job.dead"},
})

// List all webhooks
hooks, err := client.ListWebhooks(ctx)

// Delete a webhook
err = client.DeleteWebhook(ctx, hook.ID)
```

Incoming webhook POSTs include an `X-Webhook-Signature: sha256=<hex>` header
you can verify against your secret:

```go
mac := hmac.New(sha256.New, []byte("my-hmac-secret"))
mac.Write(body)
expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Webhook-Signature"))) {
    http.Error(w, "invalid signature", 401)
}
```

## Health check

```go
if err := client.Health(ctx); err != nil {
    log.Fatal("jobqueue unreachable:", err)
}
```

## Custom HTTP client

```go
client := jobqueue.New("http://localhost:8080",
    jobqueue.WithHTTPClient(&http.Client{
        Timeout:   5 * time.Second,
        Transport: myRoundTripper,
    }),
)
```
