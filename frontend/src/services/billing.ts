/**
 * Billing API Service
 * Handles billing management: subscriptions, balances, payments, and configuration
 */

import { syncServiceClient } from './sync'

// ===== Subscription Types =====

export interface Subscription {
  id: string
  external_id: string
  user_id: string
  username?: string
  plan_code: string
  plan_name: string
  status: 'active' | 'pending' | 'canceled' | 'terminated' | 'past_due'
  billing_time: 'calendar' | 'anniversary'
  started_at: string
  ending_at?: string
  canceled_at?: string
  created_at: string
}

export interface SubscriptionListResponse {
  subscriptions: Subscription[]
  total: number
  page: number
  page_size: number
}

export interface Plan {
  code: string
  name: string
  description?: string
  amount_cents: number
  amount_currency: string
  interval: 'monthly' | 'yearly' | 'weekly'
  trial_period_days?: number
  pay_in_advance: boolean
}

export interface PlanListResponse {
  plans: Plan[]
}

// ===== Balance/Wallet Types =====

export interface Wallet {
  id: string
  lago_id: string
  user_id: string
  username?: string
  name: string
  status: 'active' | 'terminated'
  currency: string
  balance_cents: number
  consumed_credits: number
  ongoing_balance_cents: number
  credits_balance: number
  ongoing_usage_balance_cents: number
  rate_amount: string
  created_at: string
  last_transaction_at?: string
}

export interface WalletListResponse {
  wallets: Wallet[]
  total: number
  page: number
  page_size: number
}

export interface WalletTransaction {
  id: string
  wallet_id: string
  user_id: string
  type: 'inbound' | 'outbound'
  transaction_type: 'paid_credits' | 'granted' | 'usage' | 'refund'
  status: 'pending' | 'settled'
  amount: string
  credit_amount: string
  settled_at?: string
  created_at: string
}

export interface WalletTransactionListResponse {
  transactions: WalletTransaction[]
  total: number
}

// ===== Payment Types =====

export interface Payment {
  id: string
  order_no: string
  user_id: string
  username?: string
  amount_cents: number
  currency: string
  method: 'alipay' | 'wechat' | 'stripe' | 'manual'
  status: 'pending' | 'paid' | 'failed' | 'refunded' | 'canceled'
  description?: string
  wallet_id?: string
  subscription_id?: string
  pay_url?: string
  paid_at?: string
  created_at: string
  updated_at: string
}

export interface PaymentListResponse {
  payments: Payment[]
  total: number
  page: number
  page_size: number
}

// ===== Configuration Types =====

export interface BillingConfig {
  enabled: boolean
  // Casdoor
  casdoor_endpoint: string
  casdoor_client_id: string
  casdoor_client_secret: string
  casdoor_organization: string
  casdoor_application: string
  casdoor_certificate: string
  // Lago
  lago_api_url: string
  lago_api_key: string
  // Payment
  alipay_app_id: string
  alipay_private_key: string
  alipay_public_key: string
  alipay_sandbox: boolean
  wechat_app_id: string
  wechat_mch_id: string
  wechat_api_key: string
  wechat_api_key_v3: string
  wechat_serial_no: string
  wechat_private_key: string
  payment_notify_url: string
  payment_return_url: string
  // Subscription
  grace_period_hours: number
  require_subscription: boolean
}

export interface BillingStatus {
  enabled: boolean
  services: {
    name: string
    enabled: boolean
    status: string
  }[]
}

// ===== Subscription API =====

export async function listSubscriptions(params?: {
  page?: number
  page_size?: number
  user_id?: string
  status?: string
  plan_code?: string
}): Promise<SubscriptionListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })
  return syncServiceClient.fetch(`/api/v1/billing/subscriptions?${query}`)
}

export async function getSubscription(subscriptionId: string): Promise<Subscription> {
  return syncServiceClient.fetch(`/api/v1/billing/subscriptions/${subscriptionId}`)
}

export async function createSubscription(data: {
  user_id: string
  plan_code: string
}): Promise<Subscription> {
  return syncServiceClient.fetch('/api/v1/billing/subscriptions', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function cancelSubscription(subscriptionId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/subscriptions/${subscriptionId}/cancel`, {
    method: 'POST'
  })
}

export async function reactivateSubscription(subscriptionId: string): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/subscriptions/${subscriptionId}/reactivate`, {
    method: 'POST'
  })
}

export async function listPlans(): Promise<PlanListResponse> {
  return syncServiceClient.fetch('/api/v1/billing/plans')
}

// ===== Wallet/Balance API =====

export async function listWallets(params?: {
  page?: number
  page_size?: number
  user_id?: string
  status?: string
}): Promise<WalletListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })
  return syncServiceClient.fetch(`/api/v1/billing/wallets?${query}`)
}

export async function getWallet(walletId: string): Promise<Wallet> {
  return syncServiceClient.fetch(`/api/v1/billing/wallets/${walletId}`)
}

export async function createWallet(data: {
  user_id: string
  name?: string
  currency?: string
  rate_amount?: string
}): Promise<Wallet> {
  return syncServiceClient.fetch('/api/v1/billing/wallets', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function topUpWallet(walletId: string, data: {
  paid_credits: string
  granted_credits?: string
}): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/wallets/${walletId}/top-up`, {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function getWalletTransactions(walletId: string, params?: {
  page?: number
  page_size?: number
}): Promise<WalletTransactionListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined) {
      query.set(key, String(value))
    }
  })
  return syncServiceClient.fetch(`/api/v1/billing/wallets/${walletId}/transactions?${query}`)
}

// ===== Payment API =====

export async function listPayments(params?: {
  page?: number
  page_size?: number
  user_id?: string
  status?: string
  method?: string
  start_time?: string
  end_time?: string
}): Promise<PaymentListResponse> {
  const query = new URLSearchParams()
  Object.entries(params || {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      query.set(key, String(value))
    }
  })
  return syncServiceClient.fetch(`/api/v1/billing/payments?${query}`)
}

export async function getPayment(paymentId: string): Promise<Payment> {
  return syncServiceClient.fetch(`/api/v1/billing/payments/${paymentId}`)
}

export async function createPayment(data: {
  user_id: string
  amount_cents: number
  method: 'alipay' | 'wechat'
  description?: string
  wallet_id?: string
}): Promise<Payment> {
  return syncServiceClient.fetch('/api/v1/billing/payments', {
    method: 'POST',
    body: JSON.stringify(data)
  })
}

export async function refundPayment(paymentId: string, data?: {
  amount_cents?: number
  reason?: string
}): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/payments/${paymentId}/refund`, {
    method: 'POST',
    body: JSON.stringify(data || {})
  })
}

export async function manualConfirmPayment(paymentId: string, data?: {
  note?: string
}): Promise<{ message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/payments/${paymentId}/confirm`, {
    method: 'POST',
    body: JSON.stringify(data || {})
  })
}

// ===== Configuration API =====

export async function getBillingConfig(): Promise<BillingConfig> {
  return syncServiceClient.fetch('/api/v1/billing/config')
}

export async function updateBillingConfig(config: Partial<BillingConfig>): Promise<{ message: string }> {
  return syncServiceClient.fetch('/api/v1/billing/config', {
    method: 'PUT',
    body: JSON.stringify(config)
  })
}

export async function getBillingStatus(): Promise<BillingStatus> {
  return syncServiceClient.fetch('/api/v1/billing/status')
}

export async function testCasdoorConnection(): Promise<{ success: boolean; message: string }> {
  return syncServiceClient.fetch('/api/v1/billing/test/casdoor', { method: 'POST' })
}

export async function testLagoConnection(): Promise<{ success: boolean; message: string }> {
  return syncServiceClient.fetch('/api/v1/billing/test/lago', { method: 'POST' })
}

export async function testPaymentConnection(method: 'alipay' | 'wechat'): Promise<{ success: boolean; message: string }> {
  return syncServiceClient.fetch(`/api/v1/billing/test/payment/${method}`, { method: 'POST' })
}

// ===== User Subscription Quick Actions =====

export async function getUserSubscriptionStatus(userId: string): Promise<{
  has_active_subscription: boolean
  subscription?: Subscription
  in_grace_period: boolean
  expires_at?: string
}> {
  return syncServiceClient.fetch(`/api/v1/billing/users/${userId}/subscription-status`)
}

export async function getUserBalance(userId: string): Promise<{
  balance_cents: number
  currency: string
  wallets: Wallet[]
}> {
  return syncServiceClient.fetch(`/api/v1/billing/users/${userId}/balance`)
}
