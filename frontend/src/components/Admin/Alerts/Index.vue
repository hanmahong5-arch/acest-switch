<template>
  <div class="admin-alerts">
    <header class="page-header">
      <h1>{{ t('admin.alerts.title') }}</h1>
      <n-button type="primary" @click="showCreateModal = true">
        {{ t('admin.alerts.createRule') }}
      </n-button>
    </header>

    <n-tabs type="line" animated>
      <!-- Alert Rules Tab -->
      <n-tab-pane :name="'rules'" :tab="t('admin.alerts.rules')">
        <n-card>
          <n-spin :show="adminStore.alertsLoading">
            <n-data-table
              :columns="ruleColumns"
              :data="adminStore.alertRules"
              :row-key="(row) => row.id"
            />
          </n-spin>
          <n-empty v-if="!adminStore.alertsLoading && !adminStore.alertRules.length" :description="t('admin.alerts.noRules')" />
        </n-card>
      </n-tab-pane>

      <!-- Alert History Tab -->
      <n-tab-pane :name="'history'" :tab="t('admin.alerts.history')">
        <n-card>
          <n-space class="filters-row">
            <n-select
              v-model:value="historyFilters.severity"
              :placeholder="t('admin.alerts.filterBySeverity')"
              :options="severityOptions"
              clearable
              style="width: 160px"
              @update:value="loadHistory"
            />
            <n-select
              v-model:value="historyFilters.status"
              :placeholder="t('admin.alerts.filterByStatus')"
              :options="statusOptions"
              clearable
              style="width: 140px"
              @update:value="loadHistory"
            />
          </n-space>

          <n-data-table
            :columns="historyColumns"
            :data="adminStore.alertHistory"
            :row-key="(row) => row.id"
          />
          <n-empty v-if="!adminStore.alertHistory.length" :description="t('admin.alerts.noHistory')" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <!-- Create/Edit Rule Modal -->
    <n-modal v-model:show="showCreateModal" preset="card" :title="editingRule ? t('admin.alerts.editRule') : t('admin.alerts.createRule')" style="width: 600px">
      <n-form ref="formRef" :model="ruleForm" :rules="formRules" label-placement="left" label-width="120">
        <n-form-item :label="t('admin.alerts.ruleName')" path="name">
          <n-input v-model:value="ruleForm.name" :placeholder="t('admin.alerts.ruleNamePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.description')" path="description">
          <n-input v-model:value="ruleForm.description" type="textarea" :placeholder="t('admin.alerts.descriptionPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.metric')" path="metric">
          <n-select v-model:value="ruleForm.metric" :options="metricOptions" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.condition')" path="condition">
          <n-select v-model:value="ruleForm.condition" :options="conditionOptions" style="width: 160px" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.threshold')" path="threshold">
          <n-input-number v-model:value="ruleForm.threshold" :min="0" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.windowSeconds')" path="window_seconds">
          <n-input-number v-model:value="ruleForm.window_seconds" :min="60" :step="60" style="width: 100%" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.severity')" path="severity">
          <n-select v-model:value="ruleForm.severity" :options="severityOptions" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.enabled')" path="enabled">
          <n-switch v-model:value="ruleForm.enabled" />
        </n-form-item>
        <n-form-item :label="t('admin.alerts.webhookUrl')" path="webhook_url">
          <n-input v-model:value="ruleForm.webhook_url" :placeholder="t('admin.alerts.webhookUrlPlaceholder')" />
        </n-form-item>
      </n-form>

      <template #footer>
        <n-space justify="end">
          <n-button @click="closeModal">{{ t('components.main.form.actions.cancel') }}</n-button>
          <n-button type="primary" @click="saveRule" :loading="saving">{{ t('components.main.form.actions.save') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NCard, NDataTable, NButton, NSpace, NSelect, NTabs, NTabPane,
  NSpin, NTag, NModal, NForm, NFormItem, NInput, NInputNumber, NSwitch, NEmpty, NPopconfirm,
  type DataTableColumns, type FormInst, type FormRules
} from 'naive-ui'
import { useAdminStore } from '../../../stores/admin'
import { initSyncClient } from '../../../services/sync'
import { showToast } from '../../../utils/toast'
import type { AlertRule, AlertHistory } from '../../../services/admin'

const { t } = useI18n()
const adminStore = useAdminStore()

const showCreateModal = ref(false)
const editingRule = ref<AlertRule | null>(null)
const saving = ref(false)
const formRef = ref<FormInst | null>(null)

const historyFilters = reactive({
  severity: null as string | null,
  status: null as string | null,
})

const ruleForm = reactive({
  name: '',
  description: '',
  metric: 'error_rate',
  condition: 'gt' as 'gt' | 'lt' | 'eq' | 'gte' | 'lte',
  threshold: 0,
  window_seconds: 300,
  severity: 'warning' as 'info' | 'warning' | 'critical',
  enabled: true,
  notify_channels: ['webhook'] as string[],
  webhook_url: '',
})

const formRules: FormRules = {
  name: [{ required: true, message: t('admin.alerts.ruleNamePlaceholder'), trigger: 'blur' }],
  metric: [{ required: true, message: 'Metric is required', trigger: 'change' }],
  threshold: [{ required: true, type: 'number', message: 'Threshold is required', trigger: 'blur' }],
}

const metricOptions = computed(() => [
  { label: t('admin.alerts.metrics.error_rate'), value: 'error_rate' },
  { label: t('admin.alerts.metrics.latency_p99'), value: 'latency_p99' },
  { label: t('admin.alerts.metrics.latency_avg'), value: 'latency_avg' },
  { label: t('admin.alerts.metrics.cost_daily'), value: 'cost_daily' },
  { label: t('admin.alerts.metrics.requests_per_minute'), value: 'requests_per_minute' },
])

const conditionOptions = computed(() => [
  { label: t('admin.alerts.conditions.gt'), value: 'gt' },
  { label: t('admin.alerts.conditions.lt'), value: 'lt' },
  { label: t('admin.alerts.conditions.gte'), value: 'gte' },
  { label: t('admin.alerts.conditions.lte'), value: 'lte' },
  { label: t('admin.alerts.conditions.eq'), value: 'eq' },
])

const severityOptions = computed(() => [
  { label: t('admin.alerts.severities.info'), value: 'info' },
  { label: t('admin.alerts.severities.warning'), value: 'warning' },
  { label: t('admin.alerts.severities.critical'), value: 'critical' },
])

const statusOptions = computed(() => [
  { label: t('admin.alerts.firing'), value: 'firing' },
  { label: t('admin.alerts.resolved'), value: 'resolved' },
])

const ruleColumns = computed<DataTableColumns<AlertRule>>(() => [
  { title: t('admin.alerts.ruleName'), key: 'name', width: 180 },
  {
    title: t('admin.alerts.metric'),
    key: 'metric',
    width: 140,
    render: (row) => t(`admin.alerts.metrics.${row.metric}`) || row.metric
  },
  {
    title: t('admin.alerts.condition'),
    key: 'condition',
    width: 120,
    render: (row) => `${t(`admin.alerts.conditions.${row.condition}`)} ${row.threshold}`
  },
  {
    title: t('admin.alerts.severity'),
    key: 'severity',
    width: 100,
    render: (row) => h(NTag, { type: getSeverityType(row.severity), size: 'small' }, () => t(`admin.alerts.severities.${row.severity}`))
  },
  {
    title: t('admin.alerts.enabled'),
    key: 'enabled',
    width: 80,
    render: (row) => h(NTag, { type: row.enabled ? 'success' : 'default', size: 'small' }, () => row.enabled ? 'ON' : 'OFF')
  },
  {
    title: t('admin.alerts.actions'),
    key: 'actions',
    width: 160,
    render: (row) => h(NSpace, { size: 'small' }, () => [
      h(NButton, { size: 'small', onClick: () => editRule(row) }, () => t('admin.alerts.edit')),
      h(NPopconfirm, {
        onPositiveClick: () => deleteRule(row)
      }, {
        trigger: () => h(NButton, { size: 'small', type: 'error' }, () => t('admin.alerts.delete')),
        default: () => t('admin.alerts.confirmDelete')
      }),
    ])
  },
])

const historyColumns = computed<DataTableColumns<AlertHistory>>(() => [
  {
    title: t('admin.alerts.triggeredAt'),
    key: 'triggered_at',
    width: 160,
    render: (row) => formatDate(row.triggered_at)
  },
  { title: t('admin.alerts.ruleName'), key: 'rule_name', width: 180 },
  { title: t('admin.alerts.metricValue'), key: 'metric_value', width: 120 },
  { title: t('admin.alerts.threshold'), key: 'threshold', width: 100 },
  {
    title: t('admin.alerts.severity'),
    key: 'severity',
    width: 100,
    render: (row) => h(NTag, { type: getSeverityType(row.severity), size: 'small' }, () => t(`admin.alerts.severities.${row.severity}`))
  },
  {
    title: t('admin.alerts.status'),
    key: 'status',
    width: 100,
    render: (row) => h(NTag, { type: row.status === 'firing' ? 'error' : 'success', size: 'small' }, () => t(`admin.alerts.${row.status}`))
  },
  {
    title: t('admin.alerts.resolvedAt'),
    key: 'resolved_at',
    width: 160,
    render: (row) => row.resolved_at ? formatDate(row.resolved_at) : '-'
  },
])

const formatDate = (dateStr: string): string => {
  try {
    return new Date(dateStr).toLocaleString()
  } catch {
    return dateStr
  }
}

const getSeverityType = (severity: string): 'info' | 'warning' | 'error' => {
  switch (severity) {
    case 'info': return 'info'
    case 'warning': return 'warning'
    case 'critical': return 'error'
    default: return 'warning'
  }
}

const editRule = (rule: AlertRule) => {
  editingRule.value = rule
  Object.assign(ruleForm, {
    name: rule.name,
    description: rule.description || '',
    metric: rule.metric,
    condition: rule.condition,
    threshold: rule.threshold,
    window_seconds: rule.window_seconds,
    severity: rule.severity,
    enabled: rule.enabled,
    notify_channels: rule.notify_channels,
    webhook_url: rule.webhook_url || '',
  })
  showCreateModal.value = true
}

const closeModal = () => {
  showCreateModal.value = false
  editingRule.value = null
  Object.assign(ruleForm, {
    name: '',
    description: '',
    metric: 'error_rate',
    condition: 'gt',
    threshold: 0,
    window_seconds: 300,
    severity: 'warning',
    enabled: true,
    notify_channels: ['webhook'],
    webhook_url: '',
  })
}

const saveRule = async () => {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    if (editingRule.value) {
      await adminStore.editAlertRule(editingRule.value.id, ruleForm)
    } else {
      await adminStore.addAlertRule(ruleForm)
    }
    showToast(t('admin.alerts.saveSuccess'), 'success')
    closeModal()
  } catch (error) {
    showToast(t('admin.alerts.saveFailed'), 'error')
  } finally {
    saving.value = false
  }
}

const deleteRule = async (rule: AlertRule) => {
  try {
    await adminStore.removeAlertRule(rule.id)
    showToast(t('admin.alerts.deleteSuccess'), 'success')
  } catch (error) {
    showToast(t('admin.alerts.deleteFailed'), 'error')
  }
}

const loadHistory = () => {
  adminStore.loadAlertHistory({
    severity: historyFilters.severity || undefined,
    status: historyFilters.status || undefined,
  })
}

onMounted(async () => {
  await initSyncClient()
  await adminStore.loadAlertRules()
  await adminStore.loadAlertHistory()
})
</script>

<style scoped>
.admin-alerts {
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

.filters-row {
  margin-bottom: 16px;
}
</style>
