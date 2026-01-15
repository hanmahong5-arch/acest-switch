<template>
  <div class="admin-sessions">
    <header class="page-header">
      <h1>{{ t('admin.sessions.title') }}</h1>
      <n-space>
        <n-input
          v-model:value="filterUserId"
          :placeholder="t('admin.sessions.filterByUser')"
          clearable
          style="width: 200px"
          @update:value="debouncedSearch"
        />
      </n-space>
    </header>

    <n-card>
      <n-spin :show="adminStore.sessionsLoading">
        <n-data-table
          :columns="columns"
          :data="adminStore.sessions"
          :pagination="pagination"
          :row-key="(row) => row.id"
          @update:page="handlePageChange"
        />
      </n-spin>

      <div class="table-footer" v-if="adminStore.sessionsTotal > 0">
        {{ t('admin.sessions.totalSessions', { count: adminStore.sessionsTotal }) }}
      </div>
    </n-card>

    <!-- Session Detail Modal -->
    <n-modal v-model:show="showDetailModal" preset="card" :title="t('admin.sessions.detail.title')" style="width: 800px; max-height: 80vh">
      <div v-if="adminStore.currentSession">
        <n-descriptions :column="2" bordered>
          <n-descriptions-item :label="t('admin.sessions.sessionId')">
            {{ adminStore.currentSession.id }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.sessions.userId')">
            {{ adminStore.currentSession.user_id }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.sessions.sessionTitle')">
            {{ adminStore.currentSession.title || '-' }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.sessions.messageCount')">
            {{ adminStore.currentSession.message_count }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.sessions.tokenCount')">
            {{ adminStore.currentSession.token_count.toLocaleString() }}
          </n-descriptions-item>
          <n-descriptions-item :label="t('admin.sessions.cost')">
            ${{ adminStore.currentSession.cost.toFixed(4) }}
          </n-descriptions-item>
        </n-descriptions>

        <n-divider>{{ t('admin.sessions.detail.messages') }}</n-divider>

        <div class="messages-container">
          <div v-for="msg in adminStore.currentMessages" :key="msg.id" :class="['message-item', msg.role]">
            <div class="message-header">
              <n-tag :type="msg.role === 'user' ? 'info' : 'success'" size="small">
                {{ msg.role }}
              </n-tag>
              <span class="message-time">{{ formatDate(msg.created_at) }}</span>
              <span v-if="msg.model" class="message-model">{{ msg.model }}</span>
            </div>
            <div class="message-content">
              {{ truncateContent(msg.content) }}
            </div>
            <div class="message-meta" v-if="msg.tokens_input || msg.tokens_output">
              <span>{{ t('admin.sessions.detail.tokens') }}: {{ msg.tokens_input }} / {{ msg.tokens_output }}</span>
            </div>
          </div>
          <n-empty v-if="adminStore.currentMessages.length === 0" :description="t('admin.sessions.noSessions')" />
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NButton, NIcon, NSpace, NInput,
  NSpin, NTag, NModal, NDescriptions, NDescriptionsItem, NDivider,
  NEmpty, NPopconfirm,
  type DataTableColumns
} from 'naive-ui'
import { useAdminStore } from '../../../stores/admin'
import { initSyncClient } from '../../../services/sync'
import { showToast } from '../../../utils/toast'
import type { AdminSession } from '../../../services/admin'

const { t } = useI18n()
const adminStore = useAdminStore()

const filterUserId = ref('')
const showDetailModal = ref(false)

const pagination = computed(() => ({
  page: adminStore.sessionsPage,
  pageSize: adminStore.sessionsPageSize,
  pageCount: Math.ceil(adminStore.sessionsTotal / adminStore.sessionsPageSize),
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

const columns = computed<DataTableColumns<AdminSession>>(() => [
  {
    title: t('admin.sessions.sessionId'),
    key: 'id',
    width: 120,
    ellipsis: { tooltip: true },
    render: (row) => row.id.substring(0, 8) + '...'
  },
  { title: t('admin.sessions.userId'), key: 'user_id', width: 120, ellipsis: { tooltip: true } },
  { title: t('admin.sessions.sessionTitle'), key: 'title', width: 200, ellipsis: { tooltip: true } },
  { title: t('admin.sessions.messageCount'), key: 'message_count', width: 100 },
  {
    title: t('admin.sessions.tokenCount'),
    key: 'token_count',
    width: 120,
    render: (row) => row.token_count.toLocaleString()
  },
  {
    title: t('admin.sessions.cost'),
    key: 'cost',
    width: 100,
    render: (row) => `$${row.cost.toFixed(4)}`
  },
  {
    title: t('admin.sessions.createdAt'),
    key: 'created_at',
    width: 160,
    render: (row) => formatDate(row.created_at)
  },
  {
    title: t('admin.sessions.actions'),
    key: 'actions',
    width: 180,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => viewDetail(row) }, () => t('admin.sessions.viewDetail')),
      h(NPopconfirm, {
        onPositiveClick: () => deleteSession(row)
      }, {
        trigger: () => h(NButton, { size: 'small', type: 'error' }, () => t('admin.sessions.delete')),
        default: () => t('admin.sessions.confirmDelete')
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

const truncateContent = (content: string, maxLen = 200): string => {
  if (content.length <= maxLen) return content
  return content.substring(0, maxLen) + '...'
}

let searchTimeout: number | undefined
const debouncedSearch = () => {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    loadSessions()
  }, 300)
}

const loadSessions = () => {
  adminStore.loadSessions(1, adminStore.sessionsPageSize, filterUserId.value || undefined)
}

const handlePageChange = (page: number) => {
  adminStore.loadSessions(page, adminStore.sessionsPageSize, filterUserId.value || undefined)
}

const viewDetail = async (session: AdminSession) => {
  try {
    await adminStore.loadSessionDetail(session.id)
    showDetailModal.value = true
  } catch (error) {
    console.error('Failed to load session detail:', error)
  }
}

const deleteSession = async (session: AdminSession) => {
  try {
    await adminStore.removeSession(session.id)
    showToast(t('admin.sessions.deleteSuccess'), 'success')
  } catch (error) {
    showToast(t('admin.sessions.deleteFailed'), 'error')
  }
}

onMounted(async () => {
  await initSyncClient()
  await loadSessions()
})
</script>

<style scoped>
.admin-sessions {
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

.messages-container {
  max-height: 400px;
  overflow-y: auto;
}

.message-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 8px;
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
}

.message-item.user {
  background: var(--n-color-target);
}

.message-item.assistant {
  background: var(--n-color);
}

.message-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.message-time {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.message-model {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.message-content {
  font-size: 14px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.message-meta {
  margin-top: 8px;
  font-size: 12px;
  color: var(--n-text-color-3);
}
</style>
