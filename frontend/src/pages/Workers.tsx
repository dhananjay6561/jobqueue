import { WorkerGrid } from '@/components/workers/WorkerGrid'
import { useWorkers } from '@/hooks/useWorkers'

export function Workers() {
  const { data } = useWorkers()

  const active = data?.data.filter((w) => w.status === 'active').length ?? 0
  const idle = data?.data.filter((w) => w.status === 'idle').length ?? 0
  const offline = data?.data.filter((w) => w.status === 'offline').length ?? 0
  const total = data?.data.length ?? 0

  return (
    <div className="p-6 space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-[#e2e2f0]">Workers</h1>
        <div className="flex items-center gap-4 text-xs font-mono text-[#6b6b8a]">
          <span><span className="text-green-400 font-semibold">{active}</span> active</span>
          <span><span className="text-[#6b6b8a] font-semibold">{idle}</span> idle</span>
          <span><span className="text-zinc-500 font-semibold">{offline}</span> offline</span>
          <span><span className="text-[#e2e2f0] font-semibold">{total}</span> total</span>
        </div>
      </div>

      <WorkerGrid />
    </div>
  )
}
