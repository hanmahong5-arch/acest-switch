<template>
  <div class="admin-stats">
    <header class="page-header">
      <h1>{{ t('admin.stats.title') }}</h1>
      <n-button @click="refreshStats" :loading="adminStore.statsLoading">
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        {{ t('admin.dashboard.refresh') }}
      </n-button>
    </header>

    <!-- Overview Cards -->
    <section class="stats-section">
      <n-spin :show="adminStore.statsLoading">
        <n-grid :cols="5" :x-gap="16" :y-gap="16">
          <n-gi>
            <n-card>
              <n-statistic :label="t('admin.stats.requests')" :value="adminStore.statsOverview?.total_requests || 0" />
            </n-card>
          </n-gi>
          <n-gi>
            <n-card>
              <n-statistic :label="t('admin.stats.tokens')">
                {{ formatNumber((adminStore.statsOverview?.total_tokens_in || 0) + (adminStore.statsOverview?.total_tokens_out || 0)) }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi>
            <n-card>
              <n-statistic :label="t('admin.stats.cost')">
                <template #prefix>$</template>
                {{ (adminStore.statsOverview?.total_cost || 0).toFixed(2) }}
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi>
            <n-card>
              <n-statistic :label="t('admin.stats.successRate')">
                {{ (adminStore.statsOverview?.success_rate || 0).toFixed(1) }}%
              </n-statistic>
            </n-card>
          </n-gi>
          <n-gi>
            <n-card>
              <n-statistic :label="t('admin.online.today')" :value="adminStore.onlineStatus?.active_users || 0" />
            </n-card>
          </n-gi>
        </n-grid>
      </n-spin>
    </section>

    <!-- Tabs for different stats -->
    <n-tabs type="line" animated>
      <!-- Provider Stats -->
      <n-tab-pane :name="'providers'" :tab="t('admin.stats.byProvider')">
        <n-card>
          <n-data-table
            :columns="providerColumns"
            :data="adminStore.providerStats"
            :row-key="(row) => row.provider"
          />
          <n-empty v-if="!adminStore.providerStats.length" :description="t('admin.stats.noData')" />
        </n-card>
      </n-tab-pane>

      <!-- Model Stats -->
      <n-tab-pane :name="'models'" :tab="t('admin.stats.byModel')">
        <n-card>
          <n-data-table
            :columns="modelColumns"
            :data="adminStore.modelStats"
            :row-key="(row) => row.model"
          />
          <n-empty v-if="!adminStore.modelStats.length" :description="t('admin.stats.noData')" />
        </n-card>
      </n-tab-pane>

      <!-- User Stats -->
      <n-tab-pane :name="'users'" :tab="t('admin.stats.byUser')">
        <n-card>
          <n-data-table
            :columns="userColumns"
            :data="adminStore.userStatsData"
            :row-key="(row) => row.user_id"
          />
          <n-empty v-if="!adminStore.userStatsData.length" :description="t('admin.stats.noData')" />
        </n-card>
      </n-tab-pane>

      <!-- Hourly Stats -->
      <n-tab-pane :name="'hourly'" :tab="t('admin.stats.hourly')">
        <n-card>
          <n-data-table
            :columns="timeColumns"
            :data="adminStore.hourlyStats"
            :row-key="(row) => row.timestamp"
          />
          <n-empty v-if="!adminStore.hourlyStats.length" :description="t('admin.stats.noData')" />
        </n-card>
      </n-tab-pane>

      <!-- Daily Stats -->
      <n-tab-pane :name="'daily'" :tab="t('admin.stats.daily')">
        <n-card>
          <n-data-table
            :columns="timeColumns"
            :data="adminStore.dailyStats"
            :row-key="(row) => row.timestamp"
          />
          <n-empty v-if="!adminStore.dailyStats.length" :description="t('admin.stats.noData')" />
        </n-card>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NGrid, NGi, NStatistic, NButton, NIcon, NSpin,
  NTabs, NTabPane, NDataTable, NEmpty,
  type DataTableColumns
} from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { useAdminStore } from '../../../stores/admin'
import { initSyncClient } from '../../../services/sync'
import type { ProviderStats, ModelStats, UserStats, TimeWindowStats } from '../../../services/sync'

const { t } = useI18n()
const adminStore = useAdminStore()

const formatNumber = (value: number): string => {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return value.toLocaleString()
}

const formatDate = (dateStr: string): string => {
  try {
    return new Date(dateStr).toLocaleString()
  } catch {
    return dateStr
  }
}

const providerColumns = computed<DataTableColumns<ProviderStats>>(() => [
  { title: 'Provider', key: 'provider', width: 150 },
  { title: 'Requests', key: 'requests', width: 100 },
  { title: 'Tokens In', key: 'tokens_in', width: 120, render: (row) => formatNumber(row.tokens_in) },
  { title: 'Tokens Out', key: 'tokens_out', width: 120, render: (row) => formatNumber(row.tokens_out) },
  { title: 'Cost', key: 'cost', width: 100, render: (row) => `$${row.cost.toFixed(2)}` },
  { title: 'Success Rate', key: 'success_rate', width: 120, render: (row) => `${row.success_rate.toFixed(1)}%` },
  { title: 'Avg Latency', key: 'avg_latency_ms', width: 120, render: (row) => `${row.avg_latency_ms.toFixed(0)}ms` },
])

const modelColumns = computed<DataTableColumns<ModelStats>>(() => [
  { title: 'Model', key: 'model', width: 200, ellipsis: { tooltip: true } },
  { title: 'Provider', key: 'provider', width: 120 },
  { title: 'Requests', key: 'requests', width: 100 },
  { title: 'Tokens In', key: 'tokens_in', width: 120, render: (row) => formatNumber(row.tokens_in) },
  { title: 'Tokens Out', key: 'tokens_out', width: 120, render: (row) => formatNumber(row.tokens_out) },
  { title: 'Cost', key: 'cost', width: 100, render: (row) => `$${row.cost.toFixed(2)}` },
  { title: 'Avg Latency', key: 'avg_latency_ms', width: 120, render: (row) => `${row.avg_latency_ms.toFixed(0)}ms` },
])

const userColumns = computed<DataTableColumns<UserStats>>(() => [
  { title: 'User ID', key: 'user_id', width: 150, ellipsis: { tooltip: true } },
  { title: 'Requests', key: 'requests', width: 100 },
  { title: 'Tokens In', key: 'tokens_in', width: 120, render: (row) => formatNumber(row.tokens_in) },
  { title: 'Tokens Out', key: 'tokens_out', width: 120, render: (row) => formatNumber(row.tokens_out) },
  { title: 'Cost', key: 'cost', width: 100, render: (row) => `$${row.cost.toFixed(2)}` },
  { title: 'Sessions', key: 'sessions', width: 100 },
  { title: 'Messages', key: 'messages', width: 100 },
  { title: 'Last Active', key: 'last_active', width: 160, render: (row) => formatDate(row.last_active) },
])

const timeColumns = computed<DataTableColumns<TimeWindowStats>>(() => [
  { title: 'Time', key: 'timestamp', width: 180, render: (row) => formatDate(row.timestamp) },
  { title: 'Requests', key: 'requests', width: 100 },
  { title: 'Tokens In', key: 'tokens_in', width: 120, render: (row) => formatNumber(row.tokens_in) },
  { title: 'Tokens Out', key: 'tokens_out', width: 120, render: (row) => formatNumber(row.tokens_out) },
  { title: 'Cost', key: 'cost', width: 100, render: (row) => `$${row.cost.toFixed(2)}` },
  { title: 'Errors', key: 'errors', width: 80 },
  { title: 'Avg Latency', key: 'avg_latency_ms', width: 120, render: (row) => `${row.avg_latency_ms.toFixed(0)}ms` },
])

const refreshStats = async () => {
  await adminStore.loadAllStats()
  await adminStore.loadOnlineStatus()
}

onMounted(async () => {
  await initSyncClient()
  await refreshStats()
})
</script>

<style scoped>
.admin-stats {
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

.stats-section {
  margin-bottom: 24px;
}
</style>
