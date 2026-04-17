import { useState } from 'react'
import type { DLQEntry } from '@/types'
import { Button } from '@/components/ui/Button'
import { useRequeueDLQ } from '@/hooks/useStats'
import { formatRelativeTime, formatDateTime, truncateId, formatPayload } from '@/utils/formatters'
import { motion, AnimatePresence } from 'framer-motion'

interface DLQRowProps {
  entry: DLQEntry
  isSelected: boolean
  onToggleSelect: (id: string) => void
}

export function DLQRow({ entry, isSelected, onToggleSelect }: DLQRowProps) {
  const [expanded, setExpanded] = useState(false)
  const requeue = useRequeueDLQ()

  return (
    <>
      <tr
        className="border-b border-[#1e1e2e] hover:bg-white/[0.02] cursor-pointer group transition-colors"
        onClick={() => setExpanded((e) => !e)}
      >
        <td className="pl-5 py-3" onClick={(e) => e.stopPropagation()}>
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => onToggleSelect(entry.id)}
            className="accent-[#7c6af7] cursor-pointer"
            aria-label={`Select DLQ entry ${entry.id}`}
          />
        </td>
        <td className="px-4 py-3">
          <span className="text-xs text-[#6b6b8a] font-mono">{truncateId(entry.id)}</span>
        </td>
        <td className="px-4 py-3">
          <span className="text-xs text-[#e2e2f0] font-mono">{entry.type}</span>
        </td>
        <td className="px-4 py-3">
          <span className="text-xs text-red-400 font-mono tabular-nums">{entry.attempts}</span>
        </td>
        <td className="px-4 py-3 max-w-[240px]">
          <span className="text-xs text-[#6b6b8a] font-mono truncate block">{entry.last_error}</span>
        </td>
        <td className="px-4 py-3">
          <span className="text-xs text-[#6b6b8a] font-mono">{formatRelativeTime(entry.failed_at)}</span>
        </td>
        <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
          <Button
            variant="secondary"
            size="sm"
            isLoading={requeue.isPending}
            onClick={() => requeue.mutate(entry.id)}
          >
            Requeue
          </Button>
        </td>
        <td className="pl-4 pr-5 py-3">
          <svg
            className={`w-3.5 h-3.5 text-[#6b6b8a] transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M2 4l4 4 4-4" />
          </svg>
        </td>
      </tr>

      {/* Expanded detail */}
      <AnimatePresence>
        {expanded && (
          <motion.tr
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
          >
            <td colSpan={8} className="bg-[#0a0a0f] border-b border-[#1e1e2e] px-8 py-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <p className="text-[10px] text-[#6b6b8a] uppercase tracking-widest mb-2 font-mono">Full Error</p>
                  <div className="bg-red-500/5 border border-red-500/20 rounded-lg p-3">
                    <p className="text-xs text-red-300 font-mono whitespace-pre-wrap break-all">{entry.last_error}</p>
                  </div>
                </div>
                <div>
                  <p className="text-[10px] text-[#6b6b8a] uppercase tracking-widest mb-2 font-mono">Payload</p>
                  <div className="bg-[#111118] border border-[#1e1e2e] rounded-lg p-3 overflow-x-auto">
                    <pre className="text-[11px] text-[#7c6af7] font-mono whitespace-pre">
                      {formatPayload(entry.payload)}
                    </pre>
                  </div>
                </div>
              </div>
              <div className="mt-3 flex gap-4 text-[11px] text-[#6b6b8a] font-mono">
                <span>Failed: {formatDateTime(entry.failed_at)}</span>
                <span>Original ID: {truncateId(entry.original_job_id)}</span>
              </div>
            </td>
          </motion.tr>
        )}
      </AnimatePresence>
    </>
  )
}
