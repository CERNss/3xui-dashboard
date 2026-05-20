<script setup lang="ts">
defineProps<{
  icon?: string  // SVG path body
  title: string
  description?: string
  actionLabel?: string
}>()
defineEmits<{ (e: 'action'): void }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
    <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-accent-50 text-accent-600 dark:bg-accent-950 dark:text-accent-300">
      <svg v-if="icon" class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
        <path :d="icon" />
      </svg>
      <svg v-else class="h-7 w-7" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v4M12 16h.01" />
      </svg>
    </div>
    <h3 class="text-sm font-semibold text-surface-700 dark:text-surface-200">{{ title }}</h3>
    <p v-if="description" class="max-w-sm text-xs leading-relaxed text-surface-500">{{ description }}</p>
    <button
      v-if="actionLabel"
      class="mt-2 inline-flex items-center gap-1.5 rounded-lg bg-accent-600 px-4 py-2 text-sm font-medium text-white transition-brand transition hover:bg-accent-700 active:scale-95"
      @click="$emit('action')"
    >
      <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 5v14M5 12h14" />
      </svg>
      {{ actionLabel }}
    </button>
  </div>
</template>
