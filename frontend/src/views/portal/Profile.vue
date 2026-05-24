<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { portalProfileApi, type LoginMethodsResponse, type UserProfile } from '@/api/portal/profile'
import { formatError } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const profile = ref<UserProfile | null>(null)
const loginMethods = ref<LoginMethodsResponse | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const securityFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)
const linkingOIDC = ref(false)

// Change password form
const oldPw = ref('')
const newPw = ref('')
const confirmPw = ref('')
const changingPw = ref(false)
const pwFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

// Bind email form
const email = ref('')
const bindingEmail = ref(false)
const emailFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

async function load() {
  loading.value = true
  error.value = null
  try {
    const [profileRes, methodsRes] = await Promise.all([
      portalProfileApi.get(),
      portalProfileApi.loginMethods(),
    ])
    profile.value = profileRes
    loginMethods.value = methodsRes
    if (profile.value.email) email.value = profile.value.email
  } catch (e: any) {
    error.value = formatError(e, t('portal.profile.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function changePassword() {
  pwFlash.value = null
  if (newPw.value !== confirmPw.value) {
    pwFlash.value = { kind: 'err', text: t('portal.profile.pwsMustMatch') }
    return
  }
  if (newPw.value.length < 8) {
    pwFlash.value = { kind: 'err', text: t('portal.profile.newPwMin8') }
    return
  }
  changingPw.value = true
  try {
    await portalProfileApi.changePassword(oldPw.value, newPw.value)
    pwFlash.value = { kind: 'ok', text: t('portal.profile.changePwOk') }
    oldPw.value = ''
    newPw.value = ''
    confirmPw.value = ''
  } catch (e) {
    pwFlash.value = { kind: 'err', text: formatError(e, t('portal.profile.changePwFailed')) }
  } finally {
    changingPw.value = false
  }
}

async function bindEmail() {
  emailFlash.value = null
  if (!email.value || !/^.+@.+\..+$/.test(email.value)) {
    emailFlash.value = { kind: 'err', text: t('portal.profile.emailFormat') }
    return
  }
  bindingEmail.value = true
  try {
    await portalProfileApi.bindEmail(email.value)
    emailFlash.value = { kind: 'ok', text: t('portal.profile.bindFlash') }
    await load()
  } catch (e) {
    emailFlash.value = { kind: 'err', text: formatError(e, t('portal.profile.bindFailed')) }
  } finally {
    bindingEmail.value = false
  }
}

const hasOIDCOnly = computed(() => profile.value && !profile.value.email)
const providerName = computed(() => loginMethods.value?.oidc.name || t('portal.profile.loginMethods.providerFallback'))
const oidcIconSafe = computed(() => {
  const raw = loginMethods.value?.oidc.icon
  if (!raw) return null
  // Only allow http(s) and data: URIs — IDP-supplied URLs reach <img> directly.
  if (/^(https?:|data:image\/)/i.test(raw)) return raw
  return null
})
const emailBound = computed(() => loginMethods.value?.email.bound ?? !!profile.value?.email)
const emailVerified = computed(() => loginMethods.value?.email.verified ?? !!profile.value?.email_verified)
const oidcEnabled = computed(() => !!loginMethods.value?.oidc.enabled)
const oidcBound = computed(() => loginMethods.value?.oidc.bound ?? !!profile.value?.oidc_subject)
const canLinkOIDC = computed(() => oidcEnabled.value && !oidcBound.value && !linkingOIDC.value)

function scrollToEmailForm() {
  document.getElementById('email-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function startOIDCLink() {
  if (!oidcEnabled.value || oidcBound.value) return
  securityFlash.value = null
  linkingOIDC.value = true
  try {
    const { authorize_url } = await portalProfileApi.startOIDCLink('/portal/profile?linked=oidc')
    window.location.href = authorize_url
  } catch (e) {
    linkingOIDC.value = false
    securityFlash.value = { kind: 'err', text: formatError(e, t('portal.profile.loginMethods.linkFailed')) }
  }
}

onMounted(async () => {
  const linkedJustNow = route.query.linked === 'oidc'
  if (linkedJustNow) {
    await router.replace({ query: { ...route.query, linked: undefined } })
  }
  await load()
  if (linkedJustNow) {
    securityFlash.value = { kind: 'ok', text: t('portal.profile.loginMethods.linkOk', { provider: providerName.value }) }
  }
})
</script>

<template>
  <div>
    <header class="mb-7">
      <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.title') }}</h1>
      <p class="mt-1.5 text-sm text-surface-500 dark:text-surface-400">{{ $t('portal.profile.subtitle') }}</p>
    </header>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
    </p>

    <div v-if="loading" class="text-sm text-surface-500">{{ $t('app.loading') }}</div>

    <section v-else-if="profile" class="space-y-5">
      <!-- Account summary -->
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-6 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.accountInfo') }}</h2>
        <dl class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <dt class="text-xs text-surface-500">{{ $t('portal.profile.column.userId') }}</dt>
            <dd class="mt-1 font-mono text-sm text-ink-900 dark:text-surface-50">#{{ profile.id }}</dd>
          </div>
          <div>
            <dt class="text-xs text-surface-500">{{ $t('portal.profile.column.email') }}</dt>
            <dd class="mt-1 flex items-center gap-2">
              <span class="text-sm text-ink-900 dark:text-surface-50">{{ profile.email || $t('portal.profile.noEmail') }}</span>
              <span v-if="profile.email" class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-2xs font-medium ring-1 ring-inset"
                :class="profile.email_verified
                  ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                  : 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'">
                {{ profile.email_verified ? $t('portal.profile.verified') : $t('portal.profile.unverified') }}
              </span>
            </dd>
          </div>
          <div>
            <dt class="text-xs text-surface-500">{{ $t('portal.profile.column.status') }}</dt>
            <dd class="mt-1">
              <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
                :class="profile.status === 'active'
                  ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                  : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'">
                {{ profile.status === 'active' ? $t('portal.profile.status.active') : $t('portal.profile.status.suspended') }}
              </span>
            </dd>
          </div>
          <div>
            <dt class="text-xs text-surface-500">{{ $t('portal.profile.column.createdAt') }}</dt>
            <dd class="mt-1 text-sm text-ink-900 dark:text-surface-50">{{ new Date(profile.created_at).toLocaleDateString() }}</dd>
          </div>
        </dl>
      </div>

      <!-- Login methods -->
      <div class="overflow-hidden rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
        <div class="border-b border-surface-100 bg-gradient-to-r from-surface-50 to-surface-0 px-6 py-5 dark:border-surface-800 dark:from-surface-900 dark:to-surface-900">
          <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.16em] text-accent-600 dark:text-accent-300">{{ $t('portal.profile.loginMethods.eyebrow') }}</p>
              <h2 class="mt-2 text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.loginMethods.title') }}</h2>
              <p class="mt-1 max-w-2xl text-sm leading-6 text-surface-500">{{ $t('portal.profile.loginMethods.subtitle') }}</p>
            </div>
            <span class="inline-flex w-fit items-center rounded-full bg-surface-900 px-3 py-1 text-xs font-semibold text-white dark:bg-surface-50 dark:text-ink-900">
              {{ emailBound && oidcBound ? $t('portal.profile.loginMethods.multiReady') : $t('portal.profile.loginMethods.oneReady') }}
            </span>
          </div>
        </div>

        <div class="divide-y divide-surface-100 dark:divide-surface-800">
          <div class="grid gap-4 px-6 py-5 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div class="flex min-w-0 gap-4">
              <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-sky-50 text-sky-700 ring-1 ring-sky-100 dark:bg-sky-950/40 dark:text-sky-300 dark:ring-sky-800">
                <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" /></svg>
              </div>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ $t('portal.profile.loginMethods.emailTitle') }}</h3>
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-semibold ring-1 ring-inset"
                    :class="emailBound
                      ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                      : 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700'"
                  >
                    {{ emailBound ? $t('portal.profile.loginMethods.bound') : $t('portal.profile.loginMethods.unbound') }}
                  </span>
                  <span
                    v-if="emailBound"
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-semibold ring-1 ring-inset"
                    :class="emailVerified
                      ? 'bg-sky-50 text-sky-700 ring-sky-100 dark:bg-sky-950/40 dark:text-sky-300 dark:ring-sky-800'
                      : 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'"
                  >
                    {{ emailVerified ? $t('portal.profile.verified') : $t('portal.profile.unverified') }}
                  </span>
                </div>
                <p class="mt-1 truncate text-sm text-surface-600 dark:text-surface-300">{{ profile.email || $t('portal.profile.loginMethods.noEmailBound') }}</p>
                <p class="mt-1 text-xs leading-5 text-surface-500">{{ $t('portal.profile.loginMethods.emailHint') }}</p>
              </div>
            </div>
            <button
              type="button"
              class="inline-flex h-9 items-center justify-center gap-1.5 rounded-xl border border-surface-200 px-3.5 text-sm font-semibold text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
              @click="scrollToEmailForm"
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9" /><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z" /></svg>
              {{ $t('portal.profile.loginMethods.manageEmail') }}
            </button>
          </div>

          <div class="grid gap-4 px-6 py-5 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div class="flex min-w-0 gap-4">
              <div class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-emerald-50 text-emerald-700 ring-1 ring-emerald-100 dark:bg-emerald-950/40 dark:text-emerald-300 dark:ring-emerald-800">
                <img v-if="oidcIconSafe" :src="oidcIconSafe" alt="" referrerpolicy="no-referrer" class="h-full w-full object-cover" />
                <svg v-else class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M2 12h20" /><path d="M12 2a15.3 15.3 0 0 1 0 20" /><path d="M12 2a15.3 15.3 0 0 0 0 20" /></svg>
              </div>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-sm font-semibold text-ink-900 dark:text-surface-50">{{ providerName }}</h3>
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-semibold ring-1 ring-inset"
                    :class="oidcBound
                      ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
                      : oidcEnabled
                        ? 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'
                        : 'bg-surface-100 text-surface-600 ring-surface-200 dark:bg-surface-800 dark:text-surface-300 dark:ring-surface-700'"
                  >
                    {{ oidcBound ? $t('portal.profile.loginMethods.bound') : (oidcEnabled ? $t('portal.profile.loginMethods.unbound') : $t('portal.profile.loginMethods.unavailable')) }}
                  </span>
                </div>
                <p class="mt-1 text-sm text-surface-600 dark:text-surface-300">
                  {{ oidcBound ? $t('portal.profile.loginMethods.oidcBoundText', { provider: providerName }) : $t('portal.profile.loginMethods.oidcUnboundText', { provider: providerName }) }}
                </p>
                <p class="mt-1 text-xs leading-5 text-surface-500">{{ oidcEnabled ? $t('portal.profile.loginMethods.oidcHint') : $t('portal.profile.loginMethods.oidcUnavailableHint') }}</p>
              </div>
            </div>
            <button
              type="button"
              :disabled="!canLinkOIDC"
              class="inline-flex h-9 items-center justify-center gap-1.5 rounded-xl bg-ink-900 px-3.5 text-sm font-semibold text-white shadow-card transition-all hover:bg-ink-800 active:scale-[0.98] disabled:cursor-not-allowed disabled:bg-surface-200 disabled:text-surface-500 disabled:shadow-none dark:bg-accent-600 dark:hover:bg-accent-500 dark:disabled:bg-surface-800 dark:disabled:text-surface-500"
              :title="!oidcEnabled ? $t('portal.profile.loginMethods.oidcUnavailableHint') : undefined"
              @click="startOIDCLink"
            >
              <svg v-if="linkingOIDC" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
              <svg v-else class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1" /><path d="M14 11a5 5 0 0 0-7.1 0l-2 2A5 5 0 0 0 12 20.1l1.1-1.1" /></svg>
              {{ oidcBound ? $t('portal.profile.loginMethods.alreadyLinked') : (linkingOIDC ? $t('portal.profile.loginMethods.linking') : $t('portal.profile.loginMethods.linkProvider', { provider: providerName })) }}
            </button>
          </div>
        </div>

        <p
          v-if="securityFlash"
          class="mx-6 mb-5 rounded-xl px-4 py-3 text-sm ring-1 ring-inset"
          :class="securityFlash.kind === 'ok'
            ? 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
            : 'bg-red-50 text-red-600 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'"
        >
          {{ securityFlash.text }}
        </p>
      </div>

      <!-- Bind email (only when no email or for adding/changing) -->
      <div id="email-panel" class="scroll-mt-24 rounded-2xl border border-surface-100 bg-surface-0 p-6 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.bindEmail') }}</h2>
        <p class="mt-1 text-xs text-surface-500">
          <template v-if="hasOIDCOnly">{{ $t('portal.profile.oidcOnlyMsg') }}</template>
          <template v-else>{{ $t('portal.profile.regularEmailMsg') }}</template>
        </p>
        <form class="mt-4 max-w-md" @submit.prevent="bindEmail">
          <div class="relative">
            <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" /></svg>
            <input
              v-model="email"
              type="email"
              required
              placeholder="you@example.com"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900"
            />
          </div>
          <p v-if="emailFlash" class="mt-2 text-xs" :class="emailFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'">
            {{ emailFlash.text }}
          </p>
          <button
            type="submit"
            :disabled="bindingEmail"
            class="mt-4 inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500"
          >
            <svg v-if="bindingEmail" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
            {{ bindingEmail ? $t('portal.profile.bindingEmail') : $t('portal.profile.bind') }}
          </button>
        </form>
      </div>

      <!-- Change password -->
      <div class="rounded-2xl border border-surface-100 bg-surface-0 p-6 dark:border-surface-800 dark:bg-surface-900">
        <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.changePw') }}</h2>
        <p class="mt-1 text-xs text-surface-500">{{ $t('portal.profile.pwMin8') }}</p>
        <form class="mt-4 max-w-md space-y-3" @submit.prevent="changePassword">
          <div v-if="!hasOIDCOnly">
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('portal.profile.currentPw') }}</label>
            <input
              v-model="oldPw"
              type="password"
              autocomplete="current-password"
              required
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ hasOIDCOnly ? $t('auth.initPassword') : $t('portal.profile.newPw') }}</label>
            <input
              v-model="newPw"
              type="password"
              autocomplete="new-password"
              required
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900"
            />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('portal.profile.confirmPw') }}</label>
            <input
              v-model="confirmPw"
              type="password"
              autocomplete="new-password"
              required
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 px-3.5 py-2.5 text-sm transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/40 dark:border-surface-700 dark:bg-surface-900"
            />
          </div>
          <p v-if="pwFlash" class="text-xs" :class="pwFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'">
            {{ pwFlash.text }}
          </p>
          <button
            type="submit"
            :disabled="changingPw"
            class="inline-flex h-9 items-center gap-1.5 rounded-xl bg-ink-900 px-4 text-sm font-medium text-white shadow-card transition-all ease-brand hover:bg-ink-800 active:scale-[0.98] disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500"
          >
            <svg v-if="changingPw" class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.2-8.55" /></svg>
            {{ changingPw ? $t('portal.profile.updatingPw') : $t('portal.profile.updatePw') }}
          </button>
        </form>
      </div>
    </section>

    <!-- Secondary actions: Orders moved here so the primary nav
         stays at 4 items (Sub2API portal shape). -->
    <section class="mt-6 rounded-2xl border border-surface-100 bg-surface-0 px-6 py-5 dark:border-surface-800 dark:bg-surface-900">
      <h2 class="text-[15px] font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('portal.profile.otherActions') }}</h2>
      <div class="mt-3 flex flex-wrap gap-2">
        <router-link
          to="/portal/orders"
          class="inline-flex items-center gap-1.5 rounded-xl border border-surface-200 px-3.5 py-2 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
        >
          <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" /></svg>
          {{ $t('portal.profile.orderHistory') }}
        </router-link>
      </div>
    </section>
  </div>
</template>
