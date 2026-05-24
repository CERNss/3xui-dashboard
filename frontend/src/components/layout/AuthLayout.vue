<script setup lang="ts">
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useBrandingStore } from '@/stores/branding'

defineProps<{
  cardTitle?: string
  cardSubtitle?: string
}>()

const branding = useBrandingStore()
</script>

<template>
  <div class="app-dark-bg relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-surface-50 px-6 py-10 text-ink-900 dark:bg-surface-950 dark:text-surface-50">
    <div class="absolute right-4 top-4 z-20 sm:right-6 sm:top-6">
      <LocaleSwitcher variant="toolbar" />
    </div>

    <div
      class="pointer-events-none absolute inset-0 opacity-[0.32] dark:opacity-[0.16]"
      style="background-image: radial-gradient(circle, currentColor 1px, transparent 1px); background-size: 24px 24px; color: rgba(120,113,108,0.16);"
    ></div>
    <div class="pointer-events-none absolute inset-x-0 top-0 h-40 bg-gradient-to-b from-surface-50 to-transparent dark:from-surface-950"></div>
    <div class="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-surface-50 to-transparent dark:from-surface-950"></div>

    <div class="relative z-10 mb-7 flex flex-col items-center text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-500 text-white shadow-card ring-1 ring-accent-700/30">
        <img v-if="branding.iconUrl" :src="branding.iconUrl" alt="" class="h-9 w-9 rounded-xl object-cover" @error="branding.clearIcon()" />
        <svg v-else class="h-8 w-8" viewBox="0 0 24 24" fill="currentColor" stroke="none">
          <path d="M13 2 3 14h7l-1 8 11-13h-7V2z" />
        </svg>
      </div>
      <h1 class="mt-4 text-[2rem] font-bold leading-none tracking-tight text-ink-900 dark:text-surface-50">
        {{ branding.title || $t('app.title') }}
      </h1>
      <p class="mt-2.5 max-w-sm text-sm leading-6 text-surface-500">
        {{ branding.description || $t('brand.slogan') }}
      </p>
    </div>

    <div class="relative z-10 w-full max-w-md animate-scale-in rounded-2xl border border-surface-100 bg-surface-0/95 p-8 shadow-elevated backdrop-blur dark:border-surface-800 dark:bg-surface-900/90">
      <div v-if="cardTitle || $slots.title" class="mb-6 text-center">
        <h2 class="text-2xl font-bold tracking-tight text-ink-900 dark:text-surface-50">
          <slot name="title">{{ cardTitle }}</slot>
        </h2>
        <p v-if="cardSubtitle" class="mt-2 text-sm text-surface-500">{{ cardSubtitle }}</p>
      </div>
      <slot />
    </div>

    <p class="relative z-10 mt-6 text-2xs text-surface-400 dark:text-surface-600">
      {{ branding.footer || $t('brand.footer') }}
    </p>
  </div>
</template>
