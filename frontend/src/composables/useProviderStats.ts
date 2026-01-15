import { ref, computed } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useProviderStore, type ProviderTab } from '../stores'
import { fetchProviderDailyStats, type ProviderDailyStat } from '../services/logs'

/**
 * Provider 统计数据 composable
 * 提供防抖加载、缓存和格式化功能
 */
export function useProviderStats() {
  const store = useProviderStore()

  const loading = ref<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    'gemini-cli': false,
  })

  const loaded = ref<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    'gemini-cli': false,
  })

  /**
   * 加载指定平台的统计数据（带防抖）
   */
  const loadStats = useDebounceFn(async (tab: ProviderTab) => {
    if (loading.value[tab]) return

    loading.value[tab] = true
    try {
      await store.loadStats(tab)
      loaded.value[tab] = true
    } catch (error) {
      console.error(`Failed to load ${tab} stats`, error)
    } finally {
      loading.value[tab] = false
    }
  }, 300)

  /**
   * 刷新所有平台的统计数据
   */
  const refreshAll = async () => {
    await Promise.all([
      loadStats('claude'),
      loadStats('codex'),
      loadStats('gemini-cli'),
    ])
  }

  /**
   * 获取 Provider 的统计显示数据
   */
  const getStatDisplay = (tab: ProviderTab, providerName: string) => {
    const key = providerName.trim().toLowerCase()

    if (loading.value[tab] && !loaded.value[tab]) {
      return {
        state: 'loading' as const,
        message: 'Loading...',
      }
    }

    const stat = store.stats[tab][key]
    if (!stat) {
      return {
        state: 'empty' as const,
        message: 'No data today',
      }
    }

    // 计算成功率显示
    let successRateLabel = ''
    let successRateClass = ''
    if (stat.total_requests > 0) {
      const rate = stat.success_rate
      successRateLabel = `${(rate * 100).toFixed(0)}%`
      if (rate >= 0.95) {
        successRateClass = 'success'
      } else if (rate >= 0.8) {
        successRateClass = 'warning'
      } else {
        successRateClass = 'error'
      }
    }

    return {
      state: 'ready' as const,
      successRateLabel,
      successRateClass,
      requests: `${stat.total_requests} requests`,
      tokens: `${formatNumber(stat.input_tokens + stat.output_tokens)} tokens`,
      cost: formatCurrency(stat.cost_total),
    }
  }

  return {
    loading,
    loaded,
    loadStats,
    refreshAll,
    getStatDisplay,
  }
}

// 工具函数
function formatNumber(value: number): string {
  if (value >= 1000000) {
    return `${(value / 1000000).toFixed(1)}M`
  }
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)}K`
  }
  return value.toLocaleString()
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.max(value, 0))
}
