// Package jobqueue provides a Go client for the JobQueue HTTP API.
//
// Quick start:
//
//	client := jobqueue.New("http://localhost:8080")
//	// with API key auth:
//	client := jobqueue.New("http://localhost:8080", jobqueue.WithAPIKey("secret"))
//
//	job, err := client.Enqueue(ctx, jobqueue.EnqueueRequest{
//	    Type:    "send_email",
//	    Payload: map[string]any{"to": "user@example.com"},
//	})
package jobqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is a thread-safe HTTP client for the JobQueue API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the X-API-Key header on every request.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a Client pointed at baseURL (e.g. "http://localhost:8080").
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

// Enqueue submits a new job and returns the persisted Job.
func (c *Client) Enqueue(ctx context.Context, req EnqueueRequest) (*Job, error) {
	if req.Priority == 0 {
		req.Priority = 5
	}
	if req.QueueName == "" {
		req.QueueName = "default"
	}
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 3
	}
	var resp apiResponse[*Job]
	if err := c.post(ctx, "/api/v1/jobs", req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// GetJob fetches a single job by ID.
func (c *Client) GetJob(ctx context.Context, id string) (*Job, error) {
	var resp apiResponse[*Job]
	if err := c.get(ctx, "/api/v1/jobs/"+id, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// ListJobs returns a paginated list of jobs filtered by params.
func (c *Client) ListJobs(ctx context.Context, p ListJobsParams) (Page[*Job], error) {
	q := url.Values{}
	if p.Status != "" {
		q.Set("status", p.Status)
	}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.Queue != "" {
		q.Set("queue", p.Queue)
	}
	limit := p.Limit
	if limit == 0 {
		limit = 20
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(p.Offset))

	var resp apiResponse[[]*Job]
	if err := c.get(ctx, "/api/v1/jobs", q, &resp); err != nil {
		return Page[*Job]{}, err
	}
	if resp.Error != "" {
		return Page[*Job]{}, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	page := Page[*Job]{Items: resp.Data, Limit: limit, Offset: p.Offset}
	if resp.Meta.TotalCount != nil {
		page.TotalCount = *resp.Meta.TotalCount
	}
	if resp.Meta.HasMore != nil {
		page.HasMore = *resp.Meta.HasMore
	}
	return page, nil
}

// CancelJob cancels a pending job.
func (c *Client) CancelJob(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/jobs/"+id)
}

// RetryJob manually re-enqueues a failed job.
func (c *Client) RetryJob(ctx context.Context, id string) (*Job, error) {
	var resp apiResponse[*Job]
	if err := c.post(ctx, "/api/v1/jobs/"+id+"/retry", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// GetJobResult fetches the JSON result stored by the handler.
// Returns nil, nil when the job exists but stored no result.
func (c *Client) GetJobResult(ctx context.Context, id string) (map[string]any, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/jobs/"+id+"/result", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("jobqueue: job not found")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jobqueue: server error %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jobqueue: decode result: %w", err)
	}
	return result, nil
}

// ── Stats ─────────────────────────────────────────────────────────────────────

// GetStats returns current queue statistics.
func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	var resp apiResponse[*Stats]
	if err := c.get(ctx, "/api/v1/stats", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// ── DLQ ───────────────────────────────────────────────────────────────────────

// ListDLQ returns dead-letter queue entries.
func (c *Client) ListDLQ(ctx context.Context, limit, offset int) (Page[*DLQEntry], error) {
	if limit == 0 {
		limit = 20
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))

	var resp apiResponse[[]*DLQEntry]
	if err := c.get(ctx, "/api/v1/dlq", q, &resp); err != nil {
		return Page[*DLQEntry]{}, err
	}
	if resp.Error != "" {
		return Page[*DLQEntry]{}, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	page := Page[*DLQEntry]{Items: resp.Data, Limit: limit, Offset: offset}
	if resp.Meta.TotalCount != nil {
		page.TotalCount = *resp.Meta.TotalCount
	}
	return page, nil
}

// RequeueDLQ re-enqueues a dead-letter entry, resetting its attempt counter.
func (c *Client) RequeueDLQ(ctx context.Context, dlqID string) (*Job, error) {
	var resp apiResponse[map[string]any]
	if err := c.post(ctx, "/api/v1/dlq/"+dlqID+"/requeue", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return nil, nil // new job ID is in resp.Data["new_job"]["id"]
}

// ── Webhooks ──────────────────────────────────────────────────────────────────

// ListWebhooks returns all registered webhooks.
func (c *Client) ListWebhooks(ctx context.Context) ([]*Webhook, error) {
	var resp apiResponse[[]*Webhook]
	if err := c.get(ctx, "/api/v1/webhooks", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// CreateWebhook registers a new webhook endpoint.
func (c *Client) CreateWebhook(ctx context.Context, req CreateWebhookRequest) (*Webhook, error) {
	var resp apiResponse[*Webhook]
	if err := c.post(ctx, "/api/v1/webhooks", req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("jobqueue: %s", resp.Error)
	}
	return resp.Data, nil
}

// DeleteWebhook removes a webhook by ID.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/webhooks/"+id)
}

// ── Health ────────────────────────────────────────────────────────────────────

// Health checks whether the server and its dependencies are healthy.
// Returns nil when status is "ok".
func (c *Client) Health(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jobqueue: unhealthy (status %d)", resp.StatusCode)
	}
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("jobqueue: marshal request: %w", err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return req, nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	p := path
	if len(q) > 0 {
		p += "?" + q.Encode()
	}
	req, err := c.newRequest(ctx, http.MethodGet, p, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("jobqueue: server error %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
