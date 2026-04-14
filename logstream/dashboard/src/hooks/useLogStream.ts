import { useState, useEffect, useRef, useCallback } from 'react'
import type { LogEntry, LogFilter } from '../types'
import type { RestartEvent } from '../components/RestartBanner'
import { connectLogStream, type WsStatus } from '../lib/ws'

const MAX_ENTRIES = 5000
const BASE_URL = import.meta.env.VITE_API_URL || ''

interface UseLogStreamOptions {
  token: string
  filter: Partial<LogFilter>
  enabled?: boolean
}

export function useLogStream({ token, filter, enabled = true }: UseLogStreamOptions) {
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [status, setStatus] = useState<WsStatus>('disconnected')
  const [paused, setPaused] = useState(false)
  const [reconnectCount, setReconnectCount] = useState(0)
  const [restartEvent, setRestartEvent] = useState<RestartEvent | null>(null)

  // Buffer for entries received while paused
  const bufferRef = useRef<LogEntry[]>([])
  const pausedRef = useRef(false)
  const disconnectRef = useRef<(() => void) | null>(null)

  const onEntry = useCallback((entry: LogEntry) => {
    if (pausedRef.current) {
      bufferRef.current.push(entry)
      return
    }
    setEntries(prev => {
      const next = [...prev, entry]
      return next.length > MAX_ENTRIES ? next.slice(next.length - MAX_ENTRIES) : next
    })
  }, [])

  const onStatusChange = useCallback((s: WsStatus) => {
    setStatus(s)
    if (s === 'reconnecting') {
      setReconnectCount(c => c + 1)
    }
  }, [])

  const onEvent = useCallback((event: any) => {
    if (event && event.type === 'agent_restart') {
      setRestartEvent(event as RestartEvent)
    }
  }, [])

  useEffect(() => {
    if (!enabled || !token) return

    const disconnect = connectLogStream(BASE_URL, token, filter, onEntry, onStatusChange, onEvent)
    disconnectRef.current = disconnect

    return () => {
      disconnect()
      disconnectRef.current = null
    }
    // Re-connect when filter changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, enabled, JSON.stringify(filter)])

  const pause = useCallback(() => {
    pausedRef.current = true
    setPaused(true)
  }, [])

  const resume = useCallback(() => {
    pausedRef.current = false
    setPaused(false)
    // Flush buffered entries
    if (bufferRef.current.length > 0) {
      const buffered = bufferRef.current.splice(0)
      setEntries(prev => {
        const next = [...prev, ...buffered]
        return next.length > MAX_ENTRIES ? next.slice(next.length - MAX_ENTRIES) : next
      })
    }
  }, [])

  const clear = useCallback(() => {
    setEntries([])
    bufferRef.current = []
  }, [])

  return {
    entries,
    status,
    paused,
    bufferedCount: bufferRef.current.length,
    reconnectCount,
    restartEvent,
    pause,
    resume,
    clear,
  }
}
