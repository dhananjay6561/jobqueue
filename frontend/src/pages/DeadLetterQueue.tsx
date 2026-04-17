import { useState } from 'react'
import { DLQTable } from '@/components/dlq/DLQTable'

export function DeadLetterQueue() {
  const [page, setPage] = useState(0)

  return (
    <div className="p-6 space-y-5">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-lg font-semibold text-[#e2e2f0]">Dead Letter Queue</h1>
          <p className="text-xs text-[#6b6b8a] mt-0.5">
            Jobs that exceeded max_attempts without success. Requeue to retry them.
          </p>
        </div>
        <div className="flex items-center gap-2 px-3 py-1.5 bg-red-500/5 border border-red-500/20 rounded-lg">
          <span className="w-1.5 h-1.5 rounded-full bg-red-400" />
          <span className="text-xs text-red-400 font-mono">Requires attention</span>
        </div>
      </div>

      <DLQTable page={page} onPageChange={setPage} />
    </div>
  )
}
