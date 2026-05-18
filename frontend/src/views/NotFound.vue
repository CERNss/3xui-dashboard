<script setup lang="ts">
import { useRouter } from 'vue-router'
const router = useRouter()
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center gap-4 p-8 text-center">
    <h1 class="text-2xl font-semibold">404</h1>
    <p class="text-surface-500">{{ $t('errors.notFound') }}</p>
    <button
      class="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white shadow-card hover:bg-primary-700"
      @click="router.push('/')"
    >
      {{ $t('errors.backHome') }}
    </button>
  </div>
</template>
