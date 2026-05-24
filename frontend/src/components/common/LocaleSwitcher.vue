<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { useAppStore, type Locale } from '@/stores/app'

const props = withDefaults(defineProps<{
  collapsed?: boolean
  variant?: 'sidebar' | 'toolbar'
}>(), {
  collapsed: false,
  variant: 'sidebar',
})

const app = useAppStore()
const { t } = useI18n()

const nextLocale = computed<Locale>(() => app.locale === 'zh' ? 'en' : 'zh')
const currentCode = computed(() => app.locale === 'zh' ? 'ZH' : 'EN')
const nextLanguage = computed(() => nextLocale.value === 'zh' ? t('language.chinese') : t('language.english'))
const ariaLabel = computed(() => t('language.switchTo', { language: nextLanguage.value }))

const buttonClass = computed(() => props.variant === 'sidebar'
  ? [
      'group flex w-full items-center rounded-xl py-2 text-sm font-medium text-surface-600 transition-all duration-150 ease-brand hover:bg-surface-100 hover:text-ink-900 dark:text-surface-300 dark:hover:bg-surface-800 dark:hover:text-surface-50',
      props.collapsed ? 'gap-3 px-3 md:justify-center md:gap-0 md:px-2' : 'gap-3 px-3',
    ]
  : 'flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-xs font-semibold text-surface-500 transition-colors hover:bg-surface-100 hover:text-ink-900 dark:hover:bg-surface-800 dark:hover:text-surface-50')
</script>

<template>
  <button
    type="button"
    :aria-label="ariaLabel"
    :class="buttonClass"
    :title="ariaLabel"
    @click="app.toggleLocale()"
  >
    <svg
      :class="variant === 'sidebar' ? 'h-[18px] w-[18px]' : 'h-4 w-4'"
      class="shrink-0 transition-transform duration-200 ease-brand group-hover:scale-105"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-linecap="round"
      stroke-linejoin="round"
      :stroke-width="variant === 'sidebar' ? 1.6 : 1.8"
    >
      <path d="M4 5h8" />
      <path d="M8 3v2" />
      <path d="M5 9c1.2 2.8 3.2 4.9 6 6" />
      <path d="M11 9c-.8 1.9-2 3.6-3.7 5" />
      <path d="M13 21l4-9 4 9" />
      <path d="M14.4 18h5.2" />
    </svg>
    <span v-if="variant === 'sidebar'" :class="collapsed ? 'md:hidden' : ''">{{ $t('language.label') }}</span>
    <span
      :class="variant === 'sidebar'
        ? ['ml-auto rounded-md border border-surface-200 px-1.5 py-0.5 text-2xs font-semibold tracking-wide text-surface-500 dark:border-surface-700 dark:text-surface-400', collapsed ? 'md:hidden' : '']
        : 'hidden sm:inline'"
    >
      {{ currentCode }}
    </span>
  </button>
</template>
