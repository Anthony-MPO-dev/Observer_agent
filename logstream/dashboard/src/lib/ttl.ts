/**
 * TTL Manager — centralizes log retention policy on the dashboard side.
 *
 * The dashboard owns the TTL config per service. On startup and periodically,
 * it triggers DELETE /api/logs/{service_id}?days=N for each configured service.
 *
 * Config is persisted in localStorage under 'logstream_ttl_config'.
 */

import type { DashboardTTLConfig } from '../types'
import { DEFAULT_TTL_DAYS } from '../types'
import { api } from './api'

const STORAGE_KEY = 'logstream_ttl_config'
const CHECK_INTERVAL_MS = 60 * 60 * 1000 // every hour

export function loadTTLConfig(): DashboardTTLConfig {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as DashboardTTLConfig
  } catch { /* ignore */ }
  return {}
}

export function saveTTLConfig(config: DashboardTTLConfig): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(config))
  } catch { /* ignore */ }
}

export function getTTLForService(serviceId: string): number {
  const cfg = loadTTLConfig()
  return cfg[serviceId]?.ttl_days ?? DEFAULT_TTL_DAYS
}

export function setTTLForService(
  serviceId: string,
  ttl_days: number,
  auto_clean = true
): void {
  const cfg = loadTTLConfig()
  cfg[serviceId] = {
    ttl_days,
    auto_clean,
    last_cleaned: cfg[serviceId]?.last_cleaned ?? 0,
  }
  saveTTLConfig(cfg)
}

export function markCleaned(serviceId: string): void {
  const cfg = loadTTLConfig()
  if (cfg[serviceId]) {
    cfg[serviceId].last_cleaned = Math.floor(Date.now() / 1000)
  }
  saveTTLConfig(cfg)
}

/**
 * Run TTL cleanup for services that are due.
 * A service is due if last_cleaned is more than 24h ago (or never).
 */
export async function runTTLCleanup(
  serviceIds: string[],
  onResult?: (serviceId: string, ok: boolean, ttl: number) => void
): Promise<void> {
  const cfg = loadTTLConfig()
  const now = Math.floor(Date.now() / 1000)
  const oneDaySeconds = 24 * 60 * 60

  for (const serviceId of serviceIds) {
    const entry = cfg[serviceId]
    const ttl = entry?.ttl_days ?? DEFAULT_TTL_DAYS
    const auto = entry?.auto_clean ?? true
    const lastCleaned = entry?.last_cleaned ?? 0

    if (!auto) continue
    if (now - lastCleaned < oneDaySeconds) continue

    try {
      await api.deleteLogs(serviceId, ttl)
      markCleaned(serviceId)
      onResult?.(serviceId, true, ttl)
    } catch (err) {
      onResult?.(serviceId, false, ttl)
    }
  }
}

/**
 * Start a background interval that periodically checks and runs TTL cleanup.
 * Returns a cleanup function.
 */
export function startTTLScheduler(
  getServiceIds: () => string[],
  onResult?: (serviceId: string, ok: boolean, ttl: number) => void
): () => void {
  // Run once on startup after a short delay
  const initialTimer = setTimeout(() => {
    runTTLCleanup(getServiceIds(), onResult)
  }, 10_000)

  const interval = setInterval(() => {
    runTTLCleanup(getServiceIds(), onResult)
  }, CHECK_INTERVAL_MS)

  return () => {
    clearTimeout(initialTimer)
    clearInterval(interval)
  }
}
