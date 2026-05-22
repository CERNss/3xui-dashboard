import { defineStore } from 'pinia'

export type Locale = 'en' | 'zh'
type Theme = 'light' | 'dark'

const LOCALE_KEY = 'dashboard.locale'
const THEME_KEY = 'dashboard.theme'

interface State {
  locale: Locale
  theme: Theme
}

function detectLocale(): Locale {
  const stored = localStorage.getItem(LOCALE_KEY)
  if (stored === 'en' || stored === 'zh') return stored
  return navigator.language?.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

function detectTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY)
  if (stored === 'light' || stored === 'dark') return stored
  return 'light'
}

export const useAppStore = defineStore('app', {
  state: (): State => ({
    locale: detectLocale(),
    theme: detectTheme(),
  }),
  actions: {
    setLocale(loc: Locale) {
      this.locale = loc
      localStorage.setItem(LOCALE_KEY, loc)
    },
    toggleLocale() {
      this.setLocale(this.locale === 'zh' ? 'en' : 'zh')
    },
    setTheme(theme: Theme) {
      this.theme = theme
      localStorage.setItem(THEME_KEY, theme)
      document.documentElement.classList.toggle('dark', theme === 'dark')
    },
  },
})
