import { motion } from 'framer-motion'
import { useStats } from '@/hooks/useStats'
import { formatNumber, formatPercent } from '@/utils/formatters'
import { SkeletonStatBar } from '@/components/ui/Skeleton'

interface StatCardProps {
  label: string
  value: string | number
  sub?: string
  color?: string
}

function StatCard({ label, value, sub, color = 'text-[#e2e2f0]' }: StatCardProps) {
  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-4 hover:border-[#2a2a3e] transition-colors">
      <p className="text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-2 font-mono">{label}</p>
      <motion.p
        key={String(value)}
        initial={{ opacity: 0.6, y: 4 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.25 }}
        className={`text-2xl font-bold tabular-nums font-mono ${color}`}
      >
        {value}
      </motion.p>
      {sub && <p className="text-[11px] text-[#6b6b8a] mt-1 font-mono">{sub}</p>}
    </div>
  )
}

export function StatsBar() {
  const { data: stats, isLoading } = useStats()

  if (isLoading || !stats) return <SkeletonStatBar />

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
      <StatCard
        label="Total Jobs"
        value={formatNumber(stats.total_jobs)}
        sub={`${stats.pending} pending`}
        color="text-[#e2e2f0]"
      />
      <StatCard
        label="Active Workers"
        value={stats.active_workers}
        color="text-green-400"
      />
      <StatCard
        label="Jobs / min"
        value={stats.jobs_per_minute.toFixed(1)}
        color="text-[#7c6af7]"
      />
      <StatCard
        label="Failed Rate"
        value={formatPercent(stats.failed_rate)}
        color={stats.failed_rate > 0.05 ? 'text-red-400' : 'text-green-400'}
      />
      <StatCard
        label="Queue Depth"
        value={formatNumber(stats.queue_depth)}
        sub={`${stats.running} running`}
        color={stats.queue_depth > 100 ? 'text-amber-400' : 'text-[#e2e2f0]'}
      />
    </div>
  )
}
