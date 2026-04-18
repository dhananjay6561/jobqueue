import { useState } from 'react'
import type { Job, JobStatus } from '@/types'
import { JobTable } from '@/components/jobs/JobTable'
import { JobDetailDrawer } from '@/components/jobs/JobDetailDrawer'
import { EnqueueJobModal } from '@/components/jobs/EnqueueJobModal'
import { BatchEnqueueModal } from '@/components/jobs/BatchEnqueueModal'
import { Button } from '@/components/ui/Button'
import { useUiStore } from '@/store/uiStore'
import { QUEUE_NAMES } from '@/utils/constants'

const STATUS_FILTERS: Array<{ value: JobStatus | 'all'; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'dead', label: 'Dead' },
  { value: 'cancelled', label: 'Cancelled' },
]

export function Jobs() {
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)
  const [enqueueOpen, setEnqueueOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const { jobFilters, setJobFilters, resetJobFilters } = useUiStore()

  const inputClass =
    'bg-[#111118] border border-[#1e1e2e] rounded-lg px-3 py-1.5 text-xs text-[#e2e2f0] font-mono focus:outline-none focus:border-[#7c6af7]/50 transition-colors'

  return (
    <div className="p-6 space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-[#e2e2f0]">Jobs</h1>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="md" onClick={() => setBatchOpen(true)}>
            Batch
          </Button>
          <Button variant="primary" size="md" onClick={() => setEnqueueOpen(true)}>
            <svg className="w-4 h-4" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M8 3v10M3 8h10" />
            </svg>
            Enqueue Job
          </Button>
        </div>
      </div>

      {/* Filter Bar */}
      <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-4">
        <div className="flex flex-wrap items-center gap-3">
          {/* Status tabs */}
          <div className="flex items-center gap-1 bg-[#0a0a0f] border border-[#1e1e2e] rounded-lg p-0.5">
            {STATUS_FILTERS.map((f) => (
              <button
                key={f.value}
                onClick={() => setJobFilters({ status: f.value, page: 0 })}
                className={`px-3 py-1.5 rounded-md text-[11px] font-mono transition-all cursor-pointer ${
                  jobFilters.status === f.value
                    ? 'bg-[#7c6af7] text-white'
                    : 'text-[#6b6b8a] hover:text-[#e2e2f0]'
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>

          {/* Type filter */}
          <input
            type="text"
            placeholder="Filter by type…"
            className={inputClass}
            value={jobFilters.type}
            onChange={(e) => setJobFilters({ type: e.target.value, page: 0 })}
          />

          {/* Queue filter */}
          <select
            className={inputClass}
            onChange={() => setJobFilters({ page: 0 })}
            aria-label="Filter by queue"
          >
            <option value="">All queues</option>
            {QUEUE_NAMES.map((q) => (
              <option key={q} value={q}>{q}</option>
            ))}
          </select>

          {/* Reset */}
          <Button variant="ghost" size="sm" onClick={resetJobFilters}>
            Reset
          </Button>
        </div>
      </div>

      {/* Table */}
      <JobTable onRowClick={(job: Job) => setSelectedJobId(job.id)} />

      {/* Modals / Drawers */}
      <JobDetailDrawer jobId={selectedJobId} onClose={() => setSelectedJobId(null)} />
      <EnqueueJobModal isOpen={enqueueOpen} onClose={() => setEnqueueOpen(false)} />
      <BatchEnqueueModal isOpen={batchOpen} onClose={() => setBatchOpen(false)} />
    </div>
  )
}
