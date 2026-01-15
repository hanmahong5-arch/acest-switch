<template>
  <div class="billing-payments">
    <div class="page-header">
      <h2>{{ t('admin.billing.payments.title') }}</h2>
      <n-button type="primary" @click="showCreateModal = true">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        {{ t('admin.billing.payments.create') }}
      </n-button>
    </div>

    <!-- Filters -->
    <n-card size="small" class="filter-card">
      <n-space>
        <n-input
          v-model:value="filters.user_id"
          :placeholder="t('admin.billing.payments.filterUserId')"
          clearable
          style="width: 180px"
          @keyup.enter="handleSearch"
        />
        <n-select
          v-model:value="filters.status"
          :placeholder="t('admin.billing.payments.filterStatus')"
          :options="statusOptions"
          clearable
          style="width: 120px"
        />
        <n-select
          v-model:value="filters.method"
          :placeholder="t('admin.billing.payments.filterMethod')"
          :options="methodOptions"
          clearable
          style="width: 120px"
        />
        <n-date-picker
          v-model:value="dateRange"
          type="daterange"
          clearable
          :start-placeholder="t('admin.billing.payments.startTime')"
          :end-placeholder="t('admin.billing.payments.endTime')"
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

    <!-- Stats Cards -->
    <n-grid :cols="4" :x-gap="16" :y-gap="16" class="stats-grid">
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.payments.totalRevenue')" :value="totalRevenue">
            <template #prefix>¥</template>
          </n-statistic>
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.payments.pendingCount')" :value="pendingCount" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.payments.paidCount')" :value="paidCount" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card size="small">
          <n-statistic :label="t('admin.billing.payments.refundedCount')" :value="refundedCount" />
        </n-card>
      </n-gi>
    </n-grid>

    <!-- Table -->
    <n-data-table
      :columns="columns"
      :data="billingStore.payments"
      :loading="billingStore.paymentsLoading"
      :pagination="pagination"
      :row-key="(row: Payment) => row.id"
      @update:page="handlePageChange"
    />

    <!-- Create Payment Modal -->
    <n-modal v-model:show="showCreateModal" preset="card" :title="t('admin.billing.payments.createTitle')" style="width: 500px">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="100">
        <n-form-item :label="t('admin.billing.payments.userId')" path="user_id">
          <n-input v-model:value="createForm.user_id" :placeholder="t('admin.billing.payments.userIdPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('admin.billing.payments.amount')" path="amount">
          <n-input-number
            v-model:value="createForm.amount"
            :placeholder="t('admin.billing.payments.amountPlaceholder')"
            :min="0.01"
            :precision="2"
            style="width: 100%"
          >
            <template #prefix>¥</template>
          </n-input-number>
        </n-form-item>
        <n-form-item :label="t('admin.billing.payments.method')" path="method">
          <n-select
            v-model:value="createForm.method"
            :options="methodOptions"
            :placeholder="t('admin.billing.payments.methodPlaceholder')"
          />
        </n-form-item>
        <n-form-item :label="t('admin.billing.payments.description')" path="description">
          <n-input v-model:value="createForm.description" type="textarea" :placeholder="t('admin.billing.payments.descriptionPlaceholder')" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreateModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">{{ t('common.confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Refund Modal -->
    <n-modal v-model:show="showRefundModal" preset="card" :title="t('admin.billing.payments.refundTitle')" style="width: 400px">
      <n-form ref="refundFormRef" :model="refundForm" label-placement="left" label-width="80">
        <n-form-item :label="t('admin.billing.payments.refundAmount')">
          <n-input-number
            v-model:value="refundForm.amount"
            :placeholder="t('admin.billing.payments.refundAmountPlaceholder')"
            :min="0.01"
            :max="refundForm.maxAmount"
            :precision="2"
            style="width: 100%"
          >
            <template #prefix>¥</template>
          </n-input-number>
        </n-form-item>
        <n-form-item :label="t('admin.billing.payments.refundReason')">
          <n-input v-model:value="refundForm.reason" type="textarea" :placeholder="t('admin.billing.payments.refundReasonPlaceholder')" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showRefundModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="error" :loading="refunding" @click="handleRefund">{{ t('admin.billing.payments.confirmRefund') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, type VNode } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NIcon, NCard, NSpace, NInput, NSelect, NDataTable, NModal, NForm, NFormItem,
  NTag, NInputNumber, NDatePicker, NGrid, NGi, NStatistic,
  type DataTableColumns, type FormInst, type FormRules
} from 'naive-ui'
import { AddOutline, SearchOutline } from '@vicons/ionicons5'
import { useBillingStore } from '../../../../stores/billing'
import type { Payment } from '../../../../services/billing'
import { showToast } from '../../../../utils/toast'

const { t } = useI18n()
const billingStore = useBillingStore()

// Filters
const filters = ref({
  user_id: '',
  status: null as string | null,
  method: null as string | null,
})
const dateRange = ref<[number, number] | null>(null)

const statusOptions = computed(() => [
  { label: t('admin.billing.payments.statusPending'), value: 'pending' },
  { label: t('admin.billing.payments.statusPaid'), value: 'paid' },
  { label: t('admin.billing.payments.statusFailed'), value: 'failed' },
  { label: t('admin.billing.payments.statusRefunded'), value: 'refunded' },
  { label: t('admin.billing.payments.statusCanceled'), value: 'canceled' },
])

const methodOptions = computed(() => [
  { label: t('admin.billing.payments.methodAlipay'), value: 'alipay' },
  { label: t('admin.billing.payments.methodWechat'), value: 'wechat' },
  { label: t('admin.billing.payments.methodManual'), value: 'manual' },
])

// Stats
const totalRevenue = computed(() => {
  return billingStore.payments
    .filter(p => p.status === 'paid')
    .reduce((sum, p) => sum + p.amount_cents / 100, 0)
    .toFixed(2)
})

const pendingCount = computed(() => billingStore.payments.filter(p => p.status === 'pending').length)
const paidCount = computed(() => billingStore.payments.filter(p => p.status === 'paid').length)
const refundedCount = computed(() => billingStore.payments.filter(p => p.status === 'refunded').length)

// Pagination
const pagination = computed(() => ({
  page: billingStore.paymentsPage,
  pageSize: billingStore.paymentsPageSize,
  itemCount: billingStore.paymentsTotal,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

// Format amount
const formatAmount = (cents: number, currency: string) => {
  const amount = cents / 100
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(amount)
}

// Table columns
const columns = computed<DataTableColumns<Payment>>(() => [
  { title: t('admin.billing.payments.colOrderNo'), key: 'order_no', width: 180 },
  { title: t('admin.billing.payments.colUserId'), key: 'user_id', width: 120 },
  { title: t('admin.billing.payments.colUsername'), key: 'username', width: 100 },
  {
    title: t('admin.billing.payments.colAmount'),
    key: 'amount_cents',
    width: 100,
    render: (row) => formatAmount(row.amount_cents, row.currency)
  },
  {
    title: t('admin.billing.payments.colMethod'),
    key: 'method',
    width: 80,
    render: (row) => {
      const labels: Record<string, string> = {
        alipay: t('admin.billing.payments.methodAlipay'),
        wechat: t('admin.billing.payments.methodWechat'),
        manual: t('admin.billing.payments.methodManual'),
      }
      return labels[row.method] || row.method
    }
  },
  {
    title: t('admin.billing.payments.colStatus'),
    key: 'status',
    width: 80,
    render: (row) => {
      const typeMap: Record<string, 'success' | 'warning' | 'error' | 'info' | 'default'> = {
        paid: 'success',
        pending: 'warning',
        failed: 'error',
        refunded: 'info',
        canceled: 'default',
      }
      return h(NTag, { type: typeMap[row.status] || 'default', size: 'small' }, { default: () => row.status })
    }
  },
  { title: t('admin.billing.payments.colDescription'), key: 'description', width: 150, ellipsis: { tooltip: true } },
  { title: t('admin.billing.payments.colCreatedAt'), key: 'created_at', width: 160 },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) => {
      const buttons: VNode[] = []
      if (row.status === 'pending') {
        buttons.push(
          h(NButton, { size: 'small', type: 'success', onClick: () => handleConfirm(row) }, { default: () => t('admin.billing.payments.confirm') })
        )
      }
      if (row.status === 'paid') {
        buttons.push(
          h(NButton, { size: 'small', type: 'error', onClick: () => openRefund(row) }, { default: () => t('admin.billing.payments.refund') })
        )
      }
      return h(NSpace, { size: 'small' }, { default: () => buttons })
    }
  }
])

// Create payment
const showCreateModal = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInst | null>(null)
const createForm = ref({
  user_id: '',
  amount: 0,
  method: 'alipay' as 'alipay' | 'wechat',
  description: '',
})
const createRules: FormRules = {
  user_id: [{ required: true, message: t('admin.billing.payments.userIdRequired'), trigger: 'blur' }],
  amount: [{ type: 'number', required: true, min: 0.01, message: t('admin.billing.payments.amountRequired'), trigger: 'blur' }],
  method: [{ required: true, message: t('admin.billing.payments.methodRequired'), trigger: 'change' }],
}

// Refund
const showRefundModal = ref(false)
const refunding = ref(false)
const refundFormRef = ref<FormInst | null>(null)
const currentPaymentId = ref('')
const refundForm = ref({
  amount: 0,
  maxAmount: 0,
  reason: '',
})

// Methods
const handleSearch = () => {
  const params: any = {
    user_id: filters.value.user_id || undefined,
    status: filters.value.status || undefined,
    method: filters.value.method || undefined,
  }
  if (dateRange.value) {
    params.start_time = new Date(dateRange.value[0]).toISOString()
    params.end_time = new Date(dateRange.value[1]).toISOString()
  }
  billingStore.loadPayments(1, billingStore.paymentsPageSize, params)
}

const handleReset = () => {
  filters.value = { user_id: '', status: null, method: null }
  dateRange.value = null
  billingStore.loadPayments(1, billingStore.paymentsPageSize)
}

const handlePageChange = (page: number) => {
  const params: any = {
    user_id: filters.value.user_id || undefined,
    status: filters.value.status || undefined,
    method: filters.value.method || undefined,
  }
  if (dateRange.value) {
    params.start_time = new Date(dateRange.value[0]).toISOString()
    params.end_time = new Date(dateRange.value[1]).toISOString()
  }
  billingStore.loadPayments(page, billingStore.paymentsPageSize, params)
}

const handleCreate = async () => {
  try {
    await createFormRef.value?.validate()
    creating.value = true
    const payment = await billingStore.addPayment({
      user_id: createForm.value.user_id,
      amount_cents: Math.round(createForm.value.amount * 100),
      method: createForm.value.method,
      description: createForm.value.description || undefined,
    })
    showToast(t('admin.billing.payments.createSuccess'), 'success')
    showCreateModal.value = false
    createForm.value = { user_id: '', amount: 0, method: 'alipay', description: '' }

    // If there's a pay_url, open it
    if (payment?.pay_url) {
      window.open(payment.pay_url, '_blank')
    }
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    creating.value = false
  }
}

const handleConfirm = async (row: Payment) => {
  try {
    await billingStore.confirmPayment(row.id)
    showToast(t('admin.billing.payments.confirmSuccess'), 'success')
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  }
}

const openRefund = (payment: Payment) => {
  currentPaymentId.value = payment.id
  refundForm.value = {
    amount: payment.amount_cents / 100,
    maxAmount: payment.amount_cents / 100,
    reason: '',
  }
  showRefundModal.value = true
}

const handleRefund = async () => {
  try {
    refunding.value = true
    await billingStore.refund(
      currentPaymentId.value,
      Math.round(refundForm.value.amount * 100),
      refundForm.value.reason || undefined
    )
    showToast(t('admin.billing.payments.refundSuccess'), 'success')
    showRefundModal.value = false
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    refunding.value = false
  }
}

onMounted(() => {
  billingStore.loadPayments()
})
</script>

<style scoped>
.billing-payments {
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

.filter-card {
  margin-bottom: 16px;
}

.stats-grid {
  margin-bottom: 16px;
}
</style>
