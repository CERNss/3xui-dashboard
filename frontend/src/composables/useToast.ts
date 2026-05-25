import { computed, ref } from 'vue'

export type ToastKind = 'success' | 'error'

export interface ToastItem {
  id: number
  kind: ToastKind
  text: string
  durationMs: number
}

const DEFAULT_DURATION_MS = 3600

const toasts = ref<ToastItem[]>([])
const timers = new Map<number, ReturnType<typeof setTimeout>>()
let nextID = 1

export function pushToast(text: string, kind: ToastKind = 'success', durationMs = DEFAULT_DURATION_MS): number | undefined {
  const trimmed = text.trim()
  if (!trimmed) return undefined

  const existing = toasts.value.find((toast) => toast.kind === kind && toast.text === trimmed)
  if (existing) {
    const timer = timers.get(existing.id)
    if (timer) clearTimeout(timer)
    existing.durationMs = durationMs
    timers.set(existing.id, setTimeout(() => dismissToast(existing.id), durationMs))
    toasts.value = [...toasts.value]
    return existing.id
  }

  const id = nextID++
  const next = [
    ...toasts.value,
    { id, kind, text: trimmed, durationMs },
  ].slice(-4)
  const kept = new Set(next.map((toast) => toast.id))
  for (const toast of toasts.value) {
    if (!kept.has(toast.id)) clearToastTimer(toast.id)
  }
  toasts.value = next

  timers.set(id, setTimeout(() => dismissToast(id), durationMs))
  return id
}

export function dismissToast(id: number) {
  clearToastTimer(id)
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

export function clearToasts() {
  for (const id of timers.keys()) clearToastTimer(id)
  toasts.value = []
}

function clearToastTimer(id: number) {
  const timer = timers.get(id)
  if (timer) clearTimeout(timer)
  timers.delete(id)
}

export function useToasts() {
  return {
    toasts: computed(() => toasts.value),
    dismissToast,
    pushToast,
  }
}
