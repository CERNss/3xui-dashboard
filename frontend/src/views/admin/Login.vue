<script setup lang="ts">
import { ref } from 'vue'
import { formatError } from '@/utils/format'
import { useRouter, useRoute } from 'vue-router'

import AuthLayout from '@/components/layout/AuthLayout.vue'
import { adminAuthApi } from '@/api/admin/auth'
import { useAdminAuthStore } from '@/stores/adminAuth'

const router = useRouter()
const route = useRoute()
const auth = useAdminAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

async function onSubmit() {
  if (!username.value || !password.value) return
  loading.value = true
  error.value = null
  try {
    const res = await adminAuthApi.login(username.value, password.value)
    auth.setSession(res.token, res.username)
    const next = typeof route.query.next === 'string' ? route.query.next : '/admin'
    await router.push(next)
  } catch (e: any) {
    error.value = formatError(e, '登录失败 — 检查用户名密码')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout card-title="管理后台登录" card-subtitle="使用 ADMIN_USERNAME / ADMIN_PASSWORD 配置的凭证">
    <form class="space-y-4" @submit.prevent="onSubmit">
      <div>
        <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('auth.username') }}</label>
        <div class="relative">
          <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" />
          </svg>
          <input
            v-model="username"
            type="text"
            autocomplete="username"
            required
            placeholder="admin"
            class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
          />
        </div>
      </div>
      <div>
        <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('auth.password') }}</label>
        <div class="relative">
          <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
            placeholder="••••••••"
            class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
          />
        </div>
      </div>
      <button
        type="submit"
        :disabled="loading"
        class="inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-accent-600 to-accent-700 px-4 text-sm font-semibold text-white shadow-card transition-all ease-brand hover:from-accent-500 hover:to-accent-600 hover:shadow-card-hover active:scale-[0.98] disabled:opacity-60"
      >
        <svg v-if="!loading" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3" />
        </svg>
        <svg v-else class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round">
          <path d="M21 12a9 9 0 1 1-6.2-8.55" />
        </svg>
        {{ loading ? '登录中…' : '进入后台' }}
      </button>
      <p v-if="error" class="flex items-start gap-2 rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">
        <svg class="mt-0.5 h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg>
        <span>{{ error }}</span>
      </p>
    </form>
    <div class="mt-6 flex items-center justify-between border-t border-surface-100 pt-4 text-xs text-surface-500 dark:border-surface-800">
      <span>不是管理员？</span>
      <router-link to="/portal/login" class="font-medium text-accent-700 transition-colors hover:text-accent-600 dark:text-accent-300">前往用户端 →</router-link>
    </div>
  </AuthLayout>
</template>
