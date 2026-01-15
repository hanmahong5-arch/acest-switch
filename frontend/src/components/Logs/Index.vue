<template>
  <div class="logs-page">
    <div class="logs-header">
      <BaseButton variant="outline" type="button" @click="backToHome">
        {{ t('components.logs.back') }}
      </BaseButton>
      <div class="refresh-indicator">
        <span>{{ t('components.logs.nextRefresh', { seconds: countdown }) }}</span>
        <BaseButton size="sm" :disabled="loading" @click="manualRefresh">
          {{ t('components.logs.refresh') }}
        </BaseButton>
      </div>
    </div>

    <section class="logs-summary" v-if="statsCards.length">
      <article v-for="card in statsCards" :key="card.key" class="summary-card">
        <div class="summary-card__label">{{ card.label }}</div>
        <div class="summary-card__value">{{ card.value }}</div>
        <div class="summary-card__hint">{{ card.hint }}</div>
      </article>
    </section>

    <section class="logs-chart">
      <Line :data="chartData" :options="chartOptions" />
    </section>

    <form class="logs-filter-row" @submit.prevent="applyFilters">
      <div class="filter-fields">
        <label class="filter-field filter-field-search">
          <span>{{ t('components.logs.filters.search') }}</span>
          <input
            v-model="filters.search"
            type="text"
            :placeholder="t('components.logs.filters.searchPlaceholder')"
            class="mac-input"
          />
        </label>
        <label class="filter-field">
          <span>{{ t('components.logs.filters.platform') }}</span>
          <select v-model="filters.platform" class="mac-select">
            <option value="">{{ t('components.logs.filters.allPlatforms') }}</option>
            <option value="claude">Claude</option>
            <option value="codex">Codex</option>
          </select>
        </label>
        <label class="filter-field">
          <span>{{ t('components.logs.filters.provider') }}</span>
          <select v-model="filters.provider" class="mac-select">
            <option value="">{{ t('components.logs.filters.allProviders') }}</option>
            <option v-for="provider in providerOptions" :key="provider" :value="provider">
              {{ provider }}
            </option>
          </select>
        </label>
      </div>
      <div class="filter-actions">
        <BaseButton type="submit" :disabled="loading">
          {{ t('components.logs.query') }}
        </BaseButton>
      </div>
    </form>

    <section class="logs-table-wrapper">
      <table class="logs-table">
        <thead>
          <tr>
            <th class="col-time">{{ t('components.logs.table.time') }}</th>
            <th class="col-trace-id">{{ t('components.logs.table.traceId') }}</th>
            <th class="col-platform">{{ t('components.logs.table.platform') }}</th>
            <th class="col-provider">{{ t('components.logs.table.provider') }}</th>
            <th class="col-model">{{ t('components.logs.table.model') }}</th>
            <th class="col-http">{{ t('components.logs.table.httpCode') }}</th>
            <th class="col-error">{{ t('components.logs.table.errorType') }}</th>
            <th class="col-duration">{{ t('components.logs.table.duration') }}</th>
            <th class="col-actions">{{ t('components.logs.table.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in pagedLogs" :key="item.id" :class="{ 'error-row': item.http_code >= 400 }">
            <td>{{ formatTime(item.created_at) }}</td>
            <td class="trace-id-cell">
              <button
                v-if="item.trace_id"
                @click="copyTraceId(item.trace_id)"
                class="trace-id-btn"
                :title="item.trace_id"
              >
                {{ formatTraceId(item.trace_id) }}
              </button>
              <span v-else>—</span>
            </td>
            <td>{{ item.platform || '—' }}</td>
            <td>{{ item.provider || '—' }}</td>
            <td class="model-cell" :title="item.model">{{ item.model || '—' }}</td>
            <td :class="['code', httpCodeClass(item.http_code)]">{{ item.http_code }}</td>
            <td>
              <span v-if="item.error_type" :class="['error-badge', errorTypeClass(item.error_type)]">
                {{ formatErrorType(item.error_type) }}
              </span>
              <span v-else class="success-badge">✓</span>
            </td>
            <td><span :class="['duration-tag', durationColor(item.duration_sec)]">{{ formatDuration(item.duration_sec) }}</span></td>
            <td class="actions-cell">
              <BaseButton size="sm" variant="outline" @click="showDetails(item)">
                {{ t('components.logs.viewDetails') }}
              </BaseButton>
            </td>
          </tr>
          <tr v-if="!pagedLogs.length && !loading">
            <td colspan="9" class="empty">{{ t('components.logs.empty') }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="loading" class="empty">{{ t('components.logs.loading') }}</p>
    </section>

    <div class="logs-pagination">
      <span>{{ page }} / {{ totalPages }}</span>
      <div class="pagination-actions">
        <BaseButton variant="outline" size="sm" :disabled="page === 1 || loading" @click="prevPage">
          ‹
        </BaseButton>
        <BaseButton variant="outline" size="sm" :disabled="page >= totalPages || loading" @click="nextPage">
          ›
        </BaseButton>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <div v-if="selectedLog" class="modal-overlay" @click.self="closeDetails">
      <div class="modal-content">
        <div class="modal-header">
          <h2>{{ t('components.logs.detailsModal.title') }}</h2>
          <button class="modal-close" @click="closeDetails">×</button>
        </div>
        <div class="modal-tabs">
          <button
            :class="['modal-tab', { active: activeDetailTab === 'basic' }]"
            @click="switchDetailTab('basic')"
          >
            {{ t('components.logs.detailsModal.basic') }}
          </button>
          <button
            :class="['modal-tab', { active: activeDetailTab === 'body' }]"
            @click="switchDetailTab('body')"
          >
            {{ t('components.logs.detailsModal.body') }}
          </button>
        </div>
        <div class="modal-body">
          <!-- Body 标签页 -->
          <template v-if="activeDetailTab === 'body'">
            <div v-if="bodyLoading" class="body-loading">
              {{ t('components.logs.loading') }}
            </div>
            <div v-else-if="!selectedLogBody" class="body-empty">
              {{ t('components.logs.detailsModal.bodyNotRecorded') }}
            </div>
            <template v-else>
              <div class="body-meta">
                <span class="body-meta-item">
                  <span class="body-meta-label">{{ t('components.logs.detailsModal.bodySize') }}:</span>
                  {{ formatBodySize(selectedLogBody.body_size_bytes) }}
                </span>
                <span class="body-meta-item">
                  <span class="body-meta-label">{{ t('components.logs.detailsModal.expiresAt') }}:</span>
                  {{ formatTime(selectedLogBody.expires_at) }}
                </span>
              </div>
              <section class="detail-section">
                <h3>{{ t('components.logs.detailsModal.requestBody') }}</h3>
                <pre class="body-content">{{ formatJson(selectedLogBody.request_body) || '—' }}</pre>
              </section>
              <section class="detail-section">
                <h3>{{ t('components.logs.detailsModal.responseBody') }}</h3>
                <pre class="body-content">{{ formatJson(selectedLogBody.response_body) || '—' }}</pre>
              </section>
            </template>
          </template>
          <!-- 基本信息 -->
          <template v-else>
          <section class="detail-section">
            <h3>{{ t('components.logs.detailsModal.basic') }}</h3>
            <div class="detail-grid">
              <div class="detail-item">
                <span class="detail-label">Trace ID</span>
                <span class="detail-value">{{ selectedLog.trace_id || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Request ID</span>
                <span class="detail-value">{{ selectedLog.request_id || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Platform</span>
                <span class="detail-value">{{ selectedLog.platform }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Provider</span>
                <span class="detail-value">{{ selectedLog.provider }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Model</span>
                <span class="detail-value">{{ selectedLog.model }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">HTTP Code</span>
                <span :class="['detail-value', httpCodeClass(selectedLog.http_code)]">{{ selectedLog.http_code }}</span>
              </div>
            </div>
          </section>

          <!-- 请求信息 -->
          <section class="detail-section">
            <h3>{{ t('components.logs.detailsModal.request') }}</h3>
            <div class="detail-grid">
              <div class="detail-item">
                <span class="detail-label">Method</span>
                <span class="detail-value">{{ selectedLog.request_method || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Path</span>
                <span class="detail-value">{{ selectedLog.request_path || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">User Agent</span>
                <span class="detail-value" :title="selectedLog.user_agent">{{ formatUserAgent(selectedLog.user_agent) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Client IP</span>
                <span class="detail-value">{{ selectedLog.client_ip || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">User ID</span>
                <span class="detail-value">{{ selectedLog.user_id || '—' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Stream</span>
                <span class="detail-value">{{ selectedLog.is_stream ? 'Yes' : 'No' }}</span>
              </div>
            </div>
          </section>

          <!-- 错误详情 -->
          <section v-if="selectedLog.error_type" class="detail-section">
            <h3>{{ t('components.logs.detailsModal.error') }}</h3>
            <div class="detail-grid">
              <div class="detail-item">
                <span class="detail-label">Error Type</span>
                <span :class="['detail-value', 'error-badge', errorTypeClass(selectedLog.error_type)]">
                  {{ formatErrorType(selectedLog.error_type) }}
                </span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Provider Error Code</span>
                <span class="detail-value">{{ selectedLog.provider_error_code || '—' }}</span>
              </div>
              <div class="detail-item detail-item-full">
                <span class="detail-label">Error Message</span>
                <pre class="detail-value error-message">{{ selectedLog.error_message || '—' }}</pre>
              </div>
            </div>
          </section>

          <!-- Token 用量 -->
          <section class="detail-section">
            <h3>{{ t('components.logs.detailsModal.tokens') }}</h3>
            <div class="detail-grid">
              <div class="detail-item">
                <span class="detail-label">Input Tokens</span>
                <span class="detail-value">{{ formatNumber(selectedLog.input_tokens) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Output Tokens</span>
                <span class="detail-value">{{ formatNumber(selectedLog.output_tokens) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Reasoning Tokens</span>
                <span class="detail-value">{{ formatNumber(selectedLog.reasoning_tokens) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Cache Create</span>
                <span class="detail-value">{{ formatNumber(selectedLog.cache_create_tokens) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Cache Read</span>
                <span class="detail-value">{{ formatNumber(selectedLog.cache_read_tokens) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">Duration</span>
                <span class="detail-value">{{ formatDuration(selectedLog.duration_sec) }}</span>
              </div>
            </div>
          </section>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, watch, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseButton from '../common/BaseButton.vue'
import {
  fetchRequestLogs,
  fetchLogProviders,
  fetchLogStats,
  fetchRequestLogBody,
  type RequestLog,
  type LogStats,
  type LogStatsSeries,
  type RequestLogBody,
} from '../../services/logs'
import {
  Chart,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
} from 'chart.js'
import type { ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'

Chart.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const { t } = useI18n()
const router = useRouter()

const logs = ref<RequestLog[]>([])
const stats = ref<LogStats | null>(null)
const loading = ref(false)
const filters = reactive({ platform: '', provider: '', search: '' })
const page = ref(1)
const PAGE_SIZE = 15
const providerOptions = ref<string[]>([])
const statsSeries = computed<LogStatsSeries[]>(() => stats.value?.series ?? [])
const selectedLog = ref<RequestLog | null>(null)
const selectedLogBody = ref<RequestLogBody | null>(null)
const bodyLoading = ref(false)
const activeDetailTab = ref<'basic' | 'body'>('basic')

const isBrowser = typeof window !== 'undefined' && typeof document !== 'undefined'
const readDarkMode = () => (isBrowser ? document.documentElement.classList.contains('dark') : false)
const isDarkMode = ref(readDarkMode())
let themeObserver: MutationObserver | null = null

const getCssVarValue = (name: string, fallback: string) => {
  if (!isBrowser) return fallback
  const value = getComputedStyle(document.documentElement).getPropertyValue(name)
  return value?.trim() || fallback
}

const syncThemeState = () => {
  isDarkMode.value = readDarkMode()
}

const setupThemeObserver = () => {
  if (!isBrowser || themeObserver) return
  syncThemeState()
  themeObserver = new MutationObserver((mutations) => {
    if (mutations.some((mutation) => mutation.attributeName === 'class')) {
      syncThemeState()
    }
  })
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class'],
  })
}

const teardownThemeObserver = () => {
  if (!themeObserver) return
  themeObserver.disconnect()
  themeObserver = null
}

const parseLogDate = (value?: string) => {
  if (!value) return null
  const normalize = value.replace(' ', 'T')
  const attempts = [value, `${normalize}`, `${normalize}Z`]
  for (const candidate of attempts) {
    const parsed = new Date(candidate)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  const match = value.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2}) ([+-]\d{4}) UTC$/)
  if (match) {
    const [, day, time, zone] = match
    const zoneFormatted = `${zone.slice(0, 3)}:${zone.slice(3)}`
    const parsed = new Date(`${day}T${time}${zoneFormatted}`)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
    }
  }
  return null
}

const chartData = computed(() => {
  const series = statsSeries.value
  return {
    labels: series.map((item) => formatSeriesLabel(item.day)),
    datasets: [
      {
        label: t('components.logs.tokenLabels.cost'),
        data: series.map((item) => Number(((item.total_cost ?? 0)).toFixed(4))),
        borderColor: '#f97316',
        backgroundColor: 'rgba(249, 115, 22, 0.2)',
        tension: 0.3,
        fill: false,
        yAxisID: 'yCost',
      },
      {
        label: t('components.logs.tokenLabels.input'),
        data: series.map((item) => item.input_tokens ?? 0),
        borderColor: '#34d399',
        backgroundColor: 'rgba(52, 211, 153, 0.25)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.output'),
        data: series.map((item) => item.output_tokens ?? 0),
        borderColor: '#60a5fa',
        backgroundColor: 'rgba(96, 165, 250, 0.2)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.reasoning'),
        data: series.map((item) => item.reasoning_tokens ?? 0),
        borderColor: '#f472b6',
        backgroundColor: 'rgba(244, 114, 182, 0.2)',
        tension: 0.35,
        fill: true,
      },
      {
        label: t('components.logs.tokenLabels.cacheWrite'),
        data: series.map((item) => item.cache_create_tokens ?? 0),
        borderColor: '#fbbf24',
        backgroundColor: 'rgba(251, 191, 36, 0.2)',
        tension: 0.35,
        fill: false,
      },
      {
        label: t('components.logs.tokenLabels.cacheRead'),
        data: series.map((item) => item.cache_read_tokens ?? 0),
        borderColor: '#38bdf8',
        backgroundColor: 'rgba(56, 189, 248, 0.15)',
        tension: 0.35,
        fill: false,
      },
    ],
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => {
  const legendColor = getCssVarValue('--mac-text', isDarkMode.value ? '#f8fafc' : '#0f172a')
  const axisColor = getCssVarValue(
    '--mac-text-secondary',
    isDarkMode.value ? '#cbd5f5' : '#94a3b8',
  )
  const axisStrongColor = getCssVarValue('--mac-text', isDarkMode.value ? '#e2e8f0' : '#475569')
  const gridColor = isDarkMode.value ? 'rgba(148, 163, 184, 0.35)' : 'rgba(148, 163, 184, 0.2)'

  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: 'index',
      intersect: false,
    },
    plugins: {
      legend: {
        labels: {
          color: legendColor,
          font: {
            size: 12,
            weight: 500,
          },
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: axisColor },
      },
      y: {
        beginAtZero: true,
        ticks: { color: axisColor },
        grid: { color: gridColor },
      },
      yCost: {
        position: 'right',
        beginAtZero: true,
        grid: { drawOnChartArea: false },
        ticks: {
          color: axisStrongColor,
          callback: (value: string | number) => {
            const numeric = typeof value === 'number' ? value : Number(value)
            if (Number.isNaN(numeric)) return '$0'
            if (numeric >= 1) return `$${numeric.toFixed(2)}`
            return `$${numeric.toFixed(4)}`
          },
        },
      },
    },
  }
})
const formatSeriesLabel = (value?: string) => {
  if (!value) return ''
  const parsed = parseLogDate(value)
  if (parsed) {
    return `${padHour(parsed.getHours())}:00`
  }
  const match = value.match(/(\d{2}):(\d{2})/)
  if (match) {
    return `${match[1]}:${match[2]}`
  }
  return value
}

const REFRESH_INTERVAL = 30
const countdown = ref(REFRESH_INTERVAL)
let timer: number | undefined

const resetTimer = () => {
  countdown.value = REFRESH_INTERVAL
}

const startCountdown = () => {
  stopCountdown()
  timer = window.setInterval(() => {
    if (countdown.value <= 1) {
      countdown.value = REFRESH_INTERVAL
      void loadDashboard()
    } else {
      countdown.value -= 1
    }
  }, 1000)
}

const stopCountdown = () => {
  if (timer) {
    clearInterval(timer)
    timer = undefined
  }
}

const loadLogs = async () => {
  loading.value = true
  try {
    const data = await fetchRequestLogs({
      platform: filters.platform,
      provider: filters.provider,
      limit: 200,
    })
    logs.value = data ?? []
    page.value = Math.min(page.value, totalPages.value)
  } catch (error) {
    console.error('failed to load request logs', error)
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const data = await fetchLogStats(filters.platform)
    stats.value = data ?? null
  } catch (error) {
    console.error('failed to load log stats', error)
  }
}

const loadDashboard = async () => {
  await Promise.all([loadLogs(), loadStats()])
}

const filteredLogs = computed(() => {
  if (!filters.search) return logs.value

  const searchLower = filters.search.toLowerCase()
  return logs.value.filter(log => {
    return (
      log.trace_id?.toLowerCase().includes(searchLower) ||
      log.request_id?.toLowerCase().includes(searchLower) ||
      log.error_message?.toLowerCase().includes(searchLower) ||
      log.error_type?.toLowerCase().includes(searchLower) ||
      log.model?.toLowerCase().includes(searchLower) ||
      log.provider?.toLowerCase().includes(searchLower)
    )
  })
})

const pagedLogs = computed(() => {
  const start = (page.value - 1) * PAGE_SIZE
  return filteredLogs.value.slice(start, start + PAGE_SIZE)
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredLogs.value.length / PAGE_SIZE)))

const applyFilters = async () => {
  page.value = 1
  await loadDashboard()
  resetTimer()
}

const refreshLogs = () => {
  void loadDashboard()
}

const manualRefresh = () => {
  resetTimer()
  void loadDashboard()
}

const nextPage = () => {
  if (page.value < totalPages.value) {
    page.value += 1
  }
}

const prevPage = () => {
  if (page.value > 1) {
    page.value -= 1
  }
}

const backToHome = () => {
  router.push('/')
}

const padHour = (num: number) => num.toString().padStart(2, '0')

const formatTime = (value?: string) => {
  const date = parseLogDate(value)
  if (!date) return value || '—'
  return `${date.getFullYear()}-${padHour(date.getMonth() + 1)}-${padHour(date.getDate())} ${padHour(date.getHours())}:${padHour(date.getMinutes())}:${padHour(date.getSeconds())}`
}

const formatStream = (value?: boolean | number) => {
  const isOn = value === true || value === 1
  return isOn ? t('components.logs.streamOn') : t('components.logs.streamOff')
}

const formatDuration = (value?: number) => {
  if (!value || Number.isNaN(value)) return '—'
  return `${value.toFixed(2)}s`
}

const httpCodeClass = (code: number) => {
  if (code >= 500) return 'http-server-error'
  if (code >= 400) return 'http-client-error'
  if (code >= 300) return 'http-redirect'
  if (code >= 200) return 'http-success'
  return 'http-info'
}

const durationColor = (value?: number) => {
  if (!value || Number.isNaN(value)) return 'neutral'
  if (value < 2) return 'fast'
  if (value < 5) return 'medium'
  return 'slow'
}

const formatNumber = (value?: number) => {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString()
}

const formatCurrency = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '$0.0000'
  }
  if (value >= 1) {
    return `$${value.toFixed(2)}`
  }
  if (value >= 0.01) {
    return `$${value.toFixed(3)}`
  }
  return `$${value.toFixed(4)}`
}

// Ailurus PaaS 增强功能
const formatTraceId = (traceId?: string) => {
  if (!traceId) return '—'
  // 显示前 8 位
  return traceId.slice(0, 8) + '...'
}

const copyTraceId = async (traceId: string) => {
  try {
    await navigator.clipboard.writeText(traceId)
    console.log(`Trace ID copied: ${traceId}`)
    // 可以添加 toast 通知
  } catch (err) {
    console.error('Failed to copy trace ID:', err)
  }
}

const formatErrorType = (errorType?: string) => {
  if (!errorType) return '—'
  return t(`components.logs.errorTypes.${errorType}`)
}

const errorTypeClass = (errorType?: string) => {
  if (!errorType) return ''
  const typeMap: Record<string, string> = {
    'network_error': 'error-network',
    'auth_error': 'error-auth',
    'rate_limit': 'error-rate',
    'client_error': 'error-client',
    'server_error': 'error-server',
    'unknown_error': 'error-unknown'
  }
  return typeMap[errorType] || 'error-unknown'
}

const showDetails = (log: RequestLog) => {
  selectedLog.value = log
  selectedLogBody.value = null
  activeDetailTab.value = 'basic'
}

const closeDetails = () => {
  selectedLog.value = null
  selectedLogBody.value = null
  activeDetailTab.value = 'basic'
}

const switchDetailTab = async (tab: 'basic' | 'body') => {
  activeDetailTab.value = tab
  if (tab === 'body' && !selectedLogBody.value && selectedLog.value?.trace_id) {
    await loadBodyData(selectedLog.value.trace_id)
  }
}

const loadBodyData = async (traceId: string) => {
  if (!traceId) return
  bodyLoading.value = true
  try {
    const data = await fetchRequestLogBody(traceId)
    selectedLogBody.value = data
  } catch (error) {
    console.error('failed to load body data', error)
    selectedLogBody.value = null
  } finally {
    bodyLoading.value = false
  }
}

const formatBodySize = (bytes?: number) => {
  if (!bytes || bytes <= 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

const formatJson = (str?: string) => {
  if (!str) return ''
  try {
    const parsed = JSON.parse(str)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return str
  }
}

const formatUserAgent = (ua?: string) => {
  if (!ua) return '—'
  // 提取关键信息（简化显示）
  if (ua.includes('Claude')) return 'Claude Code'
  if (ua.includes('Codex')) return 'Codex CLI'
  if (ua.length > 30) return ua.slice(0, 27) + '...'
  return ua
}

const startOfTodayLocal = () => {
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  return now
}

const statsCards = computed(() => {
  const data = stats.value
  const summaryDate = summaryDateLabel.value
  const totalTokens =
    (data?.input_tokens ?? 0) + (data?.output_tokens ?? 0) + (data?.reasoning_tokens ?? 0)
  return [
    {
      key: 'requests',
      label: t('components.logs.summary.total'),
      hint: t('components.logs.summary.requests'),
      value: data ? formatNumber(data.total_requests) : '—',
    },
    {
      key: 'tokens',
      label: t('components.logs.summary.tokens'),
      hint: t('components.logs.summary.tokenHint'),
      value: data ? formatNumber(totalTokens) : '—',
    },
    {
      key: 'cacheReads',
      label: t('components.logs.summary.cache'),
      hint: t('components.logs.summary.cacheHint'),
      value: data ? formatNumber(data.cache_read_tokens) : '—',
    },
    {
      key: 'cost',
      label: t('components.logs.tokenLabels.cost'),
      hint: summaryDate ? t('components.logs.summary.todayScope', { date: summaryDate }) : '',
      value: formatCurrency(data?.cost_total ?? 0),
    },
  ]
})

const summaryDateLabel = computed(() => {
  const firstBucket = statsSeries.value.find((item) => item.day)
  const parsed = parseLogDate(firstBucket?.day ?? '')
  const date = parsed ?? startOfTodayLocal()
  return `${date.getFullYear()}-${padHour(date.getMonth() + 1)}-${padHour(date.getDate())}`
})

const loadProviderOptions = async () => {
  try {
    const list = await fetchLogProviders(filters.platform)
    providerOptions.value = list ?? []
    if (filters.provider && !providerOptions.value.includes(filters.provider)) {
      filters.provider = ''
    }
  } catch (error) {
    console.error('failed to load provider options', error)
  }
}

watch(
  () => filters.platform,
  async () => {
    await loadProviderOptions()
  },
)

onMounted(async () => {
  await Promise.all([loadDashboard(), loadProviderOptions()])
  startCountdown()
  setupThemeObserver()
})

onUnmounted(() => {
  stopCountdown()
  teardownThemeObserver()
})
</script>

<style scoped>
.logs-summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.summary-meta {
  grid-column: 1 / -1;
  font-size: 0.85rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
}

.summary-card {
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 16px;
  padding: 1rem 1.25rem;
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.1), rgba(15, 23, 42, 0));
  backdrop-filter: blur(6px);
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.summary-card__label {
  font-size: 0.85rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #475569;
}

.summary-card__value {
  font-size: 1.85rem;
  font-weight: 600;
  color: #0f172a;
}

.summary-card__hint {
  font-size: 0.85rem;
  color: #94a3b8;
}

html.dark .summary-card {
  border-color: rgba(255, 255, 255, 0.12);
  background: radial-gradient(circle at top, rgba(148, 163, 184, 0.2), rgba(15, 23, 42, 0.35));
}

html.dark .summary-card__label {
  color: rgba(248, 250, 252, 0.75);
}

html.dark .summary-card__value {
  color: rgba(248, 250, 252, 0.95);
}

html.dark .summary-card__hint {
  color: rgba(186, 194, 210, 0.8);
}

/* Ailurus PaaS 增强样式 */
.filter-field-search {
  flex: 2;
  min-width: 250px;
}

.mac-input {
  width: 100%;
  padding: 0.5rem 0.75rem;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font-size: 0.875rem;
  background: #fff;
  transition: border-color 0.2s;
}

.mac-input:focus {
  outline: none;
  border-color: #3b82f6;
}

html.dark .mac-input {
  background: rgba(15, 23, 42, 0.5);
  border-color: rgba(148, 163, 184, 0.2);
  color: #f1f5f9;
}

.error-row {
  background: rgba(239, 68, 68, 0.05);
}

html.dark .error-row {
  background: rgba(239, 68, 68, 0.1);
}

.trace-id-cell {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 0.75rem;
}

.trace-id-btn {
  padding: 0.25rem 0.5rem;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  font-family: inherit;
}

.trace-id-btn:hover {
  background: #e2e8f0;
  border-color: #cbd5e1;
}

html.dark .trace-id-btn {
  background: rgba(51, 65, 85, 0.5);
  border-color: rgba(100, 116, 139, 0.3);
  color: #cbd5e1;
}

html.dark .trace-id-btn:hover {
  background: rgba(51, 65, 85, 0.8);
}

.error-badge,
.success-badge {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
}

.success-badge {
  background: #dcfce7;
  color: #166534;
}

html.dark .success-badge {
  background: rgba(34, 197, 94, 0.2);
  color: #86efac;
}

.error-network {
  background: #fef2f2;
  color: #991b1b;
}

.error-auth {
  background: #fef3c7;
  color: #92400e;
}

.error-rate {
  background: #fce7f3;
  color: #9f1239;
}

.error-client {
  background: #fef3c7;
  color: #713f12;
}

.error-server {
  background: #fecaca;
  color: #7f1d1d;
}

.error-unknown {
  background: #e5e7eb;
  color: #374151;
}

html.dark .error-network {
  background: rgba(239, 68, 68, 0.2);
  color: #fca5a5;
}

html.dark .error-auth {
  background: rgba(234, 179, 8, 0.2);
  color: #fde047;
}

html.dark .error-rate {
  background: rgba(236, 72, 153, 0.2);
  color: #f9a8d4;
}

html.dark .error-client {
  background: rgba(217, 119, 6, 0.2);
  color: #fdba74;
}

html.dark .error-server {
  background: rgba(220, 38, 38, 0.2);
  color: #f87171;
}

.model-cell {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.actions-cell {
  text-align: center;
}

/* 详情弹窗样式 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 2rem;
}

.modal-content {
  background: #fff;
  border-radius: 12px;
  max-width: 800px;
  width: 100%;
  max-height: 90vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

html.dark .modal-content {
  background: #1e293b;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.modal-header {
  padding: 1.5rem;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

html.dark .modal-header {
  border-bottom-color: rgba(148, 163, 184, 0.2);
}

.modal-header h2 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
  color: #0f172a;
}

html.dark .modal-header h2 {
  color: #f1f5f9;
}

.modal-close {
  background: none;
  border: none;
  font-size: 2rem;
  line-height: 1;
  cursor: pointer;
  color: #64748b;
  padding: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.2s;
}

.modal-close:hover {
  background: #f1f5f9;
  color: #0f172a;
}

html.dark .modal-close:hover {
  background: rgba(51, 65, 85, 0.5);
  color: #f1f5f9;
}

.modal-body {
  padding: 1.5rem;
  overflow-y: auto;
}

.detail-section {
  margin-bottom: 2rem;
}

.detail-section:last-child {
  margin-bottom: 0;
}

.detail-section h3 {
  margin: 0 0 1rem 0;
  font-size: 1rem;
  font-weight: 600;
  color: #475569;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

html.dark .detail-section h3 {
  color: #94a3b8;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1rem;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.detail-item-full {
  grid-column: 1 / -1;
}

.detail-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

html.dark .detail-label {
  color: #94a3b8;
}

.detail-value {
  font-size: 0.875rem;
  color: #0f172a;
  word-break: break-word;
}

html.dark .detail-value {
  color: #e2e8f0;
}

.error-message {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 0.75rem;
  margin: 0;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 0.75rem;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-x: auto;
}

html.dark .error-message {
  background: rgba(15, 23, 42, 0.5);
  border-color: rgba(148, 163, 184, 0.2);
}

@media (max-width: 768px) {
  .logs-summary {
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }

  .filter-fields {
    flex-direction: column;
  }

  .filter-field-search {
    width: 100%;
  }
}

/* 详情弹窗标签页 */
.modal-tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid #e2e8f0;
  padding: 0 1.5rem;
}

html.dark .modal-tabs {
  border-bottom-color: rgba(148, 163, 184, 0.2);
}

.modal-tab {
  padding: 0.75rem 1.25rem;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  color: #64748b;
  transition: all 0.2s;
}

.modal-tab:hover {
  color: #0f172a;
}

html.dark .modal-tab:hover {
  color: #e2e8f0;
}

.modal-tab.active {
  color: #3b82f6;
  border-bottom-color: #3b82f6;
}

html.dark .modal-tab.active {
  color: #60a5fa;
  border-bottom-color: #60a5fa;
}

/* Body 标签页内容 */
.body-loading,
.body-empty {
  padding: 3rem 1rem;
  text-align: center;
  color: #64748b;
  font-size: 0.875rem;
}

html.dark .body-loading,
html.dark .body-empty {
  color: #94a3b8;
}

.body-meta {
  display: flex;
  gap: 2rem;
  margin-bottom: 1.5rem;
  padding: 0.75rem 1rem;
  background: #f8fafc;
  border-radius: 6px;
}

html.dark .body-meta {
  background: rgba(15, 23, 42, 0.5);
}

.body-meta-item {
  font-size: 0.8125rem;
  color: #475569;
}

html.dark .body-meta-item {
  color: #cbd5e1;
}

.body-meta-label {
  color: #64748b;
  margin-right: 0.5rem;
}

html.dark .body-meta-label {
  color: #94a3b8;
}

.body-content {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 1rem;
  margin: 0;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 0.75rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
  max-height: 300px;
  color: #0f172a;
}

html.dark .body-content {
  background: rgba(15, 23, 42, 0.5);
  border-color: rgba(148, 163, 184, 0.2);
  color: #e2e8f0;
}
</style>
