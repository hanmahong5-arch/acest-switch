import { Call } from '@wailsio/runtime'

export type RequestLog = {
  id: number
  trace_id?: string              // 全局追踪 ID (Ailurus PaaS 增强)
  request_id?: string            // 客户端请求 ID
  platform: string
  model: string
  provider: string
  http_code: number
  input_tokens: number
  output_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  reasoning_tokens: number
  is_stream?: boolean | number
  duration_sec?: number
  user_agent?: string            // 用户代理（识别 TUI/GUI）
  client_ip?: string             // 客户端 IP
  user_id?: string               // 用户标识（多租户）
  request_method?: string        // HTTP 方法
  request_path?: string          // 请求路径
  error_type?: string            // 错误类型（network/auth/rate_limit/server）
  error_message?: string         // 错误详细信息
  provider_error_code?: string   // 供应商错误码
  created_at: string
  total_cost?: number
  input_cost?: number
  output_cost?: number
  cache_create_cost?: number
  cache_read_cost?: number
  ephemeral_5m_cost?: number
  ephemeral_1h_cost?: number
  has_pricing?: boolean
}

type RequestLogQuery = {
  platform?: string
  provider?: string
  limit?: number
}

export const fetchRequestLogs = async (query: RequestLogQuery = {}): Promise<RequestLog[]> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const limit = query.limit ?? 100
  return Call.ByName('codeswitch/services.LogService.ListRequestLogs', platform, provider, limit)
}

export const fetchLogProviders = async (platform = ''): Promise<string[]> => {
  return Call.ByName('codeswitch/services.LogService.ListProviders', platform)
}

export type LogStatsSeries = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  total_cost: number
}

export type LogStats = {
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
  cost_input: number
  cost_output: number
  cost_cache_create: number
  cost_cache_read: number
  series: LogStatsSeries[]
}

export const fetchLogStats = async (platform = ''): Promise<LogStats> => {
  return Call.ByName('codeswitch/services.LogService.StatsSince', platform)
}

export type ProviderDailyStat = {
  provider: string
  total_requests: number
  successful_requests: number
  failed_requests: number
  success_rate: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
}

export const fetchProviderDailyStats = async (
  platform = '',
): Promise<ProviderDailyStat[]> => {
  return Call.ByName('codeswitch/services.LogService.ProviderDailyStats', platform)
}

export type HeatmapStat = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  total_cost: number
}

export const fetchHeatmapStats = async (days: number): Promise<HeatmapStat[]> => {
  const range = Number.isFinite(days) && days > 0 ? Math.floor(days) : 30
  return Call.ByName('codeswitch/services.LogService.HeatmapStats', range)
}

// 成本深度分析
export type DailyCostPoint = {
  day: string
  total_cost: number
  requests: number
}

export type CostAnalysis = {
  input_cost_ratio: number
  output_cost_ratio: number
  cache_create_cost_ratio: number
  cache_read_cost_ratio: number
  cache_read_tokens: number
  cache_saved_cost: number
  cache_saving_percent: number
  daily_avg_cost: number
  cost_trend: DailyCostPoint[]
  trend_direction: 'up' | 'down' | 'stable'
  trend_percentage: number
}

export const fetchCostAnalysis = async (
  platform = '',
  days = 7,
): Promise<CostAnalysis> => {
  return Call.ByName('codeswitch/services.LogService.CostAnalysis', platform, days)
}

// 性能与可靠性分析
export type ProviderReliabilityStat = {
  provider: string
  total_requests: number
  success_count: number
  fail_count: number
  success_rate: number
  avg_duration: number
  error_types: Record<string, number>
}

export type PerformanceAnalysis = {
  duration_p50: number
  duration_p95: number
  duration_p99: number
  duration_avg: number
  duration_min: number
  duration_max: number
  error_distribution: Record<string, number>
  total_errors: number
  error_rate: number
  provider_reliability: ProviderReliabilityStat[]
}

export const fetchPerformanceAnalysis = async (
  platform = '',
  days = 1,
): Promise<PerformanceAnalysis> => {
  return Call.ByName('codeswitch/services.LogService.PerformanceAnalysis', platform, days)
}

// 请求/响应 Body 日志
export type RequestLogBody = {
  id: number
  trace_id: string
  request_body: string
  response_body: string
  body_size_bytes: number
  created_at: string
  expires_at: string
}

export const fetchRequestLogBody = async (traceId: string): Promise<RequestLogBody | null> => {
  return Call.ByName('codeswitch/services.LogService.GetRequestLogBody', traceId)
}
