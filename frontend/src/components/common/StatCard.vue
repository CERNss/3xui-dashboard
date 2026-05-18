<template>
  <div class="card">
    <div class="flex items-center justify-between">
      <div>
        <p class="text-sm font-medium text-gray-500">{{ title }}</p>
        <p class="text-2xl font-bold text-gray-900 mt-1">{{ value }}</p>
        <p v-if="subtitle" class="text-xs text-gray-500 mt-0.5">{{ subtitle }}</p>
      </div>
      <div
        class="w-12 h-12 rounded-xl flex items-center justify-center"
        :class="iconBg"
      >
        <slot name="icon">
          <span class="text-2xl">{{ icon }}</span>
        </slot>
      </div>
    </div>
    <div v-if="trend !== undefined" class="mt-3 flex items-center gap-1 text-xs">
      <span
        class="font-medium"
        :class="trend >= 0 ? 'text-green-600' : 'text-red-600'"
      >
        {{ trend >= 0 ? '▲' : '▼' }} {{ Math.abs(trend) }}%
      </span>
      <span class="text-gray-500">vs last period</span>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  value: string | number
  subtitle?: string
  icon?: string
  iconBg?: string
  trend?: number
}>(), {
  iconBg: 'bg-primary-100'
})
</script>
