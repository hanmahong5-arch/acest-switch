/**
 * Admin API Service
 * Extends syncServiceClient for admin management functionality
 */

import { syncServiceClient } from './sync'

// ===== User Management Types =====

export interface AdminUser {
  user_id: string
  username: string
  email?: string
  is_admin: boolean
  is_disabled: boolean
  created_at: string
  last_login_at?: string
  last_active_at?: string
  login_count: number
  device_count: number
  session_count: number
  message_count: number
  total_tokens: number
  total_cost: number
}

export interface UserListResponse {
  users: AdminUser[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface UserDetailResponse {
  user: AdminUser
  stats: UserStats
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

// ===== Session Management Types =====

export interface AdminSession {
  id: string
  user_id: string
  title: string
  summary?: string
  model?: string
  provider?: string
  message_count: number
  token_count: number
  cost: number
  is_pinned: boolean
  is_archived: boolean
  last_message_at?: string
  created_at: string
  updated_at: string
}

export interface SessionMessage {
  id: string
  session_id: string
  user_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  content_type: string
  model?: string
  provider?: string
  tokens_input: number
  tokens_output: number
  tokens_reasoning: number
  cost: number
  duration_ms: number
  finish_reason?: string
  created_at: string
}

export interface SessionListResponse {
  sessions: AdminSession[]
  total: number
  page: number
  page_size: number
}

export interface SessionDetailResponse {
  session: AdminSession
  messages: SessionMessage[]
}

// ===== Audit Log Types =====

export interface AuditLog {
  id: string
  user_id?: string
  username?: string
  action: string
  resource_type: string
  resource_id?: string
  details: Record<string, unknown>
  ip_address?: string
  user_agent?: string
  device_id?: string
  result: 'success' | 'failure' | 'blocked'
  error_message?: string
  duration_ms?: number
  created_at: string
}

export interface AuditLogListResponse {
  logs: AuditLog[]
  total: number
  page: number
  page_size: number
}

// ===== Alert Types =====

export interface AlertRule {
  id: string
  name: string
  description?: string
  metric: string
  condition: 'gt' | 'lt' | 'eq' | 'gte' | 'lte'
  threshold: number
  window_seconds: number
  severity: 'info' | 'warning' | 'critical'
  enabled: boolean
  notify_channels: string[]
  webhook_url?: string
  last_triggered_at?: string
  trigger_count: number
  created_at: string
  updated_at: string
}

export interface AlertHistory {
  id: string
  rule_id: string
  rule_name: string
  metric_value: number
  threshold: number
  severity: string
  message?: string
  status: 'firing' | 'resolved'
  notified_channels?: string[]
  triggered_at: string
  resolved_at?: string
}

export interface AlertRuleListResponse {
  rules: AlertRule[]
}

export interface AlertHistoryListResponse {
  alerts: AlertHistory[]
  total: number
}

// ===== User Management API =====

export async function listUsers(params?: {
  page?: number
  page_size?: number
  search?: string
  disabled?: boolean
}): Promise<UserListResponse> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', String(params.page))
  if (params?.page_size) query.set('page_size', String(params.page_size))
  if (params?.search) query.set('search', params.search)
  if (params?.disabled !== undefined) query.set('disabled', String(params.disabled))

  return syncServiceClient.fetch(`/api/v1/admin/users?${query}`)
}

export async function getUser(userId: string): Promise<UserDetailResponse> {
  return syncServiceClient.fetch(`/api/v1/admin/users/${userId}`)
}

export async function disableUser(userId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/admin/users/${userId}/disable`, { method: 'POST' })
}

export async function enableUser(userId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/admin/users/${userId}/enable`, { method: 'POST' })
}

export async function setUserAdmin(userId: string, isAdmin: boolean): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/admin/users/${userId}/admin`, {
    method: 'POST',
    body: JSON.stringify({ is_admin: isAdmin })
  })
}

// ===== Session Management API =====

export async function listSessions(params?: {
  user_id?: string
  status?: string
  page?: number
  page_size?: number
}): Promise<SessionListResponse> {
  const query = new URLSearchParams()
  if (params?.user_id) query.set('user_id', params.user_id)
  if (params?.status) query.set('status', params.status)
  if (params?.page) query.set('page', String(params.page))
  if (params?.page_size) query.set('page_size', String(params.page_size))

  return syncServiceClient.fetch(`/api/v1/admin/sessions?${query}`)
}

export async function getSessionDetail(sessionId: string): Promise<SessionDetailResponse> {
  return syncServiceClient.fetch(`/api/v1/admin/sessions/${sessionId}`)
}

export async function deleteSession(sessionId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/admin/sessions/${sessionId}`, { method: 'DELETE' })
}

// ===== Audit Log API =====

export async function listAuditLogs(params?: {
  user_id?: string
  action?: string
  resource_type?: string
  result?: string
  start_time?: string
  end_time?: string
  page?: number
  page_size?: number
}): Promise<AuditLogListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })

  return syncServiceClient.fetch(`/api/v1/admin/audit-logs?${query}`)
}

// ===== Alert Management API =====

export async function listAlertRules(): Promise<AlertRuleListResponse> {
  return syncServiceClient.fetch('/api/v1/admin/alert-rules')
}

export async function createAlertRule(rule: Omit<AlertRule, 'id' | 'created_at' | 'updated_at' | 'last_triggered_at' | 'trigger_count'>): Promise<AlertRule> {
  return syncServiceClient.fetch('/api/v1/admin/alert-rules', {
    method: 'POST',
    body: JSON.stringify(rule)
  })
}

export async function updateAlertRule(ruleId: string, rule: Partial<AlertRule>): Promise<AlertRule> {
  return syncServiceClient.fetch(`/api/v1/admin/alert-rules/${ruleId}`, {
    method: 'PUT',
    body: JSON.stringify(rule)
  })
}

export async function deleteAlertRule(ruleId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/admin/alert-rules/${ruleId}`, { method: 'DELETE' })
}

export async function listAlertHistory(params?: {
  rule_id?: string
  severity?: string
  status?: string
  page?: number
  page_size?: number
}): Promise<AlertHistoryListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })

  return syncServiceClient.fetch(`/api/v1/admin/alert-history?${query}`)
}

// ===== Extend syncServiceClient with fetch method =====

declare module './sync' {
  interface SyncServiceClient {
    fetch<T>(path: string, options?: RequestInit): Promise<T>
  }
}

// Add fetch method to existing client if not present
if (!('fetch' in syncServiceClient)) {
  Object.defineProperty(syncServiceClient, 'fetch', {
    value: async function<T>(this: typeof syncServiceClient, path: string, options: RequestInit = {}): Promise<T> {
      const baseUrl = (this as any).baseUrl || 'http://localhost:8081'
      const accessToken = (this as any).accessToken || ''

      const url = `${baseUrl}${path}`
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...(options.headers as Record<string, string> || {}),
      }

      if (accessToken) {
        headers['Authorization'] = `Bearer ${accessToken}`
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
    },
    writable: true,
    configurable: true
  })
}
