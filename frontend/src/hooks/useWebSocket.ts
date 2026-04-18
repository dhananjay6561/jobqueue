import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWsStore } from '@/store/wsStore'
import { useUiStore } from '@/store/uiStore'
import type { Job, QueueStats, WsEvent, WorkerStatus } from '@/types'

const WS_URL = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws'
const MAX_RECONNECT_DELAY = 30000
const BASE_RECONNECT_DELAY = 1000

// normalizeEvent converts the raw wire format (from Go backend) into the
// WsEvent shape the frontend expects.
// Backend sends: { type, job_id, job_type, worker_id, payload: {...}, ts: unixMs }
// Frontend types expect flat fields and ISO timestamp strings.
function normalizeEvent(raw: Record<string, unknown>): WsEvent | null {
  const type = raw.type as WsEvent['type']
  const timestamp = new Date((raw.ts as number) ?? Date.now()).toISOString()
  const payload = (raw.payload ?? {}) as Record<string, unknown>
  const job = payload.job as Job | undefined

  switch (type) {
    case 'job.enqueued':
      return { type, timestamp, job: job! }
    case 'job.started':
      return {
        type, timestamp,
        job_id: raw.job_id as string,
        job_type: raw.job_type as string,
        worker_id: raw.worker_id as string,
      }
    case 'job.completed':
      return {
        type, timestamp,
        job_id: raw.job_id as string,
        job_type: raw.job_type as string,
        duration_ms: (payload.duration_ms as number) ?? 0,
      }
    case 'job.failed':
      return {
        type, timestamp,
        job_id: raw.job_id as string,
        job_type: raw.job_type as string,
        attempt: job?.attempts ?? (payload.attempt as number) ?? 0,
        error: job?.error_message ?? (payload.error as string) ?? 'unknown error',
      }
    case 'job.dead':
      return {
        type, timestamp,
        job_id: raw.job_id as string,
        job_type: raw.job_type as string,
        error: job?.error_message ?? (payload.error as string) ?? 'unknown error',
      }
    case 'worker.heartbeat':
      return {
        type, timestamp,
        worker_id: raw.worker_id as string,
        status: ((payload.status as WorkerStatus) ?? 'idle'),
        current_job_id: (payload.current_job_id as string | null) ?? null,
      }
    case 'stats.update':
      return { type, timestamp, stats: payload as unknown as QueueStats }
    default:
      return null
  }
}

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectDelayRef = useRef(BASE_RECONNECT_DELAY)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const unmountedRef = useRef(false)

  const { addEvent, setConnectionStatus } = useWsStore()
  const { addToast } = useUiStore()
  const queryClient = useQueryClient()

  const handleEvent = useCallback(
    (event: WsEvent) => {
      addEvent(event)

      switch (event.type) {
        case 'job.enqueued':
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['stats'] })
          addToast({ message: `New job enqueued: ${event.job.type}`, variant: 'info', duration: 3000 })
          break
        case 'job.completed':
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['stats'] })
          break
        case 'job.failed':
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['stats'] })
          addToast({ message: `Job failed: ${event.job_type} (attempt ${event.attempt})`, variant: 'warning', duration: 4000 })
          break
        case 'job.dead':
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['dlq'] })
          void queryClient.invalidateQueries({ queryKey: ['stats'] })
          addToast({ message: `Job dead: ${event.job_type}`, variant: 'error', duration: 6000 })
          break
        case 'job.started':
          void queryClient.invalidateQueries({ queryKey: ['jobs'] })
          void queryClient.invalidateQueries({ queryKey: ['workers'] })
          break
        case 'worker.heartbeat':
          void queryClient.invalidateQueries({ queryKey: ['workers'] })
          break
        case 'stats.update':
          void queryClient.setQueryData(['stats'], event.stats)
          break
      }
    },
    [addEvent, addToast, queryClient],
  )

  const connect = useCallback(() => {
    if (unmountedRef.current) return
    setConnectionStatus('connecting')

    const ws = new WebSocket(WS_URL)
    wsRef.current = ws

    ws.onopen = () => {
      setConnectionStatus('connected')
      reconnectDelayRef.current = BASE_RECONNECT_DELAY
    }

    ws.onmessage = (msg: MessageEvent<string>) => {
      try {
        const raw = JSON.parse(msg.data) as Record<string, unknown>
        const event = normalizeEvent(raw)
        if (event) handleEvent(event)
      } catch {
        // ignore malformed events
      }
    }

    ws.onclose = () => {
      if (unmountedRef.current) return
      setConnectionStatus('disconnected')
      // Exponential backoff reconnect
      const delay = Math.min(reconnectDelayRef.current, MAX_RECONNECT_DELAY)
      reconnectDelayRef.current = delay * 2
      reconnectTimeoutRef.current = setTimeout(connect, delay)
    }

    ws.onerror = () => {
      setConnectionStatus('error')
      ws.close()
    }
  }, [handleEvent, setConnectionStatus])

  useEffect(() => {
    unmountedRef.current = false
    connect()

    return () => {
      unmountedRef.current = true
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      wsRef.current?.close()
    }
  }, [connect])
}
