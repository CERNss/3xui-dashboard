<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'

interface Props {
  open: boolean
  title: string
  message?: string
  /** Visual tone for the primary action. */
  variant?: 'default' | 'danger'
  confirmLabel?: string
  cancelLabel?: string
  /** When true, the confirm button shows a spinner + is disabled. */
  busy?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'default',
  confirmLabel: '确认',
  cancelLabel: '取消',
  busy: false,
})

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
  (e: 'update:open', v: boolean): void
}>()

function close() {
  if (props.busy) return
  emit('cancel')
  emit('update:open', false)
}
function confirm() {
  if (props.busy) return
  emit('confirm')
}

// Escape key closes the modal (unless busy). Mounted once; the watch
// on `open` toggles the listener so we don't trap Escape globally
// when no modal is showing.
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}
watch(() => props.open, (v) => {
  if (v) {
    document.addEventListener('keydown', onKey)
  } else {
    document.removeEventListener('keydown', onKey)
  }
})
onMounted(() => {
  if (props.open) document.addEventListener('keydown', onKey)
})
onUnmounted(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm"
      @click.self="close"
    >
      <div class="w-full max-w-sm animate-scale-in rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900">
        <div class="px-6 pt-6">
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl"
              :class="variant === 'danger'
                ? 'bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-300'
                : 'bg-accent-50 text-accent-600 dark:bg-accent-950/40 dark:text-accent-300'"
            >
              <svg v-if="variant === 'danger'" class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
                <path d="M12 9v4M12 17h.01" />
              </svg>
              <svg v-else class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" />
                <path d="M12 8v4M12 16h.01" />
              </svg>
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ title }}</h2>
              <p v-if="message" class="mt-1.5 whitespace-pre-line text-sm text-surface-600 dark:text-surface-300">{{ message }}</p>
            </div>
          </div>
        </div>
        <footer class="mt-5 flex justify-end gap-2 border-t border-surface-100 px-6 py-4 dark:border-surface-800">
          <button
            type="button"
            :disabled="busy"
            class="inline-flex h-9 items-center rounded-xl border border-surface-200 px-4 text-sm font-medium text-surface-700 transition-all hover:bg-surface-50 active:scale-[0.98] disabled:opacity-60 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
            @click="close"
          >
            {{ cancelLabel }}
          </button>
          <button
            type="button"
            :disabled="busy"
            class="inline-flex h-9 items-center gap-1.5 rounded-xl px-4 text-sm font-medium text-white shadow-card transition-all active:scale-[0.98] disabled:opacity-60"
            :class="variant === 'danger'
              ? 'bg-red-600 hover:bg-red-700'
              : 'bg-ink-900 hover:bg-ink-800 dark:bg-accent-600 dark:hover:bg-accent-500'"
            @click="confirm"
          >
            <svg v-if="busy" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round">
              <path d="M21 12a9 9 0 1 1-6.2-8.55" />
            </svg>
            {{ confirmLabel }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>
