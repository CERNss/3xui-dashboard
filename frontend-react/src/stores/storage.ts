export const ADMIN_AUTH_STORAGE_KEY = '3xui.adminAuth'
export const PORTAL_AUTH_STORAGE_KEY = '3xui.portalAuth'
export const LEGACY_ADMIN_TOKEN_KEY = 'dashboard.admin.token'
export const LEGACY_PORTAL_TOKEN_KEY = 'dashboard.portal.token'
export const LOCALE_STORAGE_KEY = 'dashboard.locale'
export const APP_THEME_STORAGE_KEY = 'dashboard.theme'
export const THEME_STORAGE_KEY = 'cp.theme'

export function getLocalStorage(): Storage | undefined {
  if (typeof window === 'undefined') return undefined

  try {
    return window.localStorage
  } catch {
    return undefined
  }
}

export function readString(key: string): string | null {
  return getLocalStorage()?.getItem(key) ?? null
}

export function writeString(key: string, value: string): void {
  getLocalStorage()?.setItem(key, value)
}

export function removeString(key: string): void {
  getLocalStorage()?.removeItem(key)
}

export function readPersistedToken(storageKey: string, legacyKey: string): string | null {
  const stored = readString(storageKey)
  if (stored) {
    try {
      const parsed = JSON.parse(stored) as { state?: { token?: unknown }; token?: unknown }
      const token = parsed.state?.token ?? parsed.token
      if (typeof token === 'string' && token.length > 0) return token
    } catch {
      return stored
    }
  }

  const legacyToken = readString(legacyKey)
  return legacyToken && legacyToken.length > 0 ? legacyToken : null
}
