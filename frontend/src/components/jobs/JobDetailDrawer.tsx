import { Drawer } from '@/components/ui/Drawer'
import { Button } from '@/components/ui/Button'
import { JobStatusBadge } from './JobStatusBadge'
import { Badge } from '@/components/ui/Badge'
import { useJob, useRetryJob, useCancelJob } from '@/hooks/useJobs'
import { formatDateTime, formatDuration, formatPayload, truncateId } from '@/utils/formatters'
import { Skeleton } from '@/components/ui/Skeleton'

interface JobDetailDrawerProps {
  jobId: string | null
  onClose: () => void
}

interface TimelineStep {
  label: string
  time: string | null
  done: boolean
}

function Timeline({ job }: { job: { created_at: string; started_at: string | null; completed_at: string | null; status: string } }) {
  const steps: TimelineStep[] = [
    { label: 'Enqueued', time: job.created_at, done: true },
    { label: 'Started', time: job.started_at, done: !!job.started_at },
    {
      label: job.status === 'failed' || job.status === 'dead' ? 'Failed' : 'Completed',
      time: job.completed_at,
      done: !!job.completed_at,
    },
  ]

  return (
    <div className="relative">
      {steps.map((step, i) => (
        <div key={step.label} className="flex gap-4 pb-5 last:pb-0">
          {/* dot + line */}
          <div className="flex flex-col items-center">
            <div
              className={`w-2.5 h-2.5 rounded-full mt-0.5 shrink-0 ${
                step.done ? 'bg-[#7c6af7]' : 'bg-[#1e1e2e] border border-[#2a2a3e]'
              }`}
            />
            {i < steps.length - 1 && (
              <div className={`w-px flex-1 mt-1 ${step.done ? 'bg-[#7c6af7]/30' : 'bg-[#1e1e2e]'}`} />
            )}
          </div>
          <div className="pb-1">
            <p className={`text-xs font-medium ${step.done ? 'text-[#e2e2f0]' : 'text-[#6b6b8a]'}`}>
              {step.label}
            </p>
            {step.time && (
              <p className="text-[11px] text-[#6b6b8a] font-mono mt-0.5">
                {formatDateTime(step.time)}
              </p>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="border-b border-[#1e1e2e] py-5 px-6 last:border-0">
      <h3 className="text-[10px] text-[#6b6b8a] uppercase tracking-widest mb-3 font-mono">{title}</h3>
      {children}
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 mb-2.5 last:mb-0">
      <span className="text-xs text-[#6b6b8a] shrink-0">{label}</span>
      <span className="text-xs text-[#e2e2f0] text-right font-mono break-all">{value}</span>
    </div>
  )
}

export function JobDetailDrawer({ jobId, onClose }: JobDetailDrawerProps) {
  const { data: job, isLoading } = useJob(jobId ?? '')
  const retryJob = useRetryJob()
  const cancelJob = useCancelJob()

  return (
    <Drawer isOpen={!!jobId} onClose={onClose} title="Job Details">
      {isLoading && (
        <div className="p-6 space-y-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-4 w-full" />
          ))}
        </div>
      )}

      {job && (
        <div>
          {/* Header info */}
          <Section title="Overview">
            <InfoRow label="ID" value={<span className="text-[#7c6af7]">{truncateId(job.id)}</span>} />
            <InfoRow label="Full ID" value={job.id} />
            <InfoRow label="Type" value={job.type} />
            <InfoRow label="Status" value={<JobStatusBadge status={job.status} />} />
            <InfoRow
              label="Queue"
              value={<Badge variant="accent">{job.queue_name}</Badge>}
            />
            <InfoRow label="Priority" value={`P${job.priority}`} />
            <InfoRow label="Attempts" value={`${job.attempts} / ${job.max_attempts}`} />
            <InfoRow
              label="Duration"
              value={formatDuration(job.started_at ?? job.created_at, job.completed_at)}
            />
          </Section>

          {/* Timeline */}
          <Section title="Timeline">
            <Timeline job={job} />
          </Section>

          {/* Error */}
          {job.error_message && (
            <Section title="Error">
              <div className="bg-red-500/5 border border-red-500/20 rounded-lg p-3">
                <p className="text-xs text-red-300 font-mono whitespace-pre-wrap break-all leading-relaxed">
                  {job.error_message}
                </p>
              </div>
            </Section>
          )}

          {/* Payload */}
          <Section title="Payload">
            <div className="bg-[#0a0a0f] border border-[#1e1e2e] rounded-lg p-3 overflow-x-auto">
              <pre className="text-[11px] text-[#7c6af7] font-mono leading-relaxed whitespace-pre">
                {formatPayload(job.payload)}
              </pre>
            </div>
          </Section>

          {/* Actions */}
          <Section title="Actions">
            <div className="flex gap-2 flex-wrap">
              {job.status === 'failed' && (
                <Button
                  variant="primary"
                  size="sm"
                  isLoading={retryJob.isPending}
                  onClick={() => retryJob.mutate(job.id)}
                >
                  Retry Job
                </Button>
              )}
              {job.status === 'pending' && (
                <Button
                  variant="danger"
                  size="sm"
                  isLoading={cancelJob.isPending}
                  onClick={() => {
                    cancelJob.mutate(job.id)
                    onClose()
                  }}
                >
                  Cancel Job
                </Button>
              )}
              <Button variant="ghost" size="sm" onClick={onClose}>
                Close
              </Button>
            </div>
          </Section>
        </div>
      )}
    </Drawer>
  )
}
