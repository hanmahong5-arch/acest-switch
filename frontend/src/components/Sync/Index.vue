<template>
  <div class="sync-page">
    <!-- 顶部导航 -->
    <header class="sync-header">
      <n-button quaternary circle @click="goBack">
        <template #icon>
          <n-icon><ArrowBack /></n-icon>
        </template>
      </n-button>
      <h1>{{ t('sync.title') }}</h1>
      <div class="header-actions">
        <n-button quaternary circle :loading="refreshing" @click="refreshAll">
          <template #icon>
            <n-icon><Refresh /></n-icon>
          </template>
        </n-button>
      </div>
    </header>

    <div class="sync-content">
      <!-- 连接状态卡片 -->
      <section class="status-section">
        <div class="status-card" :class="connectionStatusClass">
          <div class="status-icon">
            <svg v-if="syncStatus.connected" viewBox="0 0 24 24" width="32" height="32">
              <path d="M22 11.08V12a10 10 0 11-5.93-9.14" fill="none" stroke="currentColor" stroke-width="2"/>
              <path d="M22 4L12 14.01l-3-3" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else viewBox="0 0 24 24" width="32" height="32">
              <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2"/>
              <path d="M15 9l-6 6M9 9l6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </div>
          <div class="status-info">
            <h3>{{ syncStatus.connected ? t('sync.status.connected') : t('sync.status.disconnected') }}</h3>
            <p>{{ syncStatus.enabled ? t('sync.status.enabled') : t('sync.status.disabled') }}</p>
          </div>
          <label class="mac-switch">
            <input type="checkbox" :checked="settings.enabled" @change="toggleSync" />
            <span></span>
          </label>
        </div>
      </section>

      <!-- NEW-API 配额 -->
      <section v-if="gatewayStatus?.success" class="quota-section">
        <n-card :title="t('gateway.quota', 'User Quota')">
          <template #header-extra>
            <n-tag :type="quotaProgressStatus">
              {{ quotaPercent }}%
            </n-tag>
          </template>
          <n-spin :show="gatewayLoading">
            <n-grid :cols="3" :x-gap="16" :y-gap="16" class="mb-4">
              <n-gi>
                <n-statistic :label="t('gateway.quotaTotal', 'Total')">
                  {{ formatQuota(gatewayStatus.quota?.quotaTotal || 0) }}
                </n-statistic>
              </n-gi>
              <n-gi>
                <n-statistic :label="t('gateway.quotaUsed', 'Used')">
                  {{ formatQuota(gatewayStatus.quota?.quotaUsed || 0) }}
                </n-statistic>
              </n-gi>
              <n-gi>
                <n-statistic :label="t('gateway.quotaRemain', 'Remaining')">
                  {{ formatQuota(gatewayStatus.quota?.quotaRemain || 0) }}
                </n-statistic>
              </n-gi>
            </n-grid>
            <n-progress
              type="line"
              :percentage="quotaPercent"
              :status="quotaProgressStatus"
              indicator-placement="inside"
            />
            <div v-if="gatewayStatus.user" class="quota-user">
              <span class="quota-user-label">{{ t('gateway.user', 'User') }}:</span>
              <span>{{ gatewayStatus.user.username || gatewayStatus.user.email }}</span>
            </div>
          </n-spin>
        </n-card>
      </section>

      <!-- 系统状态 -->
      <section v-if="syncStatus.connected && systemStatus" class="system-section">
        <h2>{{ t('sync.system.title') }}</h2>
        <div class="system-grid">
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.uptime') }}</div>
            <div class="system-value">{{ systemStatus.uptime_human }}</div>
          </div>
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.version') }}</div>
            <div class="system-value">{{ systemStatus.version }}</div>
          </div>
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.goroutines') }}</div>
            <div class="system-value">{{ systemStatus.num_goroutine }}</div>
          </div>
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.memory') }}</div>
            <div class="system-value">{{ formatBytes(systemStatus.memory?.heap_alloc_bytes || 0) }}</div>
          </div>
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.natsStatus') }}</div>
            <div class="system-value" :class="systemStatus.nats?.connected ? 'status-ok' : 'status-error'">
              {{ systemStatus.nats?.connected ? 'Connected' : 'Disconnected' }}
            </div>
          </div>
          <div class="system-card">
            <div class="system-label">{{ t('sync.system.activeRequests') }}</div>
            <div class="system-value">{{ systemStatus.requests?.active || 0 }}</div>
          </div>
        </div>
      </section>

      <!-- 统计概览 -->
      <section v-if="syncStatus.connected && statsOverview" class="stats-section">
        <h2>{{ t('sync.stats.title') }}</h2>
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-value">{{ formatNumber(statsOverview.total_requests) }}</div>
            <div class="stat-label">{{ t('sync.stats.requests') }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ formatNumber(statsOverview.total_tokens_in + statsOverview.total_tokens_out) }}</div>
            <div class="stat-label">{{ t('sync.stats.tokens') }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ formatCurrency(statsOverview.total_cost) }}</div>
            <div class="stat-label">{{ t('sync.stats.cost') }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" :class="successRateClass">{{ statsOverview.success_rate.toFixed(1) }}%</div>
            <div class="stat-label">{{ t('sync.stats.successRate') }}</div>
          </div>
        </div>

        <div class="stats-secondary">
          <div class="stat-item">
            <span class="stat-icon">👥</span>
            <span>{{ t('sync.stats.activeUsers') }}: {{ statsOverview.active_users }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-icon">🔌</span>
            <span>{{ t('sync.stats.providers') }}: {{ statsOverview.active_providers }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-icon">🤖</span>
            <span>{{ t('sync.stats.models') }}: {{ statsOverview.active_models }}</span>
          </div>
        </div>
      </section>

      <!-- 在线用户 -->
      <section v-if="syncStatus.connected && onlineStatus" class="online-section">
        <h2>{{ t('sync.online.title') }}</h2>
        <div class="online-grid">
          <div class="online-card">
            <div class="online-value">{{ onlineStatus.online_users }}</div>
            <div class="online-label">{{ t('sync.online.now') }}</div>
          </div>
          <div class="online-card">
            <div class="online-value">{{ onlineStatus.active_users }}</div>
            <div class="online-label">{{ t('sync.online.today') }}</div>
          </div>
        </div>
      </section>

      <!-- 设置表单 -->
      <section class="settings-section">
        <h2>{{ t('sync.settings.title') }}</h2>
        <div class="settings-form">
          <div class="form-group">
            <label>{{ t('sync.settings.natsUrl') }}</label>
            <input type="text" v-model="settings.nats_url" :placeholder="t('sync.settings.natsUrlPlaceholder')" />
            <button class="test-button" @click="testConnection" :disabled="testing">
              {{ testing ? t('sync.settings.testing') : t('sync.settings.test') }}
            </button>
          </div>
          <div class="form-group">
            <label>{{ t('sync.settings.syncServerUrl') }}</label>
            <input type="text" v-model="settings.sync_server_url" :placeholder="t('sync.settings.syncServerUrlPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('sync.settings.userId') }}</label>
            <input type="text" v-model="settings.user_id" :placeholder="t('sync.settings.userIdPlaceholder')" />
          </div>
          <div class="form-group">
            <label>{{ t('sync.settings.deviceName') }}</label>
            <input type="text" v-model="settings.device_name" :placeholder="t('sync.settings.deviceNamePlaceholder')" />
          </div>
          <div class="form-actions">
            <button class="save-button" @click="saveSettings" :disabled="saving">
              {{ saving ? t('sync.settings.saving') : t('sync.settings.save') }}
            </button>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton,
  NIcon,
  NCard,
  NSwitch,
  NInput,
  NProgress,
  NTag,
  NStatistic,
  NGrid,
  NGi,
  NDivider,
  NSpace,
  NSpin,
} from 'naive-ui'
import { ArrowBack, Refresh, CheckmarkCircle, CloseCircle } from '@vicons/ionicons5'
import {
  getSyncSettings,
  updateSyncSettings,
  getSyncStatus,
  testNATSConnection,
  syncServiceClient,
  initSyncClient,
  type SyncSettings,
  type SyncStatus,
  type SystemStatus,
  type StatsOverview,
  type OnlineStatus,
} from '../../services/sync'
import { getGatewayConfig, testConnection as testGatewayConnection, type ConnectionStatus } from '../../services/gateway'
import { showToast } from '../../utils/toast'

const router = useRouter()
const { t } = useI18n()

// 状态
const settings = reactive<SyncSettings>({
  enabled: false,
  nats_url: 'nats://localhost:4222',
  sync_server_url: 'http://localhost:8081',
  user_id: '',
  session_id: 'default',
  device_id: '',
  device_name: '',
})

const syncStatus = reactive<SyncStatus>({
  enabled: false,
  connected: false,
})

const systemStatus = ref<SystemStatus | null>(null)
const statsOverview = ref<StatsOverview | null>(null)
const onlineStatus = ref<OnlineStatus | null>(null)

// NEW-API Quota state
const gatewayStatus = ref<ConnectionStatus | null>(null)
const gatewayLoading = ref(false)

const refreshing = ref(false)
const testing = ref(false)
const saving = ref(false)

// Quota computed properties
const quotaPercent = computed(() => {
  if (!gatewayStatus.value?.quota) return 0
  const { quotaTotal, quotaUsed } = gatewayStatus.value.quota
  if (quotaTotal === 0) return 0
  return Math.round(((quotaTotal - quotaUsed) / quotaTotal) * 100)
})

const quotaProgressStatus = computed(() => {
  if (quotaPercent.value > 50) return 'success'
  if (quotaPercent.value > 20) return 'warning'
  return 'error'
})

const formatQuota = (value: number) => {
  return `$${(value / 500000).toFixed(2)}`
}

// 计算属性
const connectionStatusClass = computed(() => ({
  'status-connected': syncStatus.connected,
  'status-disconnected': !syncStatus.connected,
}))

const successRateClass = computed(() => {
  if (!statsOverview.value) return ''
  const rate = statsOverview.value.success_rate
  if (rate >= 95) return 'rate-good'
  if (rate >= 80) return 'rate-warn'
  return 'rate-bad'
})

// 格式化函数
const formatNumber = (n: number) => n.toLocaleString()
const formatCurrency = (n: number) => `$${n.toFixed(2)}`
const formatBytes = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

// 方法
const goBack = () => router.push('/')

const loadSettings = async () => {
  const data = await getSyncSettings()
  if (data) {
    Object.assign(settings, data)
  }
}

const loadStatus = async () => {
  const status = await getSyncStatus()
  Object.assign(syncStatus, status)
}

const loadRemoteData = async () => {
  if (!syncStatus.connected) return

  try {
    await initSyncClient()

    const [system, stats, online] = await Promise.all([
      syncServiceClient.getSystemStatus().catch(() => null),
      syncServiceClient.getStatsOverview().catch(() => null),
      syncServiceClient.getOnlineStatus().catch(() => null),
    ])

    systemStatus.value = system
    statsOverview.value = stats
    onlineStatus.value = online
  } catch (error) {
    console.error('Failed to load remote data:', error)
  }
}

const loadGatewayQuota = async () => {
  gatewayLoading.value = true
  try {
    const config = await getGatewayConfig()
    if (config.newApiEnabled && config.newApiToken) {
      gatewayStatus.value = await testGatewayConnection(config.newApiUrl, config.newApiToken)
    } else {
      gatewayStatus.value = null
    }
  } catch (error) {
    console.error('Failed to load gateway quota:', error)
    gatewayStatus.value = null
  } finally {
    gatewayLoading.value = false
  }
}

const refreshAll = async () => {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await Promise.all([
      loadStatus(),
      loadGatewayQuota(),
    ])
    await loadRemoteData()
    showToast(t('sync.status.refreshed'))
  } finally {
    refreshing.value = false
  }
}

const toggleSync = async (event: Event) => {
  const target = event.target as HTMLInputElement
  settings.enabled = target.checked
  await saveSettings()
}

const testConnection = async () => {
  if (testing.value) return
  testing.value = true
  try {
    const result = await testNATSConnection(settings.nats_url)
    if (result.success) {
      showToast(t('sync.settings.testSuccess'))
    } else {
      showToast(t('sync.settings.testFailed') + ': ' + result.message, 'error')
    }
  } finally {
    testing.value = false
  }
}

const saveSettings = async () => {
  if (saving.value) return
  saving.value = true
  try {
    const success = await updateSyncSettings(settings)
    if (success) {
      showToast(t('sync.settings.saved'))
      await loadStatus()
      await loadRemoteData()
    } else {
      showToast(t('sync.settings.saveFailed'), 'error')
    }
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await loadSettings()
  await Promise.all([
    loadStatus(),
    loadGatewayQuota(),
  ])
  await loadRemoteData()
})
</script>

<style scoped>
.sync-page {
  min-height: 100vh;
  background: var(--mac-bg-primary);
  color: var(--mac-text-primary);
}

.sync-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 24px;
  border-bottom: 1px solid var(--mac-border);
  background: var(--mac-bg-secondary);
  position: sticky;
  top: 0;
  z-index: 100;
}

.sync-header h1 {
  flex: 1;
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0;
}

.back-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--mac-text-primary);
  cursor: pointer;
  transition: background 0.2s;
}

.back-button:hover {
  background: var(--mac-bg-tertiary);
}

.header-actions {
  display: flex;
  gap: 8px;
}

.ghost-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.ghost-icon:hover {
  background: var(--mac-bg-tertiary);
  color: var(--mac-text-primary);
}

.ghost-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.spin-animation svg {
  animation: spin 1s linear infinite;
}

.sync-content {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px;
}

.sync-content section {
  margin-bottom: 32px;
}

.sync-content h2 {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 16px;
  color: var(--mac-text-secondary);
}

/* Status Card */
.status-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  border-radius: 12px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
}

.status-card.status-connected {
  border-color: var(--mac-success, #34c759);
}

.status-card.status-disconnected {
  border-color: var(--mac-error, #ff3b30);
}

.status-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
}

.status-connected .status-icon {
  background: rgba(52, 199, 89, 0.1);
  color: var(--mac-success, #34c759);
}

.status-disconnected .status-icon {
  background: rgba(255, 59, 48, 0.1);
  color: var(--mac-error, #ff3b30);
}

.status-info {
  flex: 1;
}

.status-info h3 {
  margin: 0 0 4px;
  font-size: 1.1rem;
  font-weight: 600;
}

.status-info p {
  margin: 0;
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
}

/* Mac Switch */
.mac-switch {
  position: relative;
  display: inline-block;
  width: 51px;
  height: 31px;
}

.mac-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.mac-switch span {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--mac-bg-tertiary);
  transition: 0.3s;
  border-radius: 31px;
}

.mac-switch span:before {
  position: absolute;
  content: "";
  height: 27px;
  width: 27px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: 0.3s;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(0,0,0,0.2);
}

.mac-switch input:checked + span {
  background-color: var(--mac-accent, #0a84ff);
}

.mac-switch input:checked + span:before {
  transform: translateX(20px);
}

/* System Grid */
.system-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 12px;
}

.system-card {
  padding: 16px;
  border-radius: 10px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
}

.system-label {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
  margin-bottom: 4px;
}

.system-value {
  font-size: 1rem;
  font-weight: 600;
}

.status-ok { color: var(--mac-success, #34c759); }
.status-error { color: var(--mac-error, #ff3b30); }

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.stat-card {
  padding: 20px;
  border-radius: 10px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
  text-align: center;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  margin-bottom: 4px;
}

.stat-label {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
}

.rate-good { color: var(--mac-success, #34c759); }
.rate-warn { color: var(--mac-warning, #ff9500); }
.rate-bad { color: var(--mac-error, #ff3b30); }

.stats-secondary {
  display: flex;
  gap: 24px;
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--mac-bg-secondary);
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
}

.stat-icon {
  font-size: 1rem;
}

/* Online Grid */
.online-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.online-card {
  padding: 24px;
  border-radius: 10px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
  text-align: center;
}

.online-value {
  font-size: 2rem;
  font-weight: 700;
  color: var(--mac-accent, #0a84ff);
}

.online-label {
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
  margin-top: 4px;
}

/* Settings Form */
.settings-form {
  padding: 20px;
  border-radius: 12px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  margin-bottom: 6px;
  color: var(--mac-text-secondary);
}

.form-group input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-bg-primary);
  color: var(--mac-text-primary);
  font-size: 0.875rem;
}

.form-group input:focus {
  outline: none;
  border-color: var(--mac-accent, #0a84ff);
}

.test-button {
  margin-top: 8px;
  padding: 6px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  background: transparent;
  color: var(--mac-text-primary);
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.2s;
}

.test-button:hover:not(:disabled) {
  background: var(--mac-bg-tertiary);
}

.test-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.form-actions {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.save-button {
  padding: 10px 24px;
  border: none;
  border-radius: 8px;
  background: var(--mac-accent, #0a84ff);
  color: white;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.2s;
}

.save-button:hover:not(:disabled) {
  opacity: 0.9;
}

.save-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Quota Section */
.quota-section {
  margin-bottom: 24px;
}

.quota-user {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--mac-border);
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
}

.quota-user-label {
  margin-right: 8px;
}

.mb-4 {
  margin-bottom: 16px;
}

@media (max-width: 600px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .stats-secondary {
    flex-wrap: wrap;
  }
}
</style>
