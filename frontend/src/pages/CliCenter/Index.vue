<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  NCard, NGrid, NGi, NButton, NButtonGroup, NSpace, NSpin, NAlert,
  NResult, NModal, NList, NListItem, NThing, NTag, NIcon, NInputNumber,
  NForm, NFormItem, NInput, useMessage
} from 'naive-ui'
import {
  CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined,
  WarningOutlined, SyncOutlined, PoweroffOutlined, FolderOpenOutlined,
  HeartOutlined, SettingOutlined
} from '@vicons/antd'
import {
  GetAllStatus, EnableAll, DisableAll, HealthCheck,
  OpenConfigDir, GetProxyConfig, SetProxyConfig
} from '../../../bindings/codeswitch/services/clicenterservice'

const message = useMessage()

const loading = ref(false)
const actionLoading = ref(false)
const status = ref<any>(null)
const healthResult = ref<any>(null)
const proxyConfig = ref({ port: 18100, host: '127.0.0.1', protocol: 'http' })

const showResultModal = ref(false)
const resultData = ref<any>(null)
const resultTitle = ref('')

const showConfigModal = ref(false)
const configForm = ref({ port: 18100, host: '127.0.0.1', protocol: 'http' })

const cliList = computed(() => {
  if (!status.value) return []
  return [
    { key: 'claude', name: 'Claude Code', ...status.value.claude },
    { key: 'codex', name: 'Codex CLI', ...status.value.codex },
    { key: 'gemini', name: 'Gemini CLI', ...status.value.gemini },
    { key: 'picoclaw', name: 'PicoClaw', ...status.value.picoclaw }
  ]
})

const loadData = async () => {
  try {
    loading.value = true
    const [allStatus, config] = await Promise.all([
      GetAllStatus(),
      GetProxyConfig()
    ])
    status.value = allStatus
    proxyConfig.value = config
    configForm.value = { ...config }
  } catch (e: any) {
    message.error(e.message || 'Failed to load status')
  } finally {
    loading.value = false
  }
}

const handleEnableAll = async () => {
  try {
    actionLoading.value = true
    const result = await EnableAll()
    resultData.value = result
    resultTitle.value = 'Enable All Results'
    showResultModal.value = true
    await loadData()
  } catch (e: any) {
    message.error(e.message || 'Failed to enable all')
  } finally {
    actionLoading.value = false
  }
}

const handleDisableAll = async () => {
  try {
    actionLoading.value = true
    const result = await DisableAll()
    resultData.value = result
    resultTitle.value = 'Disable All Results'
    showResultModal.value = true
    await loadData()
  } catch (e: any) {
    message.error(e.message || 'Failed to disable all')
  } finally {
    actionLoading.value = false
  }
}

const handleHealthCheck = async () => {
  try {
    actionLoading.value = true
    healthResult.value = await HealthCheck()
    message.success('Health check completed')
  } catch (e: any) {
    message.error(e.message || 'Failed to perform health check')
  } finally {
    actionLoading.value = false
  }
}

const handleOpenDir = async (cli: string) => {
  try {
    await OpenConfigDir(cli)
  } catch (e: any) {
    message.error(e.message || 'Failed to open directory')
  }
}

const openConfigModal = () => {
  configForm.value = { ...proxyConfig.value }
  showConfigModal.value = true
}

const saveProxyConfig = async () => {
  try {
    await SetProxyConfig(configForm.value)
    proxyConfig.value = { ...configForm.value }
    showConfigModal.value = false
    message.success('Proxy configuration saved')
  } catch (e: any) {
    message.error(e.message || 'Failed to save configuration')
  }
}

const getStatusType = (cli: any) => {
  if (cli.enabled) return 'success'
  if (cli.configured) return 'warning'
  return 'default'
}

const getStatusText = (cli: any) => {
  if (cli.enabled) return 'Enabled'
  if (cli.configured) return 'Configured'
  return 'Not Configured'
}

const getStatusIcon = (cli: any) => {
  if (cli.enabled) return CheckCircleOutlined
  if (cli.configured) return MinusCircleOutlined
  return CloseCircleOutlined
}

const getHealthStatusType = (status: string) => {
  switch (status) {
    case 'healthy': return 'success'
    case 'warning': return 'warning'
    case 'error': return 'error'
    default: return 'default'
  }
}

onMounted(loadData)
</script>

<template>
  <div class="cli-center p-6">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold text-gray-800 dark:text-white">CLI Configuration Center</h1>
        <p class="text-gray-500 mt-1">Manage Claude Code, Codex, Gemini CLI, and PicoClaw proxy settings</p>
      </div>
      <n-button @click="loadData" :loading="loading" quaternary circle>
        <template #icon>
          <n-icon><SyncOutlined /></n-icon>
        </template>
      </n-button>
    </div>

    <n-spin :show="loading">
      <!-- Action Bar -->
      <n-card class="mb-6">
        <div class="flex flex-wrap items-center gap-4">
          <n-button-group>
            <n-button type="primary" :loading="actionLoading" @click="handleEnableAll">
              <template #icon>
                <n-icon><PoweroffOutlined /></n-icon>
              </template>
              Enable All
            </n-button>
            <n-button :loading="actionLoading" @click="handleDisableAll">
              <template #icon>
                <n-icon><CloseCircleOutlined /></n-icon>
              </template>
              Disable All
            </n-button>
            <n-button :loading="actionLoading" @click="handleHealthCheck">
              <template #icon>
                <n-icon><HeartOutlined /></n-icon>
              </template>
              Health Check
            </n-button>
          </n-button-group>

          <n-button-group>
            <n-button @click="handleOpenDir('claude')">
              <template #icon>
                <n-icon><FolderOpenOutlined /></n-icon>
              </template>
              Claude Dir
            </n-button>
            <n-button @click="handleOpenDir('codex')">
              <template #icon>
                <n-icon><FolderOpenOutlined /></n-icon>
              </template>
              Codex Dir
            </n-button>
            <n-button @click="handleOpenDir('gemini')">
              <template #icon>
                <n-icon><FolderOpenOutlined /></n-icon>
              </template>
              Gemini Dir
            </n-button>
            <n-button @click="handleOpenDir('picoclaw')">
              <template #icon>
                <n-icon><FolderOpenOutlined /></n-icon>
              </template>
              PicoClaw Dir
            </n-button>
          </n-button-group>

          <n-button @click="openConfigModal">
            <template #icon>
              <n-icon><SettingOutlined /></n-icon>
            </template>
            Proxy Settings
          </n-button>
        </div>
      </n-card>

      <!-- Proxy Info -->
      <n-alert type="info" class="mb-6" :show-icon="false">
        <div class="flex items-center gap-4">
          <span class="font-medium">Proxy Address:</span>
          <code class="bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
            {{ proxyConfig.protocol }}://{{ proxyConfig.host }}:{{ proxyConfig.port }}
          </code>
        </div>
      </n-alert>

      <!-- CLI Status Cards -->
      <n-grid :cols="4" :x-gap="16" :y-gap="16">
        <n-gi v-for="cli in cliList" :key="cli.key">
          <n-card :title="cli.name" hoverable>
            <template #header-extra>
              <n-tag :type="getStatusType(cli)" size="small">
                <template #icon>
                  <n-icon :component="getStatusIcon(cli)" />
                </template>
                {{ getStatusText(cli) }}
              </n-tag>
            </template>

            <div class="space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-gray-500 text-sm">Base URL:</span>
                <code class="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                  {{ cli.base_url || '-' }}
                </code>
              </div>

              <div class="flex items-center justify-between">
                <span class="text-gray-500 text-sm">Config Path:</span>
                <span class="text-xs truncate max-w-[200px]" :title="cli.config_path">
                  {{ cli.config_path || '-' }}
                </span>
              </div>

              <div v-if="cli.last_error" class="text-red-500 text-xs">
                Error: {{ cli.last_error }}
              </div>
            </div>

            <template #footer>
              <n-space>
                <n-button size="small" @click="handleOpenDir(cli.key)">
                  Open Directory
                </n-button>
              </n-space>
            </template>
          </n-card>
        </n-gi>
      </n-grid>

      <!-- Health Check Results -->
      <n-card v-if="healthResult" title="Health Check Results" class="mt-6">
        <div class="flex items-center gap-2 mb-4">
          <span class="text-gray-500">Proxy Server:</span>
          <n-tag :type="healthResult.proxy_server_running ? 'success' : 'error'" size="small">
            {{ healthResult.proxy_server_running ? 'Running' : 'Not Running' }}
          </n-tag>
        </div>

        <n-grid :cols="4" :x-gap="16">
          <n-gi>
            <div class="p-4 bg-gray-50 dark:bg-gray-800 rounded">
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium">Claude Code</span>
                <n-tag :type="getHealthStatusType(healthResult.claude.status)" size="small">
                  {{ healthResult.claude.status }}
                </n-tag>
              </div>
              <p class="text-sm text-gray-500">{{ healthResult.claude.message }}</p>
            </div>
          </n-gi>
          <n-gi>
            <div class="p-4 bg-gray-50 dark:bg-gray-800 rounded">
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium">Codex CLI</span>
                <n-tag :type="getHealthStatusType(healthResult.codex.status)" size="small">
                  {{ healthResult.codex.status }}
                </n-tag>
              </div>
              <p class="text-sm text-gray-500">{{ healthResult.codex.message }}</p>
            </div>
          </n-gi>
          <n-gi>
            <div class="p-4 bg-gray-50 dark:bg-gray-800 rounded">
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium">Gemini CLI</span>
                <n-tag :type="getHealthStatusType(healthResult.gemini.status)" size="small">
                  {{ healthResult.gemini.status }}
                </n-tag>
              </div>
              <p class="text-sm text-gray-500">{{ healthResult.gemini.message }}</p>
            </div>
          </n-gi>
          <n-gi>
            <div class="p-4 bg-gray-50 dark:bg-gray-800 rounded">
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium">PicoClaw</span>
                <n-tag :type="getHealthStatusType(healthResult.picoclaw.status)" size="small">
                  {{ healthResult.picoclaw.status }}
                </n-tag>
              </div>
              <p class="text-sm text-gray-500">{{ healthResult.picoclaw.message }}</p>
            </div>
          </n-gi>
        </n-grid>
      </n-card>
    </n-spin>

    <!-- Result Modal -->
    <n-modal v-model:show="showResultModal" preset="dialog" :title="resultTitle">
      <n-result
        :status="Object.values(resultData || {}).every(v => v === 'success') ? 'success' : 'warning'"
        :title="Object.values(resultData || {}).every(v => v === 'success') ? 'All Successful' : 'Partial Success'"
      >
        <n-list>
          <n-list-item v-for="(result, cli) in resultData" :key="cli as string">
            <n-thing :title="String(cli).toUpperCase()">
              <template #description>
                <n-tag :type="result === 'success' ? 'success' : 'error'" size="small">
                  {{ result === 'success' ? 'Success' : result }}
                </n-tag>
              </template>
            </n-thing>
          </n-list-item>
        </n-list>
      </n-result>
    </n-modal>

    <!-- Config Modal -->
    <n-modal v-model:show="showConfigModal" preset="dialog" title="Proxy Configuration">
      <n-form>
        <n-form-item label="Host">
          <n-input v-model:value="configForm.host" placeholder="127.0.0.1" />
        </n-form-item>
        <n-form-item label="Port">
          <n-input-number v-model:value="configForm.port" :min="1" :max="65535" style="width: 100%" />
        </n-form-item>
        <n-form-item label="Protocol">
          <n-input v-model:value="configForm.protocol" placeholder="http" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showConfigModal = false">Cancel</n-button>
          <n-button type="primary" @click="saveProxyConfig">Save</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.cli-center {
  min-height: 100vh;
  background: var(--n-color);
}
</style>
