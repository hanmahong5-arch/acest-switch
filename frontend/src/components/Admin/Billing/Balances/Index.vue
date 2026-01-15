<template>
  <div class="billing-balances">
    <div class="page-header">
      <h2>{{ t('admin.billing.balances.title') }}</h2>
      <n-button type="primary" @click="showCreateModal = true">
        <template #icon>
          <n-icon><AddOutline /></n-icon>
        </template>
        {{ t('admin.billing.balances.create') }}
      </n-button>
    </div>

    <!-- Filters -->
    <n-card size="small" class="filter-card">
      <n-space>
        <n-input
          v-model:value="filters.user_id"
          :placeholder="t('admin.billing.balances.filterUserId')"
          clearable
          style="width: 200px"
          @keyup.enter="handleSearch"
        />
        <n-select
          v-model:value="filters.status"
          :placeholder="t('admin.billing.balances.filterStatus')"
          :options="statusOptions"
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
      :data="billingStore.wallets"
      :loading="billingStore.walletsLoading"
      :pagination="pagination"
      :row-key="(row: Wallet) => row.id"
      @update:page="handlePageChange"
    />

    <!-- Create Wallet Modal -->
    <n-modal v-model:show="showCreateModal" preset="card" :title="t('admin.billing.balances.createTitle')" style="width: 500px">
      <n-form ref="createFormRef" :model="createForm" :rules="createRules" label-placement="left" label-width="100">
        <n-form-item :label="t('admin.billing.balances.userId')" path="user_id">
          <n-input v-model:value="createForm.user_id" :placeholder="t('admin.billing.balances.userIdPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('admin.billing.balances.name')" path="name">
          <n-input v-model:value="createForm.name" :placeholder="t('admin.billing.balances.namePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('admin.billing.balances.currency')" path="currency">
          <n-select
            v-model:value="createForm.currency"
            :options="currencyOptions"
            :placeholder="t('admin.billing.balances.currencyPlaceholder')"
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

    <!-- Top Up Modal -->
    <n-modal v-model:show="showTopUpModal" preset="card" :title="t('admin.billing.balances.topUpTitle')" style="width: 500px">
      <n-form ref="topUpFormRef" :model="topUpForm" :rules="topUpRules" label-placement="left" label-width="120">
        <n-form-item :label="t('admin.billing.balances.paidCredits')" path="paid_credits">
          <n-input-number
            v-model:value="topUpForm.paid_credits"
            :placeholder="t('admin.billing.balances.paidCreditsPlaceholder')"
            :min="0"
            style="width: 100%"
          />
        </n-form-item>
        <n-form-item :label="t('admin.billing.balances.grantedCredits')" path="granted_credits">
          <n-input-number
            v-model:value="topUpForm.granted_credits"
            :placeholder="t('admin.billing.balances.grantedCreditsPlaceholder')"
            :min="0"
            style="width: 100%"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showTopUpModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" :loading="topUpLoading" @click="handleTopUp">{{ t('common.confirm') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Transactions Drawer -->
    <n-drawer v-model:show="showTransactions" :width="600" placement="right">
      <n-drawer-content :title="t('admin.billing.balances.transactions')">
        <n-data-table
          :columns="transactionColumns"
          :data="billingStore.walletTransactions"
          :row-key="(row: WalletTransaction) => row.id"
          size="small"
        />
      </n-drawer-content>
    </n-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h, type VNode } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NIcon, NCard, NSpace, NInput, NSelect, NDataTable, NModal, NForm, NFormItem,
  NTag, NInputNumber, NDrawer, NDrawerContent,
  type DataTableColumns, type FormInst, type FormRules
} from 'naive-ui'
import { AddOutline, SearchOutline } from '@vicons/ionicons5'
import { useBillingStore } from '../../../../stores/billing'
import type { Wallet, WalletTransaction } from '../../../../services/billing'
import { showToast } from '../../../../utils/toast'

const { t } = useI18n()
const billingStore = useBillingStore()

// Filters
const filters = ref({
  user_id: '',
  status: null as string | null,
})

const statusOptions = computed(() => [
  { label: t('admin.billing.balances.statusActive'), value: 'active' },
  { label: t('admin.billing.balances.statusTerminated'), value: 'terminated' },
])

const currencyOptions = [
  { label: 'CNY', value: 'CNY' },
  { label: 'USD', value: 'USD' },
  { label: 'EUR', value: 'EUR' },
]

// Pagination
const pagination = computed(() => ({
  page: billingStore.walletsPage,
  pageSize: billingStore.walletsPageSize,
  itemCount: billingStore.walletsTotal,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
}))

// Format balance
const formatBalance = (cents: number, currency: string) => {
  const amount = cents / 100
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency }).format(amount)
}

// Table columns
const columns = computed<DataTableColumns<Wallet>>(() => [
  { title: t('admin.billing.balances.colUserId'), key: 'user_id', width: 150 },
  { title: t('admin.billing.balances.colUsername'), key: 'username', width: 120 },
  { title: t('admin.billing.balances.colName'), key: 'name', width: 120 },
  {
    title: t('admin.billing.balances.colBalance'),
    key: 'balance_cents',
    width: 120,
    render: (row) => formatBalance(row.balance_cents, row.currency)
  },
  {
    title: t('admin.billing.balances.colCredits'),
    key: 'credits_balance',
    width: 100,
    render: (row) => row.credits_balance.toFixed(2)
  },
  {
    title: t('admin.billing.balances.colStatus'),
    key: 'status',
    width: 80,
    render: (row) => {
      const type = row.status === 'active' ? 'success' : 'error'
      return h(NTag, { type, size: 'small' }, { default: () => row.status })
    }
  },
  { title: t('admin.billing.balances.colCreatedAt'), key: 'created_at', width: 160 },
  {
    title: t('common.actions'),
    key: 'actions',
    width: 200,
    render: (row) => {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(NButton, { size: 'small', type: 'primary', onClick: () => openTopUp(row) }, { default: () => t('admin.billing.balances.topUp') }),
          h(NButton, { size: 'small', onClick: () => viewTransactions(row) }, { default: () => t('admin.billing.balances.viewTransactions') }),
        ]
      })
    }
  }
])

// Transaction columns
const transactionColumns = computed<DataTableColumns<WalletTransaction>>(() => [
  {
    title: t('admin.billing.balances.txType'),
    key: 'type',
    width: 80,
    render: (row) => {
      const type = row.type === 'inbound' ? 'success' : 'warning'
      return h(NTag, { type, size: 'small' }, { default: () => row.type })
    }
  },
  { title: t('admin.billing.balances.txTransactionType'), key: 'transaction_type', width: 100 },
  { title: t('admin.billing.balances.txAmount'), key: 'amount', width: 100 },
  { title: t('admin.billing.balances.txCredits'), key: 'credit_amount', width: 100 },
  {
    title: t('admin.billing.balances.txStatus'),
    key: 'status',
    width: 80,
    render: (row) => {
      const type = row.status === 'settled' ? 'success' : 'warning'
      return h(NTag, { type, size: 'small' }, { default: () => row.status })
    }
  },
  { title: t('admin.billing.balances.txCreatedAt'), key: 'created_at', width: 160 },
])

// Create wallet
const showCreateModal = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInst | null>(null)
const createForm = ref({
  user_id: '',
  name: '',
  currency: 'CNY',
})
const createRules: FormRules = {
  user_id: [{ required: true, message: t('admin.billing.balances.userIdRequired'), trigger: 'blur' }],
}

// Top up
const showTopUpModal = ref(false)
const topUpLoading = ref(false)
const topUpFormRef = ref<FormInst | null>(null)
const currentWalletId = ref('')
const topUpForm = ref({
  paid_credits: 0,
  granted_credits: 0,
})
const topUpRules: FormRules = {
  paid_credits: [{ type: 'number', min: 0, message: t('admin.billing.balances.paidCreditsRequired'), trigger: 'blur' }],
}

// Transactions drawer
const showTransactions = ref(false)

// Methods
const handleSearch = () => {
  billingStore.loadWallets(1, billingStore.walletsPageSize, {
    user_id: filters.value.user_id || undefined,
    status: filters.value.status || undefined,
  })
}

const handleReset = () => {
  filters.value = { user_id: '', status: null }
  billingStore.loadWallets(1, billingStore.walletsPageSize)
}

const handlePageChange = (page: number) => {
  billingStore.loadWallets(page, billingStore.walletsPageSize, {
    user_id: filters.value.user_id || undefined,
    status: filters.value.status || undefined,
  })
}

const handleCreate = async () => {
  try {
    await createFormRef.value?.validate()
    creating.value = true
    await billingStore.addWallet(createForm.value.user_id, createForm.value.name, createForm.value.currency)
    showToast(t('admin.billing.balances.createSuccess'), 'success')
    showCreateModal.value = false
    createForm.value = { user_id: '', name: '', currency: 'CNY' }
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    creating.value = false
  }
}

const openTopUp = (wallet: Wallet) => {
  currentWalletId.value = wallet.id
  topUpForm.value = { paid_credits: 0, granted_credits: 0 }
  showTopUpModal.value = true
}

const handleTopUp = async () => {
  try {
    await topUpFormRef.value?.validate()
    topUpLoading.value = true
    await billingStore.topUp(
      currentWalletId.value,
      String(topUpForm.value.paid_credits),
      topUpForm.value.granted_credits ? String(topUpForm.value.granted_credits) : undefined
    )
    showToast(t('admin.billing.balances.topUpSuccess'), 'success')
    showTopUpModal.value = false
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    topUpLoading.value = false
  }
}

const viewTransactions = async (wallet: Wallet) => {
  try {
    await billingStore.loadWalletTransactions(wallet.id)
    showTransactions.value = true
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  }
}

onMounted(() => {
  billingStore.loadWallets()
})
</script>

<style scoped>
.billing-balances {
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
</style>
