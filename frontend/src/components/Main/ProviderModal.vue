<template>
  <BaseModal
    :open="open"
    :title="isEditing ? t('components.main.form.editTitle') : t('components.main.form.createTitle')"
    @close="$emit('close')"
  >
    <form class="vendor-form" @submit.prevent="handleSubmit">
      <!-- 代理地址提示 -->
      <div class="form-tip">
        <div class="tip-icon">
          <svg viewBox="0 0 16 16" width="14" height="14">
            <path d="M8 1a7 7 0 100 14A7 7 0 008 1zm0 13A6 6 0 118 2a6 6 0 010 12zm0-9.5a.75.75 0 01.75.75v4a.75.75 0 01-1.5 0v-4A.75.75 0 018 4.5zm0 7.5a1 1 0 100-2 1 1 0 000 2z" fill="currentColor"/>
          </svg>
        </div>
        <div class="tip-content">
          <p class="tip-title">{{ t('components.main.form.proxyTip.title') }}</p>
          <p class="tip-text">{{ t('components.main.form.proxyTip.text') }}</p>
          <code class="tip-code">{{ PROXY_ADDRESS }}</code>
        </div>
      </div>

      <label class="form-field">
        <span>{{ t('components.main.form.labels.name') }}</span>
        <BaseInput
          v-model="form.name"
          type="text"
          :placeholder="t('components.main.form.placeholders.name')"
          required
          :disabled="isEditing"
        />
        <span class="field-hint">{{ t('components.main.form.hints.name') }}</span>
      </label>

      <label class="form-field">
        <span class="label-row">
          {{ t('components.main.form.labels.apiUrl') }}
          <span v-if="errors.apiUrl" class="field-error">
            {{ errors.apiUrl }}
          </span>
        </span>
        <BaseInput
          v-model="form.apiUrl"
          type="text"
          :placeholder="t('components.main.form.placeholders.apiUrl')"
          required
          :class="{ 'has-error': !!errors.apiUrl }"
        />
        <span class="field-hint">{{ t('components.main.form.hints.apiUrl') }}</span>
      </label>

      <label class="form-field">
        <span>{{ t('components.main.form.labels.apiKey') }}</span>
        <BaseInput
          v-model="form.apiKey"
          type="text"
          :placeholder="t('components.main.form.placeholders.apiKey')"
        />
        <span class="field-hint">{{ t('components.main.form.hints.apiKey') }}</span>
      </label>

      <label class="form-field">
        <span>{{ t('components.main.form.labels.officialSite') }}</span>
        <BaseInput
          v-model="form.officialSite"
          type="text"
          :placeholder="t('components.main.form.placeholders.officialSite')"
        />
      </label>

      <div class="form-field">
        <span>{{ t('components.main.form.labels.icon') }}</span>
        <Listbox v-model="form.icon" v-slot="{ open: listOpen }">
          <div class="icon-select">
            <ListboxButton class="icon-select-button">
              <span class="icon-preview" v-html="iconSvg(form.icon)" aria-hidden="true"></span>
              <span class="icon-select-label">{{ form.icon }}</span>
              <svg viewBox="0 0 20 20" aria-hidden="true">
                <path d="M6 8l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill="none" />
              </svg>
            </ListboxButton>
            <ListboxOptions v-if="listOpen" class="icon-select-options">
              <ListboxOption
                v-for="iconName in iconOptions"
                :key="iconName"
                :value="iconName"
                v-slot="{ active, selected }"
              >
                <div :class="['icon-option', { active, selected }]">
                  <span class="icon-preview" v-html="iconSvg(iconName)" aria-hidden="true"></span>
                  <span class="icon-name">{{ iconName }}</span>
                </div>
              </ListboxOption>
            </ListboxOptions>
          </div>
        </Listbox>
      </div>

      <div class="form-field">
        <ModelWhitelistEditor v-model="form.supportedModels" />
      </div>

      <div class="form-field">
        <ModelMappingEditor v-model="form.modelMapping" />
      </div>

      <div class="form-field">
        <span>{{ t('components.main.form.labels.level') }}</span>
        <div class="level-select-wrapper">
          <select v-model.number="form.level" class="level-select">
            <option v-for="lv in 10" :key="lv" :value="lv">
              Level {{ lv }} - {{ t(`components.main.levelDesc.${levelDescKeys[lv - 1]}`) }}
            </option>
          </select>
        </div>
        <span class="field-hint">{{ t('components.main.form.hints.level') }}</span>
      </div>

      <!-- Color Customization -->
      <div class="form-field color-section">
        <span>{{ t('components.main.form.labels.colors') }}</span>
        <div class="color-pickers">
          <label class="color-picker">
            <span class="color-label">{{ t('components.main.form.labels.tint') }}</span>
            <div class="color-input-wrapper">
              <input
                v-model="form.tint"
                type="color"
                class="color-input-native"
              />
              <BaseInput
                v-model="form.tint"
                type="text"
                :placeholder="t('components.main.form.placeholders.tint')"
                class="color-input-text"
              />
            </div>
          </label>

          <label class="color-picker">
            <span class="color-label">{{ t('components.main.form.labels.accent') }}</span>
            <div class="color-input-wrapper">
              <input
                v-model="form.accent"
                type="color"
                class="color-input-native"
              />
              <BaseInput
                v-model="form.accent"
                type="text"
                :placeholder="t('components.main.form.placeholders.accent')"
                class="color-input-text"
              />
            </div>
          </label>
        </div>

        <!-- Preview Card -->
        <div class="color-preview" :style="{ backgroundColor: form.tint || '#f0f0f0' }">
          <div class="preview-icon" :style="{ color: form.accent || '#0a84ff' }">
            <span v-html="iconSvg(form.icon)" aria-hidden="true"></span>
          </div>
          <p class="preview-name" :style="{ color: form.accent || '#0a84ff' }">
            {{ form.name || t('components.main.form.placeholders.name') }}
          </p>
        </div>
        <span class="field-hint">{{ t('components.main.form.hints.colors') }}</span>
      </div>

      <div class="form-field switch-field">
        <span>{{ t('components.main.form.labels.enabled') }}</span>
        <div class="switch-inline">
          <label class="mac-switch">
            <input type="checkbox" v-model="form.enabled" />
            <span></span>
          </label>
          <span class="switch-text">
            {{ form.enabled ? t('components.main.form.switch.on') : t('components.main.form.switch.off') }}
          </span>
        </div>
      </div>

      <footer class="form-actions">
        <BaseButton variant="outline" type="button" @click="$emit('close')">
          {{ t('components.main.form.actions.cancel') }}
        </BaseButton>
        <BaseButton type="submit">
          {{ t('components.main.form.actions.save') }}
        </BaseButton>
      </footer>
    </form>
  </BaseModal>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Listbox, ListboxButton, ListboxOptions, ListboxOption } from '@headlessui/vue'
import BaseModal from '../common/BaseModal.vue'
import BaseInput from '../common/BaseInput.vue'
import BaseButton from '../common/BaseButton.vue'
import ModelWhitelistEditor from '../common/ModelWhitelistEditor.vue'
import ModelMappingEditor from '../common/ModelMappingEditor.vue'
import lobeIcons from '../../icons/lobeIconMap'

const { t } = useI18n()
const PROXY_ADDRESS = 'http://127.0.0.1:18100'

export interface ProviderFormData {
  name: string
  apiUrl: string
  apiKey: string
  officialSite: string
  icon: string
  enabled: boolean
  supportedModels: Record<string, boolean>
  modelMapping: Record<string, string>
  level: number
  tint: string
  accent: string
}

const props = defineProps<{
  open: boolean
  isEditing: boolean
  initialData?: Partial<ProviderFormData>
}>()

const emit = defineEmits<{
  close: []
  save: [data: ProviderFormData]
}>()

const iconOptions = Object.keys(lobeIcons).sort((a, b) => a.localeCompare(b))
const defaultIconKey = iconOptions[0] ?? 'aicoding'

const levelDescKeys = [
  'highest', 'high', 'mediumHigh', 'medium', 'normal',
  'mediumLow', 'low', 'lower', 'veryLow', 'lowest',
]

const defaultFormValues = (): ProviderFormData => ({
  name: '',
  apiUrl: '',
  apiKey: '',
  officialSite: '',
  icon: defaultIconKey,
  enabled: true,
  supportedModels: {},
  modelMapping: {},
  level: 1,
  tint: '#f0f0f0',
  accent: '#0a84ff',
})

const form = reactive<ProviderFormData>(defaultFormValues())
const errors = reactive({ apiUrl: '' })

// 当 modal 打开或初始数据变化时重置表单
watch(
  () => [props.open, props.initialData],
  ([isOpen]) => {
    if (isOpen) {
      errors.apiUrl = ''
      if (props.initialData) {
        Object.assign(form, defaultFormValues(), props.initialData)
      } else {
        Object.assign(form, defaultFormValues())
      }
    }
  },
  { immediate: true }
)

const iconSvg = (name: string) => {
  if (!name) return ''
  return lobeIcons[name.toLowerCase()] ?? ''
}

const handleSubmit = () => {
  const apiUrl = form.apiUrl.trim()
  errors.apiUrl = ''

  // 验证 URL
  try {
    const parsed = new URL(apiUrl)
    if (!/^https?:/.test(parsed.protocol)) throw new Error('protocol')
  } catch {
    errors.apiUrl = t('components.main.form.errors.invalidUrl')
    return
  }

  emit('save', {
    name: form.name.trim(),
    apiUrl,
    apiKey: form.apiKey.trim(),
    officialSite: form.officialSite.trim(),
    icon: (form.icon || defaultIconKey).toString().trim().toLowerCase() || defaultIconKey,
    enabled: form.enabled,
    supportedModels: form.supportedModels || {},
    modelMapping: form.modelMapping || {},
    level: form.level || 1,
    tint: form.tint || '#f0f0f0',
    accent: form.accent || '#0a84ff',
  })
}
</script>

<style scoped>
.vendor-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-tip {
  display: flex;
  gap: 12px;
  padding: 12px 14px;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
  border-radius: 8px;
  border-left: 3px solid var(--mac-accent, #0a84ff);
}

.tip-icon {
  flex-shrink: 0;
  color: var(--mac-accent, #0a84ff);
  margin-top: 2px;
}

.tip-content {
  flex: 1;
  min-width: 0;
}

.tip-title {
  margin: 0 0 4px 0;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--mac-text-primary);
}

.tip-text {
  margin: 0 0 8px 0;
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
  line-height: 1.4;
}

.tip-code {
  display: inline-block;
  padding: 4px 8px;
  background: var(--mac-bg-primary);
  border: 1px solid var(--mac-border);
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: var(--mac-accent, #0a84ff);
  user-select: all;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.form-field > span:first-child {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--mac-text-primary);
}

.label-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.field-error {
  font-size: 0.75rem;
  color: var(--mac-error, #ff3b30);
}

.field-hint {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
}

.level-select-wrapper {
  margin-top: 6px;
}

.level-select {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  background: var(--mac-bg-primary);
  color: var(--mac-text-primary);
  font-size: 0.875rem;
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='%23666' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  padding-right: 36px;
}

.level-select:focus {
  outline: none;
  border-color: var(--mac-accent);
  box-shadow: 0 0 0 2px rgba(var(--mac-accent-rgb), 0.2);
}

.switch-field {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.switch-inline {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.switch-text {
  font-size: 0.875rem;
  color: var(--mac-text-secondary);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 0.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--mac-border);
}

/* Icon Select */
.icon-select {
  position: relative;
}

.icon-select-button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  background: var(--mac-bg-primary);
  color: var(--mac-text-primary);
  font-size: 0.875rem;
  cursor: pointer;
  text-align: left;
}

.icon-select-button:hover {
  border-color: var(--mac-border-hover);
}

.icon-select-button svg:last-child {
  margin-left: auto;
  width: 16px;
  height: 16px;
  color: var(--mac-text-secondary);
}

.icon-preview {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-preview :deep(svg) {
  width: 100%;
  height: 100%;
}

.icon-select-label {
  flex: 1;
}

.icon-select-options {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  z-index: 50;
  margin-top: 4px;
  max-height: 200px;
  overflow-y: auto;
  background: var(--mac-bg-primary);
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.icon-option {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 8px 12px;
  cursor: pointer;
}

.icon-option.active {
  background: var(--mac-bg-secondary);
}

.icon-option.selected {
  color: var(--mac-accent);
}

.icon-name {
  font-size: 0.875rem;
}

/* Mac Switch */
.mac-switch {
  position: relative;
  display: inline-block;
  cursor: pointer;
}

.mac-switch input {
  opacity: 0;
  width: 0;
  height: 0;
  position: absolute;
}

.mac-switch span {
  display: block;
  width: 44px;
  height: 24px;
  background: var(--switch-off, #e5e5ea);
  border-radius: 12px;
  position: relative;
  transition: background-color 0.2s ease;
}

.mac-switch span::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  background: white;
  border-radius: 50%;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  transition: transform 0.2s ease;
}

.mac-switch input:checked + span {
  background: var(--mac-accent, #0a84ff);
}

.mac-switch input:checked + span::after {
  transform: translateX(20px);
}

/* Color Section */
.color-section {
  margin-top: 0.5rem;
}

.color-pickers {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
  margin-bottom: 1rem;
}

.color-picker {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.color-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--mac-text-secondary);
}

.color-input-wrapper {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.color-input-native {
  width: 48px;
  height: 38px;
  border: 1px solid var(--mac-border);
  border-radius: 6px;
  cursor: pointer;
  padding: 2px;
  background: var(--mac-bg-primary);
}

.color-input-native::-webkit-color-swatch-wrapper {
  padding: 0;
}

.color-input-native::-webkit-color-swatch {
  border: none;
  border-radius: 4px;
}

.color-input-text {
  flex: 1;
}

.color-preview {
  padding: 1.5rem;
  border-radius: 8px;
  text-align: center;
  border: 1px solid var(--mac-border);
  transition: background-color 0.2s ease;
  margin-top: 0.5rem;
}

.preview-icon {
  font-size: 2rem;
  margin-bottom: 0.5rem;
  transition: color 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 2.5rem;
}

.preview-icon :deep(svg) {
  width: 2rem;
  height: 2rem;
}

.preview-name {
  font-weight: 500;
  font-size: 0.9375rem;
  margin: 0;
  transition: color 0.2s ease;
}
</style>
