interface Job {
    id: string;
    type: string;
    payload: Record<string, unknown>;
    priority: number;
    status: 'pending' | 'running' | 'completed' | 'failed' | 'dead' | 'cancelled';
    attempts: number;
    max_attempts: number;
    queue_name: string;
    scheduled_at: string;
    created_at: string;
    started_at?: string;
    completed_at?: string;
    worker_id?: string;
    error_message?: string;
    result?: Record<string, unknown>;
    api_key_id?: string;
}
interface EnqueueRequest {
    type: string;
    payload?: Record<string, unknown>;
    priority?: number;
    max_attempts?: number;
    queue_name?: string;
    scheduled_at?: string;
}
interface ListJobsParams {
    status?: string;
    type?: string;
    queue?: string;
    limit?: number;
    offset?: number;
}
interface Page<T> {
    items: T[];
    total_count: number;
    limit: number;
    offset: number;
    has_more: boolean;
}
interface Stats {
    total_jobs: number;
    pending: number;
    running: number;
    completed: number;
    failed: number;
    dead: number;
    cancelled: number;
    active_workers: number;
    jobs_per_minute: number;
    failed_rate: number;
    queue_depth: number;
    dlq_count: number;
}
interface DLQEntry {
    id: string;
    type: string;
    payload: Record<string, unknown>;
    priority: number;
    queue_name: string;
    max_attempts: number;
    total_attempts: number;
    last_error?: string;
    died_at: string;
    original_created_at: string;
    requeued: boolean;
    api_key_id?: string;
}
interface Webhook {
    id: string;
    url: string;
    secret?: string;
    events: string[];
    enabled: boolean;
    created_at: string;
    updated_at: string;
}
interface CreateWebhookRequest {
    url: string;
    secret?: string;
    events?: string[];
    enabled?: boolean;
}
interface CronSchedule {
    id: string;
    name: string;
    job_type: string;
    payload: Record<string, unknown>;
    queue_name: string;
    priority: number;
    max_attempts: number;
    cron_expression: string;
    enabled: boolean;
    last_run_at?: string;
    next_run_at: string;
    created_at: string;
}
interface CreateCronRequest {
    name: string;
    job_type: string;
    payload?: Record<string, unknown>;
    queue_name?: string;
    priority?: number;
    max_attempts?: number;
    cron_expression: string;
    enabled?: boolean;
}
interface PatchCronRequest {
    enabled?: boolean;
    cron_expression?: string;
    payload?: Record<string, unknown>;
}
interface APIKey {
    id: string;
    name: string;
    key_prefix: string;
    tier: 'free' | 'pro' | 'business';
    jobs_used: number;
    jobs_limit: number;
    reset_at: string;
    enabled: boolean;
    created_at: string;
}
interface ClientOptions {
    apiKey?: string;
    timeout?: number;
    fetch?: typeof globalThis.fetch;
}
declare class JobQueueError extends Error {
    readonly status?: number | undefined;
    constructor(message: string, status?: number | undefined);
}
declare class JobQueueClient {
    private readonly baseURL;
    private readonly apiKey;
    private readonly timeout;
    private readonly _fetch;
    constructor(baseURL: string, options?: ClientOptions);
    enqueue(req: EnqueueRequest): Promise<Job>;
    enqueueBatch(reqs: EnqueueRequest[]): Promise<Job[]>;
    getJob(id: string): Promise<Job>;
    listJobs(params?: ListJobsParams): Promise<Page<Job>>;
    cancelJob(id: string): Promise<void>;
    retryJob(id: string): Promise<Job>;
    getJobResult(id: string): Promise<Record<string, unknown> | null>;
    getStats(): Promise<Stats>;
    listDLQ(limit?: number, offset?: number): Promise<Page<DLQEntry>>;
    requeueDLQ(dlqId: string): Promise<{
        new_job: Job;
        dlq_entry_id: string;
    }>;
    listWebhooks(): Promise<Webhook[]>;
    createWebhook(req: CreateWebhookRequest): Promise<Webhook>;
    deleteWebhook(id: string): Promise<void>;
    listCron(): Promise<CronSchedule[]>;
    createCron(req: CreateCronRequest): Promise<CronSchedule>;
    patchCron(id: string, patch: PatchCronRequest): Promise<CronSchedule>;
    deleteCron(id: string): Promise<void>;
    listAPIKeys(): Promise<APIKey[]>;
    createAPIKey(name: string, tier?: APIKey['tier']): Promise<{
        key: APIKey;
        raw_key: string;
    }>;
    deleteAPIKey(id: string): Promise<void>;
    getUsage(): Promise<APIKey>;
    health(): Promise<{
        status: string;
        checks: Record<string, string>;
        uptime: string;
    }>;
    private _headers;
    private _request;
    private _unwrap;
    private _get;
    private _getPage;
    private _post;
    private _patch;
    private _delete;
}

export { type APIKey, type ClientOptions, type CreateCronRequest, type CreateWebhookRequest, type CronSchedule, type DLQEntry, type EnqueueRequest, type Job, JobQueueClient, JobQueueError, type ListJobsParams, type Page, type PatchCronRequest, type Stats, type Webhook, JobQueueClient as default };
