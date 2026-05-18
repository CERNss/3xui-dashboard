<template>
  <div class="flex items-center justify-center" :class="fullPage ? 'min-h-64' : ''">
    <svg
      class="animate-spin text-primary-600"
      :class="sizeClass"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </svg>
    <span v-if="label" class="ml-2 text-sm text-gray-600">{{ label }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  size?: 'sm' | 'md' | 'lg'
  fullPage?: boolean
  label?: string
}>(), { size: 'md', fullPage: false })

const sizeClass = computed(() => ({
  'w-4 h-4': props.size === 'sm',
  'w-6 h-6': props.size === 'md',
  'w-10 h-10': props.size === 'lg'
}))
</script>
