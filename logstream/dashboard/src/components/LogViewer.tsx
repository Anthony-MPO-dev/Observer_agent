import React, { useRef, useEffect, useState, useCallback, useMemo } from 'react'
import clsx from 'clsx'
import type { LogFilter } from '../types'
import { useLogStream } from '../hooks/useLogStream'
import LogLine from './LogLine'
import FilterBar from './FilterBar'
import StreamSelector from './StreamSelector'
import LiveCharts from './LiveCharts'
import RestartBanner from './RestartBanner'

interface Props {
  token: string
  filter: LogFilter
  onFilterChange: (f: LogFilter) => void
  onStatusChange?: (status: string, reconnectCount: number) => void
}

const ROW_HEIGHT = 32
const BUFFER_ROWS = 20

export default function LogViewer({ token, filter, onFilterChange, onStatusChange }: Props) {
  const { entries, status, paused, bufferedCount, reconnectCount, restartEvent, pause, resume, clear } =
    useLogStream({ token, filter })

  useEffect(() => {
    onStatusChange?.(status, reconnectCount)
  }, [status, reconnectCount, onStatusChange])

  // Local stream filter (task_id) — doesn't reconnect WS, just filters display
  const [streamTaskId, setStreamTaskId] = useState('')
  const [showCharts, setShowCharts] = useState(false)

  const visibleEntries = useMemo(() => {
    if (!streamTaskId) return entries
    return entries.filter(e => ((e as any).task_id || '') === streamTaskId)
  }, [entries, streamTaskId])

  const containerRef = useRef<HTMLDivElement>(null)
  const [autoScroll, setAutoScroll] = useState(true)
  const [scrollTop, setScrollTop] = useState(0)
  const [containerHeight, setContainerHeight] = useState(600)
  const autoScrollRef = useRef(true)

  useEffect(() => {
    if (!autoScrollRef.current || paused) return
    const el = containerRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [visibleEntries.length, paused])

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const obs = new ResizeObserver(() => setContainerHeight(el.clientHeight))
    obs.observe(el)
    return () => obs.disconnect()
  }, [])

  const handleScroll = useCallback(() => {
    const el = containerRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 60
    setAutoScroll(atBottom)
    autoScrollRef.current = atBottom
    setScrollTop(el.scrollTop)
  }, [])

  function scrollToBottom() {
    const el = containerRef.current
    if (el) { el.scrollTop = el.scrollHeight; setAutoScroll(true); autoScrollRef.current = true }
  }

  const totalHeight  = visibleEntries.length * ROW_HEIGHT
  const startIdx     = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - BUFFER_ROWS)
  const visibleCount = Math.ceil(containerHeight / ROW_HEIGHT) + BUFFER_ROWS * 2
  const endIdx       = Math.min(visibleEntries.length, startIdx + visibleCount)
  const slicedEntries = visibleEntries.slice(startIdx, endIdx)
  const paddingTop   = startIdx * ROW_HEIGHT
  const paddingBottom = (visibleEntries.length - endIdx) * ROW_HEIGHT

  const statusColor = { connected: 'text-green-400', connecting: 'text-yellow-400', reconnecting: 'text-orange-400', disconnected: 'text-red-400' }[status]
  const statusLabel = { connected: 'Conectado', connecting: 'Conectando...', reconnecting: 'Reconectando...', disconnected: 'Desconectado' }[status]

  return (
    <div className="flex flex-col h-full">
      {/* Restart banner */}
      <RestartBanner event={restartEvent} />

      {/* Header */}
      <div className="flex-shrink-0 flex items-center justify-between px-4 py-2 bg-gray-900/80 border-b border-gray-800">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-1.5">
            <span className={clsx('w-2 h-2 rounded-full flex-shrink-0',
              status === 'connected' ? 'bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.6)]' :
              status === 'connecting' || status === 'reconnecting' ? 'bg-yellow-400 animate-pulse' : 'bg-red-500'
            )} />
            <span className={clsx('text-xs font-medium', statusColor)}>{statusLabel}</span>
            {reconnectCount > 0 && <span className="text-xs text-gray-600">({reconnectCount}x)</span>}
          </div>
          <span className="text-xs text-gray-500">
            {visibleEntries.length.toLocaleString('pt-BR')}
            {streamTaskId && entries.length !== visibleEntries.length && (
              <span className="text-gray-600"> de {entries.length.toLocaleString('pt-BR')}</span>
            )} entradas
          </span>
        </div>

        <div className="flex items-center gap-2">
          {/* Charts toggle */}
          <button
            onClick={() => setShowCharts(v => !v)}
            title="Gráficos"
            className={clsx(
              'flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium transition-colors border',
              showCharts
                ? 'bg-purple-700/30 border-purple-600/50 text-purple-300'
                : 'bg-gray-700/30 border-gray-700 text-gray-400 hover:bg-gray-700/60 hover:text-gray-300'
            )}
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
            Gráficos
          </button>

          {/* Pause/Resume */}
          <button
            onClick={paused ? resume : pause}
            className={clsx(
              'flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium transition-colors',
              paused
                ? 'bg-green-700/30 border border-green-700/50 text-green-300 hover:bg-green-700/50'
                : 'bg-yellow-700/30 border border-yellow-700/50 text-yellow-300 hover:bg-yellow-700/50'
            )}
          >
            {paused ? (
              <>
                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd" />
                </svg>
                Retomar
                {bufferedCount > 0 && <span className="bg-green-600 text-white rounded-full px-1.5 py-0.5 text-[10px]">+{bufferedCount}</span>}
              </>
            ) : (
              <>
                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                </svg>
                Pausar
              </>
            )}
          </button>

          {/* Clear */}
          <button
            onClick={() => { clear(); setStreamTaskId('') }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-gray-700/30 border border-gray-700 text-gray-400 hover:bg-gray-700/60 hover:text-gray-300 transition-colors"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            Limpar
          </button>
        </div>
      </div>

      {/* Charts panel */}
      {showCharts && <LiveCharts entries={entries} />}

      {/* Stream selector */}
      <StreamSelector entries={entries} activeTaskId={streamTaskId} onSelect={setStreamTaskId} />

      {/* Filter bar */}
      <FilterBar filter={filter} onChange={onFilterChange} />

      {/* Log list */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto overflow-x-auto relative"
        style={{ minHeight: 0 }}
      >
        {visibleEntries.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-600">
            <svg className="w-12 h-12 mb-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1}
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <p className="text-sm">
              {entries.length > 0 ? 'Nenhum log para este stream.' : 'Aguardando logs...'}
            </p>
            {status === 'disconnected' && <p className="text-xs mt-1 text-red-500">WebSocket desconectado</p>}
          </div>
        ) : (
          <div style={{ height: totalHeight, position: 'relative' }}>
            <div style={{ transform: `translateY(${paddingTop}px)` }}>
              {slicedEntries.map((entry, i) => (
                <LogLine key={(entry as any).id || `${startIdx + i}`} entry={entry} index={startIdx + i} />
              ))}
            </div>
            {paddingBottom > 0 && <div style={{ height: paddingBottom }} />}
          </div>
        )}
      </div>

      {/* Scroll to bottom */}
      {(!autoScroll || paused) && visibleEntries.length > 0 && (
        <div className="flex-shrink-0 flex items-center justify-center gap-3 py-2 bg-gray-900/90 border-t border-gray-800">
          {paused && bufferedCount > 0 && (
            <span className="text-xs text-yellow-400">
              {bufferedCount} nova{bufferedCount !== 1 ? 's' : ''} entrada{bufferedCount !== 1 ? 's' : ''} em buffer
            </span>
          )}
          {!autoScroll && (
            <button onClick={scrollToBottom}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-blue-700/30 border border-blue-700/50 text-blue-300 hover:bg-blue-700/60 transition">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
              </svg>
              Ir para o final
            </button>
          )}
        </div>
      )}
    </div>
  )
}
