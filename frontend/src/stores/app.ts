import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Toast, ToastType } from '@/types'

export const useAppStore = defineStore('app', () => {
  const loading = ref(false)
  const sidebarOpen = ref(false)
  const toasts = ref<Toast[]>([])
  let toastId = 0

  const hasToasts = computed(() => toasts.value.length > 0)

  function addToast(type: ToastType, message: string, duration = 4000) {
    const id = ++toastId
    toasts.value.push({ id, type, message })
    setTimeout(() => removeToast(id), duration)
  }

  function removeToast(id: number) {
    const idx = toasts.value.findIndex((t) => t.id === id)
    if (idx !== -1) toasts.value.splice(idx, 1)
  }

  function success(msg: string) { addToast('success', msg) }
  function error(msg: string) { addToast('error', msg, 6000) }
  function warn(msg: string) { addToast('warning', msg) }
  function info(msg: string) { addToast('info', msg) }

  function toggleSidebar() { sidebarOpen.value = !sidebarOpen.value }
  function closeSidebar() { sidebarOpen.value = false }

  return {
    loading,
    sidebarOpen,
    toasts,
    hasToasts,
    addToast,
    removeToast,
    success,
    error,
    warn,
    info,
    toggleSidebar,
    closeSidebar
  }
})
