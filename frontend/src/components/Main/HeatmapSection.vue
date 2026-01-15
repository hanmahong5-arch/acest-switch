<template>
  <section
    v-if="visible"
    ref="containerRef"
    class="contrib-wall"
    :aria-label="t('components.main.heatmap.ariaLabel')"
  >
    <!-- Loading 状态 -->
    <div v-if="loading" class="heatmap-state heatmap-loading">
      <div class="loading-spinner" />
      <span class="state-text">{{ t('components.main.heatmap.loading') }}</span>
    </div>

    <!-- Error 状态 -->
    <div v-else-if="error" class="heatmap-state heatmap-error">
      <div class="error-icon">
        <svg viewBox="0 0 24 24" width="32" height="32">
          <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/>
          <line x1="12" y1="8" x2="12" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          <line x1="12" y1="16" x2="12.01" y2="16" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="error-content">
        <h4 class="error-title">{{ t('components.main.heatmap.loadFailed') }}</h4>
        <p class="error-message">{{ error.message }}</p>
        <details v-if="error.details" class="error-details">
          <summary>{{ t('common.technicalDetails') }}</summary>
          <pre>{{ error.details }}</pre>
        </details>
      </div>
      <button v-if="error.retryable" class="retry-btn" @click="$emit('retry')">
        <svg viewBox="0 0 24 24" width="14" height="14">
          <path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        {{ t('common.retry') }}
      </button>
    </div>

    <!-- Empty 状态 -->
    <div v-else-if="isEmpty" class="heatmap-state heatmap-empty">
      <div class="empty-icon">
        <svg viewBox="0 0 24 24" width="32" height="32">
          <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" stroke-width="2" fill="none"/>
          <path d="M3 9h18M9 21V9" stroke="currentColor" stroke-width="2"/>
        </svg>
      </div>
      <p class="state-text">{{ t('components.main.heatmap.noData') }}</p>
      <span class="state-hint">{{ t('components.main.heatmap.noDataHint') }}</span>
    </div>

    <!-- 正常数据 -->
    <template v-else>
      <div class="contrib-legend">
        <span>{{ t('components.main.heatmap.legendLow') }}</span>
        <span v-for="level in 5" :key="level" :class="['legend-box', `gh-level-${level - 1}`]" />
        <span>{{ t('components.main.heatmap.legendHigh') }}</span>
      </div>

      <div class="contrib-grid">
        <div
          v-for="(week, weekIndex) in heatmapData"
          :key="weekIndex"
          class="contrib-column"
        >
          <div
            v-for="(day, dayIndex) in week"
            :key="dayIndex"
            class="contrib-cell"
            :class="`gh-level-${day.intensity}`"
            @mouseenter="showTooltip(day, $event)"
            @mousemove="showTooltip(day, $event)"
            @mouseleave="hideTooltip"
          />
        </div>
      </div>
    </template>

    <div
      v-if="tooltip.visible"
      ref="tooltipRef"
      class="contrib-tooltip"
      :class="tooltip.placement"
      :style="{ left: `${tooltip.left}px`, top: `${tooltip.top}px` }"
    >
      <p class="tooltip-heading">{{ formattedTooltipLabel }}</p>
      <ul class="tooltip-metrics">
        <li v-for="metric in tooltipMetrics" :key="metric.key">
          <span class="metric-label">{{ metric.label }}</span>
          <span class="metric-value">{{ metric.value }}</span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UsageHeatmapWeek, UsageHeatmapDay } from '../../data/usageHeatmap'
import type { AppError } from '../../types/error'

const props = defineProps<{
  heatmapData: UsageHeatmapWeek[]
  visible: boolean
  loading?: boolean
  error?: AppError | null
}>()

defineEmits<{
  retry: []
}>()

const isEmpty = computed(() => {
  if (props.loading || props.error) return false
  if (!props.heatmapData || props.heatmapData.length === 0) return true
  // 检查是否所有数据都是空的
  return props.heatmapData.every(week =>
    week.every(day => day.requests === 0 && day.cost === 0)
  )
})

const { t, locale } = useI18n()

const containerRef = ref<HTMLElement | null>(null)
const tooltipRef = ref<HTMLElement | null>(null)

type TooltipPlacement = 'above' | 'below'

const tooltip = reactive({
  visible: false,
  label: '',
  dateKey: '',
  left: 0,
  top: 0,
  placement: 'above' as TooltipPlacement,
  requests: 0,
  inputTokens: 0,
  outputTokens: 0,
  reasoningTokens: 0,
  cost: 0,
})

const formatMetric = (value: number) => value.toLocaleString()

const tooltipDateFormatter = computed(() =>
  new Intl.DateTimeFormat(locale.value || 'en', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
)

const currencyFormatter = computed(() =>
  new Intl.NumberFormat(locale.value || 'en', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
)

const formattedTooltipLabel = computed(() => {
  if (!tooltip.dateKey) return tooltip.label
  const date = new Date(tooltip.dateKey)
  if (Number.isNaN(date.getTime())) {
    return tooltip.label
  }
  return tooltipDateFormatter.value.format(date)
})

const formattedTooltipAmount = computed(() =>
  currencyFormatter.value.format(Math.max(tooltip.cost, 0))
)

const tooltipMetrics = computed(() => [
  {
    key: 'cost',
    label: t('components.main.heatmap.metrics.cost'),
    value: formattedTooltipAmount.value,
  },
  {
    key: 'requests',
    label: t('components.main.heatmap.metrics.requests'),
    value: formatMetric(tooltip.requests),
  },
  {
    key: 'inputTokens',
    label: t('components.main.heatmap.metrics.inputTokens'),
    value: formatMetric(tooltip.inputTokens),
  },
  {
    key: 'outputTokens',
    label: t('components.main.heatmap.metrics.outputTokens'),
    value: formatMetric(tooltip.outputTokens),
  },
  {
    key: 'reasoningTokens',
    label: t('components.main.heatmap.metrics.reasoningTokens'),
    value: formatMetric(tooltip.reasoningTokens),
  },
])

// Tooltip positioning constants
const TOOLTIP_DEFAULT_WIDTH = 220
const TOOLTIP_DEFAULT_HEIGHT = 120
const TOOLTIP_VERTICAL_OFFSET = 12
const TOOLTIP_HORIZONTAL_MARGIN = 20
const TOOLTIP_VERTICAL_MARGIN = 24

const clamp = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const getTooltipSize = () => {
  const rect = tooltipRef.value?.getBoundingClientRect()
  return {
    width: rect?.width ?? TOOLTIP_DEFAULT_WIDTH,
    height: rect?.height ?? TOOLTIP_DEFAULT_HEIGHT,
  }
}

const viewportSize = () => {
  if (typeof window !== 'undefined') {
    return { width: window.innerWidth, height: window.innerHeight }
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    return {
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
    }
  }
  return {
    width: containerRef.value?.clientWidth ?? 0,
    height: containerRef.value?.clientHeight ?? 0,
  }
}

const showTooltip = (day: UsageHeatmapDay, event: MouseEvent) => {
  const target = event.currentTarget as HTMLElement | null
  const cellRect = target?.getBoundingClientRect()
  if (!cellRect) return

  tooltip.label = day.label
  tooltip.dateKey = day.dateKey
  tooltip.requests = day.requests
  tooltip.inputTokens = day.inputTokens
  tooltip.outputTokens = day.outputTokens
  tooltip.reasoningTokens = day.reasoningTokens
  tooltip.cost = day.cost

  const { width: tooltipWidth, height: tooltipHeight } = getTooltipSize()
  const { width: viewportWidth, height: viewportHeight } = viewportSize()

  const centerX = cellRect.left + cellRect.width / 2
  const halfWidth = tooltipWidth / 2
  const minLeft = TOOLTIP_HORIZONTAL_MARGIN + halfWidth
  const maxLeft = viewportWidth > 0 ? viewportWidth - halfWidth - TOOLTIP_HORIZONTAL_MARGIN : centerX
  tooltip.left = clamp(centerX, minLeft, maxLeft)

  const anchorTop = cellRect.top
  const anchorBottom = cellRect.bottom
  const canShowAbove = anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET >= TOOLTIP_VERTICAL_MARGIN
  const viewportBottomLimit = viewportHeight > 0 ? viewportHeight - tooltipHeight - TOOLTIP_VERTICAL_MARGIN : anchorBottom
  const shouldPlaceBelow = !canShowAbove
  tooltip.placement = shouldPlaceBelow ? 'below' : 'above'

  const desiredTop = shouldPlaceBelow
    ? anchorBottom + TOOLTIP_VERTICAL_OFFSET
    : anchorTop - tooltipHeight - TOOLTIP_VERTICAL_OFFSET
  tooltip.top = clamp(desiredTop, TOOLTIP_VERTICAL_MARGIN, viewportBottomLimit)
  tooltip.visible = true
}

const hideTooltip = () => {
  tooltip.visible = false
}
</script>

<style scoped>
.contrib-wall {
  position: relative;
  margin-block: 1.5rem;
  padding: 1rem;
  background: var(--card-bg);
  border: 1px solid var(--border-subtle);
  border-radius: 0.75rem;
}

.contrib-legend {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  justify-content: flex-end;
  margin-bottom: 0.75rem;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.legend-box {
  width: 12px;
  height: 12px;
  border-radius: 2px;
}

.contrib-grid {
  display: flex;
  gap: 3px;
  overflow-x: auto;
  padding-bottom: 0.5rem;
}

.contrib-column {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.contrib-cell {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  cursor: pointer;
  transition: transform 0.1s ease;
}

.contrib-cell:hover {
  transform: scale(1.2);
}

/* GitHub-style intensity levels */
.gh-level-0 { background: var(--gh-level-0, #ebedf0); }
.gh-level-1 { background: var(--gh-level-1, #9be9a8); }
.gh-level-2 { background: var(--gh-level-2, #40c463); }
.gh-level-3 { background: var(--gh-level-3, #30a14e); }
.gh-level-4 { background: var(--gh-level-4, #216e39); }

:root.dark .gh-level-0 { background: var(--gh-level-0-dark, #161b22); }
:root.dark .gh-level-1 { background: var(--gh-level-1-dark, #0e4429); }
:root.dark .gh-level-2 { background: var(--gh-level-2-dark, #006d32); }
:root.dark .gh-level-3 { background: var(--gh-level-3-dark, #26a641); }
:root.dark .gh-level-4 { background: var(--gh-level-4-dark, #39d353); }

.contrib-tooltip {
  position: fixed;
  z-index: 1000;
  min-width: 180px;
  padding: 0.75rem 1rem;
  background: var(--contrib-tooltip-bg, #ffffff);
  color: var(--contrib-tooltip-text, rgba(15, 23, 42, 0.7));
  border-radius: 0.5rem;
  border: 1px solid var(--contrib-tooltip-border, rgba(15, 23, 42, 0.12));
  box-shadow: var(--contrib-tooltip-shadow, 0 20px 40px rgba(15, 23, 42, 0.15));
  transform: translateX(-50%);
  pointer-events: none;
}

.contrib-tooltip.below::before,
.contrib-tooltip.above::after {
  content: '';
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
}

.contrib-tooltip.above::after {
  bottom: -12px;
  border-top-color: var(--contrib-tooltip-bg, #ffffff);
}

.contrib-tooltip.below::before {
  top: -12px;
  border-bottom-color: var(--contrib-tooltip-bg, #ffffff);
}

.tooltip-heading {
  margin: 0 0 0.5rem;
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--contrib-tooltip-heading, #0f172a);
}

.tooltip-metrics {
  list-style: none;
  margin: 0;
  padding: 0;
}

.tooltip-metrics li {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.25rem 0;
  font-size: 0.8125rem;
}

.metric-label {
  color: var(--contrib-tooltip-text, rgba(15, 23, 42, 0.7));
}

.metric-value {
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  color: var(--contrib-tooltip-value, #0f172a);
}

/* 三态 UI 样式 */
.heatmap-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 2rem;
  min-height: 120px;
  color: var(--text-muted);
}

.state-text {
  font-size: 0.875rem;
}

.state-hint {
  font-size: 0.75rem;
  opacity: 0.7;
}

/* Loading 状态 */
.heatmap-loading .loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-subtle);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Error 状态 */
.heatmap-error {
  color: var(--error-text, #dc3545);
}

.heatmap-error .error-icon {
  color: var(--error-text, #dc3545);
  opacity: 0.8;
}

.heatmap-error .error-content {
  text-align: center;
}

.heatmap-error .error-title {
  margin: 0 0 0.25rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
}

.heatmap-error .error-message {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--text-muted);
}

.heatmap-error .error-details {
  margin-top: 0.5rem;
  font-size: 0.75rem;
  text-align: left;
  max-width: 100%;
}

.heatmap-error .error-details summary {
  cursor: pointer;
  color: var(--text-muted);
  opacity: 0.8;
}

.heatmap-error .error-details pre {
  margin: 0.5rem 0 0;
  padding: 0.5rem;
  background: var(--code-bg, #f6f8fa);
  border-radius: 4px;
  font-size: 0.6875rem;
  overflow-x: auto;
  max-width: 300px;
  white-space: pre-wrap;
  word-break: break-all;
}

:root.dark .heatmap-error .error-details pre {
  background: var(--code-bg-dark, #161b22);
}

.heatmap-error .retry-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  margin-top: 0.75rem;
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  color: var(--accent-primary);
  background: transparent;
  border: 1px solid var(--accent-primary);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.heatmap-error .retry-btn:hover {
  background: var(--accent-primary);
  color: #fff;
}

/* Empty 状态 */
.heatmap-empty .empty-icon {
  color: var(--text-muted);
  opacity: 0.5;
}
</style>
