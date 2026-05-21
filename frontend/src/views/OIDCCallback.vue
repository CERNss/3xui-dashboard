<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { portalAuthApi } from '@/api/portal/auth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { formatError } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const auth = usePortalAuthStore()
const error = ref<string | null>(null)

onMounted(async () => {
  const code = typeof route.query.code === 'string' ? route.query.code : ''
  const state = typeof route.query.state === 'string' ? route.query.state : ''
  if (!code || !state) {
    error.value = 'IDP 没有返回 code + state 参数。请回到登录页重新发起。'
    return
  }
  try {
    const res = await portalAuthApi.oidcCallback(code, state)
    auth.setSession(res.token, { id: res.user_id, email: res.email })
    // Default landing matches the Sub2API-style portal slim-down:
    // primary action is the subscription view.
    router.replace('/portal/subscription')
  } catch (e: any) {
    error.value = formatError(e, 'OIDC 登录失败')
  }
})
</script>

<template>
  <div class="flex min-h-[60vh] items-center justify-center p-6">
    <div class="w-full max-w-md rounded-2xl border border-surface-100 bg-surface-0 p-6 text-center dark:border-surface-800 dark:bg-surface-900">
      <h1 class="text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">
        {{ error ? '登录失败' : '正在登录…' }}
      </h1>
      <p v-if="!error" class="mt-2 text-sm text-surface-500">从身份提供商返回，正在交换令牌。</p>
      <p v-else class="mt-3 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
        {{ error }}
      </p>
      <router-link
        v-if="error"
        to="/login"
        class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 dark:bg-accent-600 dark:hover:bg-accent-500"
      >
        返回登录
      </router-link>
    </div>
  </div>
</template>
