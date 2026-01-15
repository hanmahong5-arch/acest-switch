/**
 * 同步服务 API
 * 用于与 Sync Service 和本地 SyncSettingsService 通信
 */

import {
  GetSettings as WailsGetSettings,
  UpdateSettings as WailsUpdateSettings,
  GetStatus as WailsGetStatus,
  TestConnection as WailsTestConnection,
} from '../../bindings/codeswitch/services/syncsettingsservice'

// ===== 类型定义 =====

export interface SyncSettings {
  enabled: boolean
  nats_url: string
  sync_server_url: string
  user_id: string
  session_id: string
  device_id: string
  device_name: string
  access_token?: string
}

export interface SyncStatus {
  enabled: boolean
  connected: boolean
}

export interface SystemStatus {
  status: string
  uptime_seconds: number
  uptime_human: string
  version: string
  go_version: string
  num_goroutine: number
  num_cpu: number
  memory: {
    alloc_bytes: number
    sys_bytes: number
    heap_alloc_bytes: number
    num_gc: number
  }
  nats: {
    connected: boolean
    last_error?: string
    reconnects: number
  }
  requests: {
    active: number
    peak: number
  }
}

export interface StatsOverview {
  total_requests: number
  total_tokens_in: number
  total_tokens_out: number
  total_cost: number
  total_errors: number
  success_rate: number
  active_users: number
  active_providers: number
  active_models: number
}

export interface TimeWindowStats {
  timestamp: string
  requests: number
  tokens_in: number
  tokens_out: number
  cost: number
  errors: number
  avg_latency_ms: number
}

export interface ProviderStats {
  provider: string
  requests: number
  tokens_in: number
  tokens_out: number
  cost: number
  errors: number
  success_rate: number
  avg_latency_ms: number
  last_used: string
}

export interface ModelStats {
  model: string
  provider: string
  requests: number
  tokens_in: number
  tokens_out: number
  cost: number
  avg_latency_ms: number
  last_used: string
}

export interface UserStats {
  user_id: string
  requests: number
  tokens_in: number
  tokens_out: number
  cost: number
  sessions: number
  messages: number
  last_active: string
}

export interface OnlineStatus {
  online_users: number
  active_users: number
}

// ===== 本地 Wails API（SyncSettingsService）=====

export async function getSyncSettings(): Promise<SyncSettings | null> {
  try {
    const settings = await WailsGetSettings()
    return settings as SyncSettings
  } catch (error) {
    console.error('Failed to get sync settings:', error)
    return null
  }
}

export async function updateSyncSettings(settings: SyncSettings): Promise<boolean> {
  try {
    await WailsUpdateSettings(settings)
    return true
  } catch (error) {
    console.error('Failed to update sync settings:', error)
    return false
  }
}

export async function getSyncStatus(): Promise<SyncStatus> {
  try {
    const status = await WailsGetStatus()
    return status as SyncStatus
  } catch (error) {
    console.error('Failed to get sync status:', error)
    return { enabled: false, connected: false }
  }
}

export async function testNATSConnection(natsUrl: string): Promise<{ success: boolean; message: string }> {
  try {
    const [success, message] = await WailsTestConnection(natsUrl)
    return { success, message }
  } catch (error) {
    console.error('Failed to test NATS connection:', error)
    return { success: false, message: String(error) }
  }
}

// ===== 远程 Sync Service API =====

class SyncServiceClient {
  private baseUrl: string = 'http://localhost:8081'
  private accessToken: string = ''

  setBaseUrl(url: string) {
    this.baseUrl = url.replace(/\/$/, '')
  }

  setAccessToken(token: string) {
    this.accessToken = token
  }

  async fetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${path}`
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    }

    if (this.accessToken) {
      headers['Authorization'] = `Bearer ${this.accessToken}`
    }

    const response = await fetch(url, {
      ...options,
      headers,
    })

    if (!response.ok) {
      const errorText = await response.text()
      throw new Error(`HTTP ${response.status}: ${errorText}`)
    }

    return response.json()
  }

  // 健康检查
  async health(): Promise<{ status: string }> {
    return this.fetch('/health')
  }

  // 系统状态
  async getSystemStatus(): Promise<SystemStatus> {
    return this.fetch('/api/v1/admin/system/status')
  }

  // 统计概览
  async getStatsOverview(): Promise<StatsOverview> {
    return this.fetch('/api/v1/admin/stats/overview')
  }

  // 小时统计
  async getHourlyStats(): Promise<{ stats: TimeWindowStats[] }> {
    return this.fetch('/api/v1/admin/stats/hourly')
  }

  // 日统计
  async getDailyStats(): Promise<{ stats: TimeWindowStats[] }> {
    return this.fetch('/api/v1/admin/stats/daily')
  }

  // 供应商统计
  async getProviderStats(): Promise<{ providers: ProviderStats[] }> {
    return this.fetch('/api/v1/admin/stats/providers')
  }

  // 模型统计
  async getModelStats(): Promise<{ models: ModelStats[] }> {
    return this.fetch('/api/v1/admin/stats/models')
  }

  // 用户统计
  async getUserStats(): Promise<{ users: UserStats[] }> {
    return this.fetch('/api/v1/admin/stats/users')
  }

  // 在线用户
  async getOnlineStatus(): Promise<OnlineStatus> {
    return this.fetch('/api/v1/admin/online')
  }

  // Prometheus metrics
  async getMetrics(): Promise<string> {
    const response = await fetch(`${this.baseUrl}/metrics`)
    return response.text()
  }
}

export const syncServiceClient = new SyncServiceClient()

// ===== 便捷函数 =====

export async function initSyncClient(): Promise<boolean> {
  try {
    const settings = await getSyncSettings()
    if (!settings) return false

    syncServiceClient.setBaseUrl(settings.sync_server_url)
    if (settings.access_token) {
      syncServiceClient.setAccessToken(settings.access_token)
    }

    return true
  } catch (error) {
    console.error('Failed to init sync client:', error)
    return false
  }
}
