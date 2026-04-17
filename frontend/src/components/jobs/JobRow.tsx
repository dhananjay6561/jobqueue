import { motion } from 'framer-motion'
import type { Job } from '@/types'
import { JobStatusBadge } from './JobStatusBadge'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { truncateId, formatRelativeTime, formatDuration } from '@/utils/formatters'

interface JobRowProps {
  job: Job
  isNew?: boolean
  onClick: (job: Job) => void
}

function PriorityBadge({ priority }: { priority: number }) {
  const variant =
    priority >= 8 ? 'critical' : priority >= 5 ? 'warning' : priority >= 3 ? 'info' : 'muted'
  return (
    <Badge variant={variant as 'critical' | 'warning' | 'info' | 'muted'}>P{priority}</Badge>
  )
}

export function JobRow({ job, isNew = false, onClick }: JobRowProps) {
  const duration = formatDuration(job.started_at ?? job.created_at, job.completed_at)

  return (
    <motion.tr
      layout
      initial={isNew ? { backgroundColor: 'rgba(124, 106, 247, 0.15)' } : undefined}
      animate={{ backgroundColor: 'rgba(0,0,0,0)' }}
      transition={{ duration: 1.5 }}
      onClick={() => onClick(job)}
      className="group border-b border-[#1e1e2e] hover:bg-white/[0.02] cursor-pointer transition-colors relative"
    >
      {/* Hover accent line */}
      <td className="w-0 p-0">
        <span className="absolute left-0 top-0 h-full w-0.5 bg-[#7c6af7] opacity-0 group-hover:opacity-100 transition-opacity rounded-r-full" />
      </td>

      <td className="pl-5 pr-4 py-3">
        <span className="font-mono text-xs text-[#6b6b8a] hover:text-[#7c6af7] transition-colors">
          {truncateId(job.id)}
        </span>
      </td>

      <td className="px-4 py-3">
        <span className="text-[13px] text-[#e2e2f0] font-mono">{job.type}</span>
      </td>

      <td className="px-4 py-3">
        <PriorityBadge priority={job.priority} />
      </td>

      <td className="px-4 py-3">
        <JobStatusBadge status={job.status} />
      </td>

      <td className="px-4 py-3">
        <span className="text-xs text-[#6b6b8a] font-mono">{formatRelativeTime(job.created_at)}</span>
      </td>

      <td className="px-4 py-3">
        <span className="text-xs text-[#6b6b8a] font-mono">{duration}</span>
      </td>

      <td className="pl-4 pr-5 py-3" onClick={(e) => e.stopPropagation()}>
        <Button variant="ghost" size="sm" onClick={() => onClick(job)}>
          Details
        </Button>
      </td>
    </motion.tr>
  )
}
