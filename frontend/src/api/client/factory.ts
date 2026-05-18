// Axios instance factory used by the admin and portal API clients.
// The admin console and the user portal each get their own instance so
// that tokens, base URLs, and 401-redirect targets stay strictly
// separated — there is no shared auth state between the two apps.

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios'

import type { ApiEnvelope, ApiError } from '@/types/api'

export interface ClientOptions {
  baseURL: string
  tokenStorageKey: string
  loginPath: string
}

export function createApiClient(opts: ClientOptions): AxiosInstance {
  const instance = axios.create({
    baseURL: opts.baseURL,
    timeout: 30_000,
    headers: { 'Content-Type': 'application/json' },
  })

  instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem(opts.tokenStorageKey)
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
      return response
    },
    (error: AxiosError<ApiEnvelope<unknown>>) => {
      const status = error.response?.status ?? 0
      if (status === 401) {
        localStorage.removeItem(opts.tokenStorageKey)
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
