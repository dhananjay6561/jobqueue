import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { ToastContainer } from '@/components/ui/Toast'
import { useWebSocket } from '@/hooks/useWebSocket'

export function Layout() {
  // Initialize WebSocket connection once at layout level
  useWebSocket()

  return (
    <div className="flex h-screen bg-[#0a0a0f] text-[#e2e2f0] overflow-hidden">
      <Sidebar />
      <div className="flex flex-col flex-1 min-w-0 overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-y-auto overflow-x-hidden">
          <Outlet />
        </main>
      </div>
      <ToastContainer />
    </div>
  )
}
