<script setup lang="ts">
import { useToasts } from '@/composables/useToast'

const { toasts, dismissToast } = useToasts()
</script>

<template>
  <Teleport to="body">
    <div class="fixed right-4 top-4 z-[80] flex w-[min(22rem,calc(100vw-2rem))] flex-col gap-2 sm:right-6 sm:top-6">
      <TransitionGroup name="toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="relative overflow-hidden rounded-lg border bg-surface-900 text-surface-50 shadow-elevated dark:border-surface-700 dark:bg-surface-900"
          :class="toast.kind === 'success'
            ? 'border-accent-500/30'
            : 'border-red-500/35'"
        >
          <div class="flex items-start gap-3 px-4 py-3.5">
            <span
              class="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full"
              :class="toast.kind === 'success'
                ? 'text-emerald-400'
                : 'text-red-300'"
            >
              <svg v-if="toast.kind === 'success'" class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M20 6 9 17l-5-5" />
                <circle cx="12" cy="12" r="9" />
              </svg>
              <svg v-else class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="9" />
                <path d="M12 8v4" />
                <path d="M12 16h.01" />
              </svg>
            </span>
            <p class="min-w-0 flex-1 text-sm font-medium leading-5">{{ toast.text }}</p>
            <button
              type="button"
              class="-mr-1 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-800 hover:text-surface-100"
              :aria-label="$t('toast.dismiss')"
              @click="dismissToast(toast.id)"
            >
              <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M18 6 6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div
            class="h-1 origin-left animate-toast-progress"
            :class="toast.kind === 'success' ? 'bg-accent-500' : 'bg-red-400'"
            :style="{ animationDuration: `${toast.durationMs}ms` }"
          />
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 180ms cubic-bezier(0.16, 1, 0.3, 1),
    transform 220ms cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.98);
}

.toast-move {
  transition: transform 220ms cubic-bezier(0.16, 1, 0.3, 1);
}
</style>
