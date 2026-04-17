import { formatRelativeTime } from '@/utils/formatters'
import type { Worker } from '@/types'
import { Badge } from '@/components/ui/Badge'

interface WorkerCardProps {
  worker: Worker
}

const STATUS_CONFIG: Record<
  Worker['status'],
  { label: string; variant: 'success' | 'warning' | 'muted'; pulse: boolean }
> = {
  active: { label: 'Active', variant: 'success', pulse: true },
  idle: { label: 'Idle', variant: 'muted', pulse: false },
  offline: { label: 'Offline', variant: 'muted', pulse: false },
}

export function WorkerCard({ worker }: WorkerCardProps) {
  const statusCfg = STATUS_CONFIG[worker.status]

  // Consider worker stale if last heartbeat >30s ago
  const lastSeenMs = Date.now() - new Date(worker.last_seen).getTime()
  const isStale = lastSeenMs > 30_000

  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-5 hover:border-[#2a2a3e] transition-colors group">
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-2.5">
          {/* Worker icon */}
          <div className="w-8 h-8 rounded-lg bg-[#7c6af7]/10 border border-[#7c6af7]/20 flex items-center justify-center">
            <svg className="w-4 h-4 text-[#7c6af7]" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM3 19a7 7 0 0114 0" />
            </svg>
          </div>
          <div>
            <p className="text-xs font-semibold text-[#e2e2f0] font-mono">
              {worker.id.slice(0, 12)}…
            </p>
            <p className="text-[10px] text-[#6b6b8a]">
              {formatRelativeTime(worker.last_seen)}
              {isStale && <span className="text-amber-400 ml-1">• stale</span>}
            </p>
          </div>
        </div>
        <Badge variant={statusCfg.variant} dot pulse={statusCfg.pulse}>
          {statusCfg.label}
        </Badge>
      </div>

      {/* Current job */}
      <div className="mb-4">
        {worker.current_job_id ? (
          <div className="bg-amber-400/5 border border-amber-400/20 rounded-lg px-3 py-2">
            <p className="text-[10px] text-[#6b6b8a] mb-0.5">Working on</p>
            <p className="text-xs text-amber-300 font-mono font-medium">
              {worker.current_job_type ?? 'unknown'}
            </p>
            <p className="text-[10px] text-[#6b6b8a] font-mono mt-0.5">
              {worker.current_job_id.slice(0, 12)}…
            </p>
          </div>
        ) : (
          <div className="bg-white/[0.02] border border-[#1e1e2e] rounded-lg px-3 py-2">
            <p className="text-[10px] text-[#6b6b8a]">No current job</p>
          </div>
        )}
      </div>

      {/* Stats */}
      <div className="flex items-center gap-4 text-[11px] text-[#6b6b8a] font-mono">
        <span>
          <span className="text-[#e2e2f0] font-semibold">{worker.jobs_processed}</span> processed
        </span>
        <span>
          Started {formatRelativeTime(worker.started_at)}
        </span>
      </div>
    </div>
  )
}
