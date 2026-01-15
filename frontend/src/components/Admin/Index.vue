<template>
  <div class="admin-layout">
    <!-- Sidebar Navigation -->
    <aside class="admin-sidebar">
      <div class="sidebar-header">
        <n-button quaternary circle @click="goBack" class="back-btn">
          <template #icon>
            <n-icon><ArrowBack /></n-icon>
          </template>
        </n-button>
        <h1 class="sidebar-title">{{ t('admin.title') }}</h1>
      </div>

      <n-menu
        :value="activeMenu"
        :options="menuOptions"
        @update:value="handleMenuChange"
      />

      <div class="sidebar-footer">
        <n-tag :type="syncConnected ? 'success' : 'error'" size="small">
          <template #icon>
            <n-icon>
              <CheckmarkCircle v-if="syncConnected" />
              <CloseCircle v-else />
            </n-icon>
          </template>
          {{ syncConnected ? t('admin.syncConnected') : t('admin.syncDisconnected') }}
        </n-tag>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="admin-content">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  NButton, NIcon, NMenu, NTag,
  type MenuOption
} from 'naive-ui'
import {
  ArrowBack,
  CheckmarkCircle,
  CloseCircle,
  SpeedometerOutline,
  PeopleOutline,
  ChatbubblesOutline,
  StatsChartOutline,
  DocumentTextOutline,
  NotificationsOutline,
  CardOutline,
  WalletOutline,
  ReceiptOutline,
  SettingsOutline,
} from '@vicons/ionicons5'
import { initSyncClient, getSyncStatus } from '../../services/sync'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const syncConnected = ref(false)

const activeMenu = computed(() => {
  const path = route.path
  if (path.includes('/admin/dashboard')) return 'dashboard'
  if (path.includes('/admin/users')) return 'users'
  if (path.includes('/admin/sessions')) return 'sessions'
  if (path.includes('/admin/stats')) return 'stats'
  if (path.includes('/admin/audit')) return 'audit'
  if (path.includes('/admin/alerts')) return 'alerts'
  if (path.includes('/admin/billing/subscriptions')) return 'billing-subscriptions'
  if (path.includes('/admin/billing/balances')) return 'billing-balances'
  if (path.includes('/admin/billing/payments')) return 'billing-payments'
  if (path.includes('/admin/billing/settings')) return 'billing-settings'
  return 'dashboard'
})

const renderIcon = (icon: any) => {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: t('admin.nav.dashboard'),
    key: 'dashboard',
    icon: renderIcon(SpeedometerOutline),
  },
  {
    label: t('admin.nav.users'),
    key: 'users',
    icon: renderIcon(PeopleOutline),
  },
  {
    label: t('admin.nav.sessions'),
    key: 'sessions',
    icon: renderIcon(ChatbubblesOutline),
  },
  {
    label: t('admin.nav.stats'),
    key: 'stats',
    icon: renderIcon(StatsChartOutline),
  },
  {
    label: t('admin.nav.audit'),
    key: 'audit',
    icon: renderIcon(DocumentTextOutline),
  },
  {
    label: t('admin.nav.alerts'),
    key: 'alerts',
    icon: renderIcon(NotificationsOutline),
  },
  {
    type: 'divider',
    key: 'billing-divider',
  },
  {
    label: t('admin.nav.billing'),
    key: 'billing',
    icon: renderIcon(CardOutline),
    children: [
      {
        label: t('admin.nav.billingSubscriptions'),
        key: 'billing-subscriptions',
        icon: renderIcon(CardOutline),
      },
      {
        label: t('admin.nav.billingBalances'),
        key: 'billing-balances',
        icon: renderIcon(WalletOutline),
      },
      {
        label: t('admin.nav.billingPayments'),
        key: 'billing-payments',
        icon: renderIcon(ReceiptOutline),
      },
      {
        label: t('admin.nav.billingSettings'),
        key: 'billing-settings',
        icon: renderIcon(SettingsOutline),
      },
    ],
  },
])

const goBack = () => {
  router.push('/')
}

const handleMenuChange = (key: string) => {
  // Handle billing sub-routes
  if (key.startsWith('billing-')) {
    const subRoute = key.replace('billing-', '')
    router.push(`/admin/billing/${subRoute}`)
  } else {
    router.push(`/admin/${key}`)
  }
}

const checkSyncStatus = async () => {
  try {
    await initSyncClient()
    const status = await getSyncStatus()
    syncConnected.value = status.connected
  } catch {
    syncConnected.value = false
  }
}

onMounted(() => {
  void checkSyncStatus()
})
</script>

<style scoped>
.admin-layout {
  display: flex;
  height: 100vh;
  background: var(--n-color);
}

.admin-sidebar {
  width: 240px;
  background: var(--n-card-color);
  border-right: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px;
  border-bottom: 1px solid var(--n-border-color);
}

.back-btn {
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: var(--n-text-color-1);
}

.sidebar-footer {
  margin-top: auto;
  padding: 16px;
  border-top: 1px solid var(--n-border-color);
}

.admin-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}
</style>
