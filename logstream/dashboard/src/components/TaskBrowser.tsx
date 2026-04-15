import React, { useState, useCallback, useMemo } from 'react'
import clsx from 'clsx'
import { api } from '../lib/api'
import type { TaskInfo } from '../types'

const PAGE_SIZE = 50

interface Props {
  serviceIds: string[]
  fromTs?: number
  toTs?: number
  selectedTaskId: string
  onSelect: (taskId: string, serviceId: string) => void
}

export default function TaskBrowser({ serviceIds, fromTs, toTs, selectedTaskId, onSelect }: Props) {
  const [tasks, setTasks] = useState<TaskInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [expanded, setExpanded] = useState(true)
  const [filterText, setFilterText] = useState('')
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)

  const load = useCallback(async (pageNum: number = 0) => {
    setLoading(true)
    setError(null)
    try {
      const resp = await api.getTasks({
        service_ids: serviceIds.length > 0 ? serviceIds : undefined,
        from: fromTs,
        to: toTs,
        limit: PAGE_SIZE,
        offset: pageNum * PAGE_SIZE,
      })
      setTasks(resp.tasks)
      setTotal(resp.total)
      setPage(pageNum)
      setLoaded(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao buscar tasks')
    } finally {
      setLoading(false)
    }
  }, [serviceIds, fromTs, toTs])

  const totalPages = Math.ceil(total / PAGE_SIZE)

  // Group by service_id + apply text filter
  const grouped = useMemo(() => {
    const map = new Map<string, { serviceName: string; tasks: TaskInfo[] }>()
    for (const t of tasks) {
      const key = t.service_id
      if (!map.has(key)) {
        map.set(key, { serviceName: t.service_name || key, tasks: [] })
      }
      map.get(key)!.tasks.push(t)
    }
    if (filterText) {
      const q = filterText.toLowerCase()
      for (const [key, group] of map) {
        group.tasks = group.tasks.filter(t =>
          t.task_id.toLowerCase().includes(q) ||
          (t.service_name || '').toLowerCase().includes(q) ||
          (t.worker_type || '').toLowerCase().includes(q)
        )
        if (group.tasks.length === 0) map.delete(key)
      }
    }
    return map
  }, [tasks, filterText])

  const totalFiltered = useMemo(() => {
    let n = 0
    for (const g of grouped.values()) n += g.tasks.length
    return n
  }, [grouped])

  function formatTime(ms: number) {
    if (!ms) return '-'
    const d = new Date(ms)
    return `${d.toLocaleDateString('pt-BR')} ${d.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' })}`
  }

  function formatDuration(firstMs: number, lastMs: number) {
    const diff = lastMs - firstMs
    if (diff < 1000) return '<1s'
    if (diff < 60_000) return `${Math.round(diff / 1000)}s`
    if (diff < 3600_000) return `${Math.round(diff / 60_000)}min`
    return `${(diff / 3600_000).toFixed(1)}h`
  }

  return (
    <div className="flex-shrink-0 border-b border-gray-800 bg-gray-900/60">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-2">
        <button
          onClick={() => setExpanded(v => !v)}
          className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-gray-200 transition"
        >
          <svg className={clsx('w-3 h-3 transition-transform', expanded && 'rotate-90')} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          <span className="font-medium">Tasks / Arquivos</span>
        </button>

        <button
          onClick={() => load(0)}
          disabled={loading}
          className="flex items-center gap-1 px-2.5 py-1 rounded text-[10px] font-semibold bg-blue-600/80 hover:bg-blue-500 disabled:bg-blue-800 disabled:cursor-not-allowed text-white transition"
        >
          {loading ? (
            <svg className="animate-spin w-3 h-3" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
          ) : (
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          )}
          {loaded ? 'Atualizar' : 'Carregar'}
        </button>

        {loaded && (
          <span className="text-[10px] text-gray-500">
            {totalFiltered} de {total} task{total !== 1 ? 's' : ''}
            {totalPages > 1 && (
              <span className="ml-1 text-gray-600">
                (pg {page + 1}/{totalPages})
              </span>
            )}
          </span>
        )}

        {selectedTaskId && (
          <button
            onClick={() => onSelect('', '')}
            className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium bg-blue-600/25 border border-blue-500/50 text-blue-300 hover:bg-blue-600/40 transition ml-auto"
          >
            <span className="truncate max-w-[120px]">{selectedTaskId.slice(0, 8)}...</span>
            <svg className="w-3 h-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        )}
      </div>

      {/* Body */}
      {expanded && loaded && (
        <div className="px-4 pb-3">
          {/* Search */}
          <input
            type="text"
            value={filterText}
            onChange={e => setFilterText(e.target.value)}
            placeholder="Filtrar tasks por ID, servi\u00e7o, tipo..."
            className="w-full bg-gray-800 border border-gray-700 rounded px-2.5 py-1.5 text-xs text-white placeholder-gray-600 focus:outline-none focus:ring-1 focus:ring-blue-500 mb-2"
          />

          {error && (
            <div className="text-xs text-red-400 mb-2">{error}</div>
          )}

          {/* Task list grouped by service */}
          <div className="max-h-[280px] overflow-y-auto space-y-2 scrollbar-thin scrollbar-thumb-gray-700">
            {grouped.size === 0 && (
              <div className="text-xs text-gray-600 text-center py-4">
                {tasks.length === 0 ? 'Nenhum task encontrado no per\u00edodo.' : 'Nenhum resultado para o filtro.'}
              </div>
            )}

            {Array.from(grouped.entries()).map(([svcId, group]) => (
              <div key={svcId}>
                <div className="flex items-center gap-1.5 mb-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-cyan-500 flex-shrink-0" />
                  <span className="text-[10px] font-semibold text-gray-400 uppercase tracking-wider">
                    {group.serviceName}
                  </span>
                  <span className="text-[10px] text-gray-600">({group.tasks.length})</span>
                </div>

                <div className="grid gap-1">
                  {group.tasks.map(t => (
                    <button
                      key={t.service_id + ':' + t.task_id}
                      onClick={() => onSelect(t.task_id, t.service_id)}
                      className={clsx(
                        'w-full text-left px-3 py-2 rounded border transition-colors',
                        selectedTaskId === t.task_id
                          ? 'bg-blue-600/20 border-blue-500/50 text-blue-200'
                          : 'bg-gray-800/50 border-gray-700/50 text-gray-300 hover:bg-gray-800 hover:border-gray-600'
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <code className="text-[11px] font-mono truncate">{t.task_id}</code>
                          {t.worker_type && t.worker_type !== 'unknown' && (
                            <span className="flex-shrink-0 px-1.5 py-0.5 rounded text-[9px] font-medium bg-gray-700 text-gray-400">
                              {t.worker_type}
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-2 flex-shrink-0">
                          {t.error_count > 0 && (
                            <span className="text-[10px] text-red-400 font-bold">{t.error_count}e</span>
                          )}
                          {t.warn_count > 0 && (
                            <span className="text-[10px] text-yellow-400">{t.warn_count}w</span>
                          )}
                          <span className="text-[10px] text-gray-500">{t.count} logs</span>
                        </div>
                      </div>
                      <div className="flex items-center gap-3 mt-1 text-[10px] text-gray-500">
                        <span>{formatTime(t.first_seen)}</span>
                        <span className="text-gray-600">dur: {formatDuration(t.first_seen, t.last_seen)}</span>
                        {t.queue && t.queue !== 'unknown' && (
                          <span className="text-gray-600">fila: {t.queue}</span>
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-2 pt-2 border-t border-gray-800">
              <button
                onClick={() => load(page - 1)}
                disabled={page === 0 || loading}
                className="px-2.5 py-1 rounded text-[10px] font-medium bg-gray-800 border border-gray-700 text-gray-400 hover:text-white hover:border-gray-500 disabled:opacity-30 disabled:cursor-not-allowed transition"
              >
                Anterior
              </button>
              <span className="text-[10px] text-gray-500">
                {page * PAGE_SIZE + 1}\u2013{Math.min((page + 1) * PAGE_SIZE, total)} de {total}
              </span>
              <button
                onClick={() => load(page + 1)}
                disabled={page >= totalPages - 1 || loading}
                className="px-2.5 py-1 rounded text-[10px] font-medium bg-gray-800 border border-gray-700 text-gray-400 hover:text-white hover:border-gray-500 disabled:opacity-30 disabled:cursor-not-allowed transition"
              >
                Pr\u00f3ximo
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}