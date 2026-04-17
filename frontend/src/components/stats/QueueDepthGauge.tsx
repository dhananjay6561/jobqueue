import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts'
import { format } from 'date-fns'
import { useStats } from '@/hooks/useStats'

interface QueueDepthPoint {
  timestamp: string
  depth: number
}

interface QueueDepthGaugeProps {
  data: QueueDepthPoint[]
}

function StatusDonut() {
  const { data: stats } = useStats()
  if (!stats) return null

  const pieData = [
    { name: 'Pending', value: stats.pending, color: '#3b82f6' },
    { name: 'Running', value: stats.running, color: '#f59e0b' },
    { name: 'Completed', value: stats.completed, color: '#22c55e' },
    { name: 'Failed', value: stats.failed, color: '#ef4444' },
    { name: 'Dead', value: stats.dead, color: '#dc2626' },
  ].filter((d) => d.value > 0)

  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-5">
      <h3 className="text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-4 font-mono">
        Status Breakdown
      </h3>
      <div className="flex items-center gap-6">
        <ResponsiveContainer width={100} height={100}>
          <PieChart>
            <Pie
              data={pieData}
              cx="50%"
              cy="50%"
              innerRadius={30}
              outerRadius={46}
              dataKey="value"
              strokeWidth={0}
            >
              {pieData.map((entry, i) => (
                <Cell key={i} fill={entry.color} />
              ))}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="space-y-1.5">
          {pieData.map((d) => (
            <div key={d.name} className="flex items-center gap-2 text-xs font-mono">
              <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: d.color }} />
              <span className="text-[#6b6b8a]">{d.name}</span>
              <span className="text-[#e2e2f0] ml-auto pl-3">{d.value}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function CustomTooltip({ active, payload, label }: {
  active?: boolean
  payload?: Array<{ value: number }>
  label?: string
}) {
  if (!active || !payload?.length) return null
  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-lg px-3 py-2 text-xs font-mono">
      <p className="text-[#6b6b8a]">{label}</p>
      <p className="text-[#7c6af7] font-semibold">{payload[0]?.value} jobs</p>
    </div>
  )
}

export function QueueDepthGauge({ data }: QueueDepthGaugeProps) {
  const formatted = data.map((d) => ({
    ...d,
    time: format(new Date(d.timestamp), 'HH:mm'),
  }))

  return (
    <div className="space-y-4">
      <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-5">
        <h3 className="text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-4 font-mono">
          Queue Depth Over Time
        </h3>
        <ResponsiveContainer width="100%" height={140}>
          <AreaChart data={formatted} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="queueGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#7c6af7" stopOpacity={0.3} />
                <stop offset="95%" stopColor="#7c6af7" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#1e1e2e" />
            <XAxis
              dataKey="time"
              tick={{ fill: '#6b6b8a', fontSize: 10, fontFamily: 'JetBrains Mono' }}
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              tick={{ fill: '#6b6b8a', fontSize: 10, fontFamily: 'JetBrains Mono' }}
              tickLine={false}
              axisLine={false}
            />
            <Tooltip content={<CustomTooltip />} />
            <Area
              type="monotone"
              dataKey="depth"
              stroke="#7c6af7"
              strokeWidth={2}
              fill="url(#queueGrad)"
              dot={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
      <StatusDonut />
    </div>
  )
}
