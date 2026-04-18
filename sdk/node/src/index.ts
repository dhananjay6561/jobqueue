// JobQueue Node.js SDK
// Works in Node.js 18+ (native fetch) and any environment with a global fetch.

export interface Job {
  id: string
  type: string
  payload: Record<string, unknown>
  priority: number
  status: 'pending' | 'running' | 'completed' | 'failed' | 'dead' | 'cancelled'
  attempts: number
  max_attempts: number
  queue_name: string
  scheduled_at: string
  created_at: string
  started_at?: string
  completed_at?: string
  worker_id?: string
  error_message?: string
  result?: Record<string, unknown>
  api_key_id?: string
}

export interface EnqueueRequest {
  type: string
  payload?: Record<string, unknown>
  priority?: number
  max_attempts?: number
  queue_name?: string
  scheduled_at?: string
}

export interface ListJobsParams {
  status?: string
  type?: string
  queue?: string
  limit?: number
  offset?: number
}

export interface Page<T> {
  items: T[]
  total_count: number
  limit: number
  offset: number
  has_more: boolean
}

export interface Stats {
  total_jobs: number
  pending: number
  running: number
  completed: number
  failed: number
  dead: number
  cancelled: number
  active_workers: number
  jobs_per_minute: number
  failed_rate: number
  queue_depth: number
  dlq_count: number
}

export interface DLQEntry {
  id: string
  type: string
  payload: Record<string, unknown>
  priority: number
  queue_name: string
  max_attempts: number
  total_attempts: number
  last_error?: string
  died_at: string
  original_created_at: string
  requeued: boolean
  api_key_id?: string
}

export interface Webhook {
  id: string
  url: string
  secret?: string
  events: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateWebhookRequest {
  url: string
  secret?: string
  events?: string[]
  enabled?: boolean
}

export interface CronSchedule {
  id: string
  name: string
  job_type: string
  payload: Record<string, unknown>
  queue_name: string
  priority: number
  max_attempts: number
  cron_expression: string
  enabled: boolean
  last_run_at?: string
  next_run_at: string
  created_at: string
}

export interface CreateCronRequest {
  name: string
  job_type: string
  payload?: Record<string, unknown>
  queue_name?: string
  priority?: number
  max_attempts?: number
  cron_expression: string
  enabled?: boolean
}

export interface PatchCronRequest {
  enabled?: boolean
  cron_expression?: string
  payload?: Record<string, unknown>
}

export interface APIKey {
  id: string
  name: string
  key_prefix: string
  tier: 'free' | 'pro' | 'business'
  jobs_used: number
  jobs_limit: number
  reset_at: string
  enabled: boolean
  created_at: string
}

export interface ClientOptions {
  apiKey?: string
  timeout?: number
  fetch?: typeof globalThis.fetch
}

interface ApiResponse<T> {
  data: T
  error?: string
  meta?: {
    request_id?: string
    total_count?: number
    limit?: number
    offset?: number
    has_more?: boolean
  }
}

export class JobQueueError extends Error {
  constructor(message: string, public readonly status?: number) {
    super(message)
    this.name = 'JobQueueError'
  }
}

export class JobQueueClient {
  private readonly baseURL: string
  private readonly apiKey: string
  private readonly timeout: number
  private readonly _fetch: typeof globalThis.fetch

  constructor(baseURL: string, options: ClientOptions = {}) {
    this.baseURL = baseURL.replace(/\/$/, '')
    this.apiKey = options.apiKey ?? ''
    this.timeout = options.timeout ?? 30_000
    this._fetch = options.fetch ?? globalThis.fetch.bind(globalThis)
  }

  // ── Jobs ────────────────────────────────────────────────────────────────────

  async enqueue(req: EnqueueRequest): Promise<Job> {
    const body: EnqueueRequest = {
      priority: 5,
      max_attempts: 3,
      queue_name: 'default',
      ...req,
    }
    return this._post<Job>('/api/v1/jobs', body)
  }

  async enqueueBatch(reqs: EnqueueRequest[]): Promise<Job[]> {
    return this._post<Job[]>('/api/v1/jobs/batch', reqs)
  }

  async getJob(id: string): Promise<Job> {
    return this._get<Job>(`/api/v1/jobs/${id}`)
  }

  async listJobs(params: ListJobsParams = {}): Promise<Page<Job>> {
    const q = new URLSearchParams()
    if (params.status) q.set('status', params.status)
    if (params.type) q.set('type', params.type)
    if (params.queue) q.set('queue', params.queue)
    q.set('limit', String(params.limit ?? 20))
    q.set('offset', String(params.offset ?? 0))
    return this._getPage<Job>(`/api/v1/jobs?${q}`)
  }

  async cancelJob(id: string): Promise<void> {
    return this._delete(`/api/v1/jobs/${id}`)
  }

  async retryJob(id: string): Promise<Job> {
    return this._post<Job>(`/api/v1/jobs/${id}/retry`, null)
  }

  async getJobResult(id: string): Promise<Record<string, unknown> | null> {
    const res = await this._request('GET', `/api/v1/jobs/${id}/result`)
    if (res.status === 204) return null
    if (!res.ok) {
      const body = await res.json().catch(() => ({})) as { error?: string }
      throw new JobQueueError(body.error ?? `HTTP ${res.status}`, res.status)
    }
    return res.json() as Promise<Record<string, unknown>>
  }

  // ── Stats ───────────────────────────────────────────────────────────────────

  async getStats(): Promise<Stats> {
    return this._get<Stats>('/api/v1/stats')
  }

  // ── DLQ ─────────────────────────────────────────────────────────────────────

  async listDLQ(limit = 20, offset = 0): Promise<Page<DLQEntry>> {
    return this._getPage<DLQEntry>(`/api/v1/dlq?limit=${limit}&offset=${offset}`)
  }

  async requeueDLQ(dlqId: string): Promise<{ new_job: Job; dlq_entry_id: string }> {
    return this._post(`/api/v1/dlq/${dlqId}/requeue`, null)
  }

  // ── Webhooks ─────────────────────────────────────────────────────────────────

  async listWebhooks(): Promise<Webhook[]> {
    return this._get<Webhook[]>('/api/v1/webhooks')
  }

  async createWebhook(req: CreateWebhookRequest): Promise<Webhook> {
    return this._post<Webhook>('/api/v1/webhooks', req)
  }

  async deleteWebhook(id: string): Promise<void> {
    return this._delete(`/api/v1/webhooks/${id}`)
  }

  // ── Cron ─────────────────────────────────────────────────────────────────────

  async listCron(): Promise<CronSchedule[]> {
    return this._get<CronSchedule[]>('/api/v1/cron')
  }

  async createCron(req: CreateCronRequest): Promise<CronSchedule> {
    return this._post<CronSchedule>('/api/v1/cron', req)
  }

  async patchCron(id: string, patch: PatchCronRequest): Promise<CronSchedule> {
    return this._patch<CronSchedule>(`/api/v1/cron/${id}`, patch)
  }

  async deleteCron(id: string): Promise<void> {
    return this._delete(`/api/v1/cron/${id}`)
  }

  // ── API Keys ──────────────────────────────────────────────────────────────────

  async listAPIKeys(): Promise<APIKey[]> {
    return this._get<APIKey[]>('/api/v1/keys')
  }

  async createAPIKey(name: string, tier: APIKey['tier'] = 'free'): Promise<{ key: APIKey; raw_key: string }> {
    return this._post('/api/v1/keys', { name, tier })
  }

  async deleteAPIKey(id: string): Promise<void> {
    return this._delete(`/api/v1/keys/${id}`)
  }

  async getUsage(): Promise<APIKey> {
    return this._get<APIKey>('/api/v1/usage')
  }

  // ── Health ────────────────────────────────────────────────────────────────────

  async health(): Promise<{ status: string; checks: Record<string, string>; uptime: string }> {
    const res = await this._request('GET', '/health')
    if (!res.ok) throw new JobQueueError(`unhealthy (status ${res.status})`, res.status)
    return res.json()
  }

  // ── Internal helpers ───────────────────────────────────────────────────────────

  private _headers(): Record<string, string> {
    const h: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    }
    if (this.apiKey) h['X-API-Key'] = this.apiKey
    return h
  }

  private async _request(method: string, path: string, body?: unknown): Promise<Response> {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeout)
    try {
      return await this._fetch(this.baseURL + path, {
        method,
        headers: this._headers(),
        body: body != null ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      })
    } finally {
      clearTimeout(timer)
    }
  }

  private async _unwrap<T>(res: Response): Promise<T> {
    const envelope = await res.json() as ApiResponse<T>
    if (envelope.error) throw new JobQueueError(envelope.error, res.status)
    return envelope.data
  }

  private async _get<T>(path: string): Promise<T> {
    const res = await this._request('GET', path)
    return this._unwrap<T>(res)
  }

  private async _getPage<T>(path: string): Promise<Page<T>> {
    const res = await this._request('GET', path)
    const envelope = await res.json() as ApiResponse<T[]>
    if (envelope.error) throw new JobQueueError(envelope.error, res.status)
    return {
      items: envelope.data ?? [],
      total_count: envelope.meta?.total_count ?? 0,
      limit: envelope.meta?.limit ?? 20,
      offset: envelope.meta?.offset ?? 0,
      has_more: envelope.meta?.has_more ?? false,
    }
  }

  private async _post<T>(path: string, body: unknown): Promise<T> {
    const res = await this._request('POST', path, body)
    return this._unwrap<T>(res)
  }

  private async _patch<T>(path: string, body: unknown): Promise<T> {
    const res = await this._request('PATCH', path, body)
    return this._unwrap<T>(res)
  }

  private async _delete(path: string): Promise<void> {
    const res = await this._request('DELETE', path)
    if (!res.ok) {
      const body = await res.json().catch(() => ({})) as { error?: string }
      throw new JobQueueError(body.error ?? `HTTP ${res.status}`, res.status)
    }
  }
}

export default JobQueueClient
