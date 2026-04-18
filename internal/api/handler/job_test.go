// Package handler — job_test.go contains unit tests for the job HTTP handlers.
// All database and broker interactions are exercised through mock implementations
// of the store.JobStorer and queue.Broker interfaces.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dj/jobqueue/internal/queue"
	"github.com/dj/jobqueue/internal/store"
)

// ─── Mock implementations ─────────────────────────────────────────────────────

// mockJobStore is a minimal in-memory store used in handler tests.
type mockJobStore struct {
	jobs     map[uuid.UUID]*queue.Job
	dlqItems map[uuid.UUID]*queue.DLQEntry
	workers  []*queue.WorkerInfo

	// Inject errors for failure-path testing.
	createErr   error
	getErr      error
	cancelErr   error
	retryErr    error
}

func newMockJobStore() *mockJobStore {
	return &mockJobStore{
		jobs:     make(map[uuid.UUID]*queue.Job),
		dlqItems: make(map[uuid.UUID]*queue.DLQEntry),
	}
}

func (m *mockJobStore) CreateJob(_ context.Context, job *queue.Job) (*queue.Job, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	now := time.Now()
	job.CreatedAt = now
	job.Status = queue.StatusPending
	m.jobs[job.ID] = job
	return job, nil
}

func (m *mockJobStore) GetJob(_ context.Context, id uuid.UUID) (*queue.Job, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return job, nil
}

func (m *mockJobStore) ListJobs(_ context.Context, filter store.JobFilter) (store.Page[*queue.Job], error) {
	items := make([]*queue.Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		items = append(items, j)
	}
	return store.Page[*queue.Job]{
		Items:      items,
		TotalCount: int64(len(items)),
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

func (m *mockJobStore) MarkJobStarted(_ context.Context, id uuid.UUID, workerID string) (*queue.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	job.Status = queue.StatusRunning
	job.WorkerID = &workerID
	return job, nil
}

func (m *mockJobStore) MarkJobCompleted(_ context.Context, id uuid.UUID, _ json.RawMessage) (*queue.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	job.Status = queue.StatusCompleted
	return job, nil
}

func (m *mockJobStore) GetJobResult(_ context.Context, id uuid.UUID) (json.RawMessage, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return job.Result, nil
}

func (m *mockJobStore) MarkJobFailed(_ context.Context, id uuid.UUID, errMsg string) (*queue.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	job.Status = queue.StatusFailed
	job.ErrorMessage = &errMsg
	return job, nil
}

func (m *mockJobStore) MarkJobDead(_ context.Context, id uuid.UUID, _ string) error {
	job, ok := m.jobs[id]
	if !ok {
		return store.ErrNotFound
	}
	job.Status = queue.StatusDead
	return nil
}

func (m *mockJobStore) CancelJob(_ context.Context, id uuid.UUID) error {
	if m.cancelErr != nil {
		return m.cancelErr
	}
	if _, ok := m.jobs[id]; !ok {
		return store.ErrConflict
	}
	delete(m.jobs, id)
	return nil
}

func (m *mockJobStore) ResetJobForRetry(_ context.Context, id uuid.UUID) (*queue.Job, error) {
	if m.retryErr != nil {
		return nil, m.retryErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, store.ErrConflict
	}
	job.Status = queue.StatusPending
	return job, nil
}

func (m *mockJobStore) InsertDLQ(_ context.Context, job *queue.Job) error {
	m.dlqItems[job.ID] = &queue.DLQEntry{ID: job.ID, Type: job.Type}
	return nil
}

func (m *mockJobStore) ListDLQ(_ context.Context, filter store.DLQFilter) (store.Page[*queue.DLQEntry], error) {
	items := make([]*queue.DLQEntry, 0)
	for _, e := range m.dlqItems {
		items = append(items, e)
	}
	return store.Page[*queue.DLQEntry]{
		Items:      items,
		TotalCount: int64(len(items)),
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}, nil
}

func (m *mockJobStore) GetDLQEntry(_ context.Context, id uuid.UUID) (*queue.DLQEntry, error) {
	entry, ok := m.dlqItems[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return entry, nil
}

func (m *mockJobStore) MarkDLQRequeued(_ context.Context, _, _ uuid.UUID) error { return nil }

func (m *mockJobStore) UpsertWorker(_ context.Context, _ string) error { return nil }
func (m *mockJobStore) UpdateWorkerHeartbeat(_ context.Context, _ string, _ *uuid.UUID) error {
	return nil
}
func (m *mockJobStore) UpdateWorkerStats(_ context.Context, _ string, _, _ int64) error { return nil }
func (m *mockJobStore) MarkWorkerStopped(_ context.Context, _ string) error             { return nil }
func (m *mockJobStore) ListWorkers(_ context.Context, _ bool) ([]*queue.WorkerInfo, error) {
	return m.workers, nil
}
func (m *mockJobStore) GetStats(_ context.Context) (store.JobStats, error) {
	return store.JobStats{}, nil
}
func (m *mockJobStore) Ping(_ context.Context) error { return nil }
func (m *mockJobStore) CreateWebhook(_ context.Context, _, _ string, _ []string, _ bool) (*queue.Webhook, error) {
	return &queue.Webhook{}, nil
}
func (m *mockJobStore) ListWebhooks(_ context.Context) ([]*queue.Webhook, error) {
	return []*queue.Webhook{}, nil
}
func (m *mockJobStore) ListEnabledWebhooks(_ context.Context) ([]*queue.Webhook, error) {
	return []*queue.Webhook{}, nil
}
func (m *mockJobStore) DeleteWebhook(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockJobStore) CreateCronSchedule(_ context.Context, s *queue.CronSchedule) (*queue.CronSchedule, error) {
	return s, nil
}
func (m *mockJobStore) ListCronSchedules(_ context.Context) ([]*queue.CronSchedule, error) {
	return []*queue.CronSchedule{}, nil
}
func (m *mockJobStore) ListDueCronSchedules(_ context.Context, _ time.Time) ([]*queue.CronSchedule, error) {
	return []*queue.CronSchedule{}, nil
}
func (m *mockJobStore) UpdateCronRun(_ context.Context, _ uuid.UUID, _, _ time.Time) error {
	return nil
}
func (m *mockJobStore) DeleteCronSchedule(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockJobStore) CreateAPIKey(_ context.Context, name string, tier queue.APIKeyTier) (*queue.APIKey, string, error) {
	return &queue.APIKey{Name: name, Tier: tier}, "qly_test", nil
}
func (m *mockJobStore) GetAPIKeyByHash(_ context.Context, _ string) (*queue.APIKey, error) {
	return nil, store.ErrNotFound
}
func (m *mockJobStore) ListAPIKeys(_ context.Context) ([]*queue.APIKey, error) {
	return []*queue.APIKey{}, nil
}
func (m *mockJobStore) IncrementAPIKeyUsage(_ context.Context, _ string) (*queue.APIKey, error) {
	return &queue.APIKey{}, nil
}
func (m *mockJobStore) DeleteAPIKey(_ context.Context, _ uuid.UUID) error { return nil }

// mockBroker records enqueue/remove calls for assertion.
type mockBroker struct {
	enqueued []string
	removed  []string
	pingErr  error
}

func (b *mockBroker) Enqueue(_ context.Context, job *queue.Job) error {
	b.enqueued = append(b.enqueued, job.ID.String())
	return nil
}
func (b *mockBroker) Dequeue(_ context.Context, _ string) (string, error) { return "", nil }
func (b *mockBroker) Remove(_ context.Context, _, jobID string) error {
	b.removed = append(b.removed, jobID)
	return nil
}
func (b *mockBroker) PromoteDelayed(_ context.Context) (int64, error) { return 0, nil }
func (b *mockBroker) QueueDepth(_ context.Context, _ string) (int64, error) {
	return int64(len(b.enqueued)), nil
}
func (b *mockBroker) DelayedDepth(_ context.Context) (int64, error)   { return 0, nil }
func (b *mockBroker) Ping(_ context.Context) error                     { return b.pingErr }

// ─── Helper ───────────────────────────────────────────────────────────────────

func newJobHandlerWithMocks(t *testing.T) (*JobHandler, *mockJobStore, *mockBroker) {
	t.Helper()
	s := newMockJobStore()
	b := &mockBroker{}
	h := NewJobHandler(s, b, nil, 5)
	return h, s, b
}

// ─── EnqueueJob tests ─────────────────────────────────────────────────────────

func TestEnqueueJob_Success(t *testing.T) {
	t.Parallel()

	handler, _, broker := newJobHandlerWithMocks(t)

	body := `{"type":"send_email","payload":{"to":"a@b.com"},"priority":8}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.EnqueueJob(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Len(t, broker.enqueued, 1, "one job should be pushed to the broker")

	var resp response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Nil(t, resp.Error)
}

func TestEnqueueJob_MissingType(t *testing.T) {
	t.Parallel()

	handler, _, _ := newJobHandlerWithMocks(t)

	body := `{"payload":{"to":"a@b.com"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.EnqueueJob(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestEnqueueJob_InvalidJSON(t *testing.T) {
	t.Parallel()

	handler, _, _ := newJobHandlerWithMocks(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.EnqueueJob(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEnqueueJob_StoreError(t *testing.T) {
	t.Parallel()

	handler, mockStore, _ := newJobHandlerWithMocks(t)
	mockStore.createErr = errors.New("db connection lost")

	body := `{"type":"noop"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.EnqueueJob(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ─── GetJob tests ─────────────────────────────────────────────────────────────

func TestGetJob_Found(t *testing.T) {
	t.Parallel()

	handler, mockStore, _ := newJobHandlerWithMocks(t)

	jobID := uuid.New()
	mockStore.jobs[jobID] = &queue.Job{
		ID:     jobID,
		Type:   "noop",
		Status: queue.StatusPending,
	}

	r := chi.NewRouter()
	r.Get("/api/v1/jobs/{id}", handler.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetJob_NotFound(t *testing.T) {
	t.Parallel()

	handler, _, _ := newJobHandlerWithMocks(t)

	r := chi.NewRouter()
	r.Get("/api/v1/jobs/{id}", handler.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetJob_InvalidUUID(t *testing.T) {
	t.Parallel()

	handler, _, _ := newJobHandlerWithMocks(t)

	r := chi.NewRouter()
	r.Get("/api/v1/jobs/{id}", handler.GetJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ─── CancelJob tests ──────────────────────────────────────────────────────────

func TestCancelJob_PendingJob(t *testing.T) {
	t.Parallel()

	handler, mockStore, broker := newJobHandlerWithMocks(t)

	jobID := uuid.New()
	mockStore.jobs[jobID] = &queue.Job{
		ID:        jobID,
		Type:      "noop",
		Status:    queue.StatusPending,
		QueueName: "default",
	}

	r := chi.NewRouter()
	r.Delete("/api/v1/jobs/{id}", handler.CancelJob)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+jobID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, broker.removed, jobID.String(), "job should be removed from broker")
}

func TestCancelJob_RunningJobReturnsConflict(t *testing.T) {
	t.Parallel()

	handler, mockStore, _ := newJobHandlerWithMocks(t)

	jobID := uuid.New()
	mockStore.jobs[jobID] = &queue.Job{
		ID:     jobID,
		Type:   "noop",
		Status: queue.StatusRunning,
	}

	r := chi.NewRouter()
	r.Delete("/api/v1/jobs/{id}", handler.CancelJob)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/jobs/"+jobID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

// ─── ListJobs pagination tests ────────────────────────────────────────────────

func TestListJobs_DefaultPagination(t *testing.T) {
	t.Parallel()

	handler, _, _ := newJobHandlerWithMocks(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()

	handler.ListJobs(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}
