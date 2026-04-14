import React, { useState, useCallback } from 'react'
import type { LogFilter, LogEntry, QueryResponse } from '../types'
import { api } from '../lib/api'
import LogLine from './LogLine'
import FilterBar from './FilterBar'
import HistoryCharts from './HistoryCharts'
import TaskBrowser from './TaskBrowser'

interface Props {
  initialFilter: LogFilter
}

const PAGE_SIZE = 50

function toUnix(dateStr: string, endOfDay = false): number | undefined {
  if (!dateStr) return undefined
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return undefined
  if (endOfDay) d.setHours(23, 59, 59, 999)
  return Math.floor(d.getTime() / 1000)
}

export default function HistoryViewer({ initialFilter }: Props) {
  const [filter, setFilter] = useState<LogFilter>(initialFilter)
  const [fromDate, setFromDate] = useState('')
  const [toDate, setToDate] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<QueryResponse | null>(null)
  const [offset, setOffset] = useState(0)
  const [showCharts, setShowCharts] = useState(true)

  const search = useCallback(async (newOffset = 0) => {
    setLoading(true)
    setError(null)
    try {
      const resp = await api.getLogs({
        ...filter,
        from: toUnix(fromDate),
        to: toUnix(toDate, true),
        limit: PAGE_SIZE,
        offset: newOffset,
      })
      setResult(resp)
      setOffset(newOffset)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao buscar logs')
    } finally {
      setLoading(false)
    }
  }, [filter, fromDate, toDate])

  // Called when user picks a task from TaskBrowser
  const handleTaskSelect = useCallback((taskId: string, serviceId: string) => {
    setFilter(f => ({
      ...f,
      task_id: taskId,
      service_ids: serviceId ? [serviceId] : f.service_ids,
    }))
    // Auto-search when selecting a task
    if (taskId) {
      setTimeout(() => {
        // Use the updated filter values directly
        setLoading(true)
        setError(null)
        const searchFilter = {
          ...filter,
          task_id: taskId,
          service_ids: serviceId ? [serviceId] : filter.service_ids,
        }
        api.getLogs({
          ...searchFilter,
          from: toUnix(fromDate),
          to: toUnix(toDate, true),
          limit: PAGE_SIZE,
          offset: 0,
        }).then(resp => {
          setResult(resp)
          setOffset(0)
        }).catch(err => {
          setError(err instanceof Error ? err.message : 'Erro ao buscar logs')
        }).finally(() => {
          setLoading(false)
        })
      }, 0)
    }
  }, [filter, fromDate, toDate])

  function downloadJsonl() {
    if (!result) return
    const lines = result.entries.map(e => JSON.stringify(e)).join('\n')
    const blob = new Blob([lines], { type: 'application/x-ndjson' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `logs_${new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')}.jsonl`
    a.click()
    URL.revokeObjectURL(url)
  }

  const currentPage = Math.floor(offset / PAGE_SIZE) + 1
  const totalPages  = result ? Math.ceil(result.total / PAGE_SIZE) : 0

  // Compute unix ms for TaskBrowser date range
  const taskBrowserFrom = toUnix(fromDate)
  const taskBrowserTo = toUnix(toDate, true)

  return (
    <div className="flex flex-col h-full">
      {/* Controls */}
      <div className="flex-shrink-0 bg-gray-900/80 border-b border-gray-800 p-4 space-y-3">
        <div className="flex items-center gap-3 flex-wrap">
          <span className="text-xs text-gray-500 flex-shrink-0">Período:</span>
          <input type="date" value={fromDate} onChange={e => setFromDate(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded px-2.5 py-1 text-xs text-white focus:outline-none focus:ring-1 focus:ring-blue-500" />
          <span className="text-gray-600 text-xs">até</span>
          <input type="date" value={toDate} onChange={e => setToDate(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded px-2.5 py-1 text-xs text-white focus:outline-none focus:ring-1 focus:ring-blue-500" />

          <button onClick={() => search(0)} disabled={loading}
            className="flex items-center gap-1.5 px-4 py-1.5 rounded text-xs font-semibold bg-blue-600 hover:bg-blue-500 disabled:bg-blue-800 disabled:cursor-not-allowed text-white transition">
            {loading
              ? <><svg className="animate-spin w-3 h-3" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>Buscando...</>
              : <><svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>Buscar</>}
          </button>

          {result && (
            <>
              <button onClick={downloadJsonl}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-gray-700/50 border border-gray-700 text-gray-300 hover:bg-gray-700 transition">
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
                Exportar .jsonl
              </button>
              <button onClick={() => setShowCharts(v => !v)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium border transition-colors ${showCharts ? 'bg-purple-700/30 border-purple-600/50 text-purple-300' : 'bg-gray-700/30 border-gray-700 text-gray-400 hover:bg-gray-700/60 hover:text-gray-300'}`}>
                <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>
                Gráficos
              </button>
            </>
          )}
        </div>
        <FilterBar filter={filter} onChange={setFilter} />
      </div>

      {error && (
        <div className="flex-shrink-0 mx-4 mt-3 bg-red-900/30 border border-red-700 rounded px-4 py-2.5 text-red-300 text-sm">{error}</div>
      )}

      {/* Task Browser */}
      <TaskBrowser
        serviceIds={filter.service_ids}
        fromTs={taskBrowserFrom}
        toTs={taskBrowserTo}
        selectedTaskId={filter.task_id}
        onSelect={handleTaskSelect}
      />

      {/* Charts */}
      {result && showCharts && result.entries.length > 0 && (
        <HistoryCharts entries={result.entries} total={result.total} />
      )}

      {/* Results header */}
      {result && (
        <div className="flex-shrink-0 flex items-center justify-between px-4 py-2 border-b border-gray-800 bg-gray-900/40">
          <span className="text-xs text-gray-400">
            {result.total.toLocaleString('pt-BR')} resultado{result.total !== 1 ? 's' : ''}
            {result.total > 0 && <span className="text-gray-600"> — exibindo {offset + 1}–{Math.min(offset + PAGE_SIZE, result.total)}</span>}
          </span>
          {totalPages > 1 && (
            <div className="flex items-center gap-2">
              <button onClick={() => search(Math.max(0, offset - PAGE_SIZE))} disabled={offset === 0 || loading}
                className="px-2.5 py-1 rounded text-xs font-medium bg-gray-800 border border-gray-700 text-gray-300 hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition">Anterior</button>
              <span className="text-xs text-gray-500">{currentPage} / {totalPages}</span>
              <button onClick={() => search(offset + PAGE_SIZE)} disabled={!result.has_more || loading}
                className="px-2.5 py-1 rounded text-xs font-medium bg-gray-800 border border-gray-700 text-gray-300 hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition">Próxima</button>
            </div>
          )}
        </div>
      )}

      {/* Results */}
      <div className="flex-1 overflow-y-auto" style={{ minHeight: 0 }}>
        {!result && !loading && (
          <div className="flex flex-col items-center justify-center h-full text-gray-600">
            <svg className="w-12 h-12 mb-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <p className="text-sm">Configure os filtros e clique em Buscar</p>
            <p className="text-xs mt-1 text-gray-700">ou use o browser de Tasks para navegar pelos arquivos disponíveis</p>
          </div>
        )}
        {result && result.entries.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-gray-600">
            <p className="text-sm">Nenhum log encontrado para os filtros aplicados.</p>
          </div>
        )}
        {result && result.entries.map((entry, i) => (
          <LogLine key={(entry as any).id || i} entry={entry} index={i} />
        ))}
      </div>
    </div>
  )
}
