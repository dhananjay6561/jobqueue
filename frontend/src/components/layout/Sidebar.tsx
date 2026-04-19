import { NavLink } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useUiStore } from '@/store/uiStore'
import { useWsStore } from '@/store/wsStore'
import { useAuthStore } from '@/store/authStore'
import { clsx } from 'clsx'

interface NavItem {
  to: string
  label: string
  icon: React.ReactNode
  badge?: number
}

function NavIcon({ path }: { path: string }) {
  return (
    <svg className="w-4 h-4 shrink-0" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d={path} />
    </svg>
  )
}

const navItems: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: <NavIcon path="M3 3h6v6H3zM11 3h6v6h-6zM3 11h6v6H3zM11 11h6v6h-6z" /> },
  { to: '/jobs', label: 'Jobs', icon: <NavIcon path="M4 5h12M4 10h12M4 15h7" /> },
  { to: '/workers', label: 'Workers', icon: <NavIcon path="M13 6a3 3 0 11-6 0 3 3 0 016 0zM3 19a7 7 0 0114 0" /> },
  { to: '/dlq', label: 'Dead Letter', icon: <NavIcon path="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" /> },
  { to: '/cron', label: 'Cron', icon: <NavIcon path="M10 2a8 8 0 100 16A8 8 0 0010 2zm0 3v5l3 3" /> },
  { to: '/billing', label: 'Billing', icon: <NavIcon path="M3 5h14M3 10h14M3 15h7" /> },
]

const wsStatusConfig = {
  connected: { color: 'bg-green-400', label: 'Connected', pulse: true },
  connecting: { color: 'bg-amber-400', label: 'Connecting…', pulse: true },
  disconnected: { color: 'bg-zinc-500', label: 'Disconnected', pulse: false },
  error: { color: 'bg-red-400', label: 'Error', pulse: false },
}

export function Sidebar() {
  const { sidebarCollapsed, toggleSidebar } = useUiStore()
  const { connectionStatus } = useWsStore()
  const { user, clearAuth } = useAuthStore()
  const ws = wsStatusConfig[connectionStatus]

  return (
    <motion.aside
      animate={{ width: sidebarCollapsed ? 60 : 220 }}
      transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
      className="relative shrink-0 flex flex-col h-full bg-[#0d0d14] border-r border-[#1e1e2e] overflow-hidden z-10"
    >
      {/* Logo */}
      <div className="flex items-center gap-3 px-4 h-14 border-b border-[#1e1e2e] shrink-0">
        <div className="w-7 h-7 rounded-lg bg-[#7c6af7] flex items-center justify-center shrink-0 shadow-lg shadow-[#7c6af7]/30">
          <svg className="w-4 h-4 text-white" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
            <path d="M2 2h5v5H2zM9 2h5v5H9zM2 9h5v5H2zM9 9h5v5H9z" />
          </svg>
        </div>
        {!sidebarCollapsed && (
          <motion.span
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="font-semibold text-[#e2e2f0] text-sm tracking-tight whitespace-nowrap font-mono"
          >
            JobQueue
          </motion.span>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              clsx(
                'flex items-center gap-3 px-2.5 py-2 rounded-lg text-sm transition-all duration-150 group cursor-pointer',
                isActive
                  ? 'bg-[#7c6af7]/15 text-[#7c6af7] border border-[#7c6af7]/25'
                  : 'text-[#6b6b8a] hover:text-[#e2e2f0] hover:bg-white/5 border border-transparent',
              )
            }
          >
            {({ isActive }) => (
              <>
                <span className={clsx(isActive ? 'text-[#7c6af7]' : 'text-inherit')}>{item.icon}</span>
                {!sidebarCollapsed && (
                  <motion.span
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="whitespace-nowrap"
                  >
                    {item.label}
                  </motion.span>
                )}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* User / auth */}
      {!sidebarCollapsed && user && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="mx-2 px-3 py-2 rounded-lg bg-white/[0.03] border border-[#1e1e2e] flex items-center justify-between gap-2"
        >
          <span className="text-[11px] text-[#6b6b8a] truncate">{user.email}</span>
          <button
            onClick={clearAuth}
            className="text-[10px] text-[#6b6b8a] hover:text-red-400 transition-colors shrink-0"
            title="Sign out"
          >
            ⎋
          </button>
        </motion.div>
      )}

      {/* WS Status */}
      {!sidebarCollapsed && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="mx-2 mb-2 px-3 py-2.5 rounded-lg bg-white/[0.03] border border-[#1e1e2e]"
        >
          <div className="flex items-center gap-2">
            <span className="relative flex items-center justify-center w-2 h-2">
              {ws.pulse && <span className={`absolute inline-flex w-full h-full rounded-full opacity-60 animate-ping ${ws.color}`} />}
              <span className={`relative inline-flex w-2 h-2 rounded-full ${ws.color}`} />
            </span>
            <span className="text-[11px] text-[#6b6b8a]">{ws.label}</span>
          </div>
        </motion.div>
      )}

      {/* Collapse toggle */}
      <button
        onClick={toggleSidebar}
        className="flex items-center justify-center h-10 border-t border-[#1e1e2e] text-[#6b6b8a] hover:text-[#e2e2f0] hover:bg-white/5 transition-colors cursor-pointer"
        aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      >
        <svg
          className={clsx('w-4 h-4 transition-transform duration-200', sidebarCollapsed && 'rotate-180')}
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          aria-hidden="true"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M10 3L5 8l5 5" />
        </svg>
      </button>
    </motion.aside>
  )
}
