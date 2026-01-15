import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { LoadProviders, SaveProviders } from '../../bindings/codeswitch/services/providerservice'
import { fetchProviderDailyStats, type ProviderDailyStat } from '../services/logs'

// 直接使用后端绑定的 Provider 类型
import { Provider as BackendProvider } from '../../bindings/codeswitch/services/models'

// 重新导出类型
export type Provider = BackendProvider

export type ProviderTab = 'claude' | 'codex' | 'gemini-cli'

export const PROVIDER_TABS: ProviderTab[] = ['claude', 'codex', 'gemini-cli']

export const useProviderStore = defineStore('providers', () => {
  // 状态
  const providers = ref<Record<ProviderTab, Provider[]>>({
    claude: [],
    codex: [],
    'gemini-cli': [],
  })

  const loading = ref<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    'gemini-cli': false,
  })

  const stats = ref<Record<ProviderTab, Record<string, ProviderDailyStat>>>({
    claude: {},
    codex: {},
    'gemini-cli': {},
  })

  const statsLoading = ref<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    'gemini-cli': false,
  })

  const statsLoaded = ref<Record<ProviderTab, boolean>>({
    claude: false,
    codex: false,
    'gemini-cli': false,
  })

  // Getters
  const getProviders = computed(() => (tab: ProviderTab) => providers.value[tab])

  const getEnabledProviders = computed(() => (tab: ProviderTab) =>
    providers.value[tab].filter(p => p.enabled)
  )

  const getProviderStats = computed(() => (tab: ProviderTab, name: string) =>
    stats.value[tab][name.trim().toLowerCase()]
  )

  // Actions
  async function loadProviders(tab: ProviderTab) {
    if (loading.value[tab]) return

    loading.value[tab] = true
    try {
      const data = await LoadProviders(tab)
      providers.value[tab] = data || []
    } catch (error) {
      console.error(`Failed to load ${tab} providers`, error)
      providers.value[tab] = []
    } finally {
      loading.value[tab] = false
    }
  }

  async function saveProviders(tab: ProviderTab) {
    try {
      await SaveProviders(tab, providers.value[tab])
    } catch (error) {
      console.error(`Failed to save ${tab} providers`, error)
      throw error
    }
  }

  async function loadStats(tab: ProviderTab) {
    if (statsLoading.value[tab]) return

    statsLoading.value[tab] = true
    try {
      const data = await fetchProviderDailyStats(tab)
      const map: Record<string, ProviderDailyStat> = {}
      for (const stat of data) {
        const key = stat.provider?.trim().toLowerCase() ?? ''
        if (key) {
          map[key] = stat
        }
      }
      stats.value[tab] = map
      statsLoaded.value[tab] = true
    } catch (error) {
      console.error(`Failed to load ${tab} stats`, error)
    } finally {
      statsLoading.value[tab] = false
    }
  }

  function addProvider(tab: ProviderTab, provider: Provider) {
    providers.value[tab].push(provider)
  }

  function updateProvider(tab: ProviderTab, id: number, updates: Partial<Provider>) {
    const index = providers.value[tab].findIndex(p => p.id === id)
    if (index !== -1) {
      providers.value[tab][index] = { ...providers.value[tab][index], ...updates }
    }
  }

  function removeProvider(tab: ProviderTab, id: number) {
    const index = providers.value[tab].findIndex(p => p.id === id)
    if (index !== -1) {
      providers.value[tab].splice(index, 1)
    }
  }

  function reorderProviders(tab: ProviderTab, fromIndex: number, toIndex: number) {
    const list = providers.value[tab]
    const [removed] = list.splice(fromIndex, 1)
    list.splice(toIndex, 0, removed)
  }

  return {
    // State
    providers,
    loading,
    stats,
    statsLoading,
    statsLoaded,
    // Getters
    getProviders,
    getEnabledProviders,
    getProviderStats,
    // Actions
    loadProviders,
    saveProviders,
    loadStats,
    addProvider,
    updateProvider,
    removeProvider,
    reorderProviders,
  }
})
