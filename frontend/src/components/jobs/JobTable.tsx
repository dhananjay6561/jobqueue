import type { Job, JobListParams } from '@/types'
import { useJobs } from '@/hooks/useJobs'
import { JobRow } from './JobRow'
import { Button } from '@/components/ui/Button'
import { SkeletonRow } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'
import { useUiStore } from '@/store/uiStore'

interface JobTableProps {
  filterParams?: Partial<JobListParams>
  onRowClick?: (job: Job) => void
  compact?: boolean
}

const SORT_COLUMNS = [
  { key: 'created_at', label: 'Created' },
  { key: 'updated_at', label: 'Updated' },
  { key: 'priority', label: 'Priority' },
  { key: 'status', label: 'Status' },
] as const

type SortKey = (typeof SORT_COLUMNS)[number]['key']

export function JobTable({ filterParams = {}, onRowClick, compact = false }: JobTableProps) {
  const { jobFilters, setJobFilters } = useUiStore()

  const params: JobListParams = {
    limit: compact ? 10 : jobFilters.pageSize,
    offset: compact ? 0 : jobFilters.page * jobFilters.pageSize,
    status: jobFilters.status === 'all' ? undefined : jobFilters.status,
    type: jobFilters.type || undefined,
    sort_by: jobFilters.sortBy,
    sort_dir: jobFilters.sortDir,
    ...filterParams,
  }

  const { data, isLoading, error } = useJobs(params)

  const handleSort = (key: SortKey) => {
    setJobFilters({
      sortBy: key,
      sortDir: jobFilters.sortBy === key && jobFilters.sortDir === 'asc' ? 'desc' : 'asc',
    })
  }

  const SortIcon = ({ col }: { col: SortKey }) => {
    if (jobFilters.sortBy !== col) return <span className="text-[#1e1e2e] ml-1">↕</span>
    return <span className="text-[#7c6af7] ml-1">{jobFilters.sortDir === 'asc' ? '↑' : '↓'}</span>
  }

  if (error) {
    return (
      <EmptyState
        title="Failed to load jobs"
        description={(error as Error).message}
      />
    )
  }

  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[#1e1e2e]">
              <th className="w-0 p-0" />
              <th className="pl-5 pr-4 py-3 text-left">
                <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                  ID
                </span>
              </th>
              <th className="px-4 py-3 text-left">
                <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                  Type
                </span>
              </th>
              <th className="px-4 py-3 text-left">
                <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                  Priority
                </span>
              </th>
              <th className="px-4 py-3 text-left">
                <button
                  onClick={() => handleSort('status')}
                  className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono cursor-pointer hover:text-[#e2e2f0] transition-colors"
                >
                  Status <SortIcon col="status" />
                </button>
              </th>
              <th className="px-4 py-3 text-left">
                <button
                  onClick={() => handleSort('created_at')}
                  className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono cursor-pointer hover:text-[#e2e2f0] transition-colors"
                >
                  Created <SortIcon col="created_at" />
                </button>
              </th>
              <th className="px-4 py-3 text-left">
                <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                  Duration
                </span>
              </th>
              <th className="pl-4 pr-5 py-3 text-left">
                <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                  Actions
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            {isLoading &&
              Array.from({ length: compact ? 5 : 10 }).map((_, i) => (
                <SkeletonRow key={i} cols={8} />
              ))}

            {!isLoading && data?.data.length === 0 && (
              <tr>
                <td colSpan={8}>
                  <EmptyState
                    title="No jobs found"
                    description="Jobs will appear here once enqueued."
                  />
                </td>
              </tr>
            )}

            {!isLoading &&
              data?.data.map((job) => (
                <JobRow
                  key={job.id}
                  job={job}
                  onClick={(j) => {
                    onRowClick?.(j)
                  }}
                />
              ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {!compact && data && (
        <div className="flex items-center justify-between px-5 py-3 border-t border-[#1e1e2e]">
          <span className="text-xs text-[#6b6b8a] font-mono">
            {data.meta.total_count ?? 0} total • showing {data.data.length}
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              disabled={jobFilters.page === 0}
              onClick={() => setJobFilters({ page: jobFilters.page - 1 })}
            >
              ← Prev
            </Button>
            <span className="text-xs font-mono text-[#6b6b8a] tabular-nums">
              {jobFilters.page + 1}
            </span>
            <Button
              variant="ghost"
              size="sm"
              disabled={!data.meta.has_more}
              onClick={() => setJobFilters({ page: jobFilters.page + 1 })}
            >
              Next →
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
