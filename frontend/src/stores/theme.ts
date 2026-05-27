import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'
import { THEME_STORAGE_KEY, readString, writeString } from './storage'

export type ThemeMode = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

interface ThemeState {
  mode: ThemeMode
  resolvedTheme: ResolvedTheme
  set: (mode: ThemeMode) => void
  setMode: (mode: ThemeMode) => void
  toggle: () => void
  init: () => void
}

function systemTheme(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function readInitialMode(): ThemeMode {
  const stored = readString(THEME_STORAGE_KEY)
  if (stored === 'dark' || stored === 'light' || stored === 'system') return stored
  return 'system'
}

function resolveTheme(mode: ThemeMode): ResolvedTheme {
  return mode === 'system' ? systemTheme() : mode
}

function applyToHtml(mode: ThemeMode): ResolvedTheme {
  const resolvedTheme = resolveTheme(mode)
  if (typeof document !== 'undefined') {
    document.documentElement.classList.toggle('dark', resolvedTheme === 'dark')
  }
  return resolvedTheme
}

const initialMode = readInitialMode()
const initialResolvedTheme = applyToHtml(initialMode)

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      mode: initialMode,
      resolvedTheme: initialResolvedTheme,
      set: (mode) => get().setMode(mode),
      setMode: (mode) => {
        writeString(THEME_STORAGE_KEY, mode)
        set({ mode, resolvedTheme: applyToHtml(mode) })
      },
      toggle: () => {
        get().setMode(get().resolvedTheme === 'dark' ? 'light' : 'dark')
      },
      init: () => {
        const { mode } = get()
        set({ resolvedTheme: applyToHtml(mode) })
      }
    }),
    {
      name: THEME_STORAGE_KEY,
      storage: createJSONStorage(() => localStorage),
      partialize: ({ mode }) => ({ mode }),
      onRehydrateStorage: () => (state) => {
        if (!state) return
        state.resolvedTheme = applyToHtml(state.mode)
      }
    }
  )
)

export type { ThemeState }
