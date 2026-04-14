export type LogLevel = 'UNKNOWN' | 'DEBUG' | 'INFO' | 'WARNING' | 'ERROR' | 'CRITICAL'

export interface LogEntry {
  id: string
  service_id: string
  service_name: string
  worker_type: string
  queue: string
  task_id: string
  documento: string
  module: string
  level: LogLevel
  message: string
  log_file: string
  unix_ts: number
  timestamp_str: string
  agent_id: string
  is_continuation: boolean
}

export interface ServiceConfig {
  service_id: string
  ttl_days: number
  min_level: LogLevel
  batch_size: number
  flush_ms: number
  enabled: boolean
}

export interface Service {
  id: string
  name: string
  status: 'online' | 'offline'
  last_seen: number
  agent_id: string
  version: string
  config: ServiceConfig
}

export interface LogFilter {
  service_ids: string[]
  levels: string[]
  task_id: string
  documento: string
  module: string
  search: string
}

export interface Stats {
  total_today: number
  services_online: number
}

export interface LoginResponse {
  token: string
}

export interface QueryResponse {
  entries: LogEntry[]
  total: number
  has_more: boolean
}

export interface TaskInfo {
  task_id: string
  service_id: string
  service_name: string
  worker_type: string
  queue: string
  count: number
  error_count: number
  warn_count: number
  first_seen: number // unix ms
  last_seen: number  // unix ms
}

export interface TaskListResponse {
  tasks: TaskInfo[]
  total: number
}

export interface DependencyStatus {
  service_id: string
  name: string
  status: 'CLOSED' | 'OPEN' | 'HALF_OPEN'
  error_rate: number
  total_requests: number
  total_errors: number
  essential: boolean
  fallbacks: string[]
  opened_at: number | null  // unix ms
  last_ping_at: number | null
  last_ping_ok: boolean
}

export interface ServiceDeps {
  service_id: string
  dependencies: DependencyStatus[]
  updated_at: number // unix ms
}

export interface HealthmonResponse {
  services: ServiceDeps[]
}

// Dashboard-local TTL config stored in localStorage
// This centralizes retention policy on the dashboard side
export interface DashboardTTLConfig {
  [service_id: string]: {
    ttl_days: number
    last_cleaned: number  // unix timestamp of last cleanup
    auto_clean: boolean   // whether to auto-clean on schedule
  }
}

export const DEFAULT_TTL_DAYS = 30
export const LOG_LEVELS: LogLevel[] = ['UNKNOWN', 'DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL']
export const LOG_LEVELS_FILTER: LogLevel[] = ['DEBUG', 'INFO', 'WARNING', 'ERROR', 'CRITICAL']
