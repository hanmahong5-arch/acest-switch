import type { AppError } from '../types/error'
import { formatErrorForLog } from '../types/error'

export type ToastLevel = 'success' | 'error' | 'warning' | 'info'

export interface ToastOptions {
  /** 显示时长(ms)，默认 success=2400, error=4000 */
  duration?: number
  /** 技术详情（可展开） */
  details?: string
  /** 操作按钮 */
  action?: {
    label: string
    onClick: () => void
  }
}

const DEFAULT_DURATIONS: Record<ToastLevel, number> = {
  success: 2400,
  error: 4000,
  warning: 3200,
  info: 2800,
}

let toastContainer: HTMLElement | null = null

function getContainer() {
  if (toastContainer) return toastContainer

  toastContainer = document.createElement('div')
  toastContainer.className = 'mac-toast-container'
  document.body.appendChild(toastContainer)
  return toastContainer
}

/**
 * 显示 Toast 通知
 */
export function showToast(
  message: string,
  level: ToastLevel = 'success',
  options?: ToastOptions
) {
  if (!message) return

  const container = getContainer()
  const duration = options?.duration ?? DEFAULT_DURATIONS[level]

  // 创建 toast 元素
  const toast = document.createElement('div')
  toast.className = `mac-toast mac-toast-${level}`

  // 图标
  const icon = document.createElement('span')
  icon.className = 'mac-toast-icon'
  icon.innerHTML = getIconForLevel(level)
  toast.appendChild(icon)

  // 内容区
  const content = document.createElement('div')
  content.className = 'mac-toast-content'

  // 消息文本
  const messageEl = document.createElement('span')
  messageEl.className = 'mac-toast-message'
  messageEl.textContent = message
  content.appendChild(messageEl)

  // 详情展开（如果有）
  if (options?.details) {
    const detailsWrapper = document.createElement('details')
    detailsWrapper.className = 'mac-toast-details'

    const summary = document.createElement('summary')
    summary.textContent = '技术详情'
    detailsWrapper.appendChild(summary)

    const detailsContent = document.createElement('pre')
    detailsContent.textContent = options.details
    detailsWrapper.appendChild(detailsContent)

    content.appendChild(detailsWrapper)
  }

  toast.appendChild(content)

  // 操作按钮（如果有）
  if (options?.action) {
    const actionBtn = document.createElement('button')
    actionBtn.className = 'mac-toast-action'
    actionBtn.textContent = options.action.label
    actionBtn.addEventListener('click', (e) => {
      e.stopPropagation()
      options.action?.onClick()
      remove()
    })
    toast.appendChild(actionBtn)
  }

  // 关闭按钮
  const closeBtn = document.createElement('button')
  closeBtn.className = 'mac-toast-close'
  closeBtn.innerHTML = '×'
  closeBtn.addEventListener('click', remove)
  toast.appendChild(closeBtn)

  container.appendChild(toast)

  // 显示动画
  requestAnimationFrame(() => {
    toast.classList.add('mac-toast-visible')
  })

  let timeoutId: ReturnType<typeof setTimeout>

  function remove() {
    clearTimeout(timeoutId)
    toast.classList.remove('mac-toast-visible')
    toast.classList.add('mac-toast-hide')
    const handler = () => {
      toast.removeEventListener('transitionend', handler)
      toast.remove()
      if (toastContainer && toastContainer.childElementCount === 0) {
        toastContainer.remove()
        toastContainer = null
      }
    }
    toast.addEventListener('transitionend', handler)
  }

  timeoutId = setTimeout(remove, duration)

  // 返回移除函数，允许手动关闭
  return remove
}

/**
 * 显示 AppError 类型的错误 Toast
 */
export function showErrorToast(
  error: AppError,
  options?: {
    onRetry?: () => void
  }
) {
  // 打印详细日志供排错
  console.error('[AppError]', formatErrorForLog(error))

  showToast(error.message, 'error', {
    duration: 5000,
    details: error.details,
    action: error.retryable && options?.onRetry
      ? { label: '重试', onClick: options.onRetry }
      : undefined,
  })
}

/**
 * 获取对应级别的图标
 */
function getIconForLevel(level: ToastLevel): string {
  switch (level) {
    case 'success':
      return '<svg viewBox="0 0 24 24" width="18" height="18"><path d="M9 12l2 2 4-4" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"/><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/></svg>'
    case 'error':
      return '<svg viewBox="0 0 24 24" width="18" height="18"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/><line x1="12" y1="8" x2="12" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><line x1="12" y1="16" x2="12.01" y2="16" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
    case 'warning':
      return '<svg viewBox="0 0 24 24" width="18" height="18"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" stroke="currentColor" stroke-width="2" fill="none"/><line x1="12" y1="9" x2="12" y2="13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><line x1="12" y1="17" x2="12.01" y2="17" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
    case 'info':
      return '<svg viewBox="0 0 24 24" width="18" height="18"><circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/><line x1="12" y1="16" x2="12" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><line x1="12" y1="8" x2="12.01" y2="8" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>'
  }
}
