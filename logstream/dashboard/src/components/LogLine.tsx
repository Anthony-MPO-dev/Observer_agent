import React, { useState } from 'react'
import clsx from 'clsx'
import type { LogEntry, LogLevel } from '../types'

interface Props {
  entry: LogEntry
  index?: number
}

const LEVEL_CLASSES: Record<LogLevel, string> = {
  UNKNOWN:  'bg-gray-700 text-gray-300',
  DEBUG:    'bg-gray-700 text-gray-300',
  INFO:     'bg-blue-900/60 text-blue-300',
  WARNING:  'bg-yellow-900/60 text-yellow-300',
  ERROR:    'bg-red-900/60 text-red-300',
  CRITICAL: 'bg-red-700/80 text-red-100 font-bold animate-pulse',
}

const LEVEL_TEXT_CLASSES: Record<LogLevel, string> = {
  UNKNOWN:  'text-gray-400',
  DEBUG:    'text-gray-400',
  INFO:     'text-blue-300',
  WARNING:  'text-yellow-300',
  ERROR:    'text-red-400',
  CRITICAL: 'text-red-300 font-bold',
}

// unixTs may be seconds (new entries: unix_ts) or milliseconds (old: timestamp).
// Values > 1e10 are milliseconds; values <= 1e10 are seconds.
function toMs(unixTs: number): number {
  if (!unixTs) return 0
  return unixTs > 1e10 ? unixTs : unixTs * 1000
}

function formatTime(unixTs: number): string {
  const ms = toMs(unixTs)
  if (!ms) return '—'
  const d = new Date(ms)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  return `${hh}:${mm}:${ss}`
}

function formatDate(unixTs: number): string {
  const ms = toMs(unixTs)
  if (!ms) return '—'
  return new Date(ms).toLocaleString('pt-BR')
}

const LEVEL_FROM_INT: Record<number, LogLevel> = { 0: 'DEBUG', 1: 'INFO', 2: 'WARNING', 3: 'ERROR', 4: 'CRITICAL' }

function resolveLevel(raw: unknown): LogLevel {
  if (typeof raw === 'number') return LEVEL_FROM_INT[raw] ?? 'UNKNOWN'
  return ((raw as string) || 'UNKNOWN') as LogLevel
}

export default function LogLine({ entry, index }: Props) {
  const [expanded, setExpanded] = useState(false)

  const level = resolveLevel(entry.level as unknown)

  return (
    <div
      className={clsx(
        'font-mono text-xs border-b border-gray-900/60 cursor-pointer select-text transition-colors',
        expanded ? 'bg-gray-900' : index != null && index % 2 === 0 ? 'bg-transparent hover:bg-gray-900/40' : 'bg-gray-950/40 hover:bg-gray-900/40'
      )}
      onClick={() => setExpanded(v => !v)}
    >
      {/* Compact row */}
      <div className="flex items-start gap-2 px-3 py-1.5 min-h-[28px]">
        {/* Timestamp */}
        <span className="text-gray-600 flex-shrink-0 w-[90px] pt-px">{formatTime((entry as any).unix_ts || (entry as any).timestamp || 0)}</span>

        {/* Level badge */}
        <span className={clsx(
          'flex-shrink-0 px-1.5 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider w-[64px] text-center',
          LEVEL_CLASSES[level]
        )}>
          {level === 'WARNING' ? 'WARN' : level === 'CRITICAL' ? 'CRIT' : level}
        </span>

        {/* Service badge */}
        {entry.service_id && (
          <span className="flex-shrink-0 px-1 py-0.5 rounded text-[9px] font-medium bg-gray-800 text-gray-400 border border-gray-700 max-w-[100px] truncate">
            {entry.service_name || entry.service_id}
          </span>
        )}

        {/* Module */}
        {entry.module && (
          <span className="flex-shrink-0 text-cyan-600 pt-px">
            [{entry.module}]
          </span>
        )}

        {/* Document */}
        {entry.documento && (
          <span className="flex-shrink-0 text-orange-500 pt-px">
            [DOC:{entry.documento}]
          </span>
        )}

        {/* Message */}
        <span className={clsx('flex-1 break-words leading-relaxed', LEVEL_TEXT_CLASSES[level])}>
          {entry.message}
        </span>

        {/* Expand indicator */}
        <span className="flex-shrink-0 text-gray-700 pt-px">
          {expanded ? '▲' : '▼'}
        </span>
      </div>

      {/* Expanded details */}
      {expanded && (
        <div className="px-3 pb-3 pt-1 grid grid-cols-2 gap-x-6 gap-y-1 bg-gray-900 border-t border-gray-800">
          <DetailRow label="ID" value={entry.id} />
          <DetailRow label="Serviço" value={entry.service_name ? `${entry.service_name} (${entry.service_id})` : entry.service_id} />
          <DetailRow label="Task ID" value={entry.task_id} />
          <DetailRow label="Documento" value={entry.documento} />
          <DetailRow label="Módulo" value={entry.module} />
          <DetailRow label="Worker Type" value={entry.worker_type} />
          <DetailRow label="Queue" value={entry.queue} />
          <DetailRow label="Agent ID" value={entry.agent_id} />
          <DetailRow label="Log File" value={entry.log_file} />
          <DetailRow label="Timestamp" value={formatDate((entry as any).unix_ts || (entry as any).timestamp || 0)} />
          <DetailRow label="Continuação" value={entry.is_continuation ? 'sim' : 'não'} />

          {/* Full message */}
          <div className="col-span-2 mt-2">
            <span className="text-gray-500 text-[10px] uppercase tracking-wider">Mensagem completa</span>
            <pre className="mt-1 text-gray-200 whitespace-pre-wrap break-words bg-gray-950 rounded p-2 text-[11px]">
              {entry.message}
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value: string | undefined }) {
  if (!value) return null
  return (
    <div className="flex gap-2 text-[11px]">
      <span className="text-gray-500 flex-shrink-0 w-24">{label}:</span>
      <span className="text-gray-300 truncate">{value}</span>
    </div>
  )
}
