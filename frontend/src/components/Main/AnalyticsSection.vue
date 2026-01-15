<template>
  <section class="analytics-section">
    <div class="analytics-header">
      <h2 class="analytics-title">{{ t('components.main.analytics.title') }}</h2>
      <button
        class="ghost-icon"
        :class="{ 'spin-animation': loading }"
        :data-tooltip="t('components.main.analytics.refresh')"
        :disabled="loading"
        @click="refreshAnalytics"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <div class="analytics-grid">
      <!-- 成本构成 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon cost-icon">
            <svg viewBox="0 0 24 24" width="18" height="18">
              <path
                d="M12 1v22M17 5H9.5a3.5 3.5 0 000 7h5a3.5 3.5 0 010 7H6"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.costBreakdown.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!costAnalysis" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content">
          <div class="breakdown-bar">
            <div
              class="bar-segment input"
              :style="{ width: `${(costAnalysis.input_cost_ratio * 100).toFixed(1)}%` }"
            />
            <div
              class="bar-segment output"
              :style="{ width: `${(costAnalysis.output_cost_ratio * 100).toFixed(1)}%` }"
            />
            <div
              class="bar-segment cache-create"
              :style="{ width: `${(costAnalysis.cache_create_cost_ratio * 100).toFixed(1)}%` }"
            />
            <div
              class="bar-segment cache-read"
              :style="{ width: `${(costAnalysis.cache_read_cost_ratio * 100).toFixed(1)}%` }"
            />
          </div>
          <ul class="breakdown-legend">
            <li>
              <span class="legend-dot input" />
              <span class="legend-label">{{ t('components.main.analytics.costBreakdown.input') }}</span>
              <span class="legend-value">{{ formatPercent(costAnalysis.input_cost_ratio) }}</span>
            </li>
            <li>
              <span class="legend-dot output" />
              <span class="legend-label">{{ t('components.main.analytics.costBreakdown.output') }}</span>
              <span class="legend-value">{{ formatPercent(costAnalysis.output_cost_ratio) }}</span>
            </li>
            <li>
              <span class="legend-dot cache-create" />
              <span class="legend-label">{{ t('components.main.analytics.costBreakdown.cacheCreate') }}</span>
              <span class="legend-value">{{ formatPercent(costAnalysis.cache_create_cost_ratio) }}</span>
            </li>
            <li>
              <span class="legend-dot cache-read" />
              <span class="legend-label">{{ t('components.main.analytics.costBreakdown.cacheRead') }}</span>
              <span class="legend-value">{{ formatPercent(costAnalysis.cache_read_cost_ratio) }}</span>
            </li>
          </ul>
        </div>
      </div>

      <!-- 缓存节省 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon cache-icon">
            <svg viewBox="0 0 24 24" width="18" height="18">
              <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z" fill="none" stroke="currentColor" stroke-width="2"/>
              <polyline points="3.27 6.96 12 12.01 20.73 6.96" fill="none" stroke="currentColor" stroke-width="2"/>
              <line x1="12" y1="22.08" x2="12" y2="12" stroke="currentColor" stroke-width="2"/>
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.cacheSavings.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!costAnalysis" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content cache-savings-content">
          <div class="savings-value">
            <span class="savings-amount">{{ formatCurrency(costAnalysis.cache_saved_cost) }}</span>
            <span class="savings-label">{{ t('components.main.analytics.cacheSavings.saved') }}</span>
          </div>
          <div class="savings-percent" :class="savingsClass">
            <span class="percent-value">{{ formatPercent(costAnalysis.cache_saving_percent) }}</span>
            <span class="percent-label">{{ t('components.main.analytics.cacheSavings.rate') }}</span>
          </div>
          <div class="savings-tokens">
            <span class="tokens-value">{{ formatNumber(costAnalysis.cache_read_tokens) }}</span>
            <span class="tokens-label">{{ t('components.main.analytics.cacheSavings.tokens') }}</span>
          </div>
        </div>
      </div>

      <!-- 成本趋势 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon trend-icon" :class="trendIconClass">
            <svg v-if="costAnalysis?.trend_direction === 'up'" viewBox="0 0 24 24" width="18" height="18">
              <polyline points="23 6 13.5 15.5 8.5 10.5 1 18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <polyline points="17 6 23 6 23 12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else-if="costAnalysis?.trend_direction === 'down'" viewBox="0 0 24 24" width="18" height="18">
              <polyline points="23 18 13.5 8.5 8.5 13.5 1 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <polyline points="17 18 23 18 23 12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg v-else viewBox="0 0 24 24" width="18" height="18">
              <line x1="5" y1="12" x2="19" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.costTrend.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!costAnalysis" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content trend-content">
          <div class="trend-summary">
            <div class="trend-avg">
              <span class="avg-value">{{ formatCurrency(costAnalysis.daily_avg_cost) }}</span>
              <span class="avg-label">{{ t('components.main.analytics.costTrend.dailyAvg') }}</span>
            </div>
            <div class="trend-change" :class="trendClass">
              <span class="change-value">{{ formatTrendPercent(costAnalysis.trend_percentage) }}</span>
              <span class="change-label">{{ t('components.main.analytics.costTrend.change') }}</span>
            </div>
          </div>
          <div class="mini-chart">
            <div
              v-for="(point, idx) in costAnalysis.cost_trend"
              :key="idx"
              class="chart-bar"
              :style="{ height: chartBarHeight(point.total_cost) }"
              :data-tooltip="`${point.day}: ${formatCurrency(point.total_cost)}`"
            />
          </div>
        </div>
      </div>

      <!-- 响应时间 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon perf-icon">
            <svg viewBox="0 0 24 24" width="18" height="18">
              <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2" />
              <polyline points="12 6 12 12 16 14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.responseTime.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!performanceAnalysis" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content">
          <div class="percentile-grid">
            <div class="percentile-item">
              <span class="percentile-label">P50</span>
              <span class="percentile-value" :class="durationClass(performanceAnalysis.duration_p50)">
                {{ formatDuration(performanceAnalysis.duration_p50) }}
              </span>
            </div>
            <div class="percentile-item">
              <span class="percentile-label">P95</span>
              <span class="percentile-value" :class="durationClass(performanceAnalysis.duration_p95)">
                {{ formatDuration(performanceAnalysis.duration_p95) }}
              </span>
            </div>
            <div class="percentile-item">
              <span class="percentile-label">P99</span>
              <span class="percentile-value" :class="durationClass(performanceAnalysis.duration_p99)">
                {{ formatDuration(performanceAnalysis.duration_p99) }}
              </span>
            </div>
          </div>
          <div class="stats-row">
            <span class="stat-item">{{ t('components.main.analytics.responseTime.avg') }}: {{ formatDuration(performanceAnalysis.duration_avg) }}</span>
          </div>
        </div>
      </div>

      <!-- 错误分布 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon error-icon">
            <svg viewBox="0 0 24 24" width="18" height="18">
              <circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2"/>
              <line x1="12" y1="8" x2="12" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              <line x1="12" y1="16" x2="12.01" y2="16" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.errorDistribution.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!performanceAnalysis" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content">
          <div class="error-summary">
            <span class="error-total">{{ performanceAnalysis.total_errors }}</span>
            <span class="error-label">{{ t('components.main.analytics.errorDistribution.total') }}</span>
            <span class="error-rate" :class="errorRateClass">{{ formatPercent(performanceAnalysis.error_rate) }}</span>
          </div>
          <ul class="error-list" v-if="Object.keys(performanceAnalysis.error_distribution || {}).length > 0">
            <li v-for="(count, type) in performanceAnalysis.error_distribution" :key="type">
              <span class="error-type">{{ formatErrorType(type as string) }}</span>
              <span class="error-count">{{ count }}</span>
            </li>
          </ul>
          <div v-else class="no-errors">{{ t('components.main.analytics.errorDistribution.noErrors') }}</div>
        </div>
      </div>

      <!-- 供应商可靠性 -->
      <div class="analytics-card">
        <div class="card-header">
          <span class="card-icon reliability-icon">
            <svg viewBox="0 0 24 24" width="18" height="18">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </span>
          <h3 class="card-title">{{ t('components.main.analytics.providerReliability.title') }}</h3>
        </div>
        <div v-if="loading" class="card-loading">{{ t('components.main.analytics.loading') }}</div>
        <div v-else-if="!performanceAnalysis || performanceAnalysis.provider_reliability.length === 0" class="card-empty">{{ t('components.main.analytics.noData') }}</div>
        <div v-else class="card-content">
          <ul class="reliability-list">
            <li v-for="provider in performanceAnalysis.provider_reliability.slice(0, 4)" :key="provider.provider">
              <div class="provider-info">
                <span class="provider-name">{{ provider.provider }}</span>
                <span class="provider-rate" :class="reliabilityClass(provider.success_rate)">
                  {{ formatPercent(provider.success_rate) }}
                </span>
              </div>
              <div class="provider-bar">
                <div
                  class="bar-fill"
                  :class="reliabilityClass(provider.success_rate)"
                  :style="{ width: `${provider.success_rate * 100}%` }"
                />
              </div>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  fetchCostAnalysis,
  fetchPerformanceAnalysis,
  type CostAnalysis,
  type PerformanceAnalysis,
} from '../../services/logs'
import { showToast } from '../../utils/toast'
import { normalizeError } from '../../types/error'

const { t } = useI18n()

const loading = ref(false)
const costAnalysis = ref<CostAnalysis | null>(null)
const performanceAnalysis = ref<PerformanceAnalysis | null>(null)

const loadAnalytics = async () => {
  if (loading.value) return
  loading.value = true
  try {
    const [cost, perf] = await Promise.all([
      fetchCostAnalysis('', 7),
      fetchPerformanceAnalysis('', 1),
    ])
    costAnalysis.value = cost
    performanceAnalysis.value = perf
  } catch (err) {
    const appError = normalizeError(err, {
      component: 'AnalyticsSection',
      action: 'loadAnalytics',
    })
    console.error('Failed to load analytics', appError)
    showToast(appError.message, 'error', {
      details: appError.details,
    })
  } finally {
    loading.value = false
  }
}

const refreshAnalytics = () => {
  void loadAnalytics()
}

onMounted(() => {
  void loadAnalytics()
})

// 格式化函数
const formatPercent = (value: number) => {
  if (!value || Number.isNaN(value)) return '0%'
  return `${(value * 100).toFixed(1)}%`
}

const formatCurrency = (value: number) => {
  if (!value || Number.isNaN(value)) return '$0.00'
  return `$${value.toFixed(4)}`
}

const formatNumber = (value: number) => {
  if (!value || Number.isNaN(value)) return '0'
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return value.toString()
}

const formatDuration = (value: number) => {
  if (!value || Number.isNaN(value)) return '—'
  return `${value.toFixed(2)}s`
}

const formatTrendPercent = (value: number) => {
  if (!value || Number.isNaN(value)) return '0%'
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(1)}%`
}

const formatErrorType = (type: string) => {
  const map: Record<string, string> = {
    network_error: 'Network',
    auth_error: 'Auth',
    rate_limit: 'Rate Limit',
    server_error: 'Server',
  }
  return map[type] || type
}

// 样式类
const savingsClass = computed(() => {
  const percent = costAnalysis.value?.cache_saving_percent || 0
  if (percent > 0.5) return 'high'
  if (percent > 0.2) return 'medium'
  return 'low'
})

const trendClass = computed(() => {
  const direction = costAnalysis.value?.trend_direction
  if (direction === 'up') return 'trend-up'
  if (direction === 'down') return 'trend-down'
  return 'trend-stable'
})

const trendIconClass = computed(() => {
  const direction = costAnalysis.value?.trend_direction
  if (direction === 'up') return 'trend-up-icon'
  if (direction === 'down') return 'trend-down-icon'
  return 'trend-stable-icon'
})

const errorRateClass = computed(() => {
  const rate = performanceAnalysis.value?.error_rate || 0
  if (rate > 0.1) return 'high'
  if (rate > 0.05) return 'medium'
  return 'low'
})

const durationClass = (value: number) => {
  if (!value || Number.isNaN(value)) return ''
  if (value < 3) return 'fast'
  if (value < 10) return 'medium'
  return 'slow'
}

const reliabilityClass = (rate: number) => {
  if (rate >= 0.95) return 'high'
  if (rate >= 0.8) return 'medium'
  return 'low'
}

const chartBarHeight = (cost: number) => {
  if (!costAnalysis.value?.cost_trend?.length) return '0%'
  const max = Math.max(...costAnalysis.value.cost_trend.map((p) => p.total_cost))
  if (max === 0) return '4px'
  return `${Math.max((cost / max) * 100, 5)}%`
}
</script>

<style scoped>
.analytics-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 24px;
}

.analytics-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.analytics-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--mac-text);
}

.analytics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.analytics-card {
  background: var(--mac-surface);
  border: 1px solid var(--mac-border);
  border-radius: 14px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cost-icon {
  background: rgba(249, 115, 22, 0.15);
  color: #f97316;
}

.cache-icon {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
}

.trend-icon {
  background: rgba(168, 85, 247, 0.15);
  color: #a855f7;
}

.trend-up-icon {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.trend-down-icon {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.trend-stable-icon {
  background: rgba(168, 85, 247, 0.15);
  color: #a855f7;
}

.perf-icon {
  background: rgba(96, 165, 250, 0.15);
  color: #60a5fa;
}

.error-icon {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.reliability-icon {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.card-title {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--mac-text);
}

.card-loading,
.card-empty {
  font-size: 0.8rem;
  color: var(--mac-text-secondary);
  text-align: center;
  padding: 16px 0;
}

/* 成本构成 */
.breakdown-bar {
  display: flex;
  height: 10px;
  border-radius: 5px;
  overflow: hidden;
  background: var(--mac-border);
}

.bar-segment {
  transition: width 0.3s ease;
}

.bar-segment.input { background: #34d399; }
.bar-segment.output { background: #60a5fa; }
.bar-segment.cache-create { background: #fbbf24; }
.bar-segment.cache-read { background: #38bdf8; }

.breakdown-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 6px;
}

.breakdown-legend li {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.75rem;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

.legend-dot.input { background: #34d399; }
.legend-dot.output { background: #60a5fa; }
.legend-dot.cache-create { background: #fbbf24; }
.legend-dot.cache-read { background: #38bdf8; }

.legend-label {
  flex: 1;
  color: var(--mac-text-secondary);
}

.legend-value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--mac-text);
}

/* 缓存节省 */
.cache-savings-content {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  text-align: center;
}

.savings-value,
.savings-percent,
.savings-tokens {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.savings-amount {
  font-size: 1.1rem;
  font-weight: 700;
  color: #22c55e;
}

.savings-percent .percent-value {
  font-size: 1.1rem;
  font-weight: 700;
}

.savings-percent.high .percent-value { color: #22c55e; }
.savings-percent.medium .percent-value { color: #fbbf24; }
.savings-percent.low .percent-value { color: var(--mac-text-secondary); }

.tokens-value {
  font-size: 1.1rem;
  font-weight: 700;
  color: #60a5fa;
}

.savings-label,
.percent-label,
.tokens-label {
  font-size: 0.7rem;
  color: var(--mac-text-secondary);
}

/* 成本趋势 */
.trend-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.trend-summary {
  display: flex;
  justify-content: space-between;
}

.trend-avg,
.trend-change {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.avg-value {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--mac-text);
}

.avg-label,
.change-label {
  font-size: 0.7rem;
  color: var(--mac-text-secondary);
}

.change-value {
  font-size: 1rem;
  font-weight: 600;
}

.trend-up .change-value { color: #ef4444; }
.trend-down .change-value { color: #22c55e; }
.trend-stable .change-value { color: var(--mac-text-secondary); }

.mini-chart {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 40px;
}

.chart-bar {
  flex: 1;
  background: var(--mac-accent, #60a5fa);
  border-radius: 2px;
  min-height: 4px;
  transition: height 0.3s ease;
  opacity: 0.7;
}

.chart-bar:hover {
  opacity: 1;
}

/* 响应时间 */
.percentile-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.percentile-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 8px;
  background: var(--mac-surface-strong, rgba(255,255,255,0.03));
  border-radius: 8px;
}

.percentile-label {
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--mac-text-secondary);
}

.percentile-value {
  font-size: 1rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.percentile-value.fast { color: #34d399; }
.percentile-value.medium { color: #fbbf24; }
.percentile-value.slow { color: #f87171; }

.stats-row {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
  text-align: center;
}

/* 错误分布 */
.error-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}

.error-total {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--mac-text);
}

.error-label {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
  flex: 1;
}

.error-rate {
  font-size: 0.85rem;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

.error-rate.low {
  background: rgba(34, 197, 94, 0.15);
  color: #22c55e;
}

.error-rate.medium {
  background: rgba(251, 191, 36, 0.15);
  color: #fbbf24;
}

.error-rate.high {
  background: rgba(239, 68, 68, 0.15);
  color: #ef4444;
}

.error-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.error-list li {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
}

.error-type {
  color: var(--mac-text-secondary);
}

.error-count {
  font-weight: 600;
  color: var(--mac-text);
}

.no-errors {
  font-size: 0.8rem;
  color: #22c55e;
  text-align: center;
  padding: 8px;
}

/* 供应商可靠性 */
.reliability-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.reliability-list li {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.provider-info {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
}

.provider-name {
  color: var(--mac-text);
  font-weight: 500;
}

.provider-rate {
  font-weight: 600;
}

.provider-rate.high { color: #22c55e; }
.provider-rate.medium { color: #fbbf24; }
.provider-rate.low { color: #ef4444; }

.provider-bar {
  height: 6px;
  background: var(--mac-border);
  border-radius: 3px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s ease;
}

.bar-fill.high { background: #22c55e; }
.bar-fill.medium { background: #fbbf24; }
.bar-fill.low { background: #ef4444; }
</style>
