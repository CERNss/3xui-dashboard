<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { portalAuthApi, type OIDCResolveAction, type OIDCPendingResponse, type UserTokenResponse } from '@/api/portal/auth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { formatError } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const auth = usePortalAuthStore()
const error = ref<string | null>(null)
const pending = ref<OIDCPendingResponse | null>(null)
const resolving = ref<OIDCResolveAction | null>(null)
const { t } = useI18n()

function queryStringValue(value: unknown): string | null {
  return typeof value === 'string' ? value : null
}

function safeLocalRedirect(value: string | null | undefined): string | null {
  if (!value || !value.startsWith('/') || value.startsWith('//')) {
    return null
  }
  try {
    const url = new URL(value, window.location.origin)
    if (url.origin !== window.location.origin) {
      return null
    }
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

function redirectAfterLogin(res: UserTokenResponse) {
  const redirect = safeLocalRedirect(res.redirect_after)
    ?? safeLocalRedirect(res.next)
    ?? safeLocalRedirect(queryStringValue(route.query.redirect_after))
    ?? safeLocalRedirect(queryStringValue(route.query.next))
    ?? '/portal/subscription'
  router.replace(redirect)
}

function acceptToken(res: UserTokenResponse) {
  auth.setSession(res.token, { id: res.user_id, email: res.email })
  redirectAfterLogin(res)
}

function isPendingOIDCResponse(res: unknown): res is OIDCPendingResponse {
  return !!res && typeof res === 'object' && 'status' in res && (res as OIDCPendingResponse).status === 'pending'
}

onMounted(async () => {
  const code = typeof route.query.code === 'string' ? route.query.code : ''
  const state = typeof route.query.state === 'string' ? route.query.state : ''
  if (!code || !state) {
    error.value = t('auth.oidcNoCodeState')
    return
  }
  try {
    const res = await portalAuthApi.oidcCallback(code, state)
    if (isPendingOIDCResponse(res)) {
      pending.value = res
      return
    }
    acceptToken(res)
  } catch (e: any) {
    error.value = formatError(e, t('auth.oidcFailed'))
  }
})

async function resolvePending(action: OIDCResolveAction) {
  if (!pending.value) return
  resolving.value = action
  error.value = null
  try {
    const res = await portalAuthApi.oidcResolve(pending.value.pending_token, action)
    acceptToken(res)
  } catch (e) {
    error.value = formatError(e, t('auth.oidcResolveFailed'))
  } finally {
    resolving.value = null
  }
}
</script>

<template>
  <div class="flex min-h-[60vh] items-center justify-center p-6">
    <div class="w-full max-w-md rounded-2xl border border-surface-100 bg-surface-0 p-6 text-center dark:border-surface-800 dark:bg-surface-900">
      <h1 class="text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">
        {{ pending ? $t('auth.oidcDecisionTitle') : (error ? $t('auth.oidcReturningTitleFailed') : $t('auth.oidcReturningTitle')) }}
      </h1>
      <p v-if="!error && !pending" class="mt-2 text-sm text-surface-500">{{ $t('auth.oidcReturning') }}</p>
      <div v-if="pending" class="mt-4 text-left">
        <p class="text-sm leading-6 text-surface-600 dark:text-surface-300">
          {{ $t('auth.oidcDecisionBody', { email: pending.email }) }}
        </p>
        <div class="mt-4 grid gap-2">
          <button
            type="button"
            :disabled="resolving !== null"
            class="inline-flex h-10 items-center justify-center rounded-xl bg-ink-900 px-4 text-sm font-semibold text-white shadow-card transition-all hover:bg-ink-800 disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500"
            @click="resolvePending('bind')"
          >
            <svg v-if="resolving === 'bind'" class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
            {{ $t('auth.oidcBindExisting') }}
          </button>
          <button
            type="button"
            :disabled="resolving !== null"
            class="inline-flex h-10 items-center justify-center rounded-xl border border-surface-200 px-4 text-sm font-semibold text-surface-700 transition-all hover:bg-surface-50 disabled:opacity-60 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
            @click="resolvePending('recreate')"
          >
            <svg v-if="resolving === 'recreate'" class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
            {{ $t('auth.oidcRecreateAccount') }}
          </button>
        </div>
        <p class="mt-3 text-xs leading-5 text-surface-500 dark:text-surface-400">
          {{ $t(pending.existing_has_oidc ? 'auth.oidcRecreateHint' : 'auth.oidcBindHint') }}
        </p>
      </div>
      <p v-if="error" class="mt-3 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
        {{ error }}
      </p>
      <router-link
        v-if="error"
        to="/login"
        class="mt-4 inline-flex items-center gap-1.5 rounded-xl bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-card transition-all hover:bg-ink-800 dark:bg-accent-600 dark:hover:bg-accent-500"
      >
        {{ $t('auth.returnToLogin') }}
      </router-link>
    </div>
  </div>
</template>
