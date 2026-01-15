<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard,
  NSwitch,
  NInput,
  NButton,
  NSpace,
  NProgress,
  NTag,
  NSpin,
  NAlert,
  NInputGroup,
  NIcon,
  NStatistic,
  NGrid,
  NGi,
  NDivider,
} from 'naive-ui'
import {
  ArrowBack,
  Eye,
  EyeOff,
  Refresh,
  CheckmarkCircle,
  CloseCircle,
  CloudUpload,
} from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'
import { getGatewayConfig, setGatewayConfig, testConnection, type GatewayConfig, type ConnectionStatus } from '../../services/gateway'

const { t } = useI18n()
const router = useRouter()

// State
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const showToken = ref(false)
const config = ref<GatewayConfig>({
  newApiEnabled: false,
  newApiUrl: 'http://api.lurus.cn',
  newApiToken: '',
})
const connectionStatus = ref<ConnectionStatus | null>(null)
const error = ref<string | null>(null)

// Computed
const quotaPercent = computed(() => {
  if (!connectionStatus.value?.quota) return 0
  const { quotaTotal, quotaUsed } = connectionStatus.value.quota
  if (quotaTotal === 0) return 0
  return Math.round(((quotaTotal - quotaUsed) / quotaTotal) * 100)
})

const quotaProgressStatus = computed(() => {
  if (quotaPercent.value > 50) return 'success'
  if (quotaPercent.value > 20) return 'warning'
  return 'error'
})

// Methods
const goBack = () => {
  router.push('/')
}

const loadConfig = async () => {
  loading.value = true
  error.value = null
  try {
    config.value = await getGatewayConfig()
    if (config.value.newApiEnabled && config.value.newApiToken) {
      await doTestConnection()
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  saving.value = true
  error.value = null
  try {
    await setGatewayConfig(config.value)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

const doTestConnection = async () => {
  testing.value = true
  error.value = null
  connectionStatus.value = null
  try {
    connectionStatus.value = await testConnection(config.value.newApiUrl, config.value.newApiToken)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    testing.value = false
  }
}

const formatCurrency = (value: number) => {
  return `$${(value / 500000).toFixed(2)}`
}

onMounted(() => {
  loadConfig()
})
</script>

<template>
  <div class="gateway-page">
    <!-- Header -->
    <div class="page-header">
      <n-button quaternary circle @click="goBack">
        <template #icon>
          <n-icon><ArrowBack /></n-icon>
        </template>
      </n-button>
      <h1>{{ t('gateway.title', 'NEW-API Gateway Configuration') }}</h1>
    </div>

    <n-spin :show="loading">
      <!-- Error Alert -->
      <n-alert v-if="error" type="error" closable @close="error = null" class="mb-4">
        {{ error }}
      </n-alert>

      <!-- Configuration Card -->
      <n-card :title="t('gateway.config', 'Gateway Settings')" class="mb-4">
        <n-space vertical size="large">
          <!-- Enable Switch -->
          <div class="config-row">
            <span class="config-label">{{ t('gateway.enabled', 'Enable NEW-API Gateway') }}</span>
            <n-switch v-model:value="config.newApiEnabled" @update:value="saveConfig" />
          </div>

          <!-- API URL -->
          <div class="config-row">
            <span class="config-label">{{ t('gateway.url', 'Gateway URL') }}</span>
            <n-input
              v-model:value="config.newApiUrl"
              placeholder="http://api.lurus.cn"
              :disabled="!config.newApiEnabled"
              @blur="saveConfig"
              style="max-width: 400px"
            />
          </div>

          <!-- API Token -->
          <div class="config-row">
            <span class="config-label">{{ t('gateway.token', 'API Token') }}</span>
            <n-input-group style="max-width: 400px">
              <n-input
                v-model:value="config.newApiToken"
                :type="showToken ? 'text' : 'password'"
                placeholder="sk-xxx"
                :disabled="!config.newApiEnabled"
                @blur="saveConfig"
              />
              <n-button @click="showToken = !showToken" :disabled="!config.newApiEnabled">
                <template #icon>
                  <n-icon>
                    <Eye v-if="showToken" />
                    <EyeOff v-else />
                  </n-icon>
                </template>
              </n-button>
            </n-input-group>
          </div>

          <!-- Test Connection Button -->
          <n-button
            type="primary"
            :loading="testing"
            :disabled="!config.newApiEnabled || !config.newApiToken"
            @click="doTestConnection"
          >
            <template #icon>
              <n-icon><CloudUpload /></n-icon>
            </template>
            {{ t('gateway.test', 'Test Connection') }}
          </n-button>
        </n-space>
      </n-card>

      <!-- Connection Status Card -->
      <n-card v-if="connectionStatus" :title="t('gateway.status', 'Connection Status')" class="mb-4">
        <template #header-extra>
          <n-tag :type="connectionStatus.success ? 'success' : 'error'">
            <template #icon>
              <n-icon>
                <CheckmarkCircle v-if="connectionStatus.success" />
                <CloseCircle v-else />
              </n-icon>
            </template>
            {{ connectionStatus.success ? t('gateway.connected', 'Connected') : t('gateway.failed', 'Failed') }}
          </n-tag>
        </template>

        <template v-if="connectionStatus.success && connectionStatus.user">
          <!-- User Info -->
          <n-grid :cols="2" :x-gap="16" :y-gap="16" class="mb-4">
            <n-gi>
              <n-statistic :label="t('gateway.user', 'User')">
                {{ connectionStatus.user.username || connectionStatus.user.email || 'N/A' }}
              </n-statistic>
            </n-gi>
            <n-gi>
              <n-statistic :label="t('gateway.userId', 'User ID')">
                {{ connectionStatus.user.id || 'N/A' }}
              </n-statistic>
            </n-gi>
          </n-grid>

          <!-- Quota Info -->
          <template v-if="connectionStatus.quota">
            <n-divider>{{ t('gateway.quota', 'Quota') }}</n-divider>
            <n-grid :cols="3" :x-gap="16" :y-gap="16" class="mb-4">
              <n-gi>
                <n-statistic :label="t('gateway.quotaTotal', 'Total')">
                  {{ formatCurrency(connectionStatus.quota.quotaTotal) }}
                </n-statistic>
              </n-gi>
              <n-gi>
                <n-statistic :label="t('gateway.quotaUsed', 'Used')">
                  {{ formatCurrency(connectionStatus.quota.quotaUsed) }}
                </n-statistic>
              </n-gi>
              <n-gi>
                <n-statistic :label="t('gateway.quotaRemain', 'Remaining')">
                  {{ formatCurrency(connectionStatus.quota.quotaTotal - connectionStatus.quota.quotaUsed) }}
                </n-statistic>
              </n-gi>
            </n-grid>
            <n-progress
              type="line"
              :percentage="quotaPercent"
              :status="quotaProgressStatus"
              :indicator-placement="'inside'"
            />
          </template>
        </template>

        <template v-else-if="connectionStatus.error">
          <n-alert type="error">
            {{ connectionStatus.error }}
          </n-alert>
        </template>
      </n-card>

      <!-- Channels Info Card -->
      <n-card v-if="connectionStatus?.success" :title="t('gateway.channels', 'Supported Channels')">
        <n-space>
          <n-tag type="success">OpenAI</n-tag>
          <n-tag type="success">Claude</n-tag>
          <n-tag type="success">Gemini</n-tag>
          <n-tag type="success">DeepSeek</n-tag>
          <n-tag type="info">40+ Providers</n-tag>
        </n-space>
        <template #footer>
          <span class="text-secondary">
            {{ t('gateway.channelsNote', 'All requests will be routed through NEW-API for unified billing and management.') }}
          </span>
        </template>
      </n-card>
    </n-spin>
  </div>
</template>

<style scoped>
.gateway-page {
  padding: 24px;
  max-width: 800px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
}

.config-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.config-label {
  font-weight: 500;
  min-width: 200px;
}

.mb-4 {
  margin-bottom: 16px;
}

.text-secondary {
  color: var(--mac-text-secondary);
  font-size: 0.875rem;
}
</style>
