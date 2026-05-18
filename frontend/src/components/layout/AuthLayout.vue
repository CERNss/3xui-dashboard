<script setup lang="ts">
</script>

<template>
  <div class="flex h-full items-center justify-center bg-surface-50 dark:bg-surface-950">
    <div
      class="w-full max-w-sm animate-scale-in rounded-xl bg-surface-0 p-8 shadow-card dark:bg-surface-900"
    >
      <slot />
    </div>
  </div>
</template>
