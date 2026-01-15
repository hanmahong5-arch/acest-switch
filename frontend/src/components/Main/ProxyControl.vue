<template>
  <div class="proxy-control">
    <div class="header">
      <h3>{{ $t('proxyControl.title') }}</h3>
      <n-button
        size="small"
        :loading="loading"
        @click="refreshConfigs"
      >
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        {{ $t('common.refresh') }}
      </n-button>
    </div>

    <p class="description">{{ $t('proxyControl.description') }}</p>

    <!-- Loading State -->
    <div v-if="loading && apps.length === 0" class="loading-state">
      <n-spin size="large" />
      <p>{{ $t('proxyControl.loading') }}</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <n-icon size="48" color="#d03050"><WarningOutline /></n-icon>
      <p class="error-message">{{ error }}</p>
      <n-button @click="refreshConfigs">{{ $t('common.retry') }}</n-button>
    </div>

    <!-- Proxy Controls -->
    <div v-else class="proxy-list">
      <n-card
        v-for="app in apps"
        :key="app.name"
        class="app-card"
        :class="{ 'disabled': !app.enabled }"
      >
        <div class="app-header">
          <div class="app-info">
            <div class="app-icon">
              <n-icon size="32" :color="app.iconColor">
                <component :is="app.icon" />
              </n-icon>
            </div>
            <div class="app-details">
              <h4>{{ app.label }}</h4>
              <p class="app-desc">{{ app.description }}</p>
            </div>
          </div>

          <n-switch
            :value="app.enabled"
            :loading="app.toggling"
            size="large"
            @update:value="(val) => toggleProxy(app.name, val)"
          >
            <template #checked>
              {{ $t('proxyControl.enabled') }}
            </template>
            <template #unchecked>
              {{ $t('proxyControl.disabled') }}
            </template>
          </n-switch>
        </div>

        <!-- Statistics -->
        <div v-if="app.stats" class="app-stats">
          <div class="stat-item">
            <span class="stat-label">{{ $t('proxyControl.totalRequests') }}</span>
            <span class="stat-value">{{ formatNumber(app.stats.total_requests) }}</span>
          </div>

          <div v-if="app.stats.last_request_at" class="stat-item">
            <span class="stat-label">{{ $t('proxyControl.lastRequest') }}</span>
            <span class="stat-value">{{ formatTime(app.stats.last_request_at) }}</span>
          </div>
        </div>

        <!-- Status Badge -->
        <div class="app-status">
          <n-tag :type="app.enabled ? 'success' : 'default'" :bordered="false" size="small">
            <template #icon>
              <n-icon>
                <component :is="app.enabled ? CheckmarkCircleOutline : CloseCircleOutline" />
              </n-icon>
            </template>
            {{ app.enabled ? $t('proxyControl.active') : $t('proxyControl.inactive') }}
          </n-tag>
        </div>
      </n-card>
    </div>

    <!-- Summary Stats -->
    <div v-if="apps.length > 0" class="summary">
      <n-alert
        :type="allEnabled ? 'success' : allDisabled ? 'warning' : 'info'"
        :show-icon="true"
      >
        <template v-if="allEnabled">
          {{ $t('proxyControl.allEnabled') }}
        </template>
        <template v-else-if="allDisabled">
          {{ $t('proxyControl.allDisabled') }}
        </template>
        <template v-else>
          {{ $t('proxyControl.partialEnabled', { enabled: enabledCount, total: apps.length }) }}
        </template>
      </n-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NButton, NIcon, NSwitch, NTag, NSpin, NAlert, useMessage } from 'naive-ui'
import {
  RefreshOutline,
  WarningOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
  CloudOutline,
  CodeSlashOutline,
  SparklesOutline
} from '@vicons/ionicons5'
import { proxyControlApi } from '@/services/proxy-control'
import type { ProxyControlConfig, ProxyControlStats } from '@/services/proxy-control'
import { formatDistanceToNow } from 'date-fns'

const { t } = useI18n()
const message = useMessage()

interface AppConfig {
  name: string
  label: string
  description: string
  icon: any
  iconColor: string
  enabled: boolean
  toggling: boolean
  stats?: {
    total_requests: number
    last_request_at?: string
  }
}

const apps = ref<AppConfig[]>([
  {
    name: 'claude',
    label: 'Claude Code',
    description: t('proxyControl.apps.claude.description'),
    icon: CloudOutline,
    iconColor: '#D97706',
    enabled: true,
    toggling: false
  },
  {
    name: 'codex',
    label: 'Codex',
    description: t('proxyControl.apps.codex.description'),
    icon: CodeSlashOutline,
    iconColor: '#10B981',
    enabled: true,
    toggling: false
  },
  {
    name: 'gemini',
    label: 'Gemini CLI',
    description: t('proxyControl.apps.gemini.description'),
    icon: SparklesOutline,
    iconColor: '#3B82F6',
    enabled: true,
    toggling: false
  }
])

const loading = ref(false)
const error = ref<string | null>(null)

// Computed
const enabledCount = computed(() => apps.value.filter(a => a.enabled).length)
const allEnabled = computed(() => enabledCount.value === apps.value.length)
const allDisabled = computed(() => enabledCount.value === 0)

// Fetch configurations
async function fetchConfigs() {
  try {
    loading.value = true
    error.value = null

    const { configs, stats } = await proxyControlApi.getConfigs()

    // Update apps with fetched data
    apps.value.forEach(app => {
      const config = configs.find((c: ProxyControlConfig) => c.app_name === app.name)
      if (config) {
        app.enabled = config.proxy_enabled
      }

      const stat: ProxyControlStats | undefined = stats[app.name]
      if (stat) {
        app.stats = {
          total_requests: stat.total_requests,
          last_request_at: stat.last_request_at
        }
      }
    })
  } catch (err: any) {
    error.value = err.message || t('proxyControl.loadError')
  } finally {
    loading.value = false
  }
}

// Refresh configurations
async function refreshConfigs() {
  await fetchConfigs()
}

// Toggle proxy
async function toggleProxy(appName: string, enabled: boolean) {
  const app = apps.value.find(a => a.name === appName)
  if (!app) return

  try {
    app.toggling = true

    await proxyControlApi.toggleProxy(appName, enabled)

    app.enabled = enabled

    message.success(
      enabled
        ? t('proxyControl.enableSuccess', { app: app.label })
        : t('proxyControl.disableSuccess', { app: app.label })
    )
  } catch (err: any) {
    error.value = err.message || t('proxyControl.toggleError')
    message.error(t('proxyControl.toggleError'))

    // Revert on error
    app.enabled = !enabled
  } finally {
    app.toggling = false
  }
}

// Format number with commas
function formatNumber(num: number): string {
  return num.toLocaleString()
}

// Format time
function formatTime(timestamp: string): string {
  if (!timestamp) return '-'
  try {
    return formatDistanceToNow(new Date(timestamp), { addSuffix: true })
  } catch {
    return timestamp
  }
}

// Lifecycle
onMounted(() => {
  fetchConfigs()
})
</script>

<style scoped lang="scss">
.proxy-control {
  padding: 20px;

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;

    h3 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
    }
  }

  .description {
    margin: 0 0 24px;
    color: #666;
    font-size: 14px;
  }

  .loading-state,
  .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    text-align: center;

    p {
      margin: 16px 0 0;
      color: #666;
    }

    .error-message {
      color: #d03050;
    }
  }

  .proxy-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  .app-card {
    position: relative;
    transition: all 0.3s ease;

    &:hover {
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }

    &.disabled {
      opacity: 0.6;
    }
  }

  .app-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .app-info {
    display: flex;
    align-items: center;
    gap: 12px;
    flex: 1;
  }

  .app-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border-radius: 12px;
    background: #f5f5f5;
  }

  .app-details {
    flex: 1;

    h4 {
      margin: 0 0 4px;
      font-size: 16px;
      font-weight: 600;
    }

    .app-desc {
      margin: 0;
      font-size: 13px;
      color: #666;
    }
  }

  .app-stats {
    display: flex;
    gap: 24px;
    margin-bottom: 12px;
    padding: 12px;
    background: #f5f5f5;
    border-radius: 6px;

    .stat-item {
      display: flex;
      flex-direction: column;
      gap: 4px;

      .stat-label {
        font-size: 12px;
        color: #666;
      }

      .stat-value {
        font-size: 16px;
        font-weight: 600;
        color: #333;
      }
    }
  }

  .app-status {
    display: flex;
    justify-content: flex-end;
  }

  .summary {
    margin-top: 24px;
  }
}
</style>
