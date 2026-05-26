// Axios instance factory used by the admin and portal API clients.
// The admin console and the user portal each get their own instance so
// that tokens, base URLs, and 401-redirect targets stay strictly
// separated — there is no shared auth state between the two apps.

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

// API response envelope used by every JSON endpoint on the backend.
interface ApiEnvelope<T> {
  success: boolean
  msg?: string
  obj?: T
}

// Normalized error surfaced by the Axios response interceptors.
interface ApiError {
  status: number
  message: string
  // Free-form payload from the server, if any.
  data?: unknown
}

export interface ClientOptions {
  baseURL: string
  tokenStorageKey: string
  persistedStorageKey?: string
  loginPath: string
}

export function createApiClient(opts: ClientOptions): AxiosInstance {
  const instance = axios.create({
    baseURL: opts.baseURL,
    timeout: 30_000,
    headers: { 'Content-Type': 'application/json' },
  })

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const token = readToken(opts)
    if (token) {
      config.headers.set('Authorization', `Bearer ${token}`)
    }
    return config
  })

  instance.interceptors.response.use(
    (response) => {
      // Unwrap the {success,msg,obj} envelope on 2xx responses.
      const env = response.data as ApiEnvelope<unknown> | undefined
      if (env && typeof env === 'object' && 'success' in env) {
        if (!env.success) {
          const apiErr: ApiError = {
            status: response.status,
            message: env.msg ?? 'request failed',
            data: env.obj,
          }
          return Promise.reject(apiErr)
        }
        response.data = env.obj
      }
      notifySuccess(response.config.method, response.config.url)
      return response
    },
    (error: AxiosError<ApiEnvelope<unknown>>) => {
      const status = error.response?.status ?? 0
      if (status === 401) {
        localStorage.removeItem(opts.tokenStorageKey)
        if (opts.persistedStorageKey) localStorage.removeItem(opts.persistedStorageKey)
        // Avoid a redirect loop if we're already on the login page.
        if (!window.location.pathname.startsWith(opts.loginPath)) {
          const next = encodeURIComponent(window.location.pathname + window.location.search)
          window.location.assign(`${opts.loginPath}?next=${next}`)
        }
      }
      const apiErr: ApiError = {
        status,
        message: error.response?.data?.msg ?? error.message,
        data: error.response?.data,
      }
      return Promise.reject(apiErr)
    },
  )

  return instance
}

function readToken(opts: ClientOptions): string | null {
  const legacyToken = localStorage.getItem(opts.tokenStorageKey)
  if (legacyToken) return legacyToken

  if (!opts.persistedStorageKey) return null

  const stored = localStorage.getItem(opts.persistedStorageKey)
  if (!stored) return null

  try {
    const parsed = JSON.parse(stored) as { state?: { token?: unknown }; token?: unknown }
    const token = parsed.state?.token ?? parsed.token
    return typeof token === 'string' && token.length > 0 ? token : null
  } catch {
    return stored
  }
}

function notifySuccess(method?: string, url?: string) {
  const m = method?.toUpperCase()
  if (!m || m === 'GET') return
  pushToast(successMessage(url))
}

function pushToast(text: string) {
  const trimmed = text.trim()
  if (!trimmed) return
  window.dispatchEvent(
    new CustomEvent('dashboard:toast', {
      detail: { kind: 'success', text: trimmed },
    }),
  )
}

function successMessage(url?: string): string {
  const locale = currentLocale()
  if (url?.includes('/auth/login')) {
    return locale === 'zh' ? '登录成功！欢迎回来。' : 'Login successful! Welcome back.'
  }
  return locale === 'zh' ? '操作成功' : 'Operation successful'
}

function currentLocale(): 'en' | 'zh' {
  const stored = globalThis.localStorage?.getItem('dashboard.locale')
  if (stored === 'en' || stored === 'en-US') return 'en'
  if (stored === 'zh' || stored === 'zh-CN') return 'zh'
  return globalThis.navigator?.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}
