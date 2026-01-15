/**
 * Admin Pinia Store
 * State management for admin dashboard
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listUsers, getUser, disableUser, enableUser, setUserAdmin,
  listSessions, getSessionDetail, deleteSession,
  listAuditLogs,
  listAlertRules, createAlertRule, updateAlertRule, deleteAlertRule, listAlertHistory,
  type AdminUser, type AdminSession, type SessionMessage, type AuditLog, type AlertRule, type AlertHistory
} from '../services/admin'
import {
  syncServiceClient,
  type SystemStatus, type StatsOverview, type OnlineStatus,
  type TimeWindowStats, type ProviderStats, type ModelStats, type UserStats
} from '../services/sync'

export const useAdminStore = defineStore('admin', () => {
  // ===== System Status =====
  const systemStatus = ref<SystemStatus | null>(null)
  const systemLoading = ref(false)

  async function loadSystemStatus() {
    if (systemLoading.value) return
    systemLoading.value = true
    try {
      systemStatus.value = await syncServiceClient.getSystemStatus()
    } catch (error) {
      console.error('Failed to load system status:', error)
    } finally {
      systemLoading.value = false
    }
  }

  // ===== Stats Overview =====
  const statsOverview = ref<StatsOverview | null>(null)
  const hourlyStats = ref<TimeWindowStats[]>([])
  const dailyStats = ref<TimeWindowStats[]>([])
  const providerStats = ref<ProviderStats[]>([])
  const modelStats = ref<ModelStats[]>([])
  const userStatsData = ref<UserStats[]>([])
  const statsLoading = ref(false)

  async function loadStatsOverview() {
    try {
      statsOverview.value = await syncServiceClient.getStatsOverview()
    } catch (error) {
      console.error('Failed to load stats overview:', error)
    }
  }

  async function loadHourlyStats() {
    try {
      const data = await syncServiceClient.getHourlyStats()
      hourlyStats.value = data.stats || []
    } catch (error) {
      console.error('Failed to load hourly stats:', error)
    }
  }

  async function loadDailyStats() {
    try {
      const data = await syncServiceClient.getDailyStats()
      dailyStats.value = data.stats || []
    } catch (error) {
      console.error('Failed to load daily stats:', error)
    }
  }

  async function loadProviderStats() {
    try {
      const data = await syncServiceClient.getProviderStats()
      providerStats.value = data.providers || []
    } catch (error) {
      console.error('Failed to load provider stats:', error)
    }
  }

  async function loadModelStats() {
    try {
      const data = await syncServiceClient.getModelStats()
      modelStats.value = data.models || []
    } catch (error) {
      console.error('Failed to load model stats:', error)
    }
  }

  async function loadUserStatsData() {
    try {
      const data = await syncServiceClient.getUserStats()
      userStatsData.value = data.users || []
    } catch (error) {
      console.error('Failed to load user stats:', error)
    }
  }

  async function loadAllStats() {
    if (statsLoading.value) return
    statsLoading.value = true
    try {
      await Promise.all([
        loadStatsOverview(),
        loadHourlyStats(),
        loadDailyStats(),
        loadProviderStats(),
        loadModelStats(),
        loadUserStatsData(),
      ])
    } finally {
      statsLoading.value = false
    }
  }

  // ===== Online Users =====
  const onlineStatus = ref<OnlineStatus | null>(null)

  async function loadOnlineStatus() {
    try {
      onlineStatus.value = await syncServiceClient.getOnlineStatus()
    } catch (error) {
      console.error('Failed to load online status:', error)
    }
  }

  // ===== User Management =====
  const users = ref<AdminUser[]>([])
  const usersTotal = ref(0)
  const usersLoading = ref(false)
  const usersPage = ref(1)
  const usersPageSize = ref(20)

  async function loadUsers(page = 1, pageSize = 20, search = '', disabled?: boolean) {
    if (usersLoading.value) return
    usersLoading.value = true
    usersPage.value = page
    usersPageSize.value = pageSize
    try {
      const data = await listUsers({ page, page_size: pageSize, search, disabled })
      users.value = data.users || []
      usersTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load users:', error)
      users.value = []
      usersTotal.value = 0
    } finally {
      usersLoading.value = false
    }
  }

  async function toggleUserStatus(userId: string, enabled: boolean) {
    try {
      if (enabled) {
        await enableUser(userId)
      } else {
        await disableUser(userId)
      }
      await loadUsers(usersPage.value, usersPageSize.value)
    } catch (error) {
      console.error('Failed to toggle user status:', error)
      throw error
    }
  }

  async function toggleUserAdmin(userId: string, isAdmin: boolean) {
    try {
      await setUserAdmin(userId, isAdmin)
      await loadUsers(usersPage.value, usersPageSize.value)
    } catch (error) {
      console.error('Failed to toggle user admin:', error)
      throw error
    }
  }

  // ===== Session Management =====
  const sessions = ref<AdminSession[]>([])
  const sessionsTotal = ref(0)
  const sessionsLoading = ref(false)
  const sessionsPage = ref(1)
  const sessionsPageSize = ref(20)
  const currentSession = ref<AdminSession | null>(null)
  const currentMessages = ref<SessionMessage[]>([])

  async function loadSessions(page = 1, pageSize = 20, userId?: string) {
    if (sessionsLoading.value) return
    sessionsLoading.value = true
    sessionsPage.value = page
    sessionsPageSize.value = pageSize
    try {
      const data = await listSessions({ page, page_size: pageSize, user_id: userId })
      sessions.value = data.sessions || []
      sessionsTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load sessions:', error)
      sessions.value = []
      sessionsTotal.value = 0
    } finally {
      sessionsLoading.value = false
    }
  }

  async function loadSessionDetail(sessionId: string) {
    try {
      const data = await getSessionDetail(sessionId)
      currentSession.value = data.session
      currentMessages.value = data.messages || []
    } catch (error) {
      console.error('Failed to load session detail:', error)
      currentSession.value = null
      currentMessages.value = []
      throw error
    }
  }

  async function removeSession(sessionId: string) {
    try {
      await deleteSession(sessionId)
      await loadSessions(sessionsPage.value, sessionsPageSize.value)
    } catch (error) {
      console.error('Failed to delete session:', error)
      throw error
    }
  }

  // ===== Audit Logs =====
  const auditLogs = ref<AuditLog[]>([])
  const auditTotal = ref(0)
  const auditLoading = ref(false)
  const auditPage = ref(1)
  const auditPageSize = ref(50)

  async function loadAuditLogs(params?: {
    page?: number
    page_size?: number
    user_id?: string
    action?: string
    result?: string
    start_time?: string
    end_time?: string
  }) {
    if (auditLoading.value) return
    auditLoading.value = true
    auditPage.value = params?.page || 1
    auditPageSize.value = params?.page_size || 50
    try {
      const data = await listAuditLogs(params)
      auditLogs.value = data.logs || []
      auditTotal.value = data.total || 0
    } catch (error) {
      console.error('Failed to load audit logs:', error)
      auditLogs.value = []
      auditTotal.value = 0
    } finally {
      auditLoading.value = false
    }
  }

  // ===== Alert Management =====
  const alertRules = ref<AlertRule[]>([])
  const alertHistory = ref<AlertHistory[]>([])
  const alertsLoading = ref(false)

  async function loadAlertRules() {
    try {
      const data = await listAlertRules()
      alertRules.value = data.rules || []
    } catch (error) {
      console.error('Failed to load alert rules:', error)
      alertRules.value = []
    }
  }

  async function loadAlertHistory(params?: { rule_id?: string; severity?: string; status?: string }) {
    try {
      const data = await listAlertHistory({ ...params, page_size: 100 })
      alertHistory.value = data.alerts || []
    } catch (error) {
      console.error('Failed to load alert history:', error)
      alertHistory.value = []
    }
  }

  async function addAlertRule(rule: Parameters<typeof createAlertRule>[0]) {
    try {
      await createAlertRule(rule)
      await loadAlertRules()
    } catch (error) {
      console.error('Failed to create alert rule:', error)
      throw error
    }
  }

  async function editAlertRule(ruleId: string, updates: Partial<AlertRule>) {
    try {
      await updateAlertRule(ruleId, updates)
      await loadAlertRules()
    } catch (error) {
      console.error('Failed to update alert rule:', error)
      throw error
    }
  }

  async function removeAlertRule(ruleId: string) {
    try {
      await deleteAlertRule(ruleId)
      await loadAlertRules()
    } catch (error) {
      console.error('Failed to delete alert rule:', error)
      throw error
    }
  }

  return {
    // System Status
    systemStatus,
    systemLoading,
    loadSystemStatus,

    // Stats
    statsOverview,
    hourlyStats,
    dailyStats,
    providerStats,
    modelStats,
    userStatsData,
    statsLoading,
    loadStatsOverview,
    loadHourlyStats,
    loadDailyStats,
    loadProviderStats,
    loadModelStats,
    loadUserStatsData,
    loadAllStats,

    // Online
    onlineStatus,
    loadOnlineStatus,

    // Users
    users,
    usersTotal,
    usersLoading,
    usersPage,
    usersPageSize,
    loadUsers,
    toggleUserStatus,
    toggleUserAdmin,

    // Sessions
    sessions,
    sessionsTotal,
    sessionsLoading,
    sessionsPage,
    sessionsPageSize,
    currentSession,
    currentMessages,
    loadSessions,
    loadSessionDetail,
    removeSession,

    // Audit
    auditLogs,
    auditTotal,
    auditLoading,
    auditPage,
    auditPageSize,
    loadAuditLogs,

    // Alerts
    alertRules,
    alertHistory,
    alertsLoading,
    loadAlertRules,
    loadAlertHistory,
    addAlertRule,
    editAlertRule,
    removeAlertRule,
  }
})
