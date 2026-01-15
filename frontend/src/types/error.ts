/**
 * 统一错误类型系统
 * 提供世界级的错误处理能力，优雅准确的错误信息便于排错
 */

export enum ErrorCode {
  // 网络错误
  NETWORK_ERROR = 'NETWORK_ERROR',
  TIMEOUT = 'TIMEOUT',
  CONNECTION_REFUSED = 'CONNECTION_REFUSED',

  // 数据错误
  DATA_PARSE_ERROR = 'DATA_PARSE_ERROR',
  DATA_EMPTY = 'DATA_EMPTY',
  DATA_INVALID = 'DATA_INVALID',

  // 服务错误
  SERVICE_UNAVAILABLE = 'SERVICE_UNAVAILABLE',
  DATABASE_ERROR = 'DATABASE_ERROR',
  INTERNAL_ERROR = 'INTERNAL_ERROR',

  // 业务错误
  PROVIDER_ERROR = 'PROVIDER_ERROR',
  AUTH_ERROR = 'AUTH_ERROR',
  RATE_LIMIT = 'RATE_LIMIT',
  QUOTA_EXCEEDED = 'QUOTA_EXCEEDED',

  // 未知错误
  UNKNOWN = 'UNKNOWN',
}

export interface AppError {
  /** 错误代码，用于程序判断 */
  code: ErrorCode
  /** 用户友好的错误消息 */
  message: string
  /** 技术细节，用于调试 */
  details?: string
  /** 错误发生时间 */
  timestamp: Date
  /** 上下文信息 */
  context?: {
    component?: string
    action?: string
    [key: string]: unknown
  }
  /** 是否可重试 */
  retryable?: boolean
}

export interface DataState<T> {
  data: T | null
  loading: boolean
  error: AppError | null
  lastUpdated: Date | null
}

/**
 * 创建 AppError
 */
export function createAppError(
  code: ErrorCode,
  options: {
    message: string
    details?: string
    context?: AppError['context']
    retryable?: boolean
  }
): AppError {
  return {
    code,
    message: options.message,
    details: options.details,
    timestamp: new Date(),
    context: options.context,
    retryable: options.retryable ?? true,
  }
}

/**
 * 将未知错误规范化为 AppError
 */
export function normalizeError(
  error: unknown,
  context?: AppError['context']
): AppError {
  const timestamp = new Date()

  // 已经是 AppError
  if (isAppError(error)) {
    return { ...error, context: { ...error.context, ...context } }
  }

  // Error 对象
  if (error instanceof Error) {
    const message = error.message.toLowerCase()

    // 数据库表不存在
    if (message.includes('no such table')) {
      return {
        code: ErrorCode.DATABASE_ERROR,
        message: '数据库表不存在，请检查应用是否正确初始化',
        details: error.message,
        timestamp,
        context,
        retryable: false,
      }
    }

    // 超时错误
    if (message.includes('timeout') || message.includes('etimedout')) {
      return {
        code: ErrorCode.TIMEOUT,
        message: '请求超时，请检查网络连接后重试',
        details: error.message,
        timestamp,
        context,
        retryable: true,
      }
    }

    // 连接拒绝
    if (message.includes('econnrefused') || message.includes('connection refused')) {
      return {
        code: ErrorCode.CONNECTION_REFUSED,
        message: '无法连接服务，请检查服务是否正常运行',
        details: error.message,
        timestamp,
        context,
        retryable: true,
      }
    }

    // 网络错误
    if (message.includes('network') || message.includes('fetch')) {
      return {
        code: ErrorCode.NETWORK_ERROR,
        message: '网络请求失败，请检查网络连接',
        details: error.message,
        timestamp,
        context,
        retryable: true,
      }
    }

    // 认证错误
    if (message.includes('auth') || message.includes('unauthorized') || message.includes('401')) {
      return {
        code: ErrorCode.AUTH_ERROR,
        message: '认证失败，请检查 API 密钥是否正确',
        details: error.message,
        timestamp,
        context,
        retryable: false,
      }
    }

    // 限流错误
    if (message.includes('rate limit') || message.includes('429') || message.includes('too many')) {
      return {
        code: ErrorCode.RATE_LIMIT,
        message: '请求频率过高，请稍后重试',
        details: error.message,
        timestamp,
        context,
        retryable: true,
      }
    }

    // 通用错误
    return {
      code: ErrorCode.INTERNAL_ERROR,
      message: '操作失败，请稍后重试',
      details: error.message,
      timestamp,
      context,
      retryable: true,
    }
  }

  // 字符串错误
  if (typeof error === 'string') {
    return {
      code: ErrorCode.UNKNOWN,
      message: error,
      timestamp,
      context,
      retryable: true,
    }
  }

  // 未知错误
  return {
    code: ErrorCode.UNKNOWN,
    message: '发生未知错误',
    details: String(error),
    timestamp,
    context,
    retryable: true,
  }
}

/**
 * 类型守卫：检查是否为 AppError
 */
export function isAppError(error: unknown): error is AppError {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    'message' in error &&
    'timestamp' in error
  )
}

/**
 * 格式化错误为日志字符串（便于模型排错）
 */
export function formatErrorForLog(error: AppError): string {
  const lines = [
    `[${error.code}] ${error.message}`,
    `时间: ${error.timestamp.toISOString()}`,
  ]

  if (error.details) {
    lines.push(`详情: ${error.details}`)
  }

  if (error.context) {
    lines.push(`上下文: ${JSON.stringify(error.context)}`)
  }

  return lines.join('\n')
}
