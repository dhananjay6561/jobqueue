import { create } from 'zustand'
import { type LiveEvent, type WsEvent } from '@/types'

const MAX_EVENTS = 200

function wsEventToLiveEvent(event: WsEvent): LiveEvent {
  const base = { id: crypto.randomUUID(), timestamp: event.timestamp }

  switch (event.type) {
    case 'job.enqueued':
      return {
        ...base,
        type: event.type,
        message: `Job enqueued: ${event.job.type} [${event.job.id.slice(0, 8)}]`,
        color: 'info',
      }
    case 'job.started':
      return {
        ...base,
        type: event.type,
        message: `Job started: ${event.job_type} [${event.job_id.slice(0, 8)}] on ${event.worker_id.slice(0, 8)}`,
        color: 'info',
      }
    case 'job.completed':
      return {
        ...base,
        type: event.type,
        message: `Job completed: ${event.job_type} [${event.job_id.slice(0, 8)}] in ${event.duration_ms}ms`,
        color: 'success',
      }
    case 'job.failed':
      return {
        ...base,
        type: event.type,
        message: `Job failed: ${event.job_type} [${event.job_id.slice(0, 8)}] attempt #${event.attempt} — ${event.error}`,
        color: 'warning',
      }
    case 'job.dead':
      return {
        ...base,
        type: event.type,
        message: `Job dead: ${event.job_type} [${event.job_id.slice(0, 8)}] — ${event.error}`,
        color: 'error',
      }
    case 'worker.heartbeat':
      return {
        ...base,
        type: event.type,
        message: `Worker heartbeat: ${event.worker_id.slice(0, 8)} — ${event.status}`,
        color: 'muted',
      }
    case 'stats.update':
      return {
        ...base,
        type: event.type,
        message: `Stats: ${event.stats.total_jobs} total, ${event.stats.active_workers} workers`,
        color: 'muted',
      }
  }
}

interface WsStoreState {
  events: LiveEvent[]
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'error'
  addEvent: (event: WsEvent) => void
  setConnectionStatus: (status: WsStoreState['connectionStatus']) => void
  clearEvents: () => void
}

export const useWsStore = create<WsStoreState>((set) => ({
  events: [],
  connectionStatus: 'disconnected',
  addEvent: (event: WsEvent) =>
    set((state) => ({
      events: [wsEventToLiveEvent(event), ...state.events].slice(0, MAX_EVENTS),
    })),
  setConnectionStatus: (status) => set({ connectionStatus: status }),
  clearEvents: () => set({ events: [] }),
}))
