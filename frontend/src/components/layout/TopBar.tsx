import { useLocation } from 'react-router-dom'

const PAGE_TITLES: Record<string, string> = {
  '/': 'Dashboard',
  '/jobs': 'Jobs',
  '/workers': 'Workers',
  '/dlq': 'Dead Letter Queue',
}

export function TopBar() {
  const { pathname } = useLocation()
  const title = PAGE_TITLES[pathname] ?? 'Dashboard'

  return (
    <header className="flex items-center px-6 h-14 border-b border-[#1e1e2e] bg-[#0a0a0f]/80 backdrop-blur-sm shrink-0">
      <h1 className="text-sm font-semibold text-[#e2e2f0]">{title}</h1>
      <div className="flex items-center gap-3 ml-auto">
        {/* Time */}
        <span className="text-[11px] text-[#6b6b8a] font-mono tabular-nums">
          {new Date().toUTCString().slice(17, 25)} UTC
        </span>
        {/* Status dot */}
        <div className="w-2 h-2 rounded-full bg-[#7c6af7] shadow-[0_0_8px_#7c6af7]" />
      </div>
    </header>
  )
}
