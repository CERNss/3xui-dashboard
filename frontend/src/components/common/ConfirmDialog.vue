<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
        @click.self="$emit('update:modelValue', false)"
      >
        <div class="bg-white rounded-2xl shadow-xl w-full max-w-md p-6 animate-slide-up">
          <div class="flex items-start gap-4">
            <div
              class="w-10 h-10 rounded-full flex items-center justify-center shrink-0"
              :class="dangerMode ? 'bg-red-100' : 'bg-yellow-100'"
            >
              <svg class="w-5 h-5" :class="dangerMode ? 'text-red-600' : 'text-yellow-600'" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <div class="flex-1">
              <h3 class="font-semibold text-gray-900">{{ title }}</h3>
              <p class="text-sm text-gray-600 mt-1">{{ message }}</p>
            </div>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button class="btn-secondary" @click="$emit('update:modelValue', false)">Cancel</button>
            <button
              class="btn"
              :class="dangerMode ? 'btn-danger' : 'btn-primary'"
              :disabled="loading"
              @click="$emit('confirm')"
            >
              <LoadingSpinner v-if="loading" size="sm" />
              {{ confirmText }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import LoadingSpinner from './LoadingSpinner.vue'

withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  message?: string
  confirmText?: string
  dangerMode?: boolean
  loading?: boolean
}>(), {
  title: 'Confirm Action',
  message: 'Are you sure you want to proceed?',
  confirmText: 'Confirm',
  dangerMode: false,
  loading: false
})

defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm'): void
}>()
</script>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
