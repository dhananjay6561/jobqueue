// ─── Job Types ───────────────────────────────────────────────────────────────

export type JobStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'failed'
  | 'dead'
  | 'cancelled'

export type QueueName = 'default' | 'critical' | 'bulk' | 'delayed'

export interface Job {
  id: string
  type: string
  payload: Record<string, unknown>
  status: JobStatus
  priority: number
  queue_name: QueueName
  max_attempts: number
  attempts: number
  error_message: string | null
  scheduled_at: string
  started_at: string | null
  completed_at: string | null
  created_at: string
  updated_at: string
}

export interface JobListParams {
  limit?: number
  offset?: number
  status?: JobStatus
  type?: string
  queue?: QueueName
  priority_min?: number
  priority_max?: number
  sort_by?: 'created_at' | 'updated_at' | 'priority' | 'status'
  sort_dir?: 'asc' | 'desc'
}

export interface EnqueueJobRequest {
  type: string
  payload: Record<string, unknown>
  priority: number
  max_attempts: number
  queue_name: QueueName
  scheduled_at?: string
}

// ─── Worker Types ─────────────────────────────────────────────────────────────

export type WorkerStatus = 'active' | 'idle' | 'offline'

export interface Worker {
  id: string
  status: WorkerStatus
  current_job_id: string | null
  current_job_type: string | null
  jobs_processed: number
  last_seen: string
  started_at: string
}

// ─── Stats Types ──────────────────────────────────────────────────────────────

export interface QueueStats {
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
}

export interface ThroughputPoint {
  timestamp: string
  completed: number
  failed: number
}

export interface QueueDepthPoint {
  timestamp: string
  depth: number
}

// ─── DLQ Types ────────────────────────────────────────────────────────────────

export interface DLQEntry {
  id: string
  type: string
  payload: Record<string, unknown>
  priority: number
  queue_name: string
  max_attempts: number
  total_attempts: number
  last_error: string | null
  died_at: string
  original_created_at: string
  requeued: boolean
}

// ─── API Response Wrapper ─────────────────────────────────────────────────────

export interface ApiResponse<T> {
  data: T | null
  error: string | null
  meta: ApiMeta
}

export interface ApiMeta {
  request_id: string
  total_count?: number
  limit?: number
  offset?: number
  has_more?: boolean
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: ApiMeta
}

// ─── WebSocket Event Types ────────────────────────────────────────────────────

export type WsEventType =
  | 'job.enqueued'
  | 'job.started'
  | 'job.completed'
  | 'job.failed'
  | 'job.dead'
  | 'worker.heartbeat'
  | 'stats.update'

export interface WsEventBase {
  type: WsEventType
  timestamp: string
}

export interface WsJobEnqueuedEvent extends WsEventBase {
  type: 'job.enqueued'
  job: Job
}

export interface WsJobStartedEvent extends WsEventBase {
  type: 'job.started'
  job_id: string
  worker_id: string
  job_type: string
}

export interface WsJobCompletedEvent extends WsEventBase {
  type: 'job.completed'
  job_id: string
  job_type: string
  duration_ms: number
}

export interface WsJobFailedEvent extends WsEventBase {
  type: 'job.failed'
  job_id: string
  job_type: string
  attempt: number
  error: string
}

export interface WsJobDeadEvent extends WsEventBase {
  type: 'job.dead'
  job_id: string
  job_type: string
  error: string
}

export interface WsWorkerHeartbeatEvent extends WsEventBase {
  type: 'worker.heartbeat'
  worker_id: string
  status: WorkerStatus
  current_job_id: string | null
}

export interface WsStatsUpdateEvent extends WsEventBase {
  type: 'stats.update'
  stats: QueueStats
}

export type WsEvent =
  | WsJobEnqueuedEvent
  | WsJobStartedEvent
  | WsJobCompletedEvent
  | WsJobFailedEvent
  | WsJobDeadEvent
  | WsWorkerHeartbeatEvent
  | WsStatsUpdateEvent

// ─── UI State Types ───────────────────────────────────────────────────────────

export interface LiveEvent {
  id: string
  type: WsEventType
  message: string
  timestamp: string
  color: 'success' | 'error' | 'warning' | 'info' | 'muted'
}

export interface ToastMessage {
  id: string
  message: string
  variant: 'success' | 'error' | 'warning' | 'info'
  duration?: number
}
