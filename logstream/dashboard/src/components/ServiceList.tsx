import React from 'react'
import { formatDistanceToNow } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import type { Service } from '../types'
import clsx from 'clsx'

interface Props {
  services: Service[]
  selectedIds: string[]
  onToggle: (id: string) => void
}

function timeAgo(unixTs: number): string {
  if (!unixTs) return 'nunca'
  try {
    return formatDistanceToNow(new Date(unixTs * 1000), { addSuffix: true, locale: ptBR })
  } catch {
    return '—'
  }
}

export default function ServiceList({ services, selectedIds, onToggle }: Props) {
  const online = services.filter(s => s.status === 'online')
  const offline = services.filter(s => s.status === 'offline')

  return (
    <div className="flex flex-col gap-1 p-2">
      <div className="flex items-center justify-between px-2 py-1 mb-1">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Serviços</span>
        <span className="text-xs text-gray-600">{online.length}/{services.length}</span>
      </div>

      {services.length === 0 && (
        <div className="text-center text-gray-600 text-xs py-6">
          Nenhum serviço registrado
        </div>
      )}

      {[...online, ...offline].map(service => {
        const isSelected = selectedIds.includes(service.id)
        const isOnline = service.status === 'online'

        return (
          <button
            key={service.id}
            onClick={() => onToggle(service.id)}
            className={clsx(
              'w-full text-left rounded-lg px-3 py-2.5 transition-colors group',
              isSelected
                ? 'bg-blue-600/20 border border-blue-600/50'
                : 'hover:bg-gray-800 border border-transparent'
            )}
          >
            <div className="flex items-center gap-2">
              {/* Status dot */}
              <span className={clsx(
                'flex-shrink-0 w-2 h-2 rounded-full',
                isOnline ? 'bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.6)]' : 'bg-gray-600'
              )} />

              {/* Name */}
              <span className={clsx(
                'flex-1 text-sm font-medium truncate',
                isSelected ? 'text-white' : 'text-gray-300'
              )}>
                {service.name}
              </span>

              {/* Checkmark when selected */}
              {isSelected && (
                <svg className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                </svg>
              )}
            </div>

            {/* Last seen */}
            <div className="ml-4 mt-0.5">
              <span className="text-xs text-gray-600">
                {isOnline ? 'online' : timeAgo(service.last_seen)}
              </span>
            </div>
          </button>
        )
      })}

      {selectedIds.length > 0 && (
        <button
          onClick={() => selectedIds.forEach(id => onToggle(id))}
          className="mt-2 text-xs text-gray-500 hover:text-gray-300 transition text-center py-1"
        >
          Limpar seleção
        </button>
      )}
    </div>
  )
}
