import React from 'react'
import clsx from 'clsx'
import type { LogFilter } from '../types'
import { LOG_LEVELS_FILTER } from '../types'

interface Props {
  filter: LogFilter
  onChange: (filter: LogFilter) => void
}

const LEVEL_DOT_CLASSES: Record<string, string> = {
  DEBUG:    'bg-gray-400',
  INFO:     'bg-blue-400',
  WARNING:  'bg-yellow-400',
  ERROR:    'bg-red-400',
  CRITICAL: 'bg-red-600',
}

export default function FilterBar({ filter, onChange }: Props) {
  function toggleLevel(level: string) {
    const has = filter.levels.includes(level)
    const levels = has
      ? filter.levels.filter(l => l !== level)
      : [...filter.levels, level]
    onChange({ ...filter, levels })
  }

  function setField(field: keyof LogFilter, value: string) {
    onChange({ ...filter, [field]: value } as LogFilter)
  }

  function clearAll() {
    onChange({
      service_ids: filter.service_ids, // preserve service selection
      levels: [],
      task_id: '',
      documento: '',
      module: '',
      search: '',
    })
  }

  const hasFilters = filter.levels.length > 0 ||
    filter.task_id || filter.documento || filter.module || filter.search

  return (
    <div className="bg-gray-900/80 border-b border-gray-800 px-4 py-2.5 flex flex-wrap gap-3 items-center">
      {/* Level toggles */}
      <div className="flex items-center gap-1.5 flex-shrink-0">
        <span className="text-gray-500 text-xs mr-1">Nível:</span>
        {LOG_LEVELS_FILTER.map(level => {
          const active = filter.levels.includes(level)
          return (
            <button
              key={level}
              onClick={() => toggleLevel(level)}
              className={clsx(
                'flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium transition-colors border',
                active
                  ? 'bg-gray-700 border-gray-500 text-white'
                  : 'bg-transparent border-gray-700 text-gray-500 hover:border-gray-600 hover:text-gray-400'
              )}
            >
              <span className={clsx('w-1.5 h-1.5 rounded-full flex-shrink-0', LEVEL_DOT_CLASSES[level])} />
              {level === 'WARNING' ? 'WARN' : level === 'CRITICAL' ? 'CRIT' : level}
            </button>
          )
        })}
      </div>

      {/* Divider */}
      <div className="w-px h-5 bg-gray-700 flex-shrink-0" />

      {/* Text filters */}
      <div className="flex items-center gap-2 flex-wrap flex-1 min-w-0">
        <FilterInput
          placeholder="Task ID"
          value={filter.task_id}
          onChange={v => setField('task_id', v)}
        />
        <FilterInput
          placeholder="Documento (CNPJ/CPF)"
          value={filter.documento}
          onChange={v => setField('documento', v)}
        />
        <FilterInput
          placeholder="Módulo"
          value={filter.module}
          onChange={v => setField('module', v)}
        />
        <FilterInput
          placeholder="Busca livre..."
          value={filter.search}
          onChange={v => setField('search', v)}
          wide
        />
      </div>

      {/* Clear */}
      {hasFilters && (
        <button
          onClick={clearAll}
          className="flex-shrink-0 text-xs text-gray-500 hover:text-gray-300 transition flex items-center gap-1"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
          Limpar
        </button>
      )}
    </div>
  )
}

function FilterInput({
  placeholder,
  value,
  onChange,
  wide,
}: {
  placeholder: string
  value: string
  onChange: (v: string) => void
  wide?: boolean
}) {
  return (
    <input
      type="text"
      value={value}
      onChange={e => onChange(e.target.value)}
      placeholder={placeholder}
      className={clsx(
        'bg-gray-800 border border-gray-700 rounded px-2.5 py-1 text-xs text-white',
        'placeholder-gray-600 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-transparent',
        'transition',
        wide ? 'min-w-[160px]' : 'w-[130px]'
      )}
    />
  )
}
