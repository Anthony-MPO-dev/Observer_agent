import type { LogEntry, LogFilter } from '../types'
import { buildWsUrl } from './api'

export type WsStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting'

const PING_INTERVAL_MS = 30_000
const MAX_BACKOFF_MS = 30_000
const INITIAL_BACKOFF_MS = 1_000

export function connectLogStream(
  baseUrl: string,
  token: string,
  filter: Partial<LogFilter>,
  onEntry: (entry: LogEntry) => void,
  onStatusChange: (status: WsStatus) => void,
  onEvent?: (event: any) => void
): () => void {
  let ws: WebSocket | null = null
  let destroyed = false
  let reconnectAttempt = 0
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let lastMessageAt = Date.now()

  function buildUrl(): string {
    const wsBase = buildWsUrl(baseUrl)
    const params = new URLSearchParams()
    params.set('token', token)

    if (filter.service_ids && filter.service_ids.length > 0) {
      params.set('service_ids', filter.service_ids.join(','))
    }
    if (filter.levels && filter.levels.length > 0) {
      params.set('levels', filter.levels.join(','))
    }
    if (filter.task_id) params.set('task_id', filter.task_id)
    if (filter.search) params.set('search', filter.search)

    return `${wsBase}/ws/logs?${params.toString()}`
  }

  function clearTimers() {
    if (pingTimer) { clearInterval(pingTimer); pingTimer = null }
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  }

  function startPing() {
    clearTimers()
    pingTimer = setInterval(() => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        const now = Date.now()
        if (now - lastMessageAt >= PING_INTERVAL_MS) {
          try { ws.send(JSON.stringify({ type: 'ping' })) } catch { /* ignore */ }
        }
      }
    }, PING_INTERVAL_MS)
  }

  function scheduleReconnect() {
    if (destroyed) return
    const backoff = Math.min(INITIAL_BACKOFF_MS * Math.pow(2, reconnectAttempt), MAX_BACKOFF_MS)
    reconnectAttempt++
    onStatusChange('reconnecting')
    reconnectTimer = setTimeout(() => {
      if (!destroyed) connect()
    }, backoff)
  }

  function connect() {
    if (destroyed) return
    onStatusChange('connecting')

    try {
      ws = new WebSocket(buildUrl())
    } catch (err) {
      scheduleReconnect()
      return
    }

    ws.onopen = () => {
      if (destroyed) { ws?.close(); return }
      reconnectAttempt = 0
      lastMessageAt = Date.now()
      onStatusChange('connected')
      startPing()
    }

    ws.onmessage = (event) => {
      lastMessageAt = Date.now()
      try {
        const data = JSON.parse(event.data as string)
        // ignore ping/pong server messages
        if (data && data.type === 'pong') return
        // Handle non-log events (e.g., agent_restart)
        if (data && data.type === 'agent_restart') {
          onEvent?.(data)
          return
        }
        if (data && data.message !== undefined) {
          onEntry(data as LogEntry)
        }
      } catch {
        // not JSON, ignore
      }
    }

    ws.onerror = () => {
      // onerror is always followed by onclose
    }

    ws.onclose = (event) => {
      clearTimers()
      if (destroyed) return
      onStatusChange('disconnected')
      // 1000 = normal close, 1001 = going away — no reconnect needed if we triggered it
      scheduleReconnect()
    }
  }

  connect()

  // Return disconnect function
  return () => {
    destroyed = true
    clearTimers()
    if (ws) {
      ws.onclose = null
      ws.onerror = null
      ws.onmessage = null
      ws.onopen = null
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close(1000, 'Client disconnect')
      }
      ws = null
    }
    onStatusChange('disconnected')
  }
}
