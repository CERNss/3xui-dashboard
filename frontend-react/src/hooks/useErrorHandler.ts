import { useCallback, useRef } from 'react'
import { createElement } from 'react'
import { App, Button, type FormInstance } from 'antd'
import type { FieldData } from 'rc-field-form/es/interface'
import axios from 'axios'

type ErrorUiMode = 'notification' | 'message' | 'form' | 'silent'
type ErrorCategory =
  | 'network'
  | 'validation'
  | 'auth'
  | 'permission'
  | 'notFound'
  | 'conflict'
  | 'rateLimit'
  | 'server'
  | 'upstream'
  | 'unknown'

type FieldErrorMap = Record<string, string | string[]>

interface NormalizedApiError {
  category: ErrorCategory
  status?: number
  code?: string
  message: string
  details?: unknown
  fieldErrors: FieldErrorMap
  retryAfterMs?: number
}

interface HandleErrorOptions {
  mode?: ErrorUiMode
  form?: FormInstance
  fallback?: string
  actionLabel?: string
  retry?: () => void | Promise<void>
  autoRetryOnce?: boolean
}

interface ErrorHandler {
  handleError: (error: unknown, options?: HandleErrorOptions) => NormalizedApiError
  normalizeError: (error: unknown, fallback?: string) => NormalizedApiError
  toFormFields: (error: unknown) => FieldData[]
  applyFieldErrors: (form: FormInstance, error: unknown) => boolean
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined
}

function pickMessage(data: unknown): string | undefined {
  const record = asRecord(data)
  const nestedError = asRecord(record?.error)
  const message = nestedError?.message ?? record?.message ?? record?.error
  return typeof message === 'string' && message.trim().length > 0 ? message : undefined
}

function pickDetails(data: unknown): unknown {
  const record = asRecord(data)
  return record?.details ?? record?.errors ?? record?.field_errors
}

function pickFieldErrors(data: unknown): FieldErrorMap {
  const details = pickDetails(data)
  const detailsRecord = asRecord(details)
  if (!detailsRecord) return {}

  return Object.entries(detailsRecord).reduce<FieldErrorMap>((acc, [name, value]) => {
    if (typeof value === 'string') acc[name] = value
    if (Array.isArray(value)) {
      const messages = value.filter((item): item is string => typeof item === 'string')
      if (messages.length > 0) acc[name] = messages
    }
    return acc
  }, {})
}

function parseRetryAfter(value: string | undefined): number | undefined {
  if (!value) return undefined

  const seconds = Number(value)
  if (Number.isFinite(seconds)) return Math.max(0, seconds * 1000)

  const dateMs = Date.parse(value)
  if (Number.isFinite(dateMs)) return Math.max(0, dateMs - Date.now())

  return undefined
}

function categoryFor(status?: number, code?: string, hasResponse = true): ErrorCategory {
  if (code === 'ERR_NETWORK' || !hasResponse) return 'network'
  if (status === 400 || status === 422) return 'validation'
  if (status === 401) return 'auth'
  if (status === 403) return 'permission'
  if (status === 404) return 'notFound'
  if (status === 409) return 'conflict'
  if (status === 429) return 'rateLimit'
  if (status === 500) return 'server'
  if (status === 502 || status === 503 || status === 504) return 'upstream'
  return 'unknown'
}

function fallbackFor(category: ErrorCategory, fallback: string): string {
  switch (category) {
    case 'network':
      return '连不上后端 - 检查 dashboard 服务是否在跑、本机网络是否通'
    case 'validation':
      return '请求参数有问题，检查表单字段是否都填对'
    case 'auth':
      return '登录已失效，请重新登录'
    case 'permission':
      return '没有权限执行这个操作'
    case 'notFound':
      return '找不到对应资源（可能已被删除）'
    case 'conflict':
      return '冲突 - 已存在同名或同 token 的资源'
    case 'rateLimit':
      return '操作过于频繁，稍等几秒再试'
    case 'server':
      return '后端内部错误，看 dashboard 日志获取详情'
    case 'upstream':
      return '上游节点不可达 - 检查 3x-ui 面板是否在跑、token 是否有效'
    case 'unknown':
      return fallback
  }
}

function normalizeError(error: unknown, fallback = '操作失败'): NormalizedApiError {
  if (axios.isAxiosError(error)) {
    const status = error.response?.status
    const data = error.response?.data
    const category = categoryFor(status, error.code, Boolean(error.response))
    return {
      category,
      status,
      code: error.code,
      message: pickMessage(data) ?? fallbackFor(category, fallback),
      details: pickDetails(data),
      fieldErrors: pickFieldErrors(data),
      retryAfterMs: parseRetryAfter(error.response?.headers?.['retry-after'])
    }
  }

  if (error instanceof Error && error.message) {
    return {
      category: 'unknown',
      message: error.message,
      fieldErrors: {}
    }
  }

  if (typeof error === 'string' && error.length > 0) {
    return {
      category: 'unknown',
      message: error,
      fieldErrors: {}
    }
  }

  return {
    category: 'unknown',
    message: fallback,
    fieldErrors: {}
  }
}

function toFormFields(error: unknown): FieldData[] {
  const normalized = normalizeError(error)
  return Object.entries(normalized.fieldErrors).map(([name, errors]) => ({
    name: [name],
    errors: Array.isArray(errors) ? errors : [errors],
    warnings: []
  }))
}

function applyFieldErrors(form: FormInstance, error: unknown): boolean {
  const fields = toFormFields(error)
  if (fields.length === 0) return false
  form.setFields(fields)
  return true
}

export function useErrorHandler(): ErrorHandler {
  const { message, notification } = App.useApp()
  const retryTimers = useRef(new Set<string>())

  const handleError = useCallback(
    (error: unknown, options: HandleErrorOptions = {}) => {
      const normalized = normalizeError(error, options.fallback)
      const mode = options.mode ?? (normalized.category === 'validation' ? 'form' : 'notification')

      if (mode === 'form' && options.form && applyFieldErrors(options.form, error)) {
        return normalized
      }

      if (mode === 'silent') return normalized

      if (normalized.category === 'rateLimit' && normalized.retryAfterMs && options.retry) {
        const retryKey = `${normalized.message}:${normalized.retryAfterMs}`
        if (options.autoRetryOnce && !retryTimers.current.has(retryKey)) {
          retryTimers.current.add(retryKey)
          window.setTimeout(() => {
            void options.retry?.()
            retryTimers.current.delete(retryKey)
          }, normalized.retryAfterMs)
        }

        notification.warning({
          message: normalized.message,
          description: `请等待 ${Math.ceil(normalized.retryAfterMs / 1000)} 秒后重试`
        })
        return normalized
      }

      if (mode === 'message') {
        void message.error(normalized.message)
        return normalized
      }

      notification.error({
        message: normalized.message,
        description:
          normalized.category === 'server'
            ? '查看 dashboard 日志获取详情'
            : normalized.category === 'upstream'
              ? '检查对应节点面板、网络和 token 后重试'
              : undefined,
        btn: options.retry
          ? createElement(
              Button,
              { size: 'small', type: 'primary', onClick: () => void options.retry?.() },
              options.actionLabel ?? '重试'
            )
          : undefined
      })

      return normalized
    },
    [message, notification]
  )

  return {
    handleError,
    normalizeError,
    toFormFields,
    applyFieldErrors
  }
}

export type { ErrorCategory, ErrorHandler, ErrorUiMode, FieldErrorMap, HandleErrorOptions, NormalizedApiError }
