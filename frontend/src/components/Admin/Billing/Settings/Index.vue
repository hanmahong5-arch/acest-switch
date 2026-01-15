<template>
  <div class="billing-settings">
    <div class="page-header">
      <h2>{{ t('admin.billing.settings.title') }}</h2>
      <n-space>
        <n-button :loading="billingStore.configSaving" type="primary" @click="handleSave">
          {{ t('common.save') }}
        </n-button>
      </n-space>
    </div>

    <n-spin :show="billingStore.configLoading">
      <!-- Service Status -->
      <n-card :title="t('admin.billing.settings.serviceStatus')" size="small" class="settings-card">
        <n-space vertical>
          <n-alert v-if="billingStore.billingEnabled" type="success" :title="t('admin.billing.settings.enabled')" />
          <n-alert v-else type="warning" :title="t('admin.billing.settings.disabled')" />
          <n-space v-if="billingStore.billingStatus?.services">
            <n-tag
              v-for="service in billingStore.billingStatus.services"
              :key="service.name"
              :type="service.enabled ? 'success' : 'default'"
            >
              {{ service.name }}: {{ service.status }}
            </n-tag>
          </n-space>
        </n-space>
      </n-card>

      <!-- Enable Toggle -->
      <n-card :title="t('admin.billing.settings.general')" size="small" class="settings-card">
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.enableBilling')">
            <n-switch v-model:value="config.enabled" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.requireSubscription')">
            <n-switch v-model:value="config.require_subscription" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.gracePeriodHours')">
            <n-input-number v-model:value="config.grace_period_hours" :min="0" :max="720" style="width: 200px" />
          </n-form-item>
        </n-form>
      </n-card>

      <!-- Casdoor Config -->
      <n-card size="small" class="settings-card">
        <template #header>
          <n-space align="center">
            <span>{{ t('admin.billing.settings.casdoor') }}</span>
            <n-button size="small" :loading="testingCasdoor" @click="testConnection('casdoor')">
              {{ t('admin.billing.settings.testConnection') }}
            </n-button>
          </n-space>
        </template>
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.casdoorEndpoint')">
            <n-input v-model:value="config.casdoor_endpoint" :placeholder="t('admin.billing.settings.casdoorEndpointPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.casdoorClientId')">
            <n-input v-model:value="config.casdoor_client_id" :placeholder="t('admin.billing.settings.casdoorClientIdPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.casdoorClientSecret')">
            <n-input v-model:value="config.casdoor_client_secret" type="password" show-password-on="click" :placeholder="t('admin.billing.settings.casdoorClientSecretPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.casdoorOrganization')">
            <n-input v-model:value="config.casdoor_organization" :placeholder="t('admin.billing.settings.casdoorOrganizationPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.casdoorApplication')">
            <n-input v-model:value="config.casdoor_application" :placeholder="t('admin.billing.settings.casdoorApplicationPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.casdoorCertificate')">
            <n-input v-model:value="config.casdoor_certificate" type="textarea" :rows="3" :placeholder="t('admin.billing.settings.casdoorCertificatePlaceholder')" />
          </n-form-item>
        </n-form>
      </n-card>

      <!-- Lago Config -->
      <n-card size="small" class="settings-card">
        <template #header>
          <n-space align="center">
            <span>{{ t('admin.billing.settings.lago') }}</span>
            <n-button size="small" :loading="testingLago" @click="testConnection('lago')">
              {{ t('admin.billing.settings.testConnection') }}
            </n-button>
          </n-space>
        </template>
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.lagoApiUrl')">
            <n-input v-model:value="config.lago_api_url" :placeholder="t('admin.billing.settings.lagoApiUrlPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.lagoApiKey')">
            <n-input v-model:value="config.lago_api_key" type="password" show-password-on="click" :placeholder="t('admin.billing.settings.lagoApiKeyPlaceholder')" />
          </n-form-item>
        </n-form>
      </n-card>

      <!-- Alipay Config -->
      <n-card size="small" class="settings-card">
        <template #header>
          <n-space align="center">
            <span>{{ t('admin.billing.settings.alipay') }}</span>
            <n-button size="small" :loading="testingAlipay" @click="testConnection('alipay')">
              {{ t('admin.billing.settings.testConnection') }}
            </n-button>
          </n-space>
        </template>
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.alipayAppId')">
            <n-input v-model:value="config.alipay_app_id" :placeholder="t('admin.billing.settings.alipayAppIdPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.alipayPrivateKey')">
            <n-input v-model:value="config.alipay_private_key" type="textarea" :rows="3" :placeholder="t('admin.billing.settings.alipayPrivateKeyPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.alipayPublicKey')">
            <n-input v-model:value="config.alipay_public_key" type="textarea" :rows="3" :placeholder="t('admin.billing.settings.alipayPublicKeyPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.alipaySandbox')">
            <n-switch v-model:value="config.alipay_sandbox" />
          </n-form-item>
        </n-form>
      </n-card>

      <!-- WeChat Pay Config -->
      <n-card size="small" class="settings-card">
        <template #header>
          <n-space align="center">
            <span>{{ t('admin.billing.settings.wechat') }}</span>
            <n-button size="small" :loading="testingWechat" @click="testConnection('wechat')">
              {{ t('admin.billing.settings.testConnection') }}
            </n-button>
          </n-space>
        </template>
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.wechatAppId')">
            <n-input v-model:value="config.wechat_app_id" :placeholder="t('admin.billing.settings.wechatAppIdPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.wechatMchId')">
            <n-input v-model:value="config.wechat_mch_id" :placeholder="t('admin.billing.settings.wechatMchIdPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.wechatApiKey')">
            <n-input v-model:value="config.wechat_api_key" type="password" show-password-on="click" :placeholder="t('admin.billing.settings.wechatApiKeyPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.wechatApiKeyV3')">
            <n-input v-model:value="config.wechat_api_key_v3" type="password" show-password-on="click" :placeholder="t('admin.billing.settings.wechatApiKeyV3Placeholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.wechatSerialNo')">
            <n-input v-model:value="config.wechat_serial_no" :placeholder="t('admin.billing.settings.wechatSerialNoPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.wechatPrivateKey')">
            <n-input v-model:value="config.wechat_private_key" type="textarea" :rows="3" :placeholder="t('admin.billing.settings.wechatPrivateKeyPlaceholder')" />
          </n-form-item>
        </n-form>
      </n-card>

      <!-- Callback URLs -->
      <n-card :title="t('admin.billing.settings.callbacks')" size="small" class="settings-card">
        <n-form label-placement="left" label-width="180">
          <n-form-item :label="t('admin.billing.settings.notifyUrl')">
            <n-input v-model:value="config.payment_notify_url" :placeholder="t('admin.billing.settings.notifyUrlPlaceholder')" />
          </n-form-item>
          <n-form-item :label="t('admin.billing.settings.returnUrl')">
            <n-input v-model:value="config.payment_return_url" :placeholder="t('admin.billing.settings.returnUrlPlaceholder')" />
          </n-form-item>
        </n-form>
      </n-card>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NCard, NSpace, NForm, NFormItem, NInput, NInputNumber, NSwitch,
  NTag, NSpin, NAlert
} from 'naive-ui'
import { useBillingStore } from '../../../../stores/billing'
import type { BillingConfig } from '../../../../services/billing'
import { showToast } from '../../../../utils/toast'

const { t } = useI18n()
const billingStore = useBillingStore()

// Local config state
const config = reactive<BillingConfig>({
  enabled: false,
  casdoor_endpoint: '',
  casdoor_client_id: '',
  casdoor_client_secret: '',
  casdoor_organization: '',
  casdoor_application: '',
  casdoor_certificate: '',
  lago_api_url: '',
  lago_api_key: '',
  alipay_app_id: '',
  alipay_private_key: '',
  alipay_public_key: '',
  alipay_sandbox: false,
  wechat_app_id: '',
  wechat_mch_id: '',
  wechat_api_key: '',
  wechat_api_key_v3: '',
  wechat_serial_no: '',
  wechat_private_key: '',
  payment_notify_url: '',
  payment_return_url: '',
  grace_period_hours: 24,
  require_subscription: true,
})

// Test connection states
const testingCasdoor = ref(false)
const testingLago = ref(false)
const testingAlipay = ref(false)
const testingWechat = ref(false)

// Watch store config changes
watch(() => billingStore.billingConfig, (newConfig) => {
  if (newConfig) {
    Object.assign(config, newConfig)
  }
}, { immediate: true })

// Methods
const handleSave = async () => {
  try {
    await billingStore.saveBillingConfig(config)
    showToast(t('admin.billing.settings.saveSuccess'), 'success')
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  }
}

const testConnection = async (service: 'casdoor' | 'lago' | 'alipay' | 'wechat') => {
  const loadingRefs: Record<string, typeof testingCasdoor> = {
    casdoor: testingCasdoor,
    lago: testingLago,
    alipay: testingAlipay,
    wechat: testingWechat,
  }

  loadingRefs[service].value = true
  try {
    const result = await billingStore.testConnection(service)
    if (result.success) {
      showToast(result.message || t('admin.billing.settings.testSuccess'), 'success')
    } else {
      showToast(result.message || t('admin.billing.settings.testFailed'), 'error')
    }
  } catch (error) {
    if (error instanceof Error) {
      showToast(error.message, 'error')
    }
  } finally {
    loadingRefs[service].value = false
  }
}

onMounted(() => {
  billingStore.loadBillingConfig()
  billingStore.loadBillingStatus()
})
</script>

<style scoped>
.billing-settings {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.settings-card {
  margin-bottom: 16px;
}
</style>
