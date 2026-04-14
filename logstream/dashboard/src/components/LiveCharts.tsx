import React, { useMemo } from 'react'
import ReactApexChart from 'react-apexcharts'
import type { LogEntry } from '../types'

interface Props {
  entries: LogEntry[]
}

const LEVEL_COLORS: Record<string, string> = {
  CRITICAL: '#dc2626',
  ERROR:    '#ef4444',
  WARNING:  '#eab308',
  INFO:     '#3b82f6',
  DEBUG:    '#6b7280',
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

function toMs(v: number): number {
  if (!v) return 0
  return v > 1e10 ? v : v * 1000
}

const SPARKLINE_MINUTES = 20

export default function LiveCharts({ entries }: Props) {
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

  const sparklineData = useMemo(() => {
    const now = Date.now()
    const buckets = new Array(SPARKLINE_MINUTES).fill(0)
    for (const e of entries) {
      const ms = toMs((e as any).unix_ts || (e as any).timestamp || 0)
      if (!ms) continue
      const age = now - ms
      if (age < 0 || age > SPARKLINE_MINUTES * 60_000) continue
      const idx = SPARKLINE_MINUTES - 1 - Math.floor(age / 60_000)
      if (idx >= 0) buckets[idx]++
    }
    return buckets
  }, [entries])

  const now = Date.now()
  const xLabels = Array.from({ length: SPARKLINE_MINUTES }, (_, i) => {
    const d = new Date(now - (SPARKLINE_MINUTES - 1 - i) * 60_000)
    return `${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}`
  })

  const currentRate = sparklineData[sparklineData.length - 1] ?? 0
  const peakRate    = Math.max(...sparklineData, 0)
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
              show: true, label: 'Total', color: '#9ca3af', fontSize: '11px',
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

  const areaOptions: ApexCharts.ApexOptions = {
    chart: {
      type: 'area', background: 'transparent',
      toolbar: { show: false },
      animations: { enabled: false },
      zoom: { enabled: false },
    },
    stroke: { curve: 'smooth', width: 2 },
    colors: ['#3b82f6'],
    fill: {
      type: 'gradient',
      gradient: { shadeIntensity: 1, opacityFrom: 0.3, opacityTo: 0.01, stops: [0, 100] },
    },
    dataLabels: { enabled: false },
    xaxis: {
      categories: xLabels,
      labels: { style: { colors: '#6b7280', fontSize: '9px' }, rotate: 0 },
      tickAmount: 5,
      axisBorder: { show: false },
      axisTicks: { show: false },
    },
    yaxis: { labels: { style: { colors: '#6b7280', fontSize: '9px' } }, min: 0 },
    grid: { borderColor: '#1f2937', strokeDashArray: 3, padding: { left: 0, right: 8 } },
    tooltip: { theme: 'dark', y: { formatter: (v: number) => `${v} logs/min` } },
    theme: { mode: 'dark' },
  }

  if (entries.length === 0) return null

  return (
    <div className="flex-shrink-0 flex items-stretch bg-gray-950/60 border-b border-gray-800 overflow-x-auto">
      {/* Donut */}
      <div className="flex flex-col px-3 py-2 min-w-[210px] border-r border-gray-800">
        <span className="text-[10px] text-gray-500 uppercase tracking-wider mb-0.5">Distribuição de níveis</span>
        {levelData.values.length > 0
          ? <ReactApexChart options={donutOptions} series={levelData.values} type="donut" height={185} />
          : <div className="flex-1 flex items-center justify-center text-gray-700 text-xs">sem dados</div>}
      </div>

      {/* Area */}
      <div className="flex flex-col px-3 py-2 min-w-[380px] border-r border-gray-800">
        <div className="flex items-center justify-between mb-0.5">
          <span className="text-[10px] text-gray-500 uppercase tracking-wider">Atividade — últimos {SPARKLINE_MINUTES}min</span>
          <div className="flex items-center gap-3 text-[10px] text-gray-500">
            <span>agora: <span className="text-blue-400 font-medium">{currentRate}/min</span></span>
            <span>pico: <span className="text-gray-300 font-medium">{peakRate}/min</span></span>
          </div>
        </div>
        <ReactApexChart options={areaOptions} series={[{ name: 'Logs/min', data: sparklineData }]} type="area" height={175} />
      </div>

      {/* Summary */}
      <div className="flex flex-col justify-center gap-3 px-5 py-3 min-w-[150px]">
        <div>
          <div className="text-[10px] text-gray-500 uppercase tracking-wider">Em memória</div>
          <div className="text-2xl font-bold text-white">{entries.length.toLocaleString('pt-BR')}</div>
        </div>
        {totalErrors > 0 && (
          <div>
            <div className="text-[10px] text-red-500 uppercase tracking-wider">Erros</div>
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
