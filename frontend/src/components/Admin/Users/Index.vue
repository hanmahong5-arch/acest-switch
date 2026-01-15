<template>
  <div class="admin-users">
    <header class="page-header">
      <h1>{{ t('admin.users.title') }}</h1>
      <n-space>
        <n-input
          v-model:value="searchQuery"
          :placeholder="t('admin.users.searchPlaceholder')"
          clearable
          style="width: 240px"
          @update:value="debouncedSearch"
        >
          <template #prefix>
            <n-icon><SearchOutline /></n-icon>
          </template>
        </n-input>
        <n-checkbox v-model:checked="showDisabled">
          {{ t('admin.users.showDisabled') }}
        </n-checkbox>
      </n-space>
    </header>

    <n-card>
      <n-spin :show="adminStore.usersLoading">
        <n-data-table
          :columns="columns"
          :data="adminStore.users"
          :pagination="pagination"
          :row-key="(row) => row.user_id"
          @update:page="handlePageChange"
        />
      </n-spin>

      <div class="table-footer" v-if="adminStore.usersTotal > 0">
        {{ t('admin.users.totalUsers', { count: adminStore.usersTotal }) }}
      </div>
    </n-card>

    <!-- User Detail Modal -->
    <n-modal v-model:show="showDetailModal" preset="card" :title="t('admin.users.detail.title')" style="width: 600px">
      <div v-if="selectedUser">
        <n-descriptions :column="2" bordered>
          <n-descriptions-item :label="t('admin.users.username')">
            {{ selectedUser.username }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.users.email')">
            {{ selectedUser.email || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.users.role')">
            <n-tag :type="selectedUser.is_admin ? 'warning' : 'default'" size="small">
              {{ selectedUser.is_admin ? t('admin.users.admin') : t('admin.users.user') }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.users.status')">
            <n-tag :type="selectedUser.is_disabled ? 'error' : 'success'" size="small">
              {{ selectedUser.is_disabled ? t('admin.users.disabled') : t('admin.users.active') }}
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.users.createdAt')">
            {{ formatDate(selectedUser.created_at) }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.users.lastActive')">
            {{ formatDate(selectedUser.last_active_at) }}
          </n-descriptions-item>
        </n-descriptions>

        <n-divider>{{ t('admin.users.detail.statistics') }}</n-divider>

        <n-grid :cols="4" :x-gap="16" :y-gap="16">
          <n-gi>
            <n-statistic :label="t('admin.users.detail.requests')" :value="selectedUser.total_tokens || 0" />
          </n-gi>
          <n-gi>
            <n-statistic :label="t('admin.users.detail.sessions')" :value="selectedUser.session_count || 0" />
          </n-gi>
          <n-gi>
            <n-statistic :label="t('admin.users.detail.messages')" :value="selectedUser.message_count || 0" />
          </n-gi>
          <n-gi>
            <n-statistic :label="t('admin.users.detail.cost')">
              <template #prefix>$</template>
              {{ (selectedUser.total_cost || 0).toFixed(2) }}
            </n-statistic>
          </n-gi>
        </n-grid>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NButton, NIcon, NSpace, NInput, NCheckbox,
  NSpin, NTag, NModal, NDescriptions, NDescriptionsItem, NDivider,
  NGrid, NGi, NStatistic, NPopconfirm,
  type DataTableColumns
} from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import { useAdminStore } from '../../../stores/admin'
import { initSyncClient } from '../../../services/sync'
import type { AdminUser } from '../../../services/admin'

const { t } = useI18n()
const adminStore = useAdminStore()

const searchQuery = ref('')
const showDisabled = ref(false)
const showDetailModal = ref(false)
const selectedUser = ref<AdminUser | null>(null)

const pagination = computed(() => ({
  page: adminStore.usersPage,
  pageSize: adminStore.usersPageSize,
  pageCount: Math.ceil(adminStore.usersTotal / adminStore.usersPageSize),
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

const columns = computed<DataTableColumns<AdminUser>>(() => [
  { title: t('admin.users.username'), key: 'username', width: 150 },
  { title: t('admin.users.email'), key: 'email', width: 200, ellipsis: { tooltip: true } },
  {
    title: t('admin.users.role'),
    key: 'is_admin',
    width: 100,
    render: (row) => h(NTag, { type: row.is_admin ? 'warning' : 'default', size: 'small' }, () => row.is_admin ? t('admin.users.admin') : t('admin.users.user'))
  },
  {
    title: t('admin.users.status'),
    key: 'is_disabled',
    width: 100,
    render: (row) => h(NTag, { type: row.is_disabled ? 'error' : 'success', size: 'small' }, () => row.is_disabled ? t('admin.users.disabled') : t('admin.users.active'))
  },
  {
    title: t('admin.users.lastActive'),
    key: 'last_active_at',
    width: 160,
    render: (row) => formatDate(row.last_active_at)
  },
  {
    title: t('admin.users.actions'),
    key: 'actions',
    width: 200,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => viewDetail(row) }, () => t('admin.users.viewDetail')),
      h(NPopconfirm, {
        onPositiveClick: () => toggleStatus(row)
      }, {
        trigger: () => h(NButton, { size: 'small', type: row.is_disabled ? 'success' : 'error' }, () => row.is_disabled ? t('admin.users.enable') : t('admin.users.disable')),
        default: () => row.is_disabled ? t('admin.users.enable') + '?' : t('admin.users.disable') + '?'
      }),
    ])
  },
])

const formatDate = (dateStr?: string): string => {
  if (!dateStr) return '-'
  try {
    return new Date(dateStr).toLocaleString()
  } catch {
    return dateStr
  }
}

let searchTimeout: number | undefined
const debouncedSearch = () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    loadUsers()
  }, 300)
}

const loadUsers = () => {
  adminStore.loadUsers(1, adminStore.usersPageSize, searchQuery.value, showDisabled.value || undefined)
}

const handlePageChange = (page: number) => {
  adminStore.loadUsers(page, adminStore.usersPageSize, searchQuery.value, showDisabled.value || undefined)
}

const viewDetail = (user: AdminUser) => {
  selectedUser.value = user
  showDetailModal.value = true
}

const toggleStatus = async (user: AdminUser) => {
  try {
    await adminStore.toggleUserStatus(user.user_id, user.is_disabled)
  } catch (error) {
    console.error('Failed to toggle user status:', error)
  }
}

onMounted(async () => {
  await initSyncClient()
  await loadUsers()
})
</script>

<style scoped>
.admin-users {
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

.table-footer {
  margin-top: 16px;
  text-align: right;
  color: var(--n-text-color-3);
  font-size: 14px;
}
</style>
