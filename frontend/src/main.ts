import { createApp } from 'vue'
import { createPinia } from 'pinia'

// Brand typography — Geist (Vercel). Self-hosted via fontsource so no
// runtime dependency on Google Fonts.
import '@fontsource/geist-sans/400.css'
import '@fontsource/geist-sans/500.css'
import '@fontsource/geist-sans/600.css'
import '@fontsource/geist-sans/700.css'
import '@fontsource/geist-mono/400.css'
import '@fontsource/geist-mono/500.css'

import App from './App.vue'
import { router } from './router'
import { bindI18nToStore, i18n } from './i18n'
import { useBrandingStore } from './stores/branding'
import { useThemeStore } from './stores/theme'

import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)

// Apply persisted theme before mount so we don't get a light-to-dark flash.
bindI18nToStore()
useThemeStore().init()
useBrandingStore().load().catch(() => {})

app.mount('#app')
