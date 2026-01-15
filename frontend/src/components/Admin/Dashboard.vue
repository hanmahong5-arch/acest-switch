<template>
  <div class="admin-dashboard">
    <!-- Page Header -->
    <header class="page-header">
      <h1>{{ t('admin.dashboard.title') }}</h1>
      <n-button @click="refreshAll" :loading="loading">
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        {{ t('admin.dashboard.refresh') }}
      </n-button>
    </header>

    <!-- System Status Section -->
    <section class="dashboard-section">
      <h2>{{ t('admin.dashboard.systemStatus') }}</h2>
      <n-spin :show="adminStore.systemLoading">
        <n-grid :cols="6" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.uptime')">
                {{ adminStore.systemStatus?.uptime_human || '-' }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.version')">
                {{ adminStore.systemStatus?.version || '-' }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.goroutines')" :value="adminStore.systemStatus?.num_goroutine || 0" />
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.memory')">
                {{ formatBytes(adminStore.systemStatus?.memory?.heap_alloc_bytes || 0) }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.nats')">
                <n-tag :type="adminStore.systemStatus?.nats?.connected ? 'success' : 'error'" size="small">
                  {{ adminStore.systemStatus?.nats?.connected ? t('admin.system.healthy') : t('admin.system.unhealthy') }}
                </n-tag>
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.system.activeRequests')" :value="adminStore.systemStatus?.requests?.active || 0" />
            </n-card>
          </n-gi>
        </n-grid>
      </n-spin>
    </section>

    <!-- Stats Overview Section -->
    <section class="dashboard-section">
      <h2>{{ t('admin.dashboard.statsOverview') }}</h2>
      <n-spin :show="adminStore.statsLoading">
        <n-grid :cols="5" :x-gap="16" :y-gap="16" responsive="screen" :item-responsive="true">
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.stats.requests')" :value="adminStore.statsOverview?.total_requests || 0" />
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.stats.tokens')">
                {{ formatNumber((adminStore.statsOverview?.total_tokens_in || 0) + (adminStore.statsOverview?.total_tokens_out || 0)) }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.stats.cost')">
                <template #prefix>$</template>
                {{ (adminStore.statsOverview?.total_cost || 0).toFixed(2) }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.stats.successRate')">
                {{ (adminStore.statsOverview?.success_rate || 0).toFixed(1) }}%
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi span="1">
            <n-card>
              <n-statistic :label="t('admin.online.today')" :value="adminStore.onlineStatus?.active_users || 0" />
            </n-card>
          </n-gi>
        </n-grid>
      </n-spin>
    </section>

    <!-- Online Users Section -->
    <section class="dashboard-section">
      <h2>{{ t('admin.dashboard.onlineUsers') }}</h2>
      <n-grid :cols="2" :x-gap="16">
        <n-gi>
          <n-card>
            <n-statistic :label="t('admin.online.now')" :value="adminStore.onlineStatus?.online_users || 0" />
          </n-card>
        </n-gi>
        <n-gi>
          <n-card>
            <n-statistic :label="t('admin.online.today')" :value="adminStore.onlineStatus?.active_users || 0" />
          </n-card>
        </n-gi>
      </n-grid>
    </section>

    <!-- Quick Actions Section -->
    <section class="dashboard-section">
      <h2>{{ t('admin.dashboard.quickActions') }}</h2>
      <n-space>
        <n-button type="primary" @click="router.push('/admin/users')">
          <template #icon><n-icon><PeopleOutline /></n-icon></template>
          {{ t('admin.nav.users') }}
        </n-button>
        <n-button @click="router.push('/admin/sessions')">
          <template #icon><n-icon><ChatbubblesOutline /></n-icon></template>
          {{ t('admin.nav.sessions') }}
        </n-button>
        <n-button @click="router.push('/admin/stats')">
          <template #icon><n-icon><StatsChartOutline /></n-icon></template>
          {{ t('admin.nav.stats') }}
        </n-button>
        <n-button @click="router.push('/admin/audit')">
          <template #icon><n-icon><DocumentTextOutline /></n-icon></template>
          {{ t('admin.nav.audit') }}
        </n-button>
        <n-button @click="router.push('/admin/alerts')">
          <template #icon><n-icon><NotificationsOutline /></n-icon></template>
          {{ t('admin.nav.alerts') }}
        </n-button>
      </n-space>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton, NIcon, NCard, NGrid, NGi, NStatistic, NSpace, NSpin, NTag
} from 'naive-ui'
import {
  RefreshOutline,
  PeopleOutline,
  ChatbubblesOutline,
  StatsChartOutline,
  DocumentTextOutline,
  NotificationsOutline,
} from '@vicons/ionicons5'
import { useAdminStore } from '../../stores/admin'
import { initSyncClient } from '../../services/sync'

const { t } = useI18n()
const router = useRouter()
const adminStore = useAdminStore()

const loading = ref(false)

const formatBytes = (bytes: number): string => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

const formatNumber = (value: number): string => {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return value.toLocaleString()
}

const refreshAll = async () => {
  loading.value = true
  try {
    await Promise.all([
      adminStore.loadSystemStatus(),
      adminStore.loadStatsOverview(),
      adminStore.loadOnlineStatus(),
    ])
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await initSyncClient()
  await refreshAll()
})
</script>

<style scoped>
.admin-dashboard {
  max-width: 1400px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-header h1 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.dashboard-section {
  margin-bottom: 32px;
}

.dashboard-section h2 {
  font-size: 16px;
  font-weight: 500;
  margin-bottom: 16px;
  color: var(--n-text-color-2);
}
</style>
