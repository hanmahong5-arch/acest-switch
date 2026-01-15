<template>
  <article
    :class="['automation-card', { dragging: isDragging }]"
    draggable="true"
    @dragstart="$emit('drag-start')"
    @dragend="$emit('drag-end')"
    @drop="$emit('drop')"
  >
    <div class="card-leading">
      <div class="card-icon" :style="{ backgroundColor: card.tint, color: card.accent }">
        <span v-if="!iconSvg" class="icon-fallback">
          {{ initials }}
        </span>
        <span v-else class="icon-svg" v-html="iconSvg" aria-hidden="true"></span>
      </div>
      <div class="card-text">
        <div class="card-title-row">
          <p class="card-title">{{ card.name }}</p>
          <span
            v-if="card.officialSite"
            class="card-site"
            role="button"
            tabindex="0"
            @click.stop="openSite"
            @keydown.enter.stop.prevent="openSite"
            @keydown.space.stop.prevent="openSite"
          >
            {{ formattedSite }}
          </span>
        </div>
        <p class="card-metrics">
          <template v-if="stats.state !== 'ready'">
            {{ stats.message }}
          </template>
          <template v-else>
            <span
              v-if="stats.successRateLabel"
              class="card-success-rate"
              :class="stats.successRateClass"
            >
              {{ stats.successRateLabel }}
            </span>
            <span class="card-metric-separator" aria-hidden="true">·</span>
            <span>{{ stats.requests }}</span>
            <span class="card-metric-separator" aria-hidden="true">·</span>
            <span>{{ stats.tokens }}</span>
            <span class="card-metric-separator" aria-hidden="true">·</span>
            <span>{{ stats.cost }}</span>
          </template>
        </p>
      </div>
    </div>
    <div class="card-actions">
      <label class="mac-switch sm">
        <input
          type="checkbox"
          :checked="card.enabled"
          @change="$emit('toggle-enabled', !card.enabled)"
        />
        <span></span>
      </label>
      <button class="ghost-icon" @click="$emit('configure')">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M11.983 2.25a1.125 1.125 0 011.077.81l.563 2.101a7.482 7.482 0 012.326 1.343l2.08-.621a1.125 1.125 0 011.356.651l1.313 3.207a1.125 1.125 0 01-.442 1.339l-1.86 1.205a7.418 7.418 0 010 2.686l1.86 1.205a1.125 1.125 0 01.442 1.339l-1.313 3.207a1.125 1.125 0 01-1.356.651l-2.08-.621a7.482 7.482 0 01-2.326 1.343l-.563 2.101a1.125 1.125 0 01-1.077.81h-2.634a1.125 1.125 0 01-1.077-.81l-.563-2.101a7.482 7.482 0 01-2.326-1.343l-2.08.621a1.125 1.125 0 01-1.356-.651l-1.313-3.207a1.125 1.125 0 01.442-1.339l1.86-1.205a7.418 7.418 0 010-2.686l-1.86-1.205a1.125 1.125 0 01-.442-1.339l1.313-3.207a1.125 1.125 0 011.356-.651l2.08.621a7.482 7.482 0 012.326-1.343l.563-2.101a1.125 1.125 0 011.077-.81h2.634z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <button class="ghost-icon" @click="$emit('remove')">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M9 3h6m-7 4h8m-6 0v11m4-11v11M5 7h14l-.867 12.138A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.862L5 7z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Browser } from '@wailsio/runtime'
import lobeIcons from '../../icons/lobeIconMap'

export interface ProviderCardData {
  id: number
  name: string
  icon: string
  tint: string
  accent: string
  enabled: boolean
  officialSite?: string
}

export interface ProviderStats {
  state: 'loading' | 'empty' | 'ready'
  message?: string
  successRateLabel?: string
  successRateClass?: string
  requests?: string
  tokens?: string
  cost?: string
}

const props = defineProps<{
  card: ProviderCardData
  stats: ProviderStats
  isDragging?: boolean
}>()

defineEmits<{
  'toggle-enabled': [enabled: boolean]
  'configure': []
  'remove': []
  'drag-start': []
  'drag-end': []
  'drop': []
}>()

const iconSvg = computed(() => lobeIcons[props.card.icon] ?? '')

const initials = computed(() => {
  const name = props.card.name || ''
  const words = name.trim().split(/\s+/)
  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase()
  }
  return name.slice(0, 2).toUpperCase()
})

const formattedSite = computed(() => {
  const site = props.card.officialSite || ''
  try {
    const url = new URL(site.startsWith('http') ? site : `https://${site}`)
    return url.hostname.replace(/^www\./, '')
  } catch {
    return site
  }
})

const openSite = () => {
  if (props.card.officialSite) {
    const url = props.card.officialSite.startsWith('http')
      ? props.card.officialSite
      : `https://${props.card.officialSite}`
    Browser.OpenURL(url)
  }
}
</script>

<style scoped>
.automation-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1rem;
  background: var(--mac-bg-secondary);
  border: 1px solid var(--mac-border);
  border-radius: 0.75rem;
  transition: box-shadow 0.15s ease, border-color 0.15s ease, opacity 0.15s ease;
  cursor: grab;
}

.automation-card:hover {
  border-color: var(--mac-border-hover, var(--mac-border));
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.automation-card.dragging {
  opacity: 0.5;
  cursor: grabbing;
}

.card-leading {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
}

.card-icon {
  width: 40px;
  height: 40px;
  border-radius: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-fallback {
  font-size: 0.875rem;
  font-weight: 600;
}

.icon-svg {
  width: 24px;
  height: 24px;
}

.icon-svg :deep(svg) {
  width: 100%;
  height: 100%;
}

.card-text {
  flex: 1;
  min-width: 0;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.card-title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--mac-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-site {
  font-size: 0.75rem;
  color: var(--mac-text-secondary);
  cursor: pointer;
  white-space: nowrap;
}

.card-site:hover {
  color: var(--mac-accent, #0a84ff);
  text-decoration: underline;
}

.card-metrics {
  margin: 0.25rem 0 0;
  font-size: 0.8125rem;
  color: var(--mac-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.card-metric-separator {
  margin: 0 0.25rem;
  opacity: 0.5;
}

.card-success-rate {
  font-weight: 500;
}

.card-success-rate.success,
.card-success-rate.success-good {
  color: var(--mac-success, #34c759);
}

.card-success-rate.warning,
.card-success-rate.success-warn {
  color: var(--mac-warning, #ff9500);
}

.card-success-rate.error,
.card-success-rate.success-bad {
  color: var(--mac-error, #ff3b30);
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.ghost-icon {
  width: 32px;
  height: 32px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--mac-text-secondary);
  cursor: pointer;
  border-radius: 0.375rem;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s ease, background-color 0.15s ease;
}

.ghost-icon:hover {
  color: var(--mac-text-primary);
  background: var(--mac-bg-tertiary, rgba(0, 0, 0, 0.05));
}

.ghost-icon svg {
  width: 18px;
  height: 18px;
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
  background: var(--mac-switch-off, #e5e5ea);
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

.mac-switch.sm span {
  width: 36px;
  height: 20px;
  border-radius: 10px;
}

.mac-switch.sm span::after {
  width: 16px;
  height: 16px;
}

.mac-switch.sm input:checked + span::after {
  transform: translateX(16px);
}
</style>
