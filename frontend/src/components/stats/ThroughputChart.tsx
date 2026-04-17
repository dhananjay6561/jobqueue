import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'
import { format } from 'date-fns'

interface ThroughputPoint {
  timestamp: string
  completed: number
  failed: number
}

interface ThroughputChartProps {
  data: ThroughputPoint[]
}

function CustomTooltip({ active, payload, label }: {
  active?: boolean
  payload?: Array<{ color: string; name: string; value: number }>
  label?: string
}) {
  if (!active || !payload?.length) return null
  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-lg px-3 py-2.5 shadow-xl text-xs font-mono">
      <p className="text-[#6b6b8a] mb-1.5">{label}</p>
      {payload.map((p) => (
        <p key={p.name} style={{ color: p.color }}>
          {p.name}: <span className="font-semibold">{p.value}</span>
        </p>
      ))}
    </div>
  )
}

export function ThroughputChart({ data }: ThroughputChartProps) {
  const formatted = data.map((d) => ({
    ...d,
    time: format(new Date(d.timestamp), 'HH:mm'),
  }))

  return (
    <div className="bg-[#111118] border border-[#1e1e2e] rounded-xl p-5">
      <h3 className="text-[11px] text-[#6b6b8a] uppercase tracking-widest mb-4 font-mono">
        Throughput — Last 30 min
      </h3>
      <ResponsiveContainer width="100%" height={180}>
        <LineChart data={formatted} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
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
          <Legend
            iconType="circle"
            iconSize={6}
            wrapperStyle={{ fontSize: '11px', fontFamily: 'JetBrains Mono', color: '#6b6b8a' }}
          />
          <Line
            type="monotone"
            dataKey="completed"
            stroke="#22c55e"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: '#22c55e', strokeWidth: 0 }}
          />
          <Line
            type="monotone"
            dataKey="failed"
            stroke="#ef4444"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4, fill: '#ef4444', strokeWidth: 0 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
