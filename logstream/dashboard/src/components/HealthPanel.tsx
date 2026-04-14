import React, { useState, useEffect, useCallback } from 'react'
import clsx from 'clsx'
import { api } from '../lib/api'
import type { ServiceDeps, DependencyStatus } from '../types'

const POLL_INTERVAL = 15_000 // 15s

const STATUS_CONFIG: Record<string, { label: string; color: string; dot: string; bg: string; border: string }> = {
  CLOSED: {
    label: 'Online',
    color: 'text-green-300',
    dot: 'bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.6)]',
    bg: 'bg-green-900/20',
    border: 'border-green-700/40',
  },
  OPEN: {
    label: 'Indisponível',
    color: 'text-red-300',
    dot: 'bg-red-500 animate-pulse',
    bg: 'bg-red-900/20',
    border: 'border-red-700/40',
  },
  HALF_OPEN: {
    label: 'Recuperando',
    color: 'text-yellow-300',
    dot: 'bg-yellow-400 animate-pulse',
    bg: 'bg-yellow-900/20',
    border: 'border-yellow-700/40',
  },
}

function getStatusConfig(status: string) {
  return STATUS_CONFIG[status] || STATUS_CONFIG.CLOSED
}

function formatAgo(ms: number | null): string {
  if (!ms) return '-'
  const diff = Date.now() - ms
  if (diff < 60_000) return `${Math.round(diff / 1000)}s atrás`
  if (diff < 3600_000) return `${Math.round(diff / 60_000)}min atrás`
  return `${(diff / 3600_000).toFixed(1)}h atrás`
}

export default function HealthPanel() {
  const [services, setServices] = useState<ServiceDeps[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const resp = await api.getHealthmon()
      setServices(resp.services)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erro ao buscar healthmon')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
    const timer = setInterval(load, POLL_INTERVAL)
    return () => clearInterval(timer)
  }, [load])

  // Flatten all dependencies across all services
  const allDeps: (DependencyStatus & { parentService: string })[] = []
  for (const svc of services) {
    for (const dep of svc.dependencies || []) {
      allDeps.push({ ...dep, parentService: svc.service_id })
    }
  }

  const openCount = allDeps.filter(d => d.status === 'OPEN').length
  const halfOpenCount = allDeps.filter(d => d.status === 'HALF_OPEN').length
  const closedCount = allDeps.filter(d => d.status === 'CLOSED').length

  return (
    <div className="flex-shrink-0 border-b border-gray-800 bg-gray-950/60">
      {/* Header */}
      <div className="flex items-center gap-3 px-4 py-2">
        <div className="flex items-center gap-1.5">
          <svg className="w-3.5 h-3.5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">Serviços Externos</span>
        </div>

        {allDeps.length > 0 && (
          <div className="flex items-center gap-2">
            {closedCount > 0 && (
              <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-green-900/30 border border-green-700/40 text-green-300">
                {closedCount} online
              </span>
            )}
            {openCount > 0 && (
              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-red-900/40 border border-red-700/50 text-red-300 animate-pulse">
                {openCount} indisponível{openCount !== 1 ? 'eis' : ''}
              </span>
            )}
            {halfOpenCount > 0 && (
              <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-yellow-900/40 border border-yellow-700/50 text-yellow-300">
                {halfOpenCount} recuperando
              </span>
            )}
          </div>
        )}

        {error && <span className="text-[10px] text-red-500 ml-2">{error}</span>}
        {loading && allDeps.length === 0 && <span className="text-[10px] text-gray-600">Carregando...</span>}
      </div>

      {/* Dependency cards */}
      <div className="flex items-stretch gap-2 px-4 pb-3 overflow-x-auto scrollbar-thin scrollbar-thumb-gray-700">
        {allDeps.length === 0 && !loading && (
          <div className="flex items-center gap-2 py-2 text-gray-600 text-xs">
            <svg className="w-4 h-4 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>Aguardando dados do healthmon. O agent precisa estar rodando com HEALTHMON_SERVICES configurado.</span>
          </div>
        )}
        {allDeps.map(dep => {
          const cfg = getStatusConfig(dep.status)
          return (
            <div
              key={dep.parentService + ':' + dep.service_id}
              className={clsx(
                'flex-shrink-0 rounded-lg border px-3 py-2 min-w-[180px] transition-colors',
                cfg.bg, cfg.border
              )}
            >
              {/* Top row: name + status */}
              <div className="flex items-center justify-between gap-2 mb-1.5">
                <div className="flex items-center gap-1.5 min-w-0">
                  <span className={clsx('w-2 h-2 rounded-full flex-shrink-0', cfg.dot)} />
                  <span className="text-xs font-semibold text-white truncate">{dep.name}</span>
                </div>
                <span className={clsx('text-[10px] font-medium flex-shrink-0', cfg.color)}>
                  {cfg.label}
                </span>
              </div>

              {/* Metrics */}
              <div className="grid grid-cols-2 gap-x-3 gap-y-0.5 text-[10px]">
                <div className="text-gray-500">Taxa de erro</div>
                <div className={clsx(
                  'text-right font-mono',
                  dep.error_rate > 0.3 ? 'text-red-400' : dep.error_rate > 0.1 ? 'text-yellow-400' : 'text-gray-300'
                )}>
                  {(dep.error_rate * 100).toFixed(1)}%
                </div>

                <div className="text-gray-500">Requisições</div>
                <div className="text-right text-gray-300 font-mono">{dep.total_requests}</div>

                <div className="text-gray-500">Erros</div>
                <div className={clsx('text-right font-mono', dep.total_errors > 0 ? 'text-red-400' : 'text-gray-300')}>
                  {dep.total_errors}
                </div>

                <div className="text-gray-500">Último ping</div>
                <div className={clsx('text-right', dep.last_ping_ok ? 'text-green-400' : 'text-red-400')}>
                  {dep.last_ping_ok ? 'OK' : 'FALHA'} {formatAgo(dep.last_ping_at)}
                </div>

                {dep.status === 'OPEN' && dep.opened_at && (
                  <>
                    <div className="text-gray-500">Aberto desde</div>
                    <div className="text-right text-red-400">{formatAgo(dep.opened_at)}</div>
                  </>
                )}
              </div>

              {/* Tags */}
              <div className="flex items-center gap-1 mt-1.5">
                {dep.essential && (
                  <span className="px-1.5 py-0.5 rounded text-[9px] font-bold bg-red-800/40 text-red-300 border border-red-700/40">
                    ESSENCIAL
                  </span>
                )}
                {dep.fallbacks && dep.fallbacks.length > 0 && (
                  <span className="px-1.5 py-0.5 rounded text-[9px] bg-gray-800 text-gray-400 border border-gray-700">
                    fallback: {dep.fallbacks.join(', ')}
                  </span>
                )}
                {services.length > 1 && (
                  <span className="px-1.5 py-0.5 rounded text-[9px] bg-cyan-900/30 text-cyan-400 border border-cyan-800/40">
                    {dep.parentService}
                  </span>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
