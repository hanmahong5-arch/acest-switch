<template>
  <div class="admin-audit">
    <header class="page-header">
      <h1>{{ t('admin.audit.title') }}</h1>
    </header>

    <!-- Filters -->
    <n-card class="filters-card">
      <n-space>
        <n-input
          v-model:value="filters.user_id"
          :placeholder="t('admin.audit.filterByUser')"
          clearable
          style="width: 160px"
        />
        <n-select
          v-model:value="filters.action"
          :placeholder="t('admin.audit.filterByAction')"
          :options="actionOptions"
          clearable
          style="width: 160px"
        />
        <n-select
          v-model:value="filters.result"
          :placeholder="t('admin.audit.filterByResult')"
          :options="resultOptions"
          clearable
          style="width: 140px"
        />
        <n-date-picker
          v-model:value="dateRange"
          type="daterange"
          clearable
          style="width: 280px"
        />
        <n-button type="primary" @click="applyFilters">
          {{ t('common.query') || 'Query' }}
        </n-button>
      </n-space>
    </n-card>

    <!-- Audit Logs Table -->
    <n-card>
      <n-spin :show="adminStore.auditLoading">
        <n-data-table
          :columns="columns"
          :data="adminStore.auditLogs"
          :pagination="pagination"
          :row-key="(row) => row.id"
          @update:page="handlePageChange"
        />
      </n-spin>

      <div class="table-footer" v-if="adminStore.auditTotal > 0">
        {{ t('admin.audit.totalLogs', { count: adminStore.auditTotal }) }}
      </div>

      <n-empty v-if="!adminStore.auditLoading && !adminStore.auditLogs.length" :description="t('admin.audit.noLogs')" />
    </n-card>

    <!-- Log Detail Modal -->
    <n-modal v-model:show="showDetailModal" preset="card" :title="t('admin.audit.details')" style="width: 600px">
      <div v-if="selectedLog">
        <n-descriptions :column="1" bordered>
          <n-descriptions-item :label="t('admin.audit.logId')">
            {{ selectedLog.id }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.userId')">
            {{ selectedLog.user_id || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.username')">
            {{ selectedLog.username || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.action')">
            {{ selectedLog.action }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.resourceType')">
            {{ selectedLog.resource_type }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.resourceId')">
            {{ selectedLog.resource_id || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.result')">
            <n-tag :type="getResultType(selectedLog.result)" size="small">
              {{ t(`admin.audit.${selectedLog.result}`) }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.ipAddress')">
            {{ selectedLog.ip_address || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.audit.createdAt')">
            {{ formatDate(selectedLog.created_at) }}
          </n-descriptions-item>
        </n-descriptions>

        <n-divider v-if="selectedLog.details && Object.keys(selectedLog.details).length">
          {{ t('admin.audit.details') }}
        </n-divider>
        <n-code v-if="selectedLog.details && Object.keys(selectedLog.details).length" :code="JSON.stringify(selectedLog.details, null, 2)" language="json" />
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NButton, NSpace, NInput, NSelect, NDatePicker,
  NSpin, NTag, NModal, NDescriptions, NDescriptionsItem, NDivider, NCode, NEmpty,
  type DataTableColumns
} from 'naive-ui'
import { useAdminStore } from '../../../stores/admin'
import { initSyncClient } from '../../../services/sync'
import type { AuditLog } from '../../../services/admin'

const { t } = useI18n()
const adminStore = useAdminStore()

const filters = reactive({
  user_id: '',
  action: null as string | null,
  result: null as string | null,
})
const dateRange = ref<[number, number] | null>(null)
const showDetailModal = ref(false)
const selectedLog = ref<AuditLog | null>(null)

const actionOptions = [
  { label: 'user.login', value: 'user.login' },
  { label: 'user.logout', value: 'user.logout' },
  { label: 'session.create', value: 'session.create' },
  { label: 'session.delete', value: 'session.delete' },
  { label: 'user.disable', value: 'user.disable' },
  { label: 'user.enable', value: 'user.enable' },
  { label: 'admin.set', value: 'admin.set' },
]

const resultOptions = computed(() => [
  { label: t('admin.audit.success'), value: 'success' },
  { label: t('admin.audit.failure'), value: 'failure' },
  { label: t('admin.audit.blocked'), value: 'blocked' },
])

const pagination = computed(() => ({
  page: adminStore.auditPage,
  pageSize: adminStore.auditPageSize,
  pageCount: Math.ceil(adminStore.auditTotal / adminStore.auditPageSize),
  showSizePicker: true,
  pageSizes: [20, 50, 100],
}))

const columns = computed<DataTableColumns<AuditLog>>(() => [
  {
    title: t('admin.audit.createdAt'),
    key: 'created_at',
    width: 160,
    render: (row) => formatDate(row.created_at)
  },
  { title: t('admin.audit.username'), key: 'username', width: 120, ellipsis: { tooltip: true } },
  { title: t('admin.audit.action'), key: 'action', width: 140 },
  { title: t('admin.audit.resourceType'), key: 'resource_type', width: 120 },
  {
    title: t('admin.audit.result'),
    key: 'result',
    width: 100,
    render: (row) => h(NTag, { type: getResultType(row.result), size: 'small' }, () => t(`admin.audit.${row.result}`))
  },
  { title: t('admin.audit.ipAddress'), key: 'ip_address', width: 140 },
  {
    title: t('admin.audit.actions'),
    key: 'actions',
    width: 100,
    render: (row) => h(NButton, { size: 'small', onClick: () => viewDetail(row) }, () => t('admin.audit.details'))
  },
])

const formatDate = (dateStr: string): string => {
  try {
    return new Date(dateStr).toLocaleString()
  } catch {
    return dateStr
  }
}

const getResultType = (result: string): 'success' | 'error' | 'warning' => {
  switch (result) {
    case 'success': return 'success'
    case 'failure': return 'error'
    case 'blocked': return 'warning'
    default: return 'warning'
  }
}

const applyFilters = () => {
  const params: Record<string, string | undefined> = {
    user_id: filters.user_id || undefined,
    action: filters.action || undefined,
    result: filters.result || undefined,
  }
  if (dateRange.value) {
    params.start_time = new Date(dateRange.value[0]).toISOString()
    params.end_time = new Date(dateRange.value[1]).toISOString()
  }
  adminStore.loadAuditLogs({ ...params, page: 1, page_size: adminStore.auditPageSize })
}

const handlePageChange = (page: number) => {
  const params: Record<string, string | undefined> = {
    user_id: filters.user_id || undefined,
    action: filters.action || undefined,
    result: filters.result || undefined,
  }
  if (dateRange.value) {
    params.start_time = new Date(dateRange.value[0]).toISOString()
    params.end_time = new Date(dateRange.value[1]).toISOString()
  }
  adminStore.loadAuditLogs({ ...params, page, page_size: adminStore.auditPageSize })
}

const viewDetail = (log: AuditLog) => {
  selectedLog.value = log
  showDetailModal.value = true
}

onMounted(async () => {
  await initSyncClient()
  await adminStore.loadAuditLogs({ page: 1, page_size: adminStore.auditPageSize })
})
</script>

<style scoped>
.admin-audit {
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

.filters-card {
  margin-bottom: 16px;
}

.table-footer {
  margin-top: 16px;
  text-align: right;
  color: var(--n-text-color-3);
  font-size: 14px;
}
</style>
