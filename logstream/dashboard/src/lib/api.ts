import type {
  LoginResponse,
  Service,
  ServiceConfig,
  Stats,
  LogFilter,
  QueryResponse,
  TaskListResponse,
  HealthmonResponse,
} from '../types'

const BASE = import.meta.env.VITE_API_URL || ''

function getToken(): string | null {
  return localStorage.getItem('logstream_token')
}

function authHeaders(): HeadersInit {
  const token = getToken()
  return token
    ? { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' }
    : { 'Content-Type': 'application/json' }
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    localStorage.removeItem('logstream_token')
    window.location.href = '/login'
    throw new Error('Sessão expirada. Redirecionando para login...')
  }
  if (!res.ok) {
    let msg = `Erro HTTP ${res.status}`
    try {
      const body = await res.json()
      msg = body.error || body.message || msg
    } catch {
      // ignore parse error
    }
    throw new Error(msg)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export const api = {
  async login(username: string, password: string): Promise<LoginResponse> {
    const res = await fetch(`${BASE}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    return handleResponse<LoginResponse>(res)
  },

  async getServices(): Promise<Service[]> {
    const res = await fetch(`${BASE}/api/services`, {
      headers: authHeaders(),
    })
    return handleResponse<Service[]>(res)
  },

  async getStats(): Promise<Stats> {
    const res = await fetch(`${BASE}/api/stats`, {
      headers: authHeaders(),
    })
    return handleResponse<Stats>(res)
  },

  async getLogs(
    filter: Partial<LogFilter> & {
      from?: number
      to?: number
      limit?: number
      offset?: number
    }
  ): Promise<QueryResponse> {
    const params = new URLSearchParams()

    if (filter.service_ids && filter.service_ids.length > 0) {
      filter.service_ids.forEach(id => params.append('service_id', id))
    }
    if (filter.levels && filter.levels.length > 0) {
      filter.levels.forEach(l => params.append('level', l))
    }
    if (filter.task_id) params.set('task_id', filter.task_id)
    if (filter.documento) params.set('documento', filter.documento)
    if (filter.module) params.set('module', filter.module)
    if (filter.search) params.set('search', filter.search)
    if (filter.from != null) params.set('from', String(filter.from))
    if (filter.to != null) params.set('to', String(filter.to))
    if (filter.limit != null) params.set('limit', String(filter.limit))
    if (filter.offset != null) params.set('offset', String(filter.offset))

    const res = await fetch(`${BASE}/api/logs?${params.toString()}`, {
      headers: authHeaders(),
    })
    const data = await handleResponse<QueryResponse>(res)
    // Server may return entries: null when no logs exist — normalise to []
    if (!data.entries) data.entries = []
    return data
  },

  async updateConfig(serviceId: string, config: Partial<ServiceConfig>): Promise<void> {
    const res = await fetch(`${BASE}/api/services/${serviceId}/config`, {
      method: 'PUT',
      headers: authHeaders(),
      body: JSON.stringify(config),
    })
    return handleResponse<void>(res)
  },

  async deleteLogs(serviceId: string, days: number): Promise<void> {
    const res = await fetch(`${BASE}/api/logs/${serviceId}?days=${days}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    return handleResponse<void>(res)
  },

  async getTasks(opts: {
    service_ids?: string[]
    from?: number
    to?: number
    limit?: number
    offset?: number
  }): Promise<TaskListResponse> {
    const params = new URLSearchParams()
    if (opts.service_ids && opts.service_ids.length > 0) {
      opts.service_ids.forEach(id => params.append('service_id', id))
    }
    if (opts.from != null) params.set('from', String(opts.from))
    if (opts.to != null) params.set('to', String(opts.to))
    if (opts.limit != null) params.set('limit', String(opts.limit))
    if (opts.offset != null) params.set('offset', String(opts.offset))

    const res = await fetch(`${BASE}/api/logs/tasks?${params.toString()}`, {
      headers: authHeaders(),
    })
    const data = await handleResponse<TaskListResponse>(res)
    if (!data.tasks) data.tasks = []
    return data
  },

  async getHealthmon(serviceId?: string): Promise<HealthmonResponse> {
    const params = serviceId ? `?service_id=${serviceId}` : ''
    const res = await fetch(`${BASE}/api/healthmon${params}`, {
      headers: authHeaders(),
    })
    const data = await handleResponse<HealthmonResponse>(res)
    if (!data.services) data.services = []
    return data
  },
}

// Build WebSocket URL from base URL
export function buildWsUrl(baseUrl: string): string {
  const url = baseUrl || window.location.origin
  return url.replace(/^http/, 'ws').replace(/^https/, 'wss')
}
