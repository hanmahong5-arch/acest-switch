<script setup lang="ts">
import { RouterView, useRouter } from 'vue-router'
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider, darkTheme, lightTheme } from 'naive-ui'
import type { GlobalTheme } from 'naive-ui'

const router = useRouter()
const isDark = ref(false)

const applyTheme = () => {
  const userTheme = localStorage.getItem('theme')
  const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches

  isDark.value = userTheme === 'dark' || (!userTheme && systemPrefersDark)
  document.documentElement.classList.toggle('dark', isDark.value)
}

// Naive UI theme based on dark mode
const naiveTheme = computed<GlobalTheme | null>(() => isDark.value ? darkTheme : null)

// Global keyboard shortcuts handler
const handleKeyDown = (event: KeyboardEvent) => {
  // Cmd+, (Mac) or Ctrl+, (Windows/Linux) - Open Settings
  if ((event.metaKey || event.ctrlKey) && event.key === ',') {
    event.preventDefault()
    router.push('/settings')
  }
}

onMounted(() => {
  applyTheme()

  // Listen for system theme changes
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    applyTheme()
  })

  // Listen for manual theme toggle
  window.addEventListener('theme-changed', () => {
    applyTheme()
  })

  // Register global keyboard shortcuts
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  // Clean up keyboard event listener
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <n-config-provider :theme="naiveTheme">
    <n-message-provider>
      <n-dialog-provider>
        <RouterView />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
