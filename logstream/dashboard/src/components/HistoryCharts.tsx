import React, { useMemo } from 'react'
import ReactApexChart from 'react-apexcharts'
import type { LogEntry } from '../types'

interface Props {
  entries: LogEntry[]
  total: number
}

const LEVEL_COLORS: Record<string, string> = {
  CRITICAL: '#dc2626', ERROR: '#ef4444', WARNING: '#eab308', INFO: '#3b82f6', DEBUG: '#6b7280',
}
const LEVEL_LABELS: Record<string, string> = {
  CRITICAL: 'CRIT', ERROR: 'ERROR', WARNING: 'WARN', INFO: 'INFO', DEBUG: 'DEBUG',
}

function resolveLevel(raw: unknown): string {
  if (typeof raw === 'number') {
    const m: Record<number, string> = { 0: 'DEBUG', 1: 'INFO', 2: 'WARNING', 3: 'ERROR', 4: 'CRITICAL' }
    return m[raw as number] ?? 'UNKNOWN'
  }
  return (raw as string) || 'UNKNOWN'
}

export default function HistoryCharts({ entries, total }: Props) {
  const levelData = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const e of entries) {
      const lvl = resolveLevel((e as any).level)
      counts[lvl] = (counts[lvl] ?? 0) + 1
    }
    const order = ['CRITICAL', 'ERROR', 'WARNING', 'INFO', 'DEBUG']
    const labels = order.filter(l => (counts[l] ?? 0) > 0)
    return { labels, values: labels.map(l => counts[l]), colors: labels.map(l => LEVEL_COLORS[l]) }
  }, [entries])

  const moduleData = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const e of entries) {
      const mod = (e as any).module || '(geral)'
      counts[mod] = (counts[mod] ?? 0) + 1
    }
    return Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8)
  }, [entries])

  const totalErrors = (levelData.labels.includes('ERROR')    ? levelData.values[levelData.labels.indexOf('ERROR')]    : 0)
                    + (levelData.labels.includes('CRITICAL') ? levelData.values[levelData.labels.indexOf('CRITICAL')] : 0)
  const totalWarnings = levelData.labels.includes('WARNING') ? levelData.values[levelData.labels.indexOf('WARNING')] : 0

  const donutOptions: ApexCharts.ApexOptions = {
    chart: { type: 'donut', background: 'transparent', animations: { speed: 400 } },
    labels: levelData.labels.map(l => LEVEL_LABELS[l] ?? l),
    colors: levelData.colors,
    legend: { position: 'bottom', labels: { colors: '#9ca3af' }, fontSize: '10px', itemMargin: { horizontal: 6 } },
    dataLabels: {
      enabled: true,
      formatter: (_: number, opts: any) => {
        const v = opts.w.globals.series[opts.seriesIndex]
        return v > 0 ? String(v) : ''
      },
      style: { fontSize: '10px', fontWeight: 'bold', colors: ['#fff'] },
      dropShadow: { enabled: false },
    },
    plotOptions: {
      pie: {
        donut: {
          size: '62%',
          labels: {
            show: true,
            total: {
              show: true, label: 'Página', color: '#9ca3af', fontSize: '11px',
              formatter: () => entries.length.toLocaleString('pt-BR'),
            },
          },
        },
      },
    },
    stroke: { width: 2, colors: ['#030712'] },
    tooltip: { theme: 'dark' },
    theme: { mode: 'dark' },
  }

  const barOptions: ApexCharts.ApexOptions = {
    chart: { type: 'bar', background: 'transparent', toolbar: { show: false }, animations: { speed: 400 } },
    plotOptions: { bar: { horizontal: true, borderRadius: 3, dataLabels: { position: 'top' } } },
    colors: ['#0e7490'],
    dataLabels: { enabled: true, style: { fontSize: '10px', colors: ['#9ca3af'] }, offsetX: 4 },
    xaxis: {
      categories: moduleData.map(([m]) => m),
      labels: { style: { colors: '#6b7280', fontSize: '9px' } },
      axisBorder: { show: false },
    },
    yaxis: { labels: { style: { colors: '#9ca3af', fontSize: '10px' }, maxWidth: 120 } },
    grid: { borderColor: '#1f2937', strokeDashArray: 3, xaxis: { lines: { show: true } }, yaxis: { lines: { show: false } } },
    tooltip: { theme: 'dark', y: { formatter: (v: number) => `${v} logs` } },
    theme: { mode: 'dark' },
  }

  if (entries.length === 0) return null

  return (
    <div className="flex-shrink-0 flex items-stretch bg-gray-950/60 border-b border-gray-800 overflow-x-auto">
      {/* Donut */}
      <div className="flex flex-col px-3 py-2 min-w-[210px] border-r border-gray-800">
        <span className="text-[10px] text-gray-500 uppercase tracking-wider mb-0.5">Níveis (página)</span>
        {levelData.values.length > 0
          ? <ReactApexChart options={donutOptions} series={levelData.values} type="donut" height={185} />
          : <div className="flex-1 flex items-center justify-center text-gray-700 text-xs">sem dados</div>}
      </div>

      {/* Bar: top modules */}
      {moduleData.length > 0 && (
        <div className="flex flex-col px-3 py-2 min-w-[360px] border-r border-gray-800">
          <span className="text-[10px] text-gray-500 uppercase tracking-wider mb-0.5">Top módulos (página)</span>
          <ReactApexChart
            options={barOptions}
            series={[{ name: 'Logs', data: moduleData.map(([, v]) => v) }]}
            type="bar"
            height={185}
          />
        </div>
      )}

      {/* Summary */}
      <div className="flex flex-col justify-center gap-3 px-5 py-3 min-w-[160px]">
        <div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider">Total encontrado</div>
          <div className="text-2xl font-bold text-white">{total.toLocaleString('pt-BR')}</div>
        </div>
        {totalErrors > 0 && (
          <div>
            <div className="text-[10px] text-red-500 uppercase tracking-wider">Erros nesta página</div>
            <div className="text-xl font-bold text-red-400">{totalErrors.toLocaleString('pt-BR')}</div>
          </div>
        )}
        {totalWarnings > 0 && (
          <div>
            <div className="text-[10px] text-yellow-600 uppercase tracking-wider">Alertas</div>
            <div className="text-lg font-bold text-yellow-400">{totalWarnings.toLocaleString('pt-BR')}</div>
          </div>
        )}
      </div>
    </div>
  )
}
