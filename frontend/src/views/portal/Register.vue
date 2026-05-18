<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import AuthLayout from '@/components/layout/AuthLayout.vue'
import { portalAuthApi } from '@/api/portal/auth'
import { usePortalAuthStore } from '@/stores/portalAuth'

const router = useRouter()
const auth = usePortalAuthStore()

const email = ref('')
const password = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

async function onSubmit() {
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters.'
    return
  }
  loading.value = true
  error.value = null
  try {
    const res = await portalAuthApi.register(email.value, password.value)
    auth.setSession(res.token, { id: res.user_id, email: res.email })
    await router.push('/portal')
  } catch (e: any) {
    error.value = e?.message ?? 'Registration failed'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout>
    <h1 class="mb-2 text-xl font-semibold">{{ $t('nav.portal') }} · {{ $t('auth.register') }}</h1>
    <p class="mb-4 text-xs text-surface-500">
      Public registration may be disabled; if so, ask the administrator to create your account.
    </p>

    <form class="space-y-4" @submit.prevent="onSubmit">
      <div>
        <label class="mb-1 block text-sm font-medium">{{ $t('auth.email') }}</label>
        <input v-model="email" type="email" autocomplete="email" required class="w-full rounded-md border border-surface-300 bg-surface-0 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900" />
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">{{ $t('auth.password') }}</label>
        <input v-model="password" type="password" autocomplete="new-password" required minlength="8" class="w-full rounded-md border border-surface-300 bg-surface-0 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900" />
      </div>
      <button type="submit" :disabled="loading" class="w-full rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white shadow-card hover:bg-primary-700 disabled:opacity-60">
        {{ loading ? $t('app.loading') : $t('auth.submit') }}
      </button>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
    </form>

    <p class="mt-6 text-center text-sm text-surface-500">
      Already registered?
      <router-link to="/portal/login" class="text-primary-600 hover:underline">{{ $t('auth.login') }}</router-link>
    </p>
  </AuthLayout>
</template>
