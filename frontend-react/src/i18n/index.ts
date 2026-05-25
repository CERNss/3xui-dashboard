import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import { en } from './locales/en'
import { zh } from './locales/zh'

const LOCALE_KEY = 'dashboard.locale'

export type Locale = 'en' | 'zh'

function initialLocale(): Locale {
  if (typeof window !== 'undefined') {
    const stored = window.localStorage.getItem(LOCALE_KEY)
    if (stored === 'en' || stored === 'zh') return stored

    return window.navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
  }

  return 'en'
}

i18n
  .use(initReactI18next)
  .init({
    resources: {
      zh: { translation: zh },
      en: { translation: en },
    },
    lng: initialLocale(),
    fallbackLng: 'en',
    returnNull: false,
    keySeparator: '.',
    interpolation: {
      escapeValue: false,
      prefix: '{',
      suffix: '}',
    },
    saveMissing: import.meta.env.DEV,
    missingKeyHandler: import.meta.env.DEV
      ? (lngs, namespace, key) => {
          console.warn(`[i18n] Missing key "${key}" in namespace "${namespace}" for locale "${lngs}"`)
        }
      : undefined,
  })

export { i18n }
export type MessageSchema = typeof en
