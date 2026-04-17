import { useEffect, useRef, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { StatsBar } from '@/components/stats/StatsBar'
import { ThroughputChart } from '@/components/stats/ThroughputChart'
import { QueueDepthGauge } from '@/components/stats/QueueDepthGauge'
import { JobTable } from '@/components/jobs/JobTable'
import { JobDetailDrawer } from '@/components/jobs/JobDetailDrawer'
import { ErrorBoundary } from '@/components/ui/ErrorBoundary'
import { useWsStore } from '@/store/wsStore'
import { useStats } from '@/hooks/useStats'
import type { Job, LiveEvent, ThroughputPoint, QueueDepthPoint } from '@/types'

const MAX_POINTS = 30

function safeFormatTime(timestamp: string): string {
  try {
    const d = new Date(timestamp)
    if (isNaN(d.getTime())) return '??:??:??'
    return d.toTimeString().slice(0, 8)
  } catch {
    return '??:??:??'
  }
}

const EVENT_COLOR_CLASS: Record<LiveEvent['color'], string> = {
  success: 'text-green-400',
  warning: 'text-amber-400',
  error: 'text-red-400',
  info: 'text-blue-400',
  muted: 'text-[#6b6b8a]',
}

function LiveFeedInner() {
  const { events, connectionStatus } = useWsStore()

  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl flex flex-col h-full overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#1e1e2e] shrink-0">
        <h3 className="text-[10px] text-[#6b6b8a] uppercase tracking-widest font-mono">Live Events</h3>
        <div className="flex items-center gap-1.5">
          <span
            className={`w-1.5 h-1.5 rounded-full ${connectionStatus === 'connected' ? 'bg-green-400 animate-pulse' : 'bg-zinc-500'}`}
          />
          <span className="text-[10px] text-[#6b6b8a] font-mono">{connectionStatus}</span>
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-3 space-y-0.5 font-mono text-[11px]">
        <AnimatePresence initial={false}>
          {events.slice(0, 60).map((event) => (
            <motion.div
              key={event.id}
              initial={{ opacity: 0, x: 8 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.2 }}
              className="flex gap-2 py-0.5"
            >
              <span className="text-[#3a3a5e] shrink-0 select-none">
                {safeFormatTime(event.timestamp)}
              </span>
              <span className={`shrink-0 ${EVENT_COLOR_CLASS[event.color]}`}>
                [{event.type}]
              </span>
              <span className="text-[#6b6b8a] break-all">{event.message}</span>
            </motion.div>
          ))}
        </AnimatePresence>
        {events.length === 0 && (
          <p className="text-[#4a4a6a] text-center py-8 text-[11px]">Waiting for events…</p>
        )}
      </div>
    </div>
  )
}

export function Dashboard() {
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)
  const [throughputData, setThroughputData] = useState<ThroughputPoint[]>([])
  const [depthData, setDepthData] = useState<QueueDepthPoint[]>([])
  const lastTotalsRef = useRef<{ completed: number; failedCombined: number } | null>(null)
  const { data: stats } = useStats()

  useEffect(() => {
    if (!stats) return

    const timestamp = new Date().toISOString()
    const failedCombined = stats.failed + stats.dead

    const prev = lastTotalsRef.current
    const completedDelta = prev ? Math.max(0, stats.completed - prev.completed) : 0
    const failedDelta = prev ? Math.max(0, failedCombined - prev.failedCombined) : 0

    lastTotalsRef.current = {
      completed: stats.completed,
      failedCombined,
    }

    setThroughputData((current) => {
      const next = [...current, { timestamp, completed: completedDelta, failed: failedDelta }]
      return next.slice(-MAX_POINTS)
    })

    setDepthData((current) => {
      const next = [...current, { timestamp, depth: stats.queue_depth }]
      return next.slice(-MAX_POINTS)
    })
  }, [stats])

  return (
    <div className="p-6 space-y-6">
      <ErrorBoundary>
        <StatsBar />
      </ErrorBoundary>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <div className="xl:col-span-2 space-y-6">
          <ErrorBoundary>
            <ThroughputChart data={throughputData} />
          </ErrorBoundary>

          <div>
            <h2 className="text-xs font-semibold text-[#6b6b8a] uppercase tracking-widest font-mono mb-3">
              Recent Jobs
            </h2>
            <ErrorBoundary>
              <JobTable
                compact
                onRowClick={(job: Job) => setSelectedJobId(job.id)}
              />
            </ErrorBoundary>
          </div>
        </div>

        <div className="space-y-4 flex flex-col">
          <ErrorBoundary>
            <QueueDepthGauge data={depthData} />
          </ErrorBoundary>
          <div className="flex-1 min-h-[300px]">
            <ErrorBoundary>
              <LiveFeedInner />
            </ErrorBoundary>
          </div>
        </div>
      </div>

      <JobDetailDrawer jobId={selectedJobId} onClose={() => setSelectedJobId(null)} />
    </div>
  )
}
