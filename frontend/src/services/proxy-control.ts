import { GetProxyConfigs, GetProxyStats, ToggleProxy } from '../../wailsjs/go/services/ProviderRelayService'

export interface ProxyControlConfig {
  app_name: string
  proxy_enabled: boolean
  proxy_mode: string
  proxy_port?: number
  intercept_domains?: string[]
  total_requests: number
  last_request_at?: string
  last_toggled_at?: string
  created_at: string
}

export interface ProxyControlStats {
  app_name: string
  enabled: boolean
  total_requests: number
  last_request_at?: string
}

export interface ProxyConfigsResponse {
  configs: ProxyControlConfig[]
  stats: Record<string, ProxyControlStats>
}

export const proxyControlApi = {
  /**
   * Get proxy control configurations for all applications
   */
  async getConfigs(): Promise<ProxyConfigsResponse> {
    try {
      const result = await GetProxyConfigs()
      return result || { configs: [], stats: {} }
    } catch (error) {
      console.error('Failed to get proxy configs:', error)
      throw new Error('Failed to load proxy configurations')
    }
  },

  /**
   * Get proxy statistics for all applications
   */
  async getStats(): Promise<Record<string, ProxyControlStats>> {
    try {
      const stats = await GetProxyStats()
      return stats || {}
    } catch (error) {
      console.error('Failed to get proxy stats:', error)
      throw new Error('Failed to load proxy statistics')
    }
  },

  /**
   * Toggle proxy enable/disable for an application
   * @param appName - Application name ('claude', 'codex', or 'gemini')
   * @param enabled - Enable or disable proxy
   */
  async toggleProxy(appName: string, enabled: boolean): Promise<void> {
    try {
      await ToggleProxy(appName, enabled)
    } catch (error) {
      console.error(`Failed to toggle proxy for ${appName}:`, error)
      throw new Error(`Failed to ${enabled ? 'enable' : 'disable'} proxy for ${appName}`)
    }
  }
}
