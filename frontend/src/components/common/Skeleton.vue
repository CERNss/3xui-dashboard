<script setup lang="ts">
defineProps<{
  rows?: number       // number of skeleton rows (default 5)
  height?: string     // tailwind height class for each row (default h-12)
  variant?: 'table' | 'card' | 'kpi'
}>()
</script>

<template>
  <div v-if="(variant ?? 'table') === 'kpi'" class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
    <div
      v-for="i in (rows ?? 4)"
      :key="i"
      class="relative h-28 overflow-hidden rounded-2xl border border-surface-200 bg-surface-100 dark:border-surface-800 dark:bg-surface-800/40"
    >
      <div class="absolute inset-0 -translate-x-full animate-shimmer bg-skeleton-shimmer bg-[length:400px_100%] dark:bg-skeleton-shimmer-dark" />
    </div>
  </div>

  <div
    v-else-if="(variant ?? 'table') === 'card'"
    class="rounded-2xl border border-surface-200 bg-surface-100 p-6 dark:border-surface-800 dark:bg-surface-800/40"
  >
    <div class="space-y-3">
      <div
        v-for="i in (rows ?? 3)"
        :key="i"
        :class="['relative overflow-hidden rounded', height ?? 'h-4', i === 1 ? 'w-1/2' : i === 2 ? 'w-3/4' : 'w-full']"
      >
        <div class="h-full w-full bg-surface-200 dark:bg-surface-700" />
        <div class="absolute inset-0 -translate-x-full animate-shimmer bg-skeleton-shimmer bg-[length:400px_100%] dark:bg-skeleton-shimmer-dark" />
      </div>
    </div>
  </div>

  <div
    v-else
    class="overflow-hidden rounded-2xl border border-surface-200 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
  >
    <div class="divide-y divide-surface-200 dark:divide-surface-800">
      <div
        v-for="i in (rows ?? 5)"
        :key="i"
        :class="['relative overflow-hidden bg-surface-50 dark:bg-surface-800/30', height ?? 'h-14']"
      >
        <div class="absolute inset-0 -translate-x-full animate-shimmer bg-skeleton-shimmer bg-[length:400px_100%] dark:bg-skeleton-shimmer-dark" />
      </div>
    </div>
  </div>
</template>
