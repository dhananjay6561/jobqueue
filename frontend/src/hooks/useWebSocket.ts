import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWsStore } from '@/store/wsStore'
import { useUiStore } from '@/store/uiStore'
import type { WsEvent } from '@/types'

const WS_URL = import.meta.env.VITE_WS_URL ?? 'ws://localhost:8080/ws'
const MAX_RECONNECT_DELAY = 30000
const BASE_RECONNECT_DELAY = 1000

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
        const event = JSON.parse(msg.data) as WsEvent
        handleEvent(event)
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
