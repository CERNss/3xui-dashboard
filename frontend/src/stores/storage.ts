export const ADMIN_AUTH_STORAGE_KEY = '3xui.adminAuth'
export const PORTAL_AUTH_STORAGE_KEY = '3xui.portalAuth'
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

// readPersistedField pulls one field out of a zustand-persist blob
// (shape `{ state: {...}, version }`). Used by the auth stores to seed
// their initial "is this browser probably logged in" identity before
// rehydration runs. Returns null when absent or unparseable.
export function readPersistedField<T>(storageKey: string, field: string): T | null {
  const stored = readString(storageKey)
  if (!stored) return null
  try {
    const parsed = JSON.parse(stored) as { state?: Record<string, unknown> }
    const value = parsed.state?.[field]
    return (value ?? null) as T | null
  } catch {
    return null
  }
}
