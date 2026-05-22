import { config } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'

import zh from '@/i18n/locales/zh'
import en from '@/i18n/locales/en'

const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: 'zh',
  fallbackLocale: 'en',
  messages: { zh, en },
})

config.global.plugins = [...(config.global.plugins ?? []), i18n]
