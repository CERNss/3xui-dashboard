// Axios instance factory used by the admin and portal API clients.
// The admin console and the user portal each get their own instance so
// base URLs and 401-redirect targets stay strictly separated — there is
// no shared auth state between the two apps.
//
// Auth transport is the httpOnly session cookie the backend sets on
// login; the SPA never sees or stores the JWT (that's what closed the
// localStorage-XSS hole). `withCredentials` tells the browser to send
// that cookie — it only actually matters for a future cross-origin
// deployment, since same-origin requests send cookies regardless, but
// it's correct to set it. For state-changing requests we echo the
// readable double-submit CSRF cookie back in the X-CSRF-Token header.

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

// The readable CSRF cookie the backend sets + the header it must be
// echoed in. Keep in sync with backend/internal/session/cookie.go.
const CSRF_COOKIE = '3xui_csrf'
const CSRF_HEADER = 'X-CSRF-Token'

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
  loginPath: string
  // Invoked on a 401 so the owning client can drop its persisted
  // identity (the cookie is already gone/expired server-side).
  onUnauthorized?: () => void
}

export function createApiClient(opts: ClientOptions): AxiosInstance {
  const instance = axios.create({
    baseURL: opts.baseURL,
    timeout: 30_000,
    withCredentials: true,
    headers: { 'Content-Type': 'application/json' },
  })

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    // Double-submit CSRF: echo the readable cookie on unsafe methods.
    // Safe methods (GET/HEAD/OPTIONS) don't need it.
    const method = (config.method ?? 'get').toLowerCase()
    if (method !== 'get' && method !== 'head' && method !== 'options') {
      const csrf = readCookie(CSRF_COOKIE)
      if (csrf) config.headers.set(CSRF_HEADER, csrf)
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
        opts.onUnauthorized?.()
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

// readCookie pulls a single cookie value out of document.cookie.
function readCookie(name: string): string | null {
  if (typeof document === 'undefined') return null
  const prefix = `${name}=`
  for (const part of document.cookie.split(';')) {
    const trimmed = part.trim()
    if (trimmed.startsWith(prefix)) return decodeURIComponent(trimmed.slice(prefix.length))
  }
  return null
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
  if (url?.includes('/auth/logout')) {
    return ''
  }
  return locale === 'zh' ? '操作成功' : 'Operation successful'
}

function currentLocale(): 'en' | 'zh' {
  const stored = globalThis.localStorage?.getItem('dashboard.locale')
  if (stored === 'en' || stored === 'en-US') return 'en'
  if (stored === 'zh' || stored === 'zh-CN') return 'zh'
  return globalThis.navigator?.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}
