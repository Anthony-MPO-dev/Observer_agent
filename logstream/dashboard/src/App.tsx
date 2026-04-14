import React, { useState, useCallback } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import clsx from 'clsx'

import { useAuth } from './hooks/useAuth'
import { useServices } from './hooks/useServices'
import type { LogFilter, Service } from './types'

import LoginPage from './components/LoginPage'
import StatsBar from './components/StatsBar'
import ServiceList from './components/ServiceList'
import LogViewer from './components/LogViewer'
import HistoryViewer from './components/HistoryViewer'
import ConfigPanel from './components/ConfigPanel'
import HealthPanel from './components/HealthPanel'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth()
  const location = useLocation()
  if (!isAuthenticated) return <Navigate to="/login" state={{ from: location }} replace />
  return <>{children}</>
}

type ActiveTab = 'live' | 'history' | 'config'

function Dashboard() {
  const { token, logout } = useAuth()
  const { services, loading: servicesLoading } = useServices()

  const [selectedServiceIds, setSelectedServiceIds] = useState<string[]>([])
  const [activeTab, setActiveTab] = useState<ActiveTab>('live')
  const [sidebarOpen, setSidebarOpen] = useState(true)

  const [liveFilter, setLiveFilter] = useState<LogFilter>({
    service_ids: [], levels: [], task_id: '', documento: '', module: '', search: '',
  })

  const handleToggleService = useCallback((id: string) => {
    setSelectedServiceIds(prev => {
      const next = prev.includes(id) ? prev.filter(s => s !== id) : [...prev, id]
      setLiveFilter(f => ({ ...f, service_ids: next }))
      return next
    })
  }, [])

  const handleFilterChange = useCallback((f: LogFilter) => {
    setLiveFilter(f)
    setSelectedServiceIds(f.service_ids)
  }, [])

  const [wsStatus, setWsStatus] = useState<string>('disconnected')
  const [wsReconnectCount, setWsReconnectCount] = useState(0)
  const handleWsStatus = useCallback((s: string, rc: number) => {
    setWsStatus(s)
    setWsReconnectCount(rc)
  }, [])

  const servicesOnline = services.filter(s => s.status === 'online').length
  const servicesTotal  = services.length

  const selectedService: Service | null =
    selectedServiceIds.length > 0 ? (services.find(s => s.id === selectedServiceIds[0]) ?? null) : null

  const safeToken = token ?? ''

  const historyInitialFilter: LogFilter = {
    service_ids: selectedServiceIds,
    levels: [], task_id: '', documento: '', module: '', search: '',
  }

  const tabs: { id: ActiveTab; label: string }[] = [
    { id: 'live',    label: 'Ao Vivo' },
    { id: 'history', label: 'Histórico' },
    { id: 'config',  label: 'Configuração' },
  ]

  return (
    <div className="flex flex-col h-screen bg-gray-950 text-gray-100 overflow-hidden">
      <StatsBar
        servicesOnline={servicesOnline}
        servicesTotal={servicesTotal}
        totalToday={0}
        wsStatus={wsStatus}
        reconnectCount={wsReconnectCount}
      />

      <div className="flex flex-1 pt-10 overflow-hidden">
        {/* Sidebar */}
        <div className={clsx(
          'flex-shrink-0 flex flex-col bg-gray-900 border-r border-gray-800 transition-all duration-200 overflow-hidden',
          sidebarOpen ? 'w-64' : 'w-0'
        )}>
          <div className="flex items-center justify-between px-3 py-2 border-b border-gray-800 flex-shrink-0">
            <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Serviços</span>
            <button onClick={() => setSidebarOpen(false)} className="text-gray-600 hover:text-gray-300 transition p-0.5 rounded">
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
            </button>
          </div>

          <div className="flex-1 overflow-y-auto">
            {servicesLoading ? (
              <div className="flex items-center justify-center py-8 text-gray-600 text-xs">Carregando...</div>
            ) : (
              <ServiceList services={services} selectedIds={selectedServiceIds} onToggle={handleToggleService} />
            )}
          </div>

          <div className="flex-shrink-0 border-t border-gray-800 p-2">
            <button onClick={logout}
              className="w-full flex items-center gap-2 px-3 py-2 rounded text-xs text-gray-500 hover:bg-gray-800 hover:text-gray-300 transition">
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                  d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
              Sair
            </button>
          </div>
        </div>

        {/* Main */}
        <div className="flex flex-col flex-1 overflow-hidden">
          <div className="flex-shrink-0 flex items-center gap-2 px-4 border-b border-gray-800 bg-gray-900/60">
            {!sidebarOpen && (
              <button onClick={() => setSidebarOpen(true)}
                className="flex-shrink-0 text-gray-500 hover:text-gray-300 transition p-1.5 rounded hover:bg-gray-800">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </button>
            )}

            <div className="flex items-center gap-1 py-1">
              {tabs.map(tab => (
                <button key={tab.id} onClick={() => setActiveTab(tab.id)}
                  className={clsx(
                    'px-4 py-1.5 rounded text-sm font-medium transition-colors',
                    activeTab === tab.id
                      ? 'bg-blue-600/20 text-blue-300 border border-blue-600/40'
                      : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800 border border-transparent'
                  )}>
                  {tab.label}
                </button>
              ))}
            </div>

            {selectedServiceIds.length > 0 && (
              <div className="ml-auto flex items-center gap-1.5">
                <span className="text-xs text-gray-500">
                  {selectedServiceIds.length} serviço{selectedServiceIds.length !== 1 ? 's' : ''} selecionado{selectedServiceIds.length !== 1 ? 's' : ''}
                </span>
                <button onClick={() => { setSelectedServiceIds([]); setLiveFilter(f => ({ ...f, service_ids: [] })) }}
                  className="text-gray-600 hover:text-gray-400 transition text-xs" title="Limpar seleção">×</button>
              </div>
            )}
          </div>

          {/* Health monitor panel — always visible */}
          <HealthPanel />

          <div className="flex-1 overflow-hidden">
            {activeTab === 'live' && (
              <LogViewer token={safeToken} filter={liveFilter} onFilterChange={handleFilterChange} onStatusChange={handleWsStatus} />
            )}
            {activeTab === 'history' && (
              <HistoryViewer initialFilter={historyInitialFilter} />
            )}
            {activeTab === 'config' && (
              <ConfigPanel service={selectedService} onConfigSaved={() => {}} />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/*" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
    </Routes>
  )
}
