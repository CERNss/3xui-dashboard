import { createI18n } from 'vue-i18n'
import { watch, type WatchStopHandle } from 'vue'

import en from './locales/en'
import zh from './locales/zh'

import { useAppStore } from '@/stores/app'

// `pinia` is not yet active when this module is evaluated, so read
// the locale preference directly from localStorage with the same logic
// the store uses; the app store will pick the same value on mount.
function initialLocale(): 'en' | 'zh' {
  const stored = localStorage.getItem('dashboard.locale')
  if (stored === 'en' || stored === 'zh') return stored
  return navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: initialLocale(),
  fallbackLocale: 'en',
  messages: { en, zh },
})

export type MessageSchema = typeof en

// Keep the i18n locale in lockstep with the app store.
let stopLocaleWatch: WatchStopHandle | null = null

export function bindI18nToStore() {
  const app = useAppStore()
  stopLocaleWatch?.()
  stopLocaleWatch = watch(
    () => app.locale,
    (loc) => {
      i18n.global.locale.value = loc
    },
    { immediate: true },
  )
}
