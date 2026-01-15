import { GetCircuitBreakerMetrics, ResetCircuitBreaker } from '../../wailsjs/go/main/App'

export interface CircuitBreakerMetric {
  provider_id: number
  provider_name: string
  state: string
  consecutive_fails: number
  total_requests: number
  total_failures: number
  total_successes: number
  success_rate: number
  last_failure_at?: string
  circuit_opened_at?: string
}

export const circuitBreakerApi = {
  /**
   * Get circuit breaker metrics for all providers
   */
  async getMetrics(): Promise<CircuitBreakerMetric[]> {
    try {
      const metrics = await GetCircuitBreakerMetrics()
      return metrics || []
    } catch (error) {
      console.error('Failed to get circuit breaker metrics:', error)
      throw new Error('Failed to load circuit breaker metrics')
    }
  },

  /**
   * Manually reset a circuit breaker
   */
  async resetCircuitBreaker(providerId: number): Promise<void> {
    try {
      await ResetCircuitBreaker(providerId)
    } catch (error) {
      console.error(`Failed to reset circuit breaker for provider ${providerId}:`, error)
      throw new Error('Failed to reset circuit breaker')
    }
  }
}
