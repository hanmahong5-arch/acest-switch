<template>
  <div class="circuit-breaker-status">
    <div class="header">
      <h3>{{ $t('circuitBreaker.title') }}</h3>
      <n-button
        size="small"
        :loading="loading"
        @click="refreshMetrics"
      >
        <template #icon>
          <n-icon><RefreshOutline /></n-icon>
        </template>
        {{ $t('common.refresh') }}
      </n-button>
    </div>

    <!-- Loading State -->
    <div v-if="loading && metrics.length === 0" class="loading-state">
      <n-spin size="large" />
      <p>{{ $t('circuitBreaker.loading') }}</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="error-state">
      <n-icon size="48" color="#d03050"><WarningOutline /></n-icon>
      <p class="error-message">{{ error }}</p>
      <n-button @click="refreshMetrics">{{ $t('common.retry') }}</n-button>
    </div>

    <!-- Empty State -->
    <div v-else-if="metrics.length === 0" class="empty-state">
      <n-icon size="48" color="#999"><InformationCircleOutline /></n-icon>
      <p>{{ $t('circuitBreaker.noData') }}</p>
    </div>

    <!-- Metrics Display -->
    <div v-else class="metrics-grid">
      <n-card
        v-for="metric in metrics"
        :key="metric.provider_id"
        class="provider-metric-card"
        :class="getCardClass(metric.state)"
      >
        <!-- Header -->
        <div class="metric-header">
          <div class="provider-info">
            <span class="provider-name">{{ metric.provider_name }}</span>
            <n-tag
              :type="getStatusTagType(metric.state)"
              size="small"
              :bordered="false"
            >
              {{ getStatusLabel(metric.state) }}
            </n-tag>
          </div>
          <n-button
            v-if="metric.state === 'open'"
            size="small"
            type="warning"
            @click="handleManualReset(metric.provider_id)"
            :loading="resetting === metric.provider_id"
          >
            {{ $t('circuitBreaker.manualReset') }}
          </n-button>
        </div>

        <!-- Metrics -->
        <div class="metric-stats">
          <div class="stat-item">
            <span class="stat-label">{{ $t('circuitBreaker.successRate') }}</span>
            <span class="stat-value" :class="getSuccessRateClass(metric.success_rate)">
              {{ metric.success_rate.toFixed(1) }}%
            </span>
          </div>

          <div class="stat-item">
            <span class="stat-label">{{ $t('circuitBreaker.totalRequests') }}</span>
            <span class="stat-value">{{ metric.total_requests }}</span>
          </div>

          <div class="stat-item">
            <span class="stat-label">{{ $t('circuitBreaker.failures') }}</span>
            <span class="stat-value" :class="{ 'error-text': metric.consecutive_fails > 0 }">
              {{ metric.consecutive_fails }}
            </span>
          </div>
        </div>

        <!-- Timeline -->
        <div v-if="metric.last_failure_at || metric.circuit_opened_at" class="metric-timeline">
          <div v-if="metric.last_failure_at" class="timeline-item">
            <n-icon size="14"><TimeOutline /></n-icon>
            <span>{{ $t('circuitBreaker.lastFailure') }}: {{ formatTime(metric.last_failure_at) }}</span>
          </div>
          <div v-if="metric.circuit_opened_at" class="timeline-item">
            <n-icon size="14"><AlertCircleOutline /></n-icon>
            <span>{{ $t('circuitBreaker.circuitOpened') }}: {{ formatTime(metric.circuit_opened_at) }}</span>
          </div>
        </div>

        <!-- Progress Bar -->
        <div class="metric-progress">
          <div class="progress-label">
            <span>{{ $t('circuitBreaker.health') }}</span>
            <span>{{ metric.success_rate.toFixed(0) }}%</span>
          </div>
          <n-progress
            type="line"
            :percentage="metric.success_rate"
            :status="getProgressStatus(metric.success_rate)"
            :show-indicator="false"
            :height="8"
          />
        </div>
      </n-card>
    </div>

    <!-- Summary Stats -->
    <div v-if="metrics.length > 0" class="summary-stats">
      <n-statistic label="Total Providers" :value="metrics.length" />
      <n-statistic label="Healthy" :value="healthyCount" />
      <n-statistic label="Open Circuits" :value="openCount" />
      <n-statistic label="Half-Open" :value="halfOpenCount" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCard, NButton, NIcon, NTag, NProgress, NStatistic, NSpin } from 'naive-ui'
import {
  RefreshOutline,
  TimeOutline,
  AlertCircleOutline,
  InformationCircleOutline,
  WarningOutline
} from '@vicons/ionicons5'
import { circuitBreakerApi } from '@/services/circuit-breaker'
import { formatDistanceToNow } from 'date-fns'

const { t } = useI18n()

interface CircuitBreakerMetric {
  provider_id: number
  provider_name: string
  state: string
  consecutive_fails: number
  total_requests: number
  total_failures: number
  total_successes: number
  success_rate: number
  last_failure_at?: string
  circuit_opened_at?: string
}

const metrics = ref<CircuitBreakerMetric[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const resetting = ref<number | null>(null)

let refreshInterval: number | null = null

// Computed stats
const healthyCount = computed(() =>
  metrics.value.filter(m => m.state === 'closed').length
)

const openCount = computed(() =>
  metrics.value.filter(m => m.state === 'open').length
)

const halfOpenCount = computed(() =>
  metrics.value.filter(m => m.state === 'half_open').length
)

// Fetch metrics
async function fetchMetrics() {
  try {
    loading.value = true
    error.value = null
    const data = await circuitBreakerApi.getMetrics()
    metrics.value = data
  } catch (err: any) {
    error.value = err.message || t('circuitBreaker.loadError')
  } finally {
    loading.value = false
  }
}

// Refresh metrics
async function refreshMetrics() {
  await fetchMetrics()
}

// Manual reset circuit breaker
async function handleManualReset(providerId: number) {
  try {
    resetting.value = providerId
    await circuitBreakerApi.resetCircuitBreaker(providerId)
    await fetchMetrics() // Refresh metrics after reset
  } catch (err: any) {
    error.value = err.message || t('circuitBreaker.resetError')
  } finally {
    resetting.value = null
  }
}

// Get status tag type
function getStatusTagType(state: string): 'success' | 'warning' | 'error' | 'info' {
  switch (state) {
    case 'closed':
      return 'success'
    case 'open':
      return 'error'
    case 'half_open':
      return 'warning'
    default:
      return 'info'
  }
}

// Get status label
function getStatusLabel(state: string): string {
  return t(`circuitBreaker.states.${state}`)
}

// Get card class
function getCardClass(state: string): string {
  return `state-${state}`
}

// Get success rate class
function getSuccessRateClass(rate: number): string {
  if (rate >= 95) return 'success-text'
  if (rate >= 80) return 'warning-text'
  return 'error-text'
}

// Get progress status
function getProgressStatus(rate: number): 'success' | 'warning' | 'error' {
  if (rate >= 95) return 'success'
  if (rate >= 80) return 'warning'
  return 'error'
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
  fetchMetrics()
  // Auto-refresh every 10 seconds
  refreshInterval = window.setInterval(fetchMetrics, 10000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<style scoped lang="scss">
.circuit-breaker-status {
  padding: 20px;

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;

    h3 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
    }
  }

  .loading-state,
  .error-state,
  .empty-state {
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

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  .provider-metric-card {
    border-left: 4px solid transparent;
    transition: all 0.3s ease;

    &.state-closed {
      border-left-color: #18a058;
    }

    &.state-open {
      border-left-color: #d03050;
    }

    &.state-half_open {
      border-left-color: #f0a020;
    }

    &:hover {
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
    }
  }

  .metric-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;

    .provider-info {
      display: flex;
      align-items: center;
      gap: 8px;

      .provider-name {
        font-weight: 600;
        font-size: 15px;
      }
    }
  }

  .metric-stats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
    margin-bottom: 16px;

    .stat-item {
      display: flex;
      flex-direction: column;
      gap: 4px;

      .stat-label {
        font-size: 12px;
        color: #666;
      }

      .stat-value {
        font-size: 18px;
        font-weight: 600;

        &.success-text {
          color: #18a058;
        }

        &.warning-text {
          color: #f0a020;
        }

        &.error-text {
          color: #d03050;
        }
      }
    }
  }

  .metric-timeline {
    margin-bottom: 16px;
    padding: 12px;
    background: #f5f5f5;
    border-radius: 6px;

    .timeline-item {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 12px;
      color: #666;

      &:not(:last-child) {
        margin-bottom: 6px;
      }
    }
  }

  .metric-progress {
    .progress-label {
      display: flex;
      justify-content: space-between;
      margin-bottom: 8px;
      font-size: 12px;
      color: #666;
    }
  }

  .summary-stats {
    display: flex;
    gap: 24px;
    padding: 20px;
    background: #f5f5f5;
    border-radius: 8px;
  }
}
</style>
