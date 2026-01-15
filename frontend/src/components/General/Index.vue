<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Dialogs } from '@wailsio/runtime'
import ListItem from '../Setting/ListRow.vue'
import LanguageSwitcher from '../Setting/LanguageSwitcher.vue'
import ThemeSetting from '../Setting/ThemeSetting.vue'
import { fetchAppSettings, saveAppSettings, type AppSettings } from '../../services/appSettings'
import {
  fetchConfigImportStatus,
  fetchConfigImportStatusForFile,
  importFromCcSwitch,
  importFromCustomFile,
  type ConfigImportResult,
  type ConfigImportStatus,
} from '../../services/configImport'
import {
  generateShareLink,
  copyToClipboard,
  importFromDeepLink,
  type ExportOptions,
} from '../../services/importExport'
import { showToast } from '../../utils/toast'
import BaseButton from '../common/BaseButton.vue'

const router = useRouter()
const { t } = useI18n()
const heatmapEnabled = ref(true)
const homeTitleVisible = ref(true)
const autoStartEnabled = ref(false)
const bodyLogEnabled = ref(false)
const settingsLoading = ref(true)
const saveBusy = ref(false)
const importStatus = ref<ConfigImportStatus | null>(null)
const customImportStatus = ref<ConfigImportStatus | null>(null)
const importBusy = ref(false)

// Export/Import Deep Link state
const exportIncludeProviders = ref(true)
const exportIncludeMCP = ref(true)
const exportFilterAPIKeys = ref(true)
const exportPlatforms = ref<string[]>(['claude', 'codex', 'gemini-cli'])
const generatedShareLink = ref('')
const exportBusy = ref(false)
const deepLinkInput = ref('')
const deepLinkImportBusy = ref(false)

const goBack = () => {
  router.push('/')
}

const loadAppSettings = async () => {
  settingsLoading.value = true
  try {
    const data = await fetchAppSettings()
    heatmapEnabled.value = data?.show_heatmap ?? true
    homeTitleVisible.value = data?.show_home_title ?? true
    autoStartEnabled.value = data?.auto_start ?? false
    bodyLogEnabled.value = data?.enable_body_log ?? false
  } catch (error) {
    console.error('failed to load app settings', error)
    heatmapEnabled.value = true
    homeTitleVisible.value = true
    autoStartEnabled.value = false
    bodyLogEnabled.value = false
  } finally {
    settingsLoading.value = false
  }
}

const persistAppSettings = async () => {
  if (settingsLoading.value || saveBusy.value) return
  saveBusy.value = true
  try {
    const payload: AppSettings = {
      show_heatmap: heatmapEnabled.value,
      show_home_title: homeTitleVisible.value,
      auto_start: autoStartEnabled.value,
      enable_body_log: bodyLogEnabled.value,
    }
    await saveAppSettings(payload)
    window.dispatchEvent(new CustomEvent('app-settings-updated'))
  } catch (error) {
    console.error('failed to save app settings', error)
  } finally {
    saveBusy.value = false
  }
}

onMounted(() => {
  void loadAppSettings()
  void loadImportStatus()
})

const loadImportStatus = async () => {
  try {
    importStatus.value = await fetchConfigImportStatus()
  } catch (error) {
    console.error('failed to load cc-switch import status', error)
    importStatus.value = null
  }
}

const activeImportStatus = computed(() => customImportStatus.value ?? importStatus.value)
const hasCustomSelection = computed(() => Boolean(customImportStatus.value))
const shouldShowDefaultMissingHint = computed(() => {
  if (hasCustomSelection.value) return false
  const status = importStatus.value
  if (!status) return false
  return !status.config_exists
})
const pendingProviders = computed(() => activeImportStatus.value?.pending_provider_count ?? 0)
const pendingServers = computed(() => activeImportStatus.value?.pending_mcp_count ?? 0)
const configPath = computed(() => activeImportStatus.value?.config_path ?? '')
const canImportDefault = computed(() => {
  const status = importStatus.value
  if (!status) return false
  return Boolean(status.pending_providers || status.pending_mcp)
})
const canImportCustom = computed(() => {
  const status = customImportStatus.value
  if (!status) return false
  return Boolean(status.pending_providers || status.pending_mcp)
})
const canImportActive = computed(() =>
  hasCustomSelection.value ? canImportCustom.value : canImportDefault.value,
)
const showImportRow = computed(() => Boolean(importStatus.value) || hasCustomSelection.value)
const importPathLabel = computed(() => {
  if (!configPath.value) return ''
  return t('components.general.import.path', { path: configPath.value })
})
const importDetailLabel = computed(() => {
  if (shouldShowDefaultMissingHint.value) {
    return t('components.general.import.missingDefault')
  }
  if (!activeImportStatus.value) {
    return t('components.general.import.noFile')
  }
  const detail = canImportActive.value
    ? t('components.general.import.detail', {
        providers: pendingProviders.value,
        servers: pendingServers.value,
      })
    : t('components.general.import.synced')
  if (!importPathLabel.value) return detail
  return `${importPathLabel.value} · ${detail}`
})
const importButtonText = computed(() => {
  if (importBusy.value) {
    return t('components.general.import.importing')
  }
  if (hasCustomSelection.value) {
    return t('components.general.import.confirm')
  }
  if (shouldShowDefaultMissingHint.value || canImportDefault.value) {
    return t('components.general.import.cta')
  }
  return t('components.general.import.syncedButton')
})
const primaryButtonDisabled = computed(() => importBusy.value || !canImportActive.value)
const secondaryButtonLabel = computed(() =>
  hasCustomSelection.value
    ? t('components.general.import.clear')
    : t('components.general.import.upload'),
)
const secondaryButtonVariant = computed(() => 'outline' as const)

const processImportResult = async (result?: ConfigImportResult | null) => {
  if (!result) return
  if (hasCustomSelection.value && result.status?.config_path === customImportStatus.value?.config_path) {
    customImportStatus.value = result.status
  } else {
    importStatus.value = result.status
  }
  const importedProviders = result.imported_providers ?? 0
  const importedServers = result.imported_mcp ?? 0
  if (importedProviders > 0 || importedServers > 0) {
    showToast(
      t('components.main.importConfig.success', {
        providers: importedProviders,
        servers: importedServers,
      })
    )
  } else if (result.status?.config_exists) {
    showToast(t('components.main.importConfig.empty'))
  }
  await loadImportStatus()
}

const handleImportClick = async () => {
  if (importBusy.value || !importStatus.value || !canImportDefault.value) return
  importBusy.value = true
  try {
    const result = await importFromCcSwitch()
    await processImportResult(result)
  } catch (error) {
    console.error('failed to import cc-switch config', error)
    showToast(t('components.main.importConfig.error'), 'error')
  } finally {
    importBusy.value = false
  }
}

const handleConfirmCustomImport = async () => {
  const path = customImportStatus.value?.config_path
  if (!path || importBusy.value || !canImportCustom.value) return
  importBusy.value = true
  try {
    const result = await importFromCustomFile(path)
    await processImportResult(result)
  } catch (error) {
    console.error('failed to import custom cc-switch config', error)
    showToast(t('components.main.importConfig.error'), 'error')
  } finally {
    importBusy.value = false
  }
}

const handlePrimaryImport = async () => {
  if (hasCustomSelection.value) {
    await handleConfirmCustomImport()
  } else {
    await handleImportClick()
  }
}

const handleUploadClick = async () => {
  if (importBusy.value) return
  let selectedPath = ''
  try {
    const selection = await Dialogs.OpenFile({
      Title: t('components.general.import.uploadTitle'),
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsOtherFiletypes: false,
      Filters: [
        {
          DisplayName: 'JSON (*.json)',
          Pattern: '*.json',
        },
      ],
      AllowsMultipleSelection: false,
    })
    selectedPath = Array.isArray(selection) ? selection[0] : selection
    if (!selectedPath) return
    const status = await fetchConfigImportStatusForFile(selectedPath)
    customImportStatus.value = status
  } catch (error) {
    console.error('failed to load custom cc-switch config status', error)
    showToast(t('components.general.import.loadError'), 'error')
  }
}

const clearCustomSelection = () => {
  customImportStatus.value = null
}

const handleSecondaryImportAction = async () => {
  if (hasCustomSelection.value) {
    clearCustomSelection()
  } else {
    await handleUploadClick()
  }
}

// Export/Import Deep Link functions
const handleGenerateShareLink = async () => {
  if (exportBusy.value) return
  exportBusy.value = true
  generatedShareLink.value = ''
  try {
    const options: ExportOptions = {
      include_providers: exportIncludeProviders.value,
      include_mcp: exportIncludeMCP.value,
      platforms: exportPlatforms.value,
      filter_api_keys: exportFilterAPIKeys.value,
    }
    const shareLink = await generateShareLink(options)
    generatedShareLink.value = shareLink
    showToast(t('components.general.export.generated'))
  } catch (error) {
    console.error('failed to generate share link', error)
    showToast(t('components.general.export.error'), 'error')
  } finally {
    exportBusy.value = false
  }
}

const handleCopyShareLink = async () => {
  if (!generatedShareLink.value) return
  try {
    await copyToClipboard(generatedShareLink.value)
    showToast(t('components.general.export.copied'))
  } catch (error) {
    console.error('failed to copy to clipboard', error)
    showToast(t('components.general.export.copyError'), 'error')
  }
}

const togglePlatform = (platform: string) => {
  const index = exportPlatforms.value.indexOf(platform)
  if (index > -1) {
    exportPlatforms.value.splice(index, 1)
  } else {
    exportPlatforms.value.push(platform)
  }
}

const handleImportFromDeepLink = async () => {
  const link = deepLinkInput.value.trim()
  if (!link || deepLinkImportBusy.value) return
  deepLinkImportBusy.value = true
  try {
    const result = await importFromDeepLink(link)
    const importedProviders = result.imported_providers ?? 0
    const importedServers = result.imported_mcp ?? 0
    if (importedProviders > 0 || importedServers > 0) {
      showToast(
        t('components.main.importConfig.success', {
          providers: importedProviders,
          servers: importedServers,
        })
      )
      deepLinkInput.value = ''
      await loadImportStatus()
    } else {
      showToast(t('components.main.importConfig.empty'))
    }
  } catch (error) {
    console.error('failed to import from deep link', error)
    showToast(t('components.general.import.deepLinkError'), 'error')
  } finally {
    deepLinkImportBusy.value = false
  }
}
</script>

<template>
  <div class="main-shell general-shell">
    <div class="global-actions">
      <p class="global-eyebrow">{{ $t('components.general.title.application') }}</p>
      <button class="ghost-icon" :aria-label="$t('components.general.buttons.back')" @click="goBack">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M15 18l-6-6 6-6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>

    <div class="general-page">
      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.application') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.heatmap')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="heatmapEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.homeTitle')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="homeTitleVisible"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem :label="$t('components.general.label.autoStart')">
            <label class="mac-switch">
              <input
                type="checkbox"
                :disabled="settingsLoading || saveBusy"
                v-model="autoStartEnabled"
                @change="persistAppSettings"
              />
              <span></span>
            </label>
          </ListItem>
          <ListItem
            v-if="showImportRow"
            :label="$t('components.general.import.label')"
            :sub-label="importDetailLabel"
          >
            <div class="import-actions">
              <BaseButton
                size="sm"
                variant="outline"
                type="button"
                :disabled="primaryButtonDisabled"
                @click="handlePrimaryImport"
              >
                {{ importButtonText }}
              </BaseButton>
              <BaseButton
                size="sm"
                :variant="secondaryButtonVariant"
                type="button"
                :disabled="importBusy"
                @click="handleSecondaryImportAction"
              >
                {{ secondaryButtonLabel }}
              </BaseButton>
              <BaseButton
                v-if="hasCustomSelection"
                size="sm"
                variant="outline"
                type="button"
                :disabled="importBusy"
                @click="handleUploadClick"
              >
                {{ $t('components.general.import.reupload') }}
              </BaseButton>
            </div>
          </ListItem>

        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.exterior') }}</h2>
        <div class="mac-panel">
          <ListItem :label="$t('components.general.label.language')">
            <LanguageSwitcher />
          </ListItem>
          <ListItem :label="$t('components.general.label.theme')">
            <ThemeSetting />
          </ListItem>
        </div>
      </section>

      <section>
        <h2 class="mac-section-title">{{ $t('components.general.title.sharing') }}</h2>

        <!-- Export Section -->
        <div class="mac-panel export-section">
          <h3 class="section-subtitle">{{ $t('components.general.export.title') }}</h3>
          <p class="section-description">{{ $t('components.general.export.description') }}</p>

          <div class="export-options">
            <div class="option-group">
              <h4 class="option-label">{{ $t('components.general.export.includeLabel') }}</h4>
              <label class="checkbox-label">
                <input type="checkbox" v-model="exportIncludeProviders" />
                <span>{{ $t('components.general.export.includeProviders') }}</span>
              </label>
              <label class="checkbox-label">
                <input type="checkbox" v-model="exportIncludeMCP" />
                <span>{{ $t('components.general.export.includeMCP') }}</span>
              </label>
              <label class="checkbox-label">
                <input type="checkbox" v-model="exportFilterAPIKeys" />
                <span>{{ $t('components.general.export.filterAPIKeys') }}</span>
              </label>
            </div>

            <div class="option-group">
              <h4 class="option-label">{{ $t('components.general.export.platformsLabel') }}</h4>
              <div class="platform-chips">
                <label
                  v-for="platform in ['claude', 'codex', 'gemini-cli']"
                  :key="platform"
                  class="chip-label"
                  :class="{ active: exportPlatforms.includes(platform) }"
                >
                  <input
                    type="checkbox"
                    :checked="exportPlatforms.includes(platform)"
                    @change="togglePlatform(platform)"
                  />
                  <span>{{ $t(`components.general.export.platform.${platform}`) }}</span>
                </label>
              </div>
            </div>

            <div class="action-row">
              <BaseButton
                variant="primary"
                size="md"
                :disabled="exportBusy || (!exportIncludeProviders && !exportIncludeMCP)"
                @click="handleGenerateShareLink"
              >
                {{ exportBusy ? $t('components.general.export.generating') : $t('components.general.export.generate') }}
              </BaseButton>
            </div>

            <div v-if="generatedShareLink" class="generated-link">
              <div class="link-display">
                <input
                  type="text"
                  readonly
                  :value="generatedShareLink"
                  class="link-input"
                  @click="($event.target as HTMLInputElement).select()"
                />
                <BaseButton variant="outline" size="sm" @click="handleCopyShareLink">
                  {{ $t('components.general.export.copy') }}
                </BaseButton>
              </div>
              <p class="link-hint">{{ $t('components.general.export.hint') }}</p>
            </div>
          </div>
        </div>

        <!-- Import from Deep Link Section -->
        <div class="mac-panel import-deeplink-section">
          <h3 class="section-subtitle">{{ $t('components.general.import.deepLinkTitle') }}</h3>
          <p class="section-description">{{ $t('components.general.import.deepLinkDescription') }}</p>

          <div class="deeplink-input-row">
            <input
              type="text"
              v-model="deepLinkInput"
              :placeholder="$t('components.general.import.deepLinkPlaceholder')"
              class="deeplink-input"
              :disabled="deepLinkImportBusy"
              @keyup.enter="handleImportFromDeepLink"
            />
            <BaseButton
              variant="primary"
              size="md"
              :disabled="!deepLinkInput.trim() || deepLinkImportBusy"
              @click="handleImportFromDeepLink"
            >
              {{ deepLinkImportBusy ? $t('components.general.import.importing') : $t('components.general.import.importButton') }}
            </BaseButton>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.import-actions {
  display: flex;
  gap: 0.35rem;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.import-actions .btn {
  min-width: 56px;
  padding: 0.3rem 0.75rem;
  font-size: 0.7rem;
}

.import-actions .btn-outline,
.import-actions .btn-ghost {
  padding-inline: 0.75rem;
}

/* Export/Import Deep Link Sections */
.export-section,
.import-deeplink-section {
  margin-top: 1rem;
  padding: 1.25rem;
}

.section-subtitle {
  font-size: 0.95rem;
  font-weight: 600;
  margin: 0 0 0.5rem 0;
  color: var(--text-primary);
}

.section-description {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin: 0 0 1.25rem 0;
  line-height: 1.5;
}

.export-options {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.option-group {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}

.option-label {
  font-size: 0.85rem;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.85rem;
  color: var(--text-primary);
  cursor: pointer;
}

.checkbox-label input[type='checkbox'] {
  cursor: pointer;
  width: 1rem;
  height: 1rem;
}

.platform-chips {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.chip-label {
  display: inline-flex;
  align-items: center;
  padding: 0.4rem 0.85rem;
  border: 1px solid var(--border-secondary);
  border-radius: 6px;
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.2s ease;
  background-color: var(--bg-secondary);
}

.chip-label input[type='checkbox'] {
  display: none;
}

.chip-label.active {
  background-color: var(--accent-primary);
  color: white;
  border-color: var(--accent-primary);
}

.chip-label:hover {
  border-color: var(--accent-primary);
}

.action-row {
  display: flex;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.generated-link {
  margin-top: 1rem;
  padding: 1rem;
  background-color: var(--bg-tertiary);
  border-radius: 8px;
  border: 1px solid var(--border-secondary);
}

.link-display {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.link-input {
  flex: 1;
  padding: 0.6rem 0.85rem;
  border: 1px solid var(--border-secondary);
  border-radius: 6px;
  font-size: 0.8rem;
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
  background-color: var(--bg-primary);
  color: var(--text-primary);
  cursor: text;
}

.link-input:focus {
  outline: 2px solid var(--accent-primary);
  outline-offset: 0;
}

.link-hint {
  margin: 0.75rem 0 0 0;
  font-size: 0.75rem;
  color: var(--text-tertiary);
  line-height: 1.4;
}

.deeplink-input-row {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.deeplink-input {
  flex: 1;
  padding: 0.7rem 1rem;
  border: 1px solid var(--border-secondary);
  border-radius: 6px;
  font-size: 0.85rem;
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

.deeplink-input:focus {
  outline: 2px solid var(--accent-primary);
  outline-offset: 0;
  border-color: var(--accent-primary);
}

.deeplink-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.deeplink-input::placeholder {
  color: var(--text-tertiary);
  opacity: 0.7;
}
</style>
