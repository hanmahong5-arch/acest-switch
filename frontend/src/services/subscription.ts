/**
 * Subscription Service API Client
 * Connects to subscription-service microservice for subscription and quota management
 */

// Subscription Service URL (can be configured)
const SUBSCRIPTION_SERVICE_URL = 'http://localhost:18104'

class SubscriptionServiceClient {
  private baseUrl: string

  constructor(baseUrl: string = SUBSCRIPTION_SERVICE_URL) {
    this.baseUrl = baseUrl
  }

  async fetch<T>(path: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${path}`
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Unknown error' }))
      throw new Error(error.message || `HTTP ${response.status}`)
    }

    const data = await response.json()
    if (data.success === false) {
      throw new Error(data.message || 'Request failed')
    }

    return data.data
  }
}

export const subscriptionClient = new SubscriptionServiceClient()

// ===== Types =====

export interface Plan {
  id: number
  name: string
  code: string
  type: 'monthly' | 'yearly'
  quota: number
  daily_quota: number
  price_cents: number
  currency: string
  group_name: string
  fallback_group: string
  features?: string
  description?: string
  sort_order: number
  status: number
  created_at: string
  updated_at: string
}

export interface Subscription {
  id: number
  user_id: number
  plan_id: number
  plan?: Plan
  status: 'active' | 'expired' | 'cancelled' | 'pending'
  started_at: string
  expires_at: string
  auto_renew: boolean
  current_quota: number
  used_quota: number
  last_reset_at: string
  // Daily quota fields
  daily_quota: number
  today_used: number
  last_daily_reset_at: string
  current_group: string
  // Cancellation
  cancelled_at?: string
  cancel_reason?: string
  external_id?: string
  created_at: string
  updated_at: string
}

export interface QuotaStatus {
  user_id: number
  plan_code: string
  plan_name: string
  has_quota: boolean
  current_group: string
  fallback_group?: string
  daily_quota: number
  daily_used: number
  daily_remaining: number
  total_quota: number
  total_used: number
  expires_at: string
  is_fallback: boolean
  last_daily_reset_at: string
}

export interface StatsOverview {
  total_subscriptions: number
  active_subscriptions: number
  expired_subscriptions: number
  total_revenue: number
  by_plan: Record<string, number>
  by_status: Record<string, number>
}

export interface SubscriptionListResponse {
  subscriptions: Subscription[]
  total: number
  page: number
  page_size: number
}

export interface PlanListResponse {
  plans: Plan[]
}

// ===== Plan API =====

export async function listPlans(): Promise<Plan[]> {
  const data = await subscriptionClient.fetch<Plan[]>('/api/v1/plans')
  return data || []
}

export async function getPlan(code: string): Promise<Plan> {
  return subscriptionClient.fetch<Plan>(`/api/v1/plans/${code}`)
}

// ===== Subscription API =====

export async function listSubscriptions(params?: {
  page?: number
  page_size?: number
  status?: string
  plan_code?: string
}): Promise<SubscriptionListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })
  return subscriptionClient.fetch<SubscriptionListResponse>(`/api/v1/subscriptions?${query}`)
}

export async function adminListSubscriptions(params?: {
  page?: number
  page_size?: number
  status?: string
  plan_code?: string
  user_id?: string
}): Promise<SubscriptionListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })
  return subscriptionClient.fetch<SubscriptionListResponse>(`/admin/v1/subscriptions?${query}`)
}

export async function getSubscription(id: number): Promise<Subscription> {
  return subscriptionClient.fetch<Subscription>(`/api/v1/subscriptions/${id}`)
}

export async function getUserSubscription(userId: number): Promise<Subscription> {
  return subscriptionClient.fetch<Subscription>(`/api/v1/subscriptions/user/${userId}`)
}

export async function createSubscription(data: {
  user_id: number
  plan_code: string
}): Promise<Subscription> {
  return subscriptionClient.fetch<Subscription>('/api/v1/subscriptions', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function cancelSubscription(id: number, reason?: string): Promise<void> {
  await subscriptionClient.fetch(`/api/v1/subscriptions/${id}`, {
    method: 'DELETE',
    body: JSON.stringify({ reason })
  })
}

export async function renewSubscription(id: number): Promise<void> {
  await subscriptionClient.fetch(`/api/v1/subscriptions/${id}/renew`, {
    method: 'POST'
  })
}

export async function resetDailyQuota(id: number): Promise<void> {
  await subscriptionClient.fetch(`/api/v1/subscriptions/${id}/reset-daily`, {
    method: 'POST'
  })
}

// ===== Quota API =====

export async function getUserQuota(userId: number): Promise<{
  current_quota: number
  used_quota: number
  has_quota: boolean
  daily_quota: number
  today_used: number
  daily_remaining: number
  has_daily_quota: boolean
  current_group: string
  is_fallback: boolean
  last_daily_reset_at: string
  expires_at: string
  plan?: Plan
}> {
  return subscriptionClient.fetch(`/api/v1/quota/${userId}`)
}

export async function getQuotaStatus(userId: number): Promise<QuotaStatus> {
  return subscriptionClient.fetch<QuotaStatus>(`/api/v1/quota/${userId}/status`)
}

export async function deductQuota(userId: number, amount: number): Promise<void> {
  await subscriptionClient.fetch('/api/v1/quota/deduct', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId, amount })
  })
}

// ===== Stats API =====

export async function getStatsOverview(): Promise<StatsOverview> {
  return subscriptionClient.fetch<StatsOverview>('/admin/v1/stats/overview')
}

// ===== Admin Plan API =====

export async function createPlan(plan: Partial<Plan>): Promise<Plan> {
  return subscriptionClient.fetch<Plan>('/admin/v1/plans', {
    method: 'POST',
    body: JSON.stringify(plan)
  })
}

export async function updatePlan(id: number, plan: Partial<Plan>): Promise<Plan> {
  return subscriptionClient.fetch<Plan>(`/admin/v1/plans/${id}`, {
    method: 'PUT',
    body: JSON.stringify(plan)
  })
}

export async function initDefaultPlans(): Promise<void> {
  await subscriptionClient.fetch('/admin/v1/plans/init', {
    method: 'POST'
  })
}
