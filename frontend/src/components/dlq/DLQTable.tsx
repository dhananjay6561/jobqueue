import { useState } from 'react'
import { useDLQ, useRequeueDLQ } from '@/hooks/useStats'
import { DLQRow } from './DLQRow'
import { Button } from '@/components/ui/Button'
import { SkeletonRow } from '@/components/ui/Skeleton'
import { EmptyState } from '@/components/ui/EmptyState'

interface DLQTableProps {
  page?: number
  onPageChange?: (page: number) => void
}

export function DLQTable({ page = 0, onPageChange }: DLQTableProps) {
  const PAGE_SIZE = 20
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const requeue = useRequeueDLQ()

  const { data, isLoading, error } = useDLQ({ limit: PAGE_SIZE, offset: page * PAGE_SIZE })

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (!data?.data) return
    if (selectedIds.size === data.data.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(data.data.map((e) => e.id)))
    }
  }

  const handleBulkRequeue = () => {
    selectedIds.forEach((id) => requeue.mutate(id))
    setSelectedIds(new Set())
  }

  if (error) {
    return <EmptyState title="Failed to load DLQ" description={(error as Error).message} />
  }

  return (
    <div className="space-y-4">
      {/* Bulk actions */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 px-4 py-2.5 bg-[#7c6af7]/10 border border-[#7c6af7]/25 rounded-lg">
          <span className="text-sm text-[#7c6af7] font-mono">{selectedIds.size} selected</span>
          <Button
            variant="primary"
            size="sm"
            isLoading={requeue.isPending}
            onClick={handleBulkRequeue}
          >
            Requeue Selected
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
            Clear
          </Button>
        </div>
      )}

      <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[#1e1e2e]">
                <th className="pl-5 py-3 text-left">
                  <input
                    type="checkbox"
                    onChange={toggleSelectAll}
                    checked={!!(data?.data.length && selectedIds.size === data.data.length)}
                    className="accent-[#7c6af7] cursor-pointer"
                    aria-label="Select all"
                  />
                </th>
                {['ID', 'Type', 'Attempts', 'Last Error', 'Failed At', 'Action', ''].map((h) => (
                  <th key={h} className="px-4 py-3 text-left">
                    <span className="text-[10px] font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono">
                      {h}
                    </span>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {isLoading &&
                Array.from({ length: 8 }).map((_, i) => <SkeletonRow key={i} cols={8} />)}

              {!isLoading && data?.data.length === 0 && (
                <tr>
                  <td colSpan={8}>
                    <EmptyState
                      title="Dead letter queue is empty"
                      description="Failed jobs that exceed max_attempts will appear here."
                    />
                  </td>
                </tr>
              )}

              {!isLoading &&
                data?.data.map((entry) => (
                  <DLQRow
                    key={entry.id}
                    entry={entry}
                    isSelected={selectedIds.has(entry.id)}
                    onToggleSelect={toggleSelect}
                  />
                ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {data && (
          <div className="flex items-center justify-between px-5 py-3 border-t border-[#1e1e2e]">
            <span className="text-xs text-[#6b6b8a] font-mono">
              {data.meta.total_count ?? 0} entries
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                disabled={page === 0}
                onClick={() => onPageChange?.(page - 1)}
              >
                ← Prev
              </Button>
              <span className="text-xs font-mono text-[#6b6b8a] tabular-nums">{page + 1}</span>
              <Button
                variant="ghost"
                size="sm"
                disabled={!data.meta.has_more}
                onClick={() => onPageChange?.(page + 1)}
              >
                Next →
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
