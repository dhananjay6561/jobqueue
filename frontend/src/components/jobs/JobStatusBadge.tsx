import { Badge } from '@/components/ui/Badge'
import type { JobStatus } from '@/types'

interface JobStatusBadgeProps {
  status: JobStatus
}

const STATUS_CONFIG: Record<
  JobStatus,
  {
    variant: 'success' | 'warning' | 'error' | 'info' | 'muted' | 'critical'
    label: string
    dot: boolean
    pulse: boolean
  }
> = {
  pending: { variant: 'info', label: 'Pending', dot: true, pulse: false },
  running: { variant: 'warning', label: 'Running', dot: true, pulse: true },
  completed: { variant: 'success', label: 'Completed', dot: true, pulse: false },
  failed: { variant: 'error', label: 'Failed', dot: true, pulse: false },
  dead: { variant: 'critical', label: 'Dead', dot: true, pulse: false },
  cancelled: { variant: 'muted', label: 'Cancelled', dot: false, pulse: false },
}

export function JobStatusBadge({ status }: JobStatusBadgeProps) {
  const config = STATUS_CONFIG[status]
  return (
    <Badge variant={config.variant} dot={config.dot} pulse={config.pulse}>
      {config.label}
    </Badge>
  )
}
