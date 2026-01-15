/**
 * Billing Pinia Store
 * State management for billing module: subscriptions, wallets, payments, config
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listSubscriptions, getSubscription, createSubscription, cancelSubscription, reactivateSubscription, listPlans,
  listWallets, getWallet, createWallet, topUpWallet, getWalletTransactions,
  listPayments, getPayment, createPayment, refundPayment, manualConfirmPayment,
  getBillingConfig, updateBillingConfig, getBillingStatus, testCasdoorConnection, testLagoConnection, testPaymentConnection,
  type Subscription, type Plan, type Wallet, type WalletTransaction, type Payment, type BillingConfig, type BillingStatus
} from '../services/billing'

export const useBillingStore = defineStore('billing', () => {
  // ===== Billing Status =====
  const billingStatus = ref<BillingStatus | null>(null)
  const billingEnabled = ref(false)

  async function loadBillingStatus() {
    try {
      billingStatus.value = await getBillingStatus()
      billingEnabled.value = billingStatus.value?.enabled || false
    } catch (error) {
      console.error('Failed to load billing status:', error)
      billingEnabled.value = false
    }
  }

  // ===== Subscriptions =====
  const subscriptions = ref<Subscription[]>([])
  const subscriptionsTotal = ref(0)
  const subscriptionsLoading = ref(false)
  const subscriptionsPage = ref(1)
  const subscriptionsPageSize = ref(20)
  const currentSubscription = ref<Subscription | null>(null)

  const plans = ref<Plan[]>([])
  const plansLoading = ref(false)

  async function loadSubscriptions(page = 1, pageSize = 20, filters?: {
    user_id?: string
    status?: string
    plan_code?: string
  }) {
    if (subscriptionsLoading.value) return
    subscriptionsLoading.value = true
    subscriptionsPage.value = page
    subscriptionsPageSize.value = pageSize
    try {
      const data = await listSubscriptions({ page, page_size: pageSize, ...filters })
      subscriptions.value = data.subscriptions || []
      subscriptionsTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load subscriptions:', error)
      subscriptions.value = []
      subscriptionsTotal.value = 0
    } finally {
      subscriptionsLoading.value = false
    }
  }

  async function loadSubscription(subscriptionId: string) {
    try {
      currentSubscription.value = await getSubscription(subscriptionId)
    } catch (error) {
      console.error('Failed to load subscription:', error)
      currentSubscription.value = null
      throw error
    }
  }

  async function addSubscription(userId: string, planCode: string) {
    try {
      await createSubscription({ user_id: userId, plan_code: planCode })
      await loadSubscriptions(subscriptionsPage.value, subscriptionsPageSize.value)
    } catch (error) {
      console.error('Failed to create subscription:', error)
      throw error
    }
  }

  async function cancelUserSubscription(subscriptionId: string) {
    try {
      await cancelSubscription(subscriptionId)
      await loadSubscriptions(subscriptionsPage.value, subscriptionsPageSize.value)
    } catch (error) {
      console.error('Failed to cancel subscription:', error)
      throw error
    }
  }

  async function reactivateUserSubscription(subscriptionId: string) {
    try {
      await reactivateSubscription(subscriptionId)
      await loadSubscriptions(subscriptionsPage.value, subscriptionsPageSize.value)
    } catch (error) {
      console.error('Failed to reactivate subscription:', error)
      throw error
    }
  }

  async function loadPlans() {
    if (plansLoading.value) return
    plansLoading.value = true
    try {
      const data = await listPlans()
      plans.value = data.plans || []
    } catch (error) {
      console.error('Failed to load plans:', error)
      plans.value = []
    } finally {
      plansLoading.value = false
    }
  }

  // ===== Wallets/Balances =====
  const wallets = ref<Wallet[]>([])
  const walletsTotal = ref(0)
  const walletsLoading = ref(false)
  const walletsPage = ref(1)
  const walletsPageSize = ref(20)
  const currentWallet = ref<Wallet | null>(null)
  const walletTransactions = ref<WalletTransaction[]>([])

  async function loadWallets(page = 1, pageSize = 20, filters?: {
    user_id?: string
    status?: string
  }) {
    if (walletsLoading.value) return
    walletsLoading.value = true
    walletsPage.value = page
    walletsPageSize.value = pageSize
    try {
      const data = await listWallets({ page, page_size: pageSize, ...filters })
      wallets.value = data.wallets || []
      walletsTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load wallets:', error)
      wallets.value = []
      walletsTotal.value = 0
    } finally {
      walletsLoading.value = false
    }
  }

  async function loadWallet(walletId: string) {
    try {
      currentWallet.value = await getWallet(walletId)
    } catch (error) {
      console.error('Failed to load wallet:', error)
      currentWallet.value = null
      throw error
    }
  }

  async function addWallet(userId: string, name?: string, currency?: string) {
    try {
      await createWallet({ user_id: userId, name, currency })
      await loadWallets(walletsPage.value, walletsPageSize.value)
    } catch (error) {
      console.error('Failed to create wallet:', error)
      throw error
    }
  }

  async function topUp(walletId: string, paidCredits: string, grantedCredits?: string) {
    try {
      await topUpWallet(walletId, { paid_credits: paidCredits, granted_credits: grantedCredits })
      await loadWallets(walletsPage.value, walletsPageSize.value)
    } catch (error) {
      console.error('Failed to top up wallet:', error)
      throw error
    }
  }

  async function loadWalletTransactions(walletId: string, page = 1, pageSize = 50) {
    try {
      const data = await getWalletTransactions(walletId, { page, page_size: pageSize })
      walletTransactions.value = data.transactions || []
    } catch (error) {
      console.error('Failed to load wallet transactions:', error)
      walletTransactions.value = []
      throw error
    }
  }

  // ===== Payments =====
  const payments = ref<Payment[]>([])
  const paymentsTotal = ref(0)
  const paymentsLoading = ref(false)
  const paymentsPage = ref(1)
  const paymentsPageSize = ref(20)
  const currentPayment = ref<Payment | null>(null)

  async function loadPayments(page = 1, pageSize = 20, filters?: {
    user_id?: string
    status?: string
    method?: string
    start_time?: string
    end_time?: string
  }) {
    if (paymentsLoading.value) return
    paymentsLoading.value = true
    paymentsPage.value = page
    paymentsPageSize.value = pageSize
    try {
      const data = await listPayments({ page, page_size: pageSize, ...filters })
      payments.value = data.payments || []
      paymentsTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load payments:', error)
      payments.value = []
      paymentsTotal.value = 0
    } finally {
      paymentsLoading.value = false
    }
  }

  async function loadPayment(paymentId: string) {
    try {
      currentPayment.value = await getPayment(paymentId)
    } catch (error) {
      console.error('Failed to load payment:', error)
      currentPayment.value = null
      throw error
    }
  }

  async function addPayment(data: {
    user_id: string
    amount_cents: number
    method: 'alipay' | 'wechat'
    description?: string
    wallet_id?: string
  }) {
    try {
      const payment = await createPayment(data)
      await loadPayments(paymentsPage.value, paymentsPageSize.value)
      return payment
    } catch (error) {
      console.error('Failed to create payment:', error)
      throw error
    }
  }

  async function refund(paymentId: string, amountCents?: number, reason?: string) {
    try {
      await refundPayment(paymentId, { amount_cents: amountCents, reason })
      await loadPayments(paymentsPage.value, paymentsPageSize.value)
    } catch (error) {
      console.error('Failed to refund payment:', error)
      throw error
    }
  }

  async function confirmPayment(paymentId: string, note?: string) {
    try {
      await manualConfirmPayment(paymentId, { note })
      await loadPayments(paymentsPage.value, paymentsPageSize.value)
    } catch (error) {
      console.error('Failed to confirm payment:', error)
      throw error
    }
  }

  // ===== Configuration =====
  const billingConfig = ref<BillingConfig | null>(null)
  const configLoading = ref(false)
  const configSaving = ref(false)

  async function loadBillingConfig() {
    if (configLoading.value) return
    configLoading.value = true
    try {
      billingConfig.value = await getBillingConfig()
    } catch (error) {
      console.error('Failed to load billing config:', error)
      billingConfig.value = null
    } finally {
      configLoading.value = false
    }
  }

  async function saveBillingConfig(config: Partial<BillingConfig>) {
    if (configSaving.value) return
    configSaving.value = true
    try {
      await updateBillingConfig(config)
      await loadBillingConfig()
      await loadBillingStatus()
    } catch (error) {
      console.error('Failed to save billing config:', error)
      throw error
    } finally {
      configSaving.value = false
    }
  }

  async function testConnection(service: 'casdoor' | 'lago' | 'alipay' | 'wechat') {
    try {
      if (service === 'casdoor') {
        return await testCasdoorConnection()
      } else if (service === 'lago') {
        return await testLagoConnection()
      } else {
        return await testPaymentConnection(service)
      }
    } catch (error) {
      console.error(`Failed to test ${service} connection:`, error)
      throw error
    }
  }

  return {
    // Status
    billingStatus,
    billingEnabled,
    loadBillingStatus,

    // Subscriptions
    subscriptions,
    subscriptionsTotal,
    subscriptionsLoading,
    subscriptionsPage,
    subscriptionsPageSize,
    currentSubscription,
    plans,
    plansLoading,
    loadSubscriptions,
    loadSubscription,
    addSubscription,
    cancelUserSubscription,
    reactivateUserSubscription,
    loadPlans,

    // Wallets
    wallets,
    walletsTotal,
    walletsLoading,
    walletsPage,
    walletsPageSize,
    currentWallet,
    walletTransactions,
    loadWallets,
    loadWallet,
    addWallet,
    topUp,
    loadWalletTransactions,

    // Payments
    payments,
    paymentsTotal,
    paymentsLoading,
    paymentsPage,
    paymentsPageSize,
    currentPayment,
    loadPayments,
    loadPayment,
    addPayment,
    refund,
    confirmPayment,

    // Config
    billingConfig,
    configLoading,
    configSaving,
    loadBillingConfig,
    saveBillingConfig,
    testConnection,
  }
})
