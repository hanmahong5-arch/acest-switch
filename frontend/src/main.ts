import { createApp } from 'vue'
import { createPinia } from 'pinia'
import {
  create,
  NButton,
  NInput,
  NSwitch,
  NTabs,
  NTabPane,
  NModal,
  NCard,
  NProgress,
  NTag,
  NSpace,
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  NNotificationProvider,
  NLoadingBarProvider,
} from 'naive-ui'
import App from './App.vue'
import './style.css'
import { i18n, setupI18n } from './utils/i18n'
import { initTheme } from './utils/ThemeManager'
import router from './router/index'

// Create Naive UI instance with commonly used components
const naive = create({
  components: [
    NButton,
    NInput,
    NSwitch,
    NTabs,
    NTabPane,
    NModal,
    NCard,
    NProgress,
    NTag,
    NSpace,
    NConfigProvider,
    NMessageProvider,
    NDialogProvider,
    NNotificationProvider,
    NLoadingBarProvider,
  ],
})

const pinia = createPinia()

initTheme()
const isMac = navigator.userAgent.includes('Mac')
if (isMac) {
  document.documentElement.classList.add('mac')
}

async function bootstrap(){
    await setupI18n('zh')//默认语言或从后端读取
    createApp(App)
      .use(pinia)
      .use(router)
      .use(i18n)
      .use(naive)
      .mount('#app')
}
bootstrap()
