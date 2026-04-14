import React, { useMemo, useState } from 'react'
import clsx from 'clsx'
import type { LogEntry } from '../types'

interface StreamInfo {
  taskId: string      // '' means "no task / general"
  label: string
  count: number
  errorCount: number
  warnCount: number
  service: string
  serviceId: string
}

interface ServiceGroup {
  serviceId: string
  serviceName: string
  streams: StreamInfo[]
  totalCount: number
}

interface Props {
  entries: LogEntry[]
  activeTaskId: string   // '' = all
  onSelect: (taskId: string) => void
}

function resolveLevel(raw: unknown): string {
  if (typeof raw === 'number') {
    const map: Record<number, string> = { 0: 'DEBUG', 1: 'INFO', 2: 'WARNING', 3: 'ERROR', 4: 'CRITICAL' }
    return map[raw as number] ?? 'UNKNOWN'
  }
  return (raw as string) || 'UNKNOWN'
}

export default function StreamSelector({ entries, activeTaskId, onSelect }: Props) {
  const [groupByService, setGroupByService] = useState(true)

  const streams = useMemo(() => {
    const map = new Map<string, StreamInfo>()

    for (const e of entries) {
      const key = (e as any).task_id || ''
      if (!map.has(key)) {
        const logFile: string = (e as any).log_file || ''
        let label = ''
        if (key) {
          label = `Task ${key.slice(0, 8)}`
        } else if (logFile) {
          const base = logFile.split('/').pop()?.replace(/\.log$/, '') ?? logFile
          label = base.length > 30 ? base.slice(0, 30) + '\u2026' : base
        } else {
          label = 'Geral'
        }
        map.set(key, {
          taskId: key,
          label,
          count: 0,
          errorCount: 0,
          warnCount: 0,
          service: (e as any).service_name || (e as any).service_id || '',
          serviceId: (e as any).service_id || '',
        })
      }
      const s = map.get(key)!
      s.count++
      const lvl = resolveLevel((e as any).level)
      if (lvl === 'ERROR' || lvl === 'CRITICAL') s.errorCount++
      else if (lvl === 'WARNING') s.warnCount++
    }

    return Array.from(map.values()).sort((a, b) => b.count - a.count)
  }, [entries])

  const serviceGroups = useMemo(() => {
    if (!groupByService) return null
    const map = new Map<string, ServiceGroup>()
    for (const s of streams) {
      const key = s.serviceId || '_unknown'
      if (!map.has(key)) {
        map.set(key, {
          serviceId: key,
          serviceName: s.service || key,
          streams: [],
          totalCount: 0,
        })
      }
      const g = map.get(key)!
      g.streams.push(s)
      g.totalCount += s.count
    }
    return Array.from(map.values()).sort((a, b) => b.totalCount - a.totalCount)
  }, [streams, groupByService])

  if (streams.length === 0) return null

  const totalCount = entries.length

  function renderChip(s: StreamInfo) {
    return (
      <button
        key={s.taskId || '_general'}
        onClick={() => onSelect(s.taskId)}
        title={s.taskId ? `Task ID: ${s.taskId}` : 'Logs sem task ID'}
        className={clsx(
          'flex-shrink-0 flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium border transition-colors',
          activeTaskId === s.taskId
            ? 'bg-blue-600/25 border-blue-500/50 text-blue-300'
            : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-500 hover:text-gray-300'
        )}
      >
        <span className="truncate max-w-[140px]">{s.label}</span>
        <span className="text-[9px] opacity-60">{s.count}</span>
        {s.errorCount > 0 && (
          <span className="text-[9px] text-red-400 font-bold">{s.errorCount}e</span>
        )}
        {s.warnCount > 0 && (
          <span className="text-[9px] text-yellow-400">{s.warnCount}w</span>
        )}
      </button>
    )
  }

  return (
    <div className="flex-shrink-0 bg-gray-950/60 border-b border-gray-800 overflow-x-auto scrollbar-thin scrollbar-thumb-gray-700">
      <div className="flex items-center gap-1.5 px-3 py-1.5">
        <span className="text-[10px] text-gray-600 flex-shrink-0 pr-1">Streams:</span>

        {/* "All" chip */}
        <button
          onClick={() => onSelect('')}
          className={clsx(
            'flex-shrink-0 flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium border transition-colors',
            activeTaskId === ''
              ? 'bg-blue-600/25 border-blue-500/50 text-blue-300'
              : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-500 hover:text-gray-300'
          )}
        >
          Todos
          <span className="text-[9px] opacity-60">{totalCount}</span>
        </button>

        {/* Group toggle */}
        {serviceGroups && serviceGroups.length > 1 && (
          <button
            onClick={() => setGroupByService(v => !v)}
            title={groupByService ? 'Desagrupar serviços' : 'Agrupar por serviço'}
            className="flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] text-gray-500 hover:text-gray-300 border border-gray-700 hover:border-gray-600 transition"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </button>
        )}

        {/* Chips */}
        {groupByService && serviceGroups && serviceGroups.length > 1 ? (
          // Grouped by service
          serviceGroups.map(g => (
            <div key={g.serviceId} className="flex items-center gap-1 flex-shrink-0">
              <span className="text-[9px] text-cyan-600 font-semibold uppercase tracking-wider pl-1 pr-0.5 border-l border-gray-700">
                {g.serviceName}
              </span>
              {g.streams.map(s => renderChip(s))}
            </div>
          ))
        ) : (
          // Flat list
          streams.map(s => renderChip(s))
        )}
      </div>
    </div>
  )
}
