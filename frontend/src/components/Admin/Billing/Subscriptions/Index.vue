<template>
  <div class="billing-subscriptions">
    <div class="page-header">
      <h2>{{ t('admin.billing.subscriptions.title') }}</h2>
      <n-space>
        <n-button @click="handleInitPlans" :loading="initingPlans">
          <template #icon>
            <n-icon><ReloadOutline /></n-icon>
          </template>
          {{ t('admin.billing.subscriptions.initPlans') }}
        </n-button>
        <n-button type="primary" @click="showCreateModal = true">
          <template #icon>
            <n-icon><AddOutline /></n-icon>
          </template>
          {{ t('admin.billing.subscriptions.create') }}
        </n-button>
      </n-space>
    </div>

    <!-- Stats Overview -->
    <n-grid :cols="4" :x-gap="16" class="stats-grid">
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.subscriptions.totalSubscriptions')">
            <template #prefix><n-icon><PeopleOutline /></n-icon></template>
            {{ stats.total_subscriptions }}
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.subscriptions.activeSubscriptions')">
            <template #prefix><n-icon color="#18a058"><CheckmarkCircleOutline /></n-icon></template>
            {{ stats.active_subscriptions }}
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.subscriptions.expiredSubscriptions')">
            <template #prefix><n-icon color="#d03050"><CloseCircleOutline /></n-icon></template>
            {{ stats.expired_subscriptions }}
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.subscriptions.byPlan')">
            <n-space size="small">
              <n-tag v-for="(count, plan) in stats.by_plan" :key="plan" size="small" type="info">
                {{ plan }}: {{ count }}
              </n-tag>
            </n-space>
          </n-statistic>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- Filters -->
    <n-card size="small" class="filter-card">
      <n-space>
        <n-input
          v-model:value="filters.user_id"
          :placeholder="t('admin.billing.subscriptions.filterUserId')"
          clearable
          style="width: 200px"
          @keyup.enter="handleSearch"
        />
        <n-select
          v-model:value="filters.status"
          :placeholder="t('admin.billing.subscriptions.filterStatus')"
          :options="statusOptions"
          clearable
          style="width: 150px"
        />
        <n-select
          v-model:value="filters.plan_code"
          :placeholder="t('admin.billing.subscriptions.filterPlan')"
          :options="planOptions"
          clearable
          style="width: 150px"
        />
        <n-button @click="handleSearch">
          <template #icon>
            <n-icon><SearchOutline /></n-icon>
          </template>
          {{ t('common.search') }}
        </n-button>
        <n-button @click="handleReset">
          {{ t('common.reset') }}
        </n-button>
      </n-space>
    </n-card>

    <!-- Table -->
    <n-data-table
      :columns="columns"
      :data="subscriptions"
      :loading="loading"
      :pagination="pagination"
      :row-key="(row: Subscription) => row.id"
      @update:page="handlePageChange"
    />

    <!-- Create Modal -->
    <n-modal v-model:show="showCreateModal" preset="card" :title="t('admin.billing.subscriptions.createTitle')" style="width: 500px">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="100">
        <n-form-item :label="t('admin.billing.subscriptions.userId')" path="user_id">
          <n-input-number v-model:value="createForm.user_id" :placeholder="t('admin.billing.subscriptions.userIdPlaceholder')" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('admin.billing.subscriptions.plan')" path="plan_code">
          <n-select
            v-model:value="createForm.plan_code"
            :options="planOptions"
            :placeholder="t('admin.billing.subscriptions.planPlaceholder')"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">{{ t('common.confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Detail Modal -->
    <n-modal v-model:show="showDetailModal" preset="card" :title="t('admin.billing.subscriptions.detailTitle')" style="width: 700px">
      <template v-if="selectedSubscription">
        <n-descriptions :column="2" label-placement="left" bordered>
          <n-descriptions-item :label="t('admin.billing.subscriptions.userId')">
            {{ selectedSubscription.user_id }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.plan')">
            {{ selectedSubscription.plan?.name || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.status')">
            <n-tag :type="getStatusType(selectedSubscription.status)" size="small">
              {{ selectedSubscription.status }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.currentGroup')">
            <n-tag :type="selectedSubscription.current_group === selectedSubscription.plan?.fallback_group ? 'warning' : 'success'" size="small">
              {{ selectedSubscription.current_group }}
              <template v-if="selectedSubscription.current_group === selectedSubscription.plan?.fallback_group">
                ({{ t('admin.billing.subscriptions.fallback') }})
              </template>
            </n-tag>
          </n-descriptions-item>
        </n-descriptions>

        <!-- Daily Quota Section -->
        <n-divider>{{ t('admin.billing.subscriptions.dailyQuota') }}</n-divider>
        <n-grid :cols="2" :x-gap="16">
          <n-gi>
            <n-card size="small" :title="t('admin.billing.subscriptions.dailyUsage')">
              <n-progress
                type="line"
                :percentage="getDailyUsagePercent(selectedSubscription)"
                :status="getDailyUsagePercent(selectedSubscription) >= 100 ? 'error' : 'success'"
                :indicator-placement="'inside'"
              />
              <div class="quota-text">
                {{ formatCents(selectedSubscription.today_used) }} / {{ formatCents(selectedSubscription.daily_quota) }}
                <span class="quota-remaining">
                  ({{ t('admin.billing.subscriptions.remaining') }}: {{ formatCents(selectedSubscription.daily_quota - selectedSubscription.today_used) }})
                </span>
              </div>
            </n-card>
          </n-gi>
          <n-gi>
            <n-card size="small" :title="t('admin.billing.subscriptions.totalQuota')">
              <n-progress
                type="line"
                :percentage="getTotalUsagePercent(selectedSubscription)"
                :status="getTotalUsagePercent(selectedSubscription) >= 100 ? 'error' : 'success'"
                :indicator-placement="'inside'"
              />
              <div class="quota-text">
                {{ formatCents(selectedSubscription.used_quota) }} / {{ formatCents(selectedSubscription.current_quota + selectedSubscription.used_quota) }}
              </div>
            </n-card>
          </n-gi>
        </n-grid>

        <n-descriptions :column="2" label-placement="left" bordered class="mt-4">
          <n-descriptions-item :label="t('admin.billing.subscriptions.lastDailyReset')">
            {{ formatDate(selectedSubscription.last_daily_reset_at) }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.expiresAt')">
            {{ formatDate(selectedSubscription.expires_at) }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.autoRenew')">
            <n-tag :type="selectedSubscription.auto_renew ? 'success' : 'default'" size="small">
              {{ selectedSubscription.auto_renew ? t('common.yes') : t('common.no') }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.billing.subscriptions.createdAt')">
            {{ formatDate(selectedSubscription.created_at) }}
          </n-descriptions-item>
        </n-descriptions>
      </template>
      <template #footer>
        <n-space justify="end">
          <n-button
            type="warning"
            :loading="resettingDaily"
            @click="handleResetDailyQuota"
            :disabled="!selectedSubscription"
          >
            {{ t('admin.billing.subscriptions.resetDaily') }}
          </n-button>
          <n-button @click="showDetailModal = false">{{ t('common.close') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, type VNode } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NIcon, NCard, NSpace, NInput, NInputNumber, NSelect, NDataTable, NModal, NForm, NFormItem,
  NTag, NStatistic, NGrid, NGi, NProgress, NDescriptions, NDescriptionsItem, NDivider,
  type DataTableColumns, type FormInst, type FormRules
} from 'naive-ui'
import { AddOutline, SearchOutline, ReloadOutline, PeopleOutline, CheckmarkCircleOutline, CloseCircleOutline, EyeOutline } from '@vicons/ionicons5'
import { showToast } from '../../../../utils/toast'
import {
  listPlans, adminListSubscriptions, createSubscription, cancelSubscription, resetDailyQuota, getStatsOverview, initDefaultPlans,
  type Subscription, type Plan, type StatsOverview
} from '../../../../services/subscription'

const { t } = useI18n()

// State
const loading = ref(false)
const creating = ref(false)
const initingPlans = ref(false)
const resettingDaily = ref(false)
const subscriptions = ref<Subscription[]>([])
const plans = ref<Plan[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const stats = ref<StatsOverview>({
  total_subscriptions: 0,
  active_subscriptions: 0,
  expired_subscriptions: 0,
  total_revenue: 0,
  by_plan: {},
  by_status: {}
})

// Modals
const showCreateModal = ref(false)
const showDetailModal = ref(false)
const selectedSubscription = ref<Subscription | null>(null)

// Filters
const filters = ref({
  user_id: '',
  status: null as string | null,
  plan_code: null as string | null,
})

const statusOptions = computed(() => [
  { label: t('admin.billing.subscriptions.statusActive'), value: 'active' },
  { label: t('admin.billing.subscriptions.statusPending'), value: 'pending' },
  { label: t('admin.billing.subscriptions.statusExpired'), value: 'expired' },
  { label: t('admin.billing.subscriptions.statusCancelled'), value: 'cancelled' },
])

const planOptions = computed(() =>
  plans.value.map(p => ({ label: `${p.name} (${formatCents(p.daily_quota)}/day)`, value: p.code }))
)

// Pagination
const pagination = computed(() => ({
  page: page.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

// Table columns
const columns = computed<DataTableColumns<Subscription>>(() => [
  { title: t('admin.billing.subscriptions.colUserId'), key: 'user_id', width: 100 },
  {
    title: t('admin.billing.subscriptions.colPlan'),
    key: 'plan_name',
    width: 140,
    render: (row) => row.plan?.name || '-'
  },
  {
    title: t('admin.billing.subscriptions.colStatus'),
    key: 'status',
    width: 100,
    render: (row) => h(NTag, { type: getStatusType(row.status), size: 'small' }, { default: () => row.status })
  },
  {
    title: t('admin.billing.subscriptions.colCurrentGroup'),
    key: 'current_group',
    width: 120,
    render: (row) => {
      const isFallback = row.current_group === row.plan?.fallback_group
      return h(NTag, {
        type: isFallback ? 'warning' : 'success',
        size: 'small'
      }, {
        default: () => isFallback ? `${row.current_group} (fallback)` : row.current_group
      })
    }
  },
  {
    title: t('admin.billing.subscriptions.colDailyQuota'),
    key: 'daily_quota',
    width: 180,
    render: (row) => {
      const percent = getDailyUsagePercent(row)
      return h('div', { class: 'quota-cell' }, [
        h(NProgress, {
          type: 'line',
          percentage: percent,
          status: percent >= 100 ? 'error' : percent >= 80 ? 'warning' : 'success',
          showIndicator: false,
          style: 'width: 80px; display: inline-block; margin-right: 8px;'
        }),
        h('span', { class: 'quota-text-small' }, `${formatCents(row.today_used)}/${formatCents(row.daily_quota)}`)
      ])
    }
  },
  {
    title: t('admin.billing.subscriptions.colExpiresAt'),
    key: 'expires_at',
    width: 160,
    render: (row) => formatDate(row.expires_at)
  },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) => {
      const buttons: VNode[] = []

      // View details button
      buttons.push(
        h(NButton, {
          size: 'small',
          quaternary: true,
          onClick: () => handleViewDetail(row)
        }, {
          default: () => h(NIcon, null, { default: () => h(EyeOutline) })
        })
      )

      if (row.status === 'active') {
        buttons.push(
          h(NButton, { size: 'small', type: 'warning', onClick: () => handleCancel(row) }, { default: () => t('admin.billing.subscriptions.cancel') })
        )
      }

      return h(NSpace, { size: 'small' }, { default: () => buttons })
    }
  }
])

// Create form
const createFormRef = ref<FormInst | null>(null)
const createForm = ref({
  user_id: null as number | null,
  plan_code: null as string | null,
})
const createRules: FormRules = {
  user_id: [{ required: true, type: 'number', message: t('admin.billing.subscriptions.userIdRequired'), trigger: 'blur' }],
  plan_code: [{ required: true, message: t('admin.billing.subscriptions.planRequired'), trigger: 'change' }],
}

// Helpers
function getStatusType(status: string): 'success' | 'warning' | 'error' | 'info' | 'default' {
  const map: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
    active: 'success',
    pending: 'warning',
    cancelled: 'error',
    expired: 'error',
  }
  return map[status] || 'default'
}

function getDailyUsagePercent(sub: Subscription): number {
  if (!sub.daily_quota || sub.daily_quota <= 0) return 0
  return Math.min(100, Math.round((sub.today_used / sub.daily_quota) * 100))
}

function getTotalUsagePercent(sub: Subscription): number {
  const total = sub.current_quota + sub.used_quota
  if (total <= 0) return 0
  return Math.min(100, Math.round((sub.used_quota / total) * 100))
}

function formatCents(cents: number): string {
  if (cents === undefined || cents === null) return '0'
  return `¥${(cents / 100).toFixed(2)}`
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

// Methods
async function loadData() {
  loading.value = true
  try {
    const [subsData, statsData] = await Promise.all([
      adminListSubscriptions({
        page: page.value,
        page_size: pageSize.value,
        user_id: filters.value.user_id || undefined,
        status: filters.value.status || undefined,
        plan_code: filters.value.plan_code || undefined,
      }),
      getStatsOverview()
    ])
    subscriptions.value = subsData.subscriptions || []
    total.value = subsData.total || 0
    stats.value = statsData
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    subscriptions.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function loadPlans() {
  try {
    plans.value = await listPlans()
  } catch (error) {
    console.error('Failed to load plans:', error)
    plans.value = []
  }
}

const handleSearch = () => {
  page.value = 1
  loadData()
}

const handleReset = () => {
  filters.value = { user_id: '', status: null, plan_code: null }
  page.value = 1
  loadData()
}

const handlePageChange = (newPage: number) => {
  page.value = newPage
  loadData()
}

const handleCreate = async () => {
  try {
    await createFormRef.value?.validate()
    creating.value = true
    await createSubscription({
      user_id: createForm.value.user_id!,
      plan_code: createForm.value.plan_code!
    })
    showToast(t('admin.billing.subscriptions.createSuccess'), 'success')
    showCreateModal.value = false
    createForm.value = { user_id: null, plan_code: null }
    loadData()
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    creating.value = false
  }
}

const handleCancel = async (row: Subscription) => {
  try {
    await cancelSubscription(row.id)
    showToast(t('admin.billing.subscriptions.cancelSuccess'), 'success')
    loadData()
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  }
}

const handleViewDetail = (row: Subscription) => {
  selectedSubscription.value = row
  showDetailModal.value = true
}

const handleResetDailyQuota = async () => {
  if (!selectedSubscription.value) return
  try {
    resettingDaily.value = true
    await resetDailyQuota(selectedSubscription.value.id)
    showToast(t('admin.billing.subscriptions.resetDailySuccess'), 'success')
    loadData()
    showDetailModal.value = false
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    resettingDaily.value = false
  }
}

const handleInitPlans = async () => {
  try {
    initingPlans.value = true
    await initDefaultPlans()
    showToast(t('admin.billing.subscriptions.initPlansSuccess'), 'success')
    loadPlans()
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    initingPlans.value = false
  }
}

onMounted(() => {
  loadData()
  loadPlans()
})
</script>

<style scoped>
.billing-subscriptions {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.stats-grid {
  margin-bottom: 16px;
}

.filter-card {
  margin-bottom: 16px;
}

.quota-cell {
  display: flex;
  align-items: center;
}

.quota-text {
  margin-top: 8px;
  font-size: 13px;
  color: var(--n-text-color-2);
}

.quota-text-small {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.quota-remaining {
  color: var(--n-text-color-3);
  font-size: 12px;
}

.mt-4 {
  margin-top: 16px;
}
</style>
