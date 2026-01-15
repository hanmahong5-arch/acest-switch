/**
 * Gateway service for NEW-API configuration management.
 */

import { Call } from '@wailsio/runtime'

export interface GatewayConfig {
  newApiEnabled: boolean
  newApiUrl: string
  newApiToken: string
}

export interface UserInfo {
  id: number
  username: string
  email: string
  displayName?: string
}

export interface QuotaInfo {
  quotaTotal: number
  quotaUsed: number
  quotaRemain: number
  requestCount?: number
}

export interface ConnectionStatus {
  success: boolean
  user?: UserInfo
  quota?: QuotaInfo
  error?: string
}

// Backend AppSettings type (snake_case)
interface BackendAppSettings {
  show_heatmap: boolean
  show_home_title: boolean
  auto_start: boolean
  enable_body_log: boolean
  new_api_enabled: boolean
  new_api_url: string
  new_api_token: string
}

/**
 * Get current gateway configuration.
 */
export async function getGatewayConfig(): Promise<GatewayConfig> {
  try {
    const settings: BackendAppSettings = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
    return {
      newApiEnabled: settings?.new_api_enabled || false,
      newApiUrl: settings?.new_api_url || 'http://api.lurus.cn',
      newApiToken: settings?.new_api_token || '',
    }
  } catch (error) {
    console.error('[Gateway] Failed to get config:', error)
    throw error
  }
}

/**
 * Save gateway configuration.
 */
export async function setGatewayConfig(config: GatewayConfig): Promise<void> {
  try {
    const currentSettings: BackendAppSettings = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
    await Call.ByName('codeswitch/services.AppSettingsService.SaveAppSettings', {
      ...currentSettings,
      new_api_enabled: config.newApiEnabled,
      new_api_url: config.newApiUrl,
      new_api_token: config.newApiToken,
    })
  } catch (error) {
    console.error('[Gateway] Failed to save config:', error)
    throw error
  }
}

/**
 * Test connection to NEW-API server.
 */
export async function testConnection(url: string, token: string): Promise<ConnectionStatus> {
  try {
    const result = await Call.ByName('codeswitch/services.AppSettingsService.TestNewAPIConnection', url, token)
    return result as ConnectionStatus
  } catch (error) {
    console.error('[Gateway] Connection test failed:', error)
    return {
      success: false,
      error: error instanceof Error ? error.message : String(error),
    }
  }
}
