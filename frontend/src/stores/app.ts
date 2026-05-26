import { create } from 'zustand'
import { APP_THEME_STORAGE_KEY, LOCALE_STORAGE_KEY, readString, writeString } from './storage'

export type Locale = 'en' | 'zh'
export type LocaleValue = 'en-US' | 'zh-CN'
export type AppTheme = 'light' | 'dark'

interface AppState {
  locale: LocaleValue
  theme: AppTheme
  setLocale: (locale: LocaleValue) => void
  toggleLocale: () => void
  setTheme: (theme: AppTheme) => void
}

function normalizeLocale(locale: Locale | LocaleValue): LocaleValue {
  return locale === 'zh' || locale === 'zh-CN' ? 'zh-CN' : 'en-US'
}

function persistLocale(locale: LocaleValue): void {
  writeString(LOCALE_STORAGE_KEY, locale === 'zh-CN' ? 'zh' : 'en')
}

function detectLocale(): LocaleValue {
  const stored = readString(LOCALE_STORAGE_KEY)
  if (stored === 'en' || stored === 'zh' || stored === 'en-US' || stored === 'zh-CN') {
    return normalizeLocale(stored)
  }

  if (typeof navigator === 'undefined') return 'en-US'
  return navigator.language?.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

function detectTheme(): AppTheme {
  const stored = readString(APP_THEME_STORAGE_KEY)
  return stored === 'dark' || stored === 'light' ? stored : 'light'
}

function applyTheme(theme: AppTheme): void {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

export const useAppStore = create<AppState>((set, get) => ({
  locale: detectLocale(),
  theme: detectTheme(),
  setLocale: (locale) => {
    persistLocale(locale)
    set({ locale })
  },
  toggleLocale: () => {
    get().setLocale(get().locale === 'zh-CN' ? 'en-US' : 'zh-CN')
  },
  setTheme: (theme) => {
    writeString(APP_THEME_STORAGE_KEY, theme)
    applyTheme(theme)
    set({ theme })
  }
}))

export type { AppState }
