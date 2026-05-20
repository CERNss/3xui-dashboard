// Formatting utilities shared across admin views. Centralized to keep
// error/label strings consistent — no more raw axios messages leaking to UI.

import type { AxiosError } from 'axios'

/**
 * Convert any thrown value into a user-actionable message.
 *
 * Rules (in order):
 *   1. If the backend returned { error: { message } } or { message }, use that.
 *   2. If it's a network error, suggest checking connectivity / token / host.
 *   3. If it's an HTTP status we recognize, map to plain Chinese with a hint.
 *   4. Fallback: caller-supplied default.
 *
 * Never returns raw "Error: Request failed with status code 500" strings.
 */
export function formatError(e: unknown, fallback = '操作失败'): string {
  if (!e) return fallback

  // Axios error path
  const ax = e as AxiosError<{ error?: { message?: string }; message?: string }>
  if (ax?.isAxiosError) {
    const data = ax.response?.data as { error?: { message?: string }; message?: string } | undefined
    const backendMsg = data?.error?.message || data?.message
    if (backendMsg) return backendMsg

    if (ax.code === 'ERR_NETWORK' || !ax.response) {
      return '连不上后端 — 检查 dashboard 服务是否在跑、本机网络是否通'
    }

    const status = ax.response?.status
    switch (status) {
      case 400: return '请求参数有问题，检查表单字段是否都填对'
      case 401: return '登录已失效，请重新登录'
      case 403: return '没有权限执行这个操作'
      case 404: return '找不到对应资源（可能已被删除）'
      case 409: return '冲突 — 已存在同名/同 token 的资源'
      case 422: return '验证不通过，检查参数取值范围'
      case 429: return '操作过于频繁，稍等几秒再试'
      case 500: return '后端内部错误，看 dashboard 日志获取详情'
      case 502:
      case 503:
      case 504: return '上游节点不可达 — 检查 3x-ui 面板是否在跑、token 是否有效'
    }
  }

  // Plain Error or string
  if (e instanceof Error && e.message) return e.message
  if (typeof e === 'string') return e

  return fallback
}

/** Chinese label for node status. Single source of truth. */
export function nodeStatusLabel(status: string | undefined | null): string {
  switch (status) {
    case 'online':  return '在线'
    case 'offline': return '离线'
    case 'unknown': return '未知'
    default:        return status || '—'
  }
}
