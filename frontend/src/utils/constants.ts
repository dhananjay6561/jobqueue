import type { JobStatus, WsEventType } from '@/types'

export const JOB_STATUS_COLORS: Record<JobStatus, string> = {
  pending: 'text-blue-400 bg-blue-400/10 border-blue-400/20',
  running: 'text-amber-400 bg-amber-400/10 border-amber-400/20',
  completed: 'text-green-400 bg-green-400/10 border-green-400/20',
  failed: 'text-red-400 bg-red-400/10 border-red-400/20',
  dead: 'text-red-600 bg-red-600/10 border-red-600/20',
  cancelled: 'text-zinc-400 bg-zinc-400/10 border-zinc-400/20',
}

export const WS_EVENT_COLORS: Record<WsEventType, string> = {
  'job.enqueued': '#3b82f6',
  'job.started': '#f59e0b',
  'job.completed': '#22c55e',
  'job.failed': '#ef4444',
  'job.dead': '#dc2626',
  'worker.heartbeat': '#6b7280',
  'stats.update': '#6b7280',
}

export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
export const DEFAULT_PAGE_SIZE = 20

export const PRIORITY_MIN = 1
export const PRIORITY_MAX = 10

export const QUEUE_NAMES = ['default', 'critical', 'bulk', 'delayed'] as const

export const JOB_TYPES = [
  'send_email',
  'generate_report',
  'resize_image',
  'sync_data',
  'send_notification',
  'process_payment',
  'export_csv',
  'cleanup_storage',
]

export const THROUGHPUT_WINDOW_MINUTES = 30
