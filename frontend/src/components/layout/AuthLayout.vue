<script setup lang="ts">
import { useThemeStore } from '@/stores/theme'

defineProps<{
  /** Top-of-card title shown above the form. Optional — leave blank to skip. */
  cardTitle?: string
  /** One-line subtitle under the card title. */
  cardSubtitle?: string
}>()

// Theme follows system preference pre-login (see stores/theme.ts readInitial).
// The toggle lives in the post-login sidebar, not here.
const theme = useThemeStore()
</script>

<template>
  <!--
    Layered auth chrome — inspired by Sub2API:
      L1: ambient gradient + dotted grid (background)
      L2: brand block (logo + name + slogan), centered
      L3: form card (slot), restrained width, hairline border

    The grid is drawn as an SVG data-URI so it stays crisp at any DPR and
    doesn't load an extra asset.
  -->
  <div class="relative flex min-h-full flex-col items-center justify-center overflow-hidden bg-surface-50 px-6 py-10 dark:bg-[#0b1018]">
    <!-- L1a: ambient gradient blobs — Sub2API uses subtle tinted edges -->
    <div class="pointer-events-none absolute -left-40 -top-40 h-[480px] w-[480px] rounded-full bg-accent-300/20 blur-3xl dark:bg-accent-700/15"></div>
    <div class="pointer-events-none absolute -bottom-40 -right-40 h-[480px] w-[480px] rounded-full bg-primary-300/15 blur-3xl dark:bg-primary-700/10"></div>

    <!-- L1b: dotted grid overlay -->
    <div
      class="pointer-events-none absolute inset-0 opacity-[0.35] dark:opacity-[0.22]"
      :style="{
        backgroundImage:
          'radial-gradient(circle, currentColor 1px, transparent 1px)',
        backgroundSize: '24px 24px',
        color: theme.theme === 'dark' ? 'rgba(168,162,158,0.18)' : 'rgba(120,113,108,0.13)',
      }"
    ></div>

    <!-- L1c: top edge fade (so grid doesn't fight with content) -->
    <div class="pointer-events-none absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-surface-50 to-transparent dark:from-[#0b1018]"></div>
    <div class="pointer-events-none absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-surface-50 to-transparent dark:from-[#0b1018]"></div>

    <!-- L2: brand block — 集换社 pattern: bold brand-tinted name + small muted slogan -->
    <div class="relative z-10 mb-7 flex flex-col items-center text-center">
      <div class="relative">
        <!-- Solid accent square (集换社-style chunky tile) with lightning glyph -->
        <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-accent-500 text-white shadow-elevated ring-1 ring-accent-700/40">
          <svg class="h-9 w-9" viewBox="0 0 24 24" fill="currentColor" stroke="none">
            <path d="M13 2 3 14h7l-1 8 11-13h-7l0-7z" />
          </svg>
        </div>
        <!-- ambient glow that bleeds the brand color into the page -->
        <div class="absolute inset-0 -z-10 rounded-2xl bg-accent-500/40 blur-2xl"></div>
      </div>
      <h1 class="mt-5 bg-gradient-to-r from-accent-500 to-accent-700 bg-clip-text text-[2rem] font-bold leading-none tracking-tight text-transparent dark:from-accent-300 dark:to-accent-500">
        3xui Central
      </h1>
      <p class="mt-2.5 text-sm text-surface-500 dark:text-surface-400">
        Multi-node 3x-ui · Fleet 聚合 · 流量分账 · 订阅导出
      </p>
    </div>

    <!-- L3: form card (slot) — heavier shadow, bigger inner heading -->
    <div
      class="relative z-10 w-full max-w-md animate-scale-in rounded-2xl border border-surface-100 bg-surface-0/90 p-8 shadow-elevated backdrop-blur-md dark:border-surface-800 dark:bg-surface-900/80"
    >
      <div v-if="cardTitle || $slots.title" class="mb-6 text-center">
        <h2 class="text-2xl font-bold tracking-tight text-ink-900 dark:text-surface-50">
          <slot name="title">{{ cardTitle }}</slot>
        </h2>
        <p v-if="cardSubtitle" class="mt-2 text-sm text-surface-500">{{ cardSubtitle }}</p>
      </div>
      <slot />
    </div>

    <!-- Footer -->
    <p class="relative z-10 mt-6 text-2xs text-surface-400 dark:text-surface-600">
      © 2026 3xui Central · 自托管 multi-node 控制面板
    </p>
  </div>
</template>
