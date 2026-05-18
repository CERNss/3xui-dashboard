<template>
  <router-view />
  <!-- Global Toast Notifications -->
  <div class="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full">
    <TransitionGroup name="toast">
      <div
        v-for="toast in appStore.toasts"
        :key="toast.id"
        class="flex items-start gap-3 p-4 rounded-xl shadow-lg border animate-slide-up"
        :class="{
          'bg-green-50 border-green-200 text-green-800': toast.type === 'success',
          'bg-red-50 border-red-200 text-red-800': toast.type === 'error',
          'bg-yellow-50 border-yellow-200 text-yellow-800': toast.type === 'warning',
          'bg-blue-50 border-blue-200 text-blue-800': toast.type === 'info'
        }"
      >
        <span class="text-lg">
          <span v-if="toast.type === 'success'">✓</span>
          <span v-else-if="toast.type === 'error'">✗</span>
          <span v-else-if="toast.type === 'warning'">⚠</span>
          <span v-else>ℹ</span>
        </span>
        <p class="text-sm flex-1">{{ toast.message }}</p>
        <button
          class="text-current opacity-60 hover:opacity-100 text-lg leading-none"
          @click="appStore.removeToast(toast.id)"
        >
          ×
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
</script>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.2s ease;
}
.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
