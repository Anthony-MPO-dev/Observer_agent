import React from 'react'
import clsx from 'clsx'

interface Props {
  servicesOnline: number
  servicesTotal: number
  totalToday: number
  wsStatus: string
  reconnectCount: number
}

export default function StatsBar({
  servicesOnline,
  servicesTotal,
  totalToday,
  wsStatus,
  reconnectCount,
}: Props) {
  const isConnected = wsStatus === 'connected'
  const isReconnecting = wsStatus === 'reconnecting' || wsStatus === 'connecting'
  const isDisconnected = wsStatus === 'disconnected'

  const wsBadgeClass = clsx(
    'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium border',
    isConnected && 'bg-green-900/40 border-green-700/60 text-green-300',
    isReconnecting && 'bg-yellow-900/40 border-yellow-700/60 text-yellow-300',
    isDisconnected && 'bg-red-900/40 border-red-700/60 text-red-300'
  )

  const wsDotClass = clsx(
    'w-1.5 h-1.5 rounded-full flex-shrink-0',
    isConnected && 'bg-green-400 shadow-[0_0_5px_rgba(74,222,128,0.7)]',
    isReconnecting && 'bg-yellow-400 animate-pulse',
    isDisconnected && 'bg-red-500'
  )

  const wsLabel = isConnected
    ? 'Conectado'
    : isReconnecting
    ? 'Reconectando'
    : 'Desconectado'

  return (
    <div className="fixed top-0 left-0 right-0 z-50 h-10 bg-gray-900 border-b border-gray-800 flex items-center px-4 gap-6 shadow-lg">
      {/* Brand */}
      <div className="flex items-center gap-2 flex-shrink-0">
        <div className="w-5 h-5 rounded bg-blue-600 flex items-center justify-center">
          <svg className="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2.5}
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
        </div>
        <span className="text-sm font-semibold text-white tracking-tight">logstream</span>
      </div>

      {/* Divider */}
      <div className="w-px h-5 bg-gray-700 flex-shrink-0" />

      {/* Services */}
      <div className="flex items-center gap-2 flex-shrink-0">
        <span
          className={clsx(
            'w-2 h-2 rounded-full flex-shrink-0',
            servicesOnline > 0
              ? 'bg-green-400 shadow-[0_0_5px_rgba(74,222,128,0.7)]'
              : 'bg-gray-600'
          )}
        />
        <span className="text-xs text-gray-300">
          <span className="font-medium text-white">{servicesOnline}</span>
          <span className="text-gray-500">/{servicesTotal}</span>
          <span className="text-gray-500 ml-1">serviços</span>
        </span>
      </div>

      {/* Divider */}
      <div className="w-px h-5 bg-gray-700 flex-shrink-0" />

      {/* Logs hoje */}
      <div className="flex items-center gap-1.5 flex-shrink-0">
        <svg className="w-3.5 h-3.5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
        <span className="text-xs text-gray-300">
          Logs hoje:{' '}
          <span className="font-medium text-white">
            {totalToday.toLocaleString('pt-BR')}
          </span>
        </span>
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Reconnect count */}
      {reconnectCount > 0 && (
        <span className="text-xs text-orange-400 flex-shrink-0">
          {reconnectCount} reconex{reconnectCount === 1 ? 'ão' : 'ões'}
        </span>
      )}

      {/* WebSocket status badge */}
      <div className={wsBadgeClass}>
        <span className={wsDotClass} />
        <span>{wsLabel}</span>
      </div>
    </div>
  )
}
