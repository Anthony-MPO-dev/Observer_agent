import React, { useState, useEffect } from 'react'
import type { Service, LogLevel, ServiceConfig } from '../types'
import { LOG_LEVELS } from '../types'
import { api } from '../lib/api'
import {
  getTTLForService,
  setTTLForService,
  loadTTLConfig,
  runTTLCleanup,
  markCleaned,
} from '../lib/ttl'
import clsx from 'clsx'

interface Props {
  service: Service | null
  onConfigSaved?: () => void
}

export default function ConfigPanel({ service, onConfigSaved }: Props) {
  if (!service) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-gray-600 px-8">
        <svg className="w-12 h-12 mb-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1}
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <p className="text-sm text-center">Selecione um serviço na lista para configurar</p>
      </div>
    )
  }

  return <ServiceConfigForm service={service} onConfigSaved={onConfigSaved} />
}

function ServiceConfigForm({ service, onConfigSaved }: { service: Service; onConfigSaved?: () => void }) {
  const [ttlDays, setTtlDays] = useState(getTTLForService(service.id))
  const [autoClean, setAutoClean] = useState(loadTTLConfig()[service.id]?.auto_clean ?? true)
  const [minLevel, setMinLevel] = useState<LogLevel>(service.config?.min_level ?? 'DEBUG')
  const [enabled, setEnabled] = useState(service.config?.enabled ?? true)
  const [batchSize, setBatchSize] = useState(service.config?.batch_size ?? 100)
  const [flushMs, setFlushMs] = useState(service.config?.flush_ms ?? 2000)
  const [showAdvanced, setShowAdvanced] = useState(false)

  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [savedAt, setSavedAt] = useState<Date | null>(null)

  const [deleteLoading, setDeleteLoading] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteDays, setDeleteDays] = useState(ttlDays)
  const [deleteSuccess, setDeleteSuccess] = useState<string | null>(null)

  const [cleanupRunning, setCleanupRunning] = useState(false)
  const [cleanupMsg, setCleanupMsg] = useState<string | null>(null)

  // Re-init when service changes
  useEffect(() => {
    setTtlDays(getTTLForService(service.id))
    setAutoClean(loadTTLConfig()[service.id]?.auto_clean ?? true)
    setMinLevel(service.config?.min_level ?? 'DEBUG')
    setEnabled(service.config?.enabled ?? true)
    setBatchSize(service.config?.batch_size ?? 100)
    setFlushMs(service.config?.flush_ms ?? 2000)
    setSavedAt(null)
    setSaveError(null)
  }, [service.id])

  async function handleSave() {
    setSaving(true)
    setSaveError(null)
    try {
      // Save TTL to dashboard-local config (centralizes retention policy on dashboard)
      setTTLForService(service.id, ttlDays, autoClean)

      // Also push other config to server
      await api.updateConfig(service.id, {
        min_level: minLevel,
        enabled,
        batch_size: batchSize,
        flush_ms: flushMs,
        ttl_days: ttlDays,
      })

      setSavedAt(new Date())
      onConfigSaved?.()
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Erro ao salvar')
    } finally {
      setSaving(false)
    }
  }

  async function handleDeleteLogs() {
    setDeleteLoading(true)
    setDeleteError(null)
    setDeleteSuccess(null)
    try {
      await api.deleteLogs(service.id, deleteDays)
      markCleaned(service.id)
      setDeleteSuccess(`Logs com mais de ${deleteDays} dias removidos com sucesso.`)
      setShowDeleteConfirm(false)
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : 'Erro ao deletar logs')
    } finally {
      setDeleteLoading(false)
    }
  }

  async function handleRunCleanup() {
    setCleanupRunning(true)
    setCleanupMsg(null)
    try {
      await runTTLCleanup([service.id], (sid, ok, ttl) => {
        setCleanupMsg(ok
          ? `Limpeza executada: logs com mais de ${ttl} dias removidos.`
          : `Erro ao executar limpeza automática.`
        )
      })
      if (!cleanupMsg) setCleanupMsg('Limpeza não era necessária (executada recentemente).')
    } finally {
      setCleanupRunning(false)
    }
  }

  return (
    <div className="h-full overflow-y-auto p-6 space-y-6">
      {/* Service header */}
      <div className="flex items-center gap-3">
        <span className={clsx(
          'w-3 h-3 rounded-full flex-shrink-0',
          service.status === 'online'
            ? 'bg-green-400 shadow-[0_0_8px_rgba(74,222,128,0.6)]'
            : 'bg-gray-600'
        )} />
        <div>
          <h2 className="text-lg font-semibold text-white">{service.name}</h2>
          <p className="text-xs text-gray-500">ID: {service.id} · {service.status === 'online' ? 'Online' : 'Offline'}</p>
        </div>
      </div>

      {/* ─── TTL / Retenção (Dashboard-controlled) ─── */}
      <Section title="Retenção de Logs" subtitle="Gerenciado pelo dashboard">
        <div className="space-y-4">
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-sm text-gray-300">Tempo de retenção (TTL)</label>
              <span className="text-sm font-semibold text-blue-300">{ttlDays} dias</span>
            </div>
            <input
              type="range"
              min={1}
              max={90}
              value={ttlDays}
              onChange={e => setTtlDays(Number(e.target.value))}
              className="w-full h-2 bg-gray-700 rounded-full appearance-none cursor-pointer accent-blue-500"
            />
            <div className="flex justify-between text-xs text-gray-600 mt-1">
              <span>1 dia</span>
              <span>30 dias</span>
              <span>60 dias</span>
              <span>90 dias</span>
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-300">Limpeza automática</p>
              <p className="text-xs text-gray-500">Executa diariamente ao abrir o dashboard</p>
            </div>
            <Toggle value={autoClean} onChange={setAutoClean} />
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={handleRunCleanup}
              disabled={cleanupRunning}
              className="px-3 py-1.5 rounded text-xs font-medium bg-gray-700/50 border border-gray-700
                         text-gray-300 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition"
            >
              {cleanupRunning ? 'Executando...' : 'Executar limpeza agora'}
            </button>
            {cleanupMsg && (
              <span className="text-xs text-green-400">{cleanupMsg}</span>
            )}
          </div>
        </div>
      </Section>

      {/* ─── Nível mínimo ─── */}
      <Section title="Configuração do Agente">
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-gray-300">Serviço habilitado</p>
              <p className="text-xs text-gray-500">Aceitar logs deste serviço</p>
            </div>
            <Toggle value={enabled} onChange={setEnabled} />
          </div>

          <div>
            <label className="block text-sm text-gray-300 mb-2">Nível mínimo de log</label>
            <select
              value={minLevel}
              onChange={e => setMinLevel(e.target.value as LogLevel)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-white
                         focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              {LOG_LEVELS.map(l => (
                <option key={l} value={l}>{l}</option>
              ))}
            </select>
          </div>
        </div>
      </Section>

      {/* ─── Advanced ─── */}
      <div>
        <button
          onClick={() => setShowAdvanced(v => !v)}
          className="flex items-center gap-2 text-sm text-gray-500 hover:text-gray-300 transition"
        >
          <svg
            className={clsx('w-4 h-4 transition-transform', showAdvanced && 'rotate-90')}
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
          Configurações avançadas
        </button>

        {showAdvanced && (
          <div className="mt-4 bg-gray-900 rounded-lg p-4 border border-gray-800 space-y-4">
            <div>
              <label className="block text-sm text-gray-300 mb-1.5">Tamanho do lote (batch_size)</label>
              <input
                type="number"
                min={1}
                max={1000}
                value={batchSize}
                onChange={e => setBatchSize(Number(e.target.value))}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-white
                           focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-300 mb-1.5">Intervalo de flush (flush_ms)</label>
              <input
                type="number"
                min={100}
                max={60000}
                step={100}
                value={flushMs}
                onChange={e => setFlushMs(Number(e.target.value))}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-white
                           focus:outline-none focus:ring-1 focus:ring-blue-500"
              />
              <p className="text-xs text-gray-600 mt-1">{flushMs}ms</p>
            </div>
          </div>
        )}
      </div>

      {/* ─── Save ─── */}
      <div className="flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={saving}
          className="px-5 py-2 rounded text-sm font-semibold bg-blue-600 hover:bg-blue-500
                     disabled:bg-blue-800 disabled:cursor-not-allowed text-white transition"
        >
          {saving ? 'Salvando...' : 'Salvar configurações'}
        </button>
        {savedAt && (
          <span className="text-xs text-green-400">
            Salvo às {savedAt.toLocaleTimeString('pt-BR')}
          </span>
        )}
        {saveError && (
          <span className="text-xs text-red-400">{saveError}</span>
        )}
      </div>

      {/* ─── Danger zone ─── */}
      <div className="border border-red-900/50 rounded-lg p-4">
        <h3 className="text-sm font-semibold text-red-400 mb-3">Zona de perigo</h3>

        <div className="space-y-3">
          <div>
            <label className="block text-sm text-gray-300 mb-1.5">
              Apagar logs mais antigos que (dias)
            </label>
            <div className="flex items-center gap-3">
              <input
                type="number"
                min={1}
                max={365}
                value={deleteDays}
                onChange={e => setDeleteDays(Number(e.target.value))}
                className="w-24 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm text-white
                           focus:outline-none focus:ring-1 focus:ring-red-500"
              />
              <button
                onClick={() => setShowDeleteConfirm(true)}
                disabled={deleteLoading}
                className="px-3 py-2 rounded text-xs font-medium bg-red-900/30 border border-red-800
                           text-red-300 hover:bg-red-900/60 disabled:opacity-50 disabled:cursor-not-allowed transition"
              >
                {deleteLoading ? 'Deletando...' : 'Deletar logs antigos'}
              </button>
            </div>
          </div>

          {deleteSuccess && (
            <p className="text-xs text-green-400">{deleteSuccess}</p>
          )}
          {deleteError && (
            <p className="text-xs text-red-400">{deleteError}</p>
          )}
        </div>
      </div>

      {/* Delete confirm modal */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 px-4">
          <div className="bg-gray-900 border border-gray-700 rounded-xl p-6 max-w-sm w-full shadow-2xl">
            <h3 className="text-lg font-semibold text-white mb-2">Confirmar exclusao</h3>
            <p className="text-sm text-gray-400 mb-6">
              Tem certeza que deseja apagar todos os logs de{' '}
              <strong className="text-white">{service.name}</strong>{' '}
              com mais de <strong className="text-red-400">{deleteDays} dias</strong>?
              Esta ação é irreversível.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="px-4 py-2 rounded text-sm font-medium bg-gray-800 border border-gray-700
                           text-gray-300 hover:bg-gray-700 transition"
              >
                Cancelar
              </button>
              <button
                onClick={handleDeleteLogs}
                disabled={deleteLoading}
                className="px-4 py-2 rounded text-sm font-semibold bg-red-700 hover:bg-red-600
                           disabled:opacity-50 disabled:cursor-not-allowed text-white transition"
              >
                {deleteLoading ? 'Deletando...' : 'Sim, deletar'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Section({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div className="bg-gray-900/60 rounded-lg p-4 border border-gray-800">
      <div className="mb-4">
        <h3 className="text-sm font-semibold text-gray-200">{title}</h3>
        {subtitle && <p className="text-xs text-gray-500 mt-0.5">{subtitle}</p>}
      </div>
      {children}
    </div>
  )
}

function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      onClick={() => onChange(!value)}
      className={clsx(
        'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
        value ? 'bg-blue-600' : 'bg-gray-700'
      )}
    >
      <span
        className={clsx(
          'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
          value ? 'translate-x-6' : 'translate-x-1'
        )}
      />
    </button>
  )
}
