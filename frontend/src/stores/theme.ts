// Tiny theme store. Reads `theme` from localStorage on init, exposes a toggle.
// Default = 'light' (per user preference). Persisted across reloads.

import { defineStore } from 'pinia'

type Theme = 'light' | 'dark'

const STORAGE_KEY = 'cp.theme'

function readInitial(): Theme {
  if (typeof window === 'undefined') return 'light'
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored === 'dark' || stored === 'light') return stored
  // No user override — follow the OS preference. Re-checked on every fresh
  // page load; once the user toggles inside the app, the choice is persisted
  // to localStorage and wins on subsequent loads.
  if (window.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark'
  return 'light'
}

function applyToHtml(theme: Theme) {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

export const useThemeStore = defineStore('theme', {
  state: () => ({ theme: readInitial() as Theme }),
  actions: {
    set(t: Theme) {
      this.theme = t
      try { window.localStorage.setItem(STORAGE_KEY, t) } catch { /* ignore */ }
      applyToHtml(t)
    },
    toggle() {
      this.set(this.theme === 'dark' ? 'light' : 'dark')
    },
    /** Call once on app bootstrap so the saved theme is applied before first paint. */
    init() {
      applyToHtml(this.theme)
    },
  },
})
