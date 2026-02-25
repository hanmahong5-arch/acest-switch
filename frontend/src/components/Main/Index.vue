<template>
  <div class="main-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ t('components.main.hero.eyebrow') }}</p>
      <div class="today-stats" :data-tooltip="todayStatsTooltip">
        <span class="stats-icon">
          <svg viewBox="0 0 24 24" width="14" height="14">
            <path d="M18 20V10M12 20V4M6 20v-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
        <span class="stats-text">{{ todayStatsText }}</span>
      </div>
      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.controls.copyProxy')"
        @click="copyProxyAddress"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
      <button
        class="ghost-icon"
        :class="{ 'spin-animation': statsRefreshing }"
        :data-tooltip="t('components.main.controls.refresh')"
        :disabled="statsRefreshing"
        @click="refreshAllStats"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M23 4v6h-6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M1 20v-6h6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
      <button
        class="ghost-icon body-log-icon"
        :class="{ 'body-log-on': bodyLogEnabled }"
        :data-tooltip="bodyLogEnabled ? t('components.main.controls.bodyLogOn') : t('components.main.controls.bodyLogOff')"
        @click="toggleBodyLog"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path
            d="M14 2v6h6M16 13H8M16 17H8M10 9H8"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
      <button
        class="ghost-icon cli-center-btn"
        data-tooltip="CLI Configuration Center"
        @click="goToCliCenter"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M4 17l6-6-6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path
            d="M12 19h8"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
          />
        </svg>
      </button>
      <button
        class="ghost-icon distributor-btn"
        data-tooltip="AI Evangelist Mode"
        @click="goToDistributor"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <circle
            cx="9"
            cy="7"
            r="4"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          />
          <path
            d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.controls.settings')"
        @click="goToSettings"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 15a3 3 0 100-6 3 3 0 000 6z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
          <path
            d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09a1.65 1.65 0 00-1-1.51 1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09a1.65 1.65 0 001.51-1 1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
      </button>
      <button
        class="ghost-icon"
        :data-tooltip="t('components.main.controls.admin')"
        @click="goToAdmin"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          />
        </svg>
      </button>
    </div>
    <div class="contrib-page">
      <section class="contrib-hero">
        <h1 v-if="showHomeTitle">{{ t('components.main.hero.title') }}</h1>
        <!-- <p class="lead">
          {{ t('components.main.hero.lead') }}
        </p> -->
      </section>

      <HeatmapSection
        :heatmap-data="usageHeatmap"
        :visible="showHeatmap"
        :loading="usageHeatmapLoading"
        :error="usageHeatmapError"
        @retry="loadUsageHeatmap"
      />

      <AnalyticsSection />

      <section class="automation-section">
      <div class="section-header">
        <div class="tab-group" role="tablist" :aria-label="t('components.main.tabs.ariaLabel')">
          <button
            v-for="(tab, idx) in tabs"
            :key="tab.id"
            class="tab-pill"
            :class="{ active: selectedIndex === idx }"
            role="tab"
            :aria-selected="selectedIndex === idx"
            type="button"
            @click="onTabChange(idx)"
          >
            {{ tab.label }}
          </button>
        </div>
        <div class="section-controls">
          <div class="relay-toggle" :aria-label="currentProxyLabel">
            <div class="relay-switch">
              <label class="mac-switch sm">
                <input
                  type="checkbox"
                  :checked="activeProxyState"
                  :disabled="activeProxyBusy"
                  @change="onProxyToggle"
                />
                <span></span>
              </label>
              <span class="relay-tooltip-content">{{ currentProxyLabel }} · {{ t('components.main.relayToggle.tooltip') }}</span>
            </div>
          </div>
          <div class="relay-toggle" :aria-label="roundRobinEnabled ? t('components.main.roundRobin.on') : t('components.main.roundRobin.off')">
            <div class="relay-switch">
              <label class="mac-switch sm">
                <input
                  type="checkbox"
                  :checked="roundRobinEnabled"
                  @change="onRoundRobinToggle"
                />
                <span></span>
              </label>
              <span class="relay-tooltip-content">{{ t('components.main.roundRobin.label') }} · {{ roundRobinEnabled ? t('components.main.roundRobin.on') : t('components.main.roundRobin.off') }}</span>
            </div>
          </div>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.controls.mcp')"
            @click="goToMcp"
          >
            <span class="icon-svg" v-html="mcpIcon" aria-hidden="true"></span>
          </button>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.controls.skill')"
            @click="goToSkill"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M6 4h8a4 4 0 014 4v12a3 3 0 00-3-3H6z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M6 4a2 2 0 00-2 2v13c0 .55.45 1 1 1h11"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M9 8h5"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
              />
            </svg>
          </button>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.controls.sync')"
            @click="goToSync"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.controls.gateway', 'NEW-API Gateway')"
            @click="goToGateway"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M13 10V3L4 14h7v7l9-11h-7z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.logs.view')"
            @click="goToLogs"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M5 7h14M5 12h14M5 17h9"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
                fill="none"
              />
            </svg>
          </button>
          <button
            class="ghost-icon"
            :data-tooltip="t('components.main.tabs.addCard')"
            @click="openCreateModal"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M12 5v14M5 12h14"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
                fill="none"
              />
            </svg>
          </button>
        </div>
      </div>
      <div class="search-bar">
        <div class="search-input-wrapper">
          <svg class="search-icon" viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="11" cy="11" r="8" fill="none" stroke="currentColor" stroke-width="1.5"/>
            <path d="m21 21-4.35-4.35" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            class="search-input"
            :placeholder="t('components.main.search.placeholder')"
            :aria-label="t('components.main.search.placeholder')"
          />
          <button
            v-if="searchQuery"
            class="search-clear"
            :aria-label="t('components.main.search.clear')"
            @click="searchQuery = ''"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            </svg>
          </button>
        </div>
      </div>
      <div class="automation-list" @dragover.prevent>
        <ProviderCard
          v-for="card in filteredCards"
          :key="card.id"
          :card="toProviderCardData(card)"
          :stats="providerStatDisplay(card.name)"
          :is-dragging="draggingId === card.id"
          @toggle-enabled="onToggleEnabled(card, $event)"
          @configure="configure(card)"
          @remove="requestRemove(card)"
          @drag-start="onDragStart(card.id)"
          @drag-end="onDragEnd"
          @drop="onDrop(card.id)"
        />
      </div>
      </section>

      <ProviderModal
        :open="modalState.open"
        :is-editing="Boolean(modalState.editingId)"
        :initial-data="modalInitialData"
        @close="closeModal"
        @save="handleModalSave"
      />
      <BaseModal
      :open="confirmState.open"
      :title="t('components.main.form.confirmDeleteTitle')"
      variant="confirm"
      @close="closeConfirm"
    >
      <div class="confirm-body">
        <p>
          {{ t('components.main.form.confirmDeleteMessage', { name: confirmState.card?.name ?? '' }) }}
        </p>
      </div>
      <footer class="form-actions confirm-actions">
        <BaseButton variant="outline" type="button" @click="closeConfirm">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton variant="danger" type="button" @click="confirmRemove">
          {{ t('components.main.form.actions.delete') }}
        </BaseButton>
      </footer>
      </BaseModal>
      <footer v-if="appVersion" class="main-version">
        {{ t('components.main.versionLabel', { version: appVersion }) }}
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
	buildUsageHeatmapMatrix,
	generateFallbackUsageHeatmap,
	DEFAULT_HEATMAP_DAYS,
	calculateHeatmapDayRange,
	type UsageHeatmapWeek,
} from '../../data/usageHeatmap'
import { automationCardGroups, createAutomationCards, type AutomationCard } from '../../data/cards'
import lobeIcons from '../../icons/lobeIconMap'
import BaseButton from '../common/BaseButton.vue'
import BaseModal from '../common/BaseModal.vue'
import AnalyticsSection from './AnalyticsSection.vue'
import HeatmapSection from './HeatmapSection.vue'
import ProviderCard, { type ProviderCardData, type ProviderStats } from './ProviderCard.vue'
import ProviderModal, { type ProviderFormData } from './ProviderModal.vue'
import { LoadProviders, SaveProviders } from '../../../bindings/codeswitch/services/providerservice'
import { fetchProxyStatus, enableProxy, disableProxy } from '../../services/claudeSettings'
import { isRoundRobinEnabled as fetchRoundRobinStatus, setRoundRobinEnabled } from '../../services/providerRelay'
import { fetchHeatmapStats, fetchProviderDailyStats, type ProviderDailyStat } from '../../services/logs'
import { fetchCurrentVersion } from '../../services/version'
import { fetchAppSettings, saveAppSettings, type AppSettings } from '../../services/appSettings'
import { useRouter } from 'vue-router'
import { showToast } from '../../utils/toast'
import { normalizeError, type AppError } from '../../types/error'

const { t, locale } = useI18n()
const router = useRouter()
const PROXY_ADDRESS = 'http://127.0.0.1:18100'

const HEATMAP_DAYS = DEFAULT_HEATMAP_DAYS
const usageHeatmap = ref<UsageHeatmapWeek[]>(generateFallbackUsageHeatmap(HEATMAP_DAYS))
const usageHeatmapLoading = ref(false)
const usageHeatmapError = ref<AppError | null>(null)
const proxyStates = reactive<Record<ProviderTab, boolean>>({
  claude: false,
  codex: false,
  'gemini-cli': false,
  picoclaw: false,
})
const proxyBusy = reactive<Record<ProviderTab, boolean>>({
  claude: false,
  codex: false,
  'gemini-cli': false,
  picoclaw: false,
})

const providerStatsMap = reactive<Record<ProviderTab, Record<string, ProviderDailyStat>>>({
  claude: {},
  codex: {},
  'gemini-cli': {},
  picoclaw: {},
} as Record<ProviderTab, Record<string, ProviderDailyStat>>)
const providerStatsLoading = reactive<Record<ProviderTab, boolean>>({
  claude: false,
  codex: false,
  'gemini-cli': false,
  picoclaw: false,
} as Record<ProviderTab, boolean>)
const providerStatsLoaded = reactive<Record<ProviderTab, boolean>>({
  claude: false,
  codex: false,
  'gemini-cli': false,
  picoclaw: false,
} as Record<ProviderTab, boolean>)
let providerStatsTimer: number | undefined
let updateTimer: number | undefined
const showHeatmap = ref(true)
const showHomeTitle = ref(true)
const mcpIcon = lobeIcons['mcp'] ?? ''
const appVersion = ref('')
const bodyLogEnabled = ref(false)
const statsRefreshing = ref(false)

// 工具函数：用于多个地方
const formatMetric = (value: number) => value.toLocaleString()

const currencyFormatter = computed(() =>
  new Intl.NumberFormat(locale.value || 'en', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
)

const clamp = (value: number, min: number, max: number) => {
  if (max <= min) return min
  return Math.min(Math.max(value, min), max)
}

const loadAppSettings = async () => {
  try {
    const data: AppSettings = await fetchAppSettings()
    showHeatmap.value = data?.show_heatmap ?? true
    showHomeTitle.value = data?.show_home_title ?? true
    bodyLogEnabled.value = data?.enable_body_log ?? false
  } catch (error) {
    console.error('failed to load app settings', error)
    showHeatmap.value = true
    showHomeTitle.value = true
    bodyLogEnabled.value = false
  }
}

const toggleBodyLog = async () => {
  const newValue = !bodyLogEnabled.value
  try {
    const currentSettings = await fetchAppSettings()
    await saveAppSettings({
      ...currentSettings,
      enable_body_log: newValue,
    })
    bodyLogEnabled.value = newValue
    showToast(newValue ? t('components.main.status.bodyLogOn') : t('components.main.status.bodyLogOff'))
  } catch (error) {
    console.error('failed to toggle body log', error)
    showToast(t('components.main.status.bodyLogFailed'), 'error')
  }
}

const loadAppVersion = async () => {
  try {
    const version = await fetchCurrentVersion()
    appVersion.value = version || ''
  } catch (error) {
    console.error('failed to load app version', error)
  }
}

const handleAppSettingsUpdated = () => {
  void loadAppSettings()
}

const startUpdateTimer = () => {
  stopUpdateTimer()
  updateTimer = window.setInterval(() => {
    void loadAppVersion()
  }, 60 * 60 * 1000)
}

const stopUpdateTimer = () => {
  if (updateTimer) {
    clearInterval(updateTimer)
    updateTimer = undefined
  }
}

const normalizeProviderKey = (value: string) => value?.trim().toLowerCase() ?? ''

const loadUsageHeatmap = async () => {
	// 防止重复调用
	if (usageHeatmapLoading.value) {
		return
	}
	usageHeatmapLoading.value = true
	usageHeatmapError.value = null
	try {
		const rangeDays = calculateHeatmapDayRange(HEATMAP_DAYS)
		const stats = await fetchHeatmapStats(rangeDays)
		usageHeatmap.value = buildUsageHeatmapMatrix(stats, HEATMAP_DAYS)
	} catch (error) {
		console.error('Failed to load usage heatmap stats', error)
		usageHeatmapError.value = normalizeError(error, {
			component: 'HeatmapSection',
			action: 'loadUsageHeatmap',
		})
	} finally {
		usageHeatmapLoading.value = false
	}
}

const tabs = [
  { id: 'claude', label: 'Claude Code' },
  { id: 'codex', label: 'Codex' },
  { id: 'gemini-cli', label: 'Gemini-CLI' },
  { id: 'picoclaw', label: 'PicoClaw' },
] as const
type ProviderTab = (typeof tabs)[number]['id']
const providerTabIds = tabs.map((tab) => tab.id) as ProviderTab[]

const cards = reactive<Record<ProviderTab, AutomationCard[]>>({
  claude: createAutomationCards(automationCardGroups.claude),
  codex: createAutomationCards(automationCardGroups.codex),
  'gemini-cli': createAutomationCards(automationCardGroups['gemini-cli']),
  picoclaw: createAutomationCards(automationCardGroups.picoclaw),
})
const draggingId = ref<number | null>(null)

const serializeProviders = (providers: AutomationCard[]) => providers.map((provider) => ({ ...provider }))

const persistProviders = async (tabId: ProviderTab) => {
  try {
    await SaveProviders(tabId, serializeProviders(cards[tabId]))
  } catch (error) {
    console.error('Failed to save providers', error)
  }
}

const replaceProviders = (tabId: ProviderTab, data: AutomationCard[]) => {
  cards[tabId].splice(0, cards[tabId].length, ...createAutomationCards(data))
}

const loadProvidersFromDisk = async () => {
  for (const tab of providerTabIds) {
    try {
      const saved = await LoadProviders(tab)
      if (Array.isArray(saved)) {
        replaceProviders(tab, saved as AutomationCard[])
      } else {
        await persistProviders(tab)
      }
    } catch (error) {
      console.error('Failed to load providers', error)
    }
  }
}

const refreshProxyState = async (tab: ProviderTab) => {
  try {
    const status = await fetchProxyStatus(tab)
    proxyStates[tab] = Boolean(status?.enabled)
  } catch (error) {
    console.error(`Failed to fetch proxy status for ${tab}`, error)
    proxyStates[tab] = false
  }
}

const onProxyToggle = async () => {
  const tab = activeTab.value
  if (proxyBusy[tab]) return
  proxyBusy[tab] = true
  const nextState = !proxyStates[tab]
  try {
    if (nextState) {
      await enableProxy(tab)
    } else {
      await disableProxy(tab)
    }
    proxyStates[tab] = nextState
  } catch (error) {
    console.error(`Failed to toggle proxy for ${tab}`, error)
  } finally {
    proxyBusy[tab] = false
  }
}

const loadProviderStats = async (tab: ProviderTab) => {
  // 防止重复调用
  if (providerStatsLoading[tab]) {
    return
  }
  providerStatsLoading[tab] = true
  try {
    const stats = await fetchProviderDailyStats(tab)
    const mapped: Record<string, ProviderDailyStat> = {}
    ;(stats ?? []).forEach((stat) => {
      mapped[normalizeProviderKey(stat.provider)] = stat
    })
    const hadExistingStats = Object.keys(providerStatsMap[tab] ?? {}).length > 0
    if ((stats?.length ?? 0) > 0) {
      providerStatsMap[tab] = mapped
    } else if (!hadExistingStats) {
      providerStatsMap[tab] = mapped
    }
    providerStatsLoaded[tab] = true
  } catch (error) {
    console.error(`Failed to load provider stats for ${tab}`, error)
    if (!providerStatsLoaded[tab]) {
      providerStatsLoaded[tab] = true
    }
  } finally {
    providerStatsLoading[tab] = false
  }
}

type ProviderStatDisplay =
  | { state: 'loading' | 'empty'; message: string }
  | {
      state: 'ready'
      requests: string
      tokens: string
      cost: string
      successRateLabel: string
      successRateClass: string
    }

const SUCCESS_RATE_THRESHOLDS = {
  healthy: 0.95,
  warning: 0.8,
} as const

const formatSuccessRateLabel = (value: number) => {
  const percent = clamp(value, 0, 1) * 100
  const decimals = percent >= 99.5 || percent === 0 ? 0 : 1
  return `${t('components.main.providers.successRate')}: ${percent.toFixed(decimals)}%`
}

const successRateClassName = (value: number) => {
  const rate = clamp(value, 0, 1)
  if (rate >= SUCCESS_RATE_THRESHOLDS.healthy) {
    return 'success-good'
  }
  if (rate >= SUCCESS_RATE_THRESHOLDS.warning) {
    return 'success-warn'
  }
  return 'success-bad'
}

const providerStatDisplay = (providerName: string): ProviderStatDisplay => {
  const tab = activeTab.value
  if (!providerStatsLoaded[tab]) {
    return { state: 'loading', message: t('components.main.providers.loading') }
  }
  const stat = providerStatsMap[tab]?.[normalizeProviderKey(providerName)]
  if (!stat) {
    return { state: 'empty', message: t('components.main.providers.noData') }
  }
  const totalTokens = stat.input_tokens + stat.output_tokens
  const successRateValue = Number.isFinite(stat.success_rate) ? clamp(stat.success_rate, 0, 1) : null
  const successRateLabel = successRateValue !== null ? formatSuccessRateLabel(successRateValue) : ''
  const successRateClass = successRateValue !== null ? successRateClassName(successRateValue) : ''
  return {
    state: 'ready',
    requests: `${t('components.main.providers.requests')}: ${formatMetric(stat.total_requests)}`,
    tokens: `${t('components.main.providers.tokens')}: ${formatMetric(totalTokens)}`,
    cost: `${t('components.main.providers.cost')}: ${currencyFormatter.value.format(Math.max(stat.cost_total, 0))}`,
    successRateLabel,
    successRateClass,
  }
}


const startProviderStatsTimer = () => {
  stopProviderStatsTimer()
  providerStatsTimer = window.setInterval(() => {
    providerTabIds.forEach((tab) => {
      void loadProviderStats(tab)
    })
  }, 60_000)
}

const stopProviderStatsTimer = () => {
  if (providerStatsTimer) {
    clearInterval(providerStatsTimer)
    providerStatsTimer = undefined
  }
}


onMounted(async () => {
  void loadUsageHeatmap()
  await loadProvidersFromDisk()
  await Promise.all(providerTabIds.map(refreshProxyState))
  await Promise.all(providerTabIds.map((tab) => loadProviderStats(tab)))
  await loadAppSettings()
  await loadRoundRobinStatus()
  await loadAppVersion()
  startProviderStatsTimer()
  startUpdateTimer()
  window.addEventListener('app-settings-updated', handleAppSettingsUpdated)
})

onUnmounted(() => {
  stopProviderStatsTimer()
  window.removeEventListener('app-settings-updated', handleAppSettingsUpdated)
  stopUpdateTimer()
})

const selectedIndex = ref(0)
const activeTab = computed<ProviderTab>(() => tabs[selectedIndex.value]?.id ?? tabs[0].id)
const activeCards = computed(() => cards[activeTab.value] ?? [])
const searchQuery = ref('')
const filteredCards = computed(() => {
  if (!searchQuery.value.trim()) {
    return activeCards.value
  }
  const query = searchQuery.value.toLowerCase().trim()
  return activeCards.value.filter((card) => {
    return (
      card.name.toLowerCase().includes(query) ||
      card.apiUrl.toLowerCase().includes(query) ||
      (card.officialSite && card.officialSite.toLowerCase().includes(query))
    )
  })
})
const currentProxyLabel = computed(() => {
  if (activeTab.value === 'claude') {
    return t('components.main.relayToggle.hostClaude')
  } else if (activeTab.value === 'codex') {
    return t('components.main.relayToggle.hostCodex')
  } else {
    return t('components.main.relayToggle.hostGeminiCli')
  }
})
const activeProxyState = computed(() => proxyStates[activeTab.value])
const activeProxyBusy = computed(() => proxyBusy[activeTab.value])

// 轮询模式状态
const roundRobinEnabled = ref(false)

const loadRoundRobinStatus = async () => {
  try {
    roundRobinEnabled.value = await fetchRoundRobinStatus()
  } catch (err) {
    console.error('Failed to load round-robin status', err)
  }
}

const onRoundRobinToggle = async () => {
  const newValue = !roundRobinEnabled.value
  try {
    await setRoundRobinEnabled(newValue)
    roundRobinEnabled.value = newValue
  } catch (err) {
    console.error('Failed to toggle round-robin mode', err)
  }
}

const goToLogs = () => {
  router.push('/logs')
}

const goToMcp = () => {
  router.push('/mcp')
}

const goToSkill = () => {
  router.push('/skill')
}

const goToSync = () => {
  router.push('/sync')
}

const goToGateway = () => {
  router.push('/gateway')
}

const goToSettings = () => {
  router.push('/settings')
}

const goToCliCenter = () => {
  router.push('/cli-center')
}

const goToDistributor = () => {
  router.push('/distributor')
}

const goToAdmin = () => {
  router.push('/admin')
}

// 今日统计计算属性
const todayTotalStats = computed(() => {
  let totalRequests = 0
  let totalTokens = 0
  let totalCost = 0
  for (const tab of providerTabIds) {
    const statsMap = providerStatsMap[tab]
    if (!statsMap) continue
    for (const stat of Object.values(statsMap)) {
      totalRequests += stat.total_requests ?? 0
      totalTokens += (stat.input_tokens ?? 0) + (stat.output_tokens ?? 0)
      totalCost += stat.cost_total ?? 0
    }
  }
  return { totalRequests, totalTokens, totalCost }
})

const todayStatsText = computed(() => {
  const { totalRequests, totalCost } = todayTotalStats.value
  if (totalRequests === 0) return t('components.main.status.noRequests')
  const costStr = currencyFormatter.value.format(totalCost)
  return `${totalRequests} ${t('components.main.status.requests')} · ${costStr}`
})

const todayStatsTooltip = computed(() => {
  const { totalRequests, totalTokens, totalCost } = todayTotalStats.value
  const costStr = currencyFormatter.value.format(totalCost)
  return `${t('components.main.status.todayStats')}: ${totalRequests} ${t('components.main.status.requests')}, ${formatMetric(totalTokens)} tokens, ${costStr}`
})

// 复制代理地址
const copyProxyAddress = async () => {
  try {
    await navigator.clipboard.writeText(PROXY_ADDRESS)
    showToast(t('components.main.status.copied'))
  } catch (error) {
    console.error('Failed to copy proxy address', error)
    showToast(t('components.main.status.copyFailed'), 'error')
  }
}

// 刷新所有统计
const refreshAllStats = async () => {
  if (statsRefreshing.value) return
  statsRefreshing.value = true
  try {
    await Promise.all([
      loadUsageHeatmap(),
      ...providerTabIds.map((tab) => loadProviderStats(tab)),
    ])
    showToast(t('components.main.status.refreshed'))
  } catch (error) {
    console.error('Failed to refresh stats', error)
  } finally {
    statsRefreshing.value = false
  }
}

// Modal 状态
const modalState = reactive({
  open: false,
  tabId: tabs[0].id as ProviderTab,
  editingId: null as number | null,
})

const editingCard = ref<AutomationCard | null>(null)
const confirmState = reactive({ open: false, card: null as AutomationCard | null, tabId: tabs[0].id as ProviderTab })

// 计算属性：modal 的初始数据
const modalInitialData = computed(() => {
  if (!editingCard.value) return undefined
  return {
    name: editingCard.value.name,
    apiUrl: editingCard.value.apiUrl,
    apiKey: editingCard.value.apiKey,
    officialSite: editingCard.value.officialSite,
    icon: editingCard.value.icon,
    enabled: editingCard.value.enabled,
    supportedModels: editingCard.value.supportedModels || {},
    modelMapping: editingCard.value.modelMapping || {},
    level: editingCard.value.level ?? 1,
  }
})

const openCreateModal = () => {
  modalState.tabId = activeTab.value
  modalState.editingId = null
  editingCard.value = null
  modalState.open = true
}

const openEditModal = (card: AutomationCard) => {
  modalState.tabId = activeTab.value
  modalState.editingId = card.id
  editingCard.value = card
  modalState.open = true
}

const closeModal = () => {
  modalState.open = false
}

const closeConfirm = () => {
  confirmState.open = false
  confirmState.card = null
}

// 处理 modal 保存事件
const handleModalSave = (data: ProviderFormData) => {
  const list = cards[modalState.tabId]
  if (!list) return

  if (editingCard.value) {
    // 编辑现有 provider
    Object.assign(editingCard.value, {
      apiUrl: data.apiUrl || editingCard.value.apiUrl,
      apiKey: data.apiKey,
      officialSite: data.officialSite,
      icon: data.icon,
      enabled: data.enabled,
      supportedModels: data.supportedModels || {},
      modelMapping: data.modelMapping || {},
      level: data.level || 1,
    })
    void persistProviders(modalState.tabId)
  } else {
    // 创建新 provider
    const newCard: AutomationCard = {
      id: Date.now(),
      name: data.name || 'Untitled vendor',
      apiUrl: data.apiUrl,
      apiKey: data.apiKey,
      officialSite: data.officialSite,
      icon: data.icon,
      accent: '#0a84ff',
      tint: 'rgba(15, 23, 42, 0.12)',
      enabled: data.enabled,
      supportedModels: data.supportedModels || {},
      modelMapping: data.modelMapping || {},
      level: data.level || 1,
    }
    list.push(newCard)
    void persistProviders(modalState.tabId)
  }

  closeModal()
}

const configure = (card: AutomationCard) => {
  openEditModal(card)
}

const remove = (id: number, tabId: ProviderTab = activeTab.value) => {
  const list = cards[tabId]
  if (!list) return
  const index = list.findIndex((card) => card.id === id)
  if (index > -1) {
    list.splice(index, 1)
    void persistProviders(tabId)
  }
}

const requestRemove = (card: AutomationCard) => {
  confirmState.card = card
  confirmState.tabId = activeTab.value
  confirmState.open = true
}

const confirmRemove = () => {
  if (!confirmState.card) return
  remove(confirmState.card.id, confirmState.tabId)
  closeConfirm()
}

const onDragStart = (id: number) => {
  draggingId.value = id
}

const onDrop = (targetId: number) => {
  if (draggingId.value === null || draggingId.value === targetId) return
  const currentTab = activeTab.value
  const list = cards[currentTab]
  if (!list) return
  const fromIndex = list.findIndex((card) => card.id === draggingId.value)
  const toIndex = list.findIndex((card) => card.id === targetId)
  if (fromIndex === -1 || toIndex === -1) return
  const [moved] = list.splice(fromIndex, 1)
  const newIndex = fromIndex < toIndex ? toIndex - 1 : toIndex
  list.splice(newIndex, 0, moved)
  draggingId.value = null
  void persistProviders(currentTab)
}

const onDragEnd = () => {
  draggingId.value = null
}

const iconSvg = (name: string) => {
  if (!name) return ''
  return lobeIcons[name.toLowerCase()] ?? ''
}

// 将 AutomationCard 转换为 ProviderCardData
const toProviderCardData = (card: AutomationCard): ProviderCardData => ({
  id: card.id,
  name: card.name,
  icon: card.icon,
  tint: card.tint,
  accent: card.accent,
  enabled: card.enabled,
  officialSite: card.officialSite,
})

// 处理 enabled 状态切换
const onToggleEnabled = (card: AutomationCard, enabled: boolean) => {
  card.enabled = enabled
  void persistProviders(activeTab.value)
}

const onTabChange = (idx: number) => {
  selectedIndex.value = idx
  const nextTab = tabs[idx]?.id
  if (nextTab) {
    void refreshProxyState(nextTab as ProviderTab)
    void loadProviderStats(nextTab as ProviderTab)
  }
}

</script>

<style scoped>
.main-version {
  margin: 32px auto 12px;
  text-align: center;
  color: var(--mac-text-secondary);
  font-size: 0.85rem;
}

/* 今日统计 */
.today-stats {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 12px;
  background: var(--mac-bg-secondary);
  font-size: 0.75rem;
  cursor: default;
}

.stats-icon {
  display: flex;
  align-items: center;
  color: var(--mac-accent, #0a84ff);
}

.stats-text {
  color: var(--mac-text-secondary);
  white-space: nowrap;
}

/* 刷新按钮旋转动画 */
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.spin-animation svg {
  animation: spin 1s linear infinite;
}

.ghost-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Body Log 开关按钮状态 */
.body-log-icon {
  transition: color 0.2s ease, background-color 0.2s ease;
}

.body-log-icon.body-log-on {
  color: var(--mac-accent, #0a84ff);
  background-color: rgba(10, 132, 255, 0.1);
  border-radius: 6px;
}

.body-log-icon.body-log-on:hover {
  background-color: rgba(10, 132, 255, 0.15);
}

/* Search Bar */
.search-bar {
  margin: 0 0 1rem 0;
}

.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  max-width: 400px;
}

.search-icon {
  position: absolute;
  left: 12px;
  width: 18px;
  height: 18px;
  color: var(--mac-text-tertiary);
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 8px 40px 8px 40px;
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  background: var(--mac-bg-primary);
  color: var(--mac-text-primary);
  font-size: 0.875rem;
  font-family: inherit;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.search-input:focus {
  outline: none;
  border-color: var(--mac-accent, #0a84ff);
  box-shadow: 0 0 0 2px rgba(10, 132, 255, 0.1);
}

.search-input::placeholder {
  color: var(--mac-text-tertiary);
}

.search-clear {
  position: absolute;
  right: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.search-clear:hover {
  background-color: var(--mac-bg-secondary);
  color: var(--mac-text-primary);
}

.search-clear svg {
  width: 14px;
  height: 14px;
}

</style>
