<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import AuthLayout from '@/components/layout/AuthLayout.vue'
import { adminAuthApi } from '@/api/admin/auth'
import { portalAuthApi, type OIDCProvider } from '@/api/portal/auth'
import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'
import { formatError } from '@/utils/format'

const { t } = useI18n()

type Role = 'admin' | 'portal'
type Mode = 'login' | 'register'

const route = useRoute()
const router = useRouter()
const adminStore = useAdminAuthStore()
const portalStore = usePortalAuthStore()

// Mode switches the top tabs between sign-in and self-serve registration.
const mode = ref<Mode>(route.query.mode === 'register' ? 'register' : 'login')

const account = ref('')
const password = ref('')
const passwordConfirm = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

// ---- email verification code (register only) -------------------------------
// Backend sends a 6-digit code to `account` when the user clicks "发送验证码".
// Rate-limited 60s server-side; client mirrors that with `codeCooldown`.
const code = ref('')
const codeSending = ref(false)
const codeCooldown = ref(0)
const emailVerificationRequired = ref(true)
let cooldownTimer: number | undefined

function startCooldown(seconds: number) {
  codeCooldown.value = seconds
  window.clearInterval(cooldownTimer)
  cooldownTimer = window.setInterval(() => {
    codeCooldown.value -= 1
    if (codeCooldown.value <= 0) {
      codeCooldown.value = 0
      window.clearInterval(cooldownTimer)
    }
  }, 1000)
}

async function sendCode() {
  // Basic client-side guard so we don't fire a request for obviously bad emails.
  if (!account.value || !/^.+@.+\..+$/.test(account.value)) {
    error.value = t('auth.enterValidEmail')
    return
  }
  codeSending.value = true
  error.value = null
  try {
    await portalAuthApi.sendCode(account.value)
    startCooldown(60)
  } catch (e) {
    error.value = formatError(e, t('auth.codeFailedToSend'))
  } finally {
    codeSending.value = false
  }
}

// ---- OIDC providers (visible at bottom when configured) --------------------
const oidcProviders = ref<OIDCProvider[]>([])
async function loadOIDC() {
  try {
    oidcProviders.value = await portalAuthApi.oidcProviders()
  } catch {
    // Silent — endpoint may 404 on older backends; the OIDC section just hides.
    oidcProviders.value = []
  }
}

async function loadRegistrationPolicy() {
  try {
    const policy = await portalAuthApi.registrationPolicy()
    emailVerificationRequired.value = policy.email_verification_required
  } catch {
    emailVerificationRequired.value = true
  }
}

async function startOIDC(_p: OIDCProvider) {
  // POST /auth/oidc/start → returns the IDP's authorize URL.
  // Frontend navigates the browser there; the IDP eventually
  // redirects back to /oidc/callback?code=&state= which the
  // OIDCCallback.vue route handles.
  loading.value = true
  error.value = null
  try {
    const next = typeof route.query.next === 'string' ? route.query.next : '/portal'
    const { authorize_url } = await portalAuthApi.oidcStart(next)
    window.location.assign(authorize_url)
  } catch (e: any) {
    loading.value = false
    error.value = formatError(e, t('auth.oidcStarting'))
  }
}

onMounted(() => {
  loadOIDC()
  loadRegistrationPolicy()
})
onUnmounted(() => window.clearInterval(cooldownTimer))

const nextPath = computed(() => {
  return typeof route.query.next === 'string' ? route.query.next : null
})

const hintedRole = computed<Role | null>(() => {
  return route.query.hint === 'portal' || route.query.hint === 'admin'
    ? route.query.hint
    : null
})

const roleOrder = computed<Role[]>(() => {
  return hintedRole.value === 'portal' ? ['portal', 'admin'] : ['admin', 'portal']
})

// Try a single role for sign-in. Returns true on success (stores session),
// false on auth-class failures (400/401/403/404). Re-throws on network/5xx.
async function tryRole(role: Role, account: string, password: string): Promise<boolean> {
  try {
    if (role === 'admin') {
      const res = await adminAuthApi.login(account, password)
      adminStore.setSession(res.token, res.username)
    } else {
      const res = await portalAuthApi.login(account, password)
      portalStore.setSession(res.token, { id: res.user_id, email: res.email })
    }
    return true
  } catch (e: any) {
    const status = e?.status ?? e?.response?.status
    if (status === 400 || status === 401 || status === 403 || status === 404) {
      return false
    }
    throw e
  }
}

async function doLogin() {
  // Admin and portal share the same email format. Prefer the route hint
  // when present, then continue to the other role on auth-class failures.
  let succeededAs: Role | null = null
  try {
    for (const role of roleOrder.value) {
      if (await tryRole(role, account.value, password.value)) {
        succeededAs = role
        break
      }
    }
  } catch (e) {
    error.value = formatError(e, t('auth.loginFailed'))
    return
  }

  if (!succeededAs) {
    error.value = t('auth.wrongCredentials')
    return
  }

  // Honor ?next= only when the role matches the protected area.
  let target = succeededAs === 'admin' ? '/admin' : '/portal'
  if (nextPath.value) {
    const wantsAdmin = nextPath.value.startsWith('/admin')
    const wantsPortal = nextPath.value.startsWith('/portal')
    if ((wantsAdmin && succeededAs === 'admin') || (wantsPortal && succeededAs === 'portal')) {
      target = nextPath.value
    }
  }
  await router.push(target)
}

async function doRegister() {
  if (password.value !== passwordConfirm.value) {
    error.value = t('auth.passwordsMustMatch')
    return
  }
  if (password.value.length < 8) {
    error.value = t('auth.passwordTooShort')
    return
  }
  if (emailVerificationRequired.value && (!code.value || code.value.length !== 6)) {
    error.value = t('auth.codeMustBe6')
    return
  }
  try {
    const res = await portalAuthApi.register(
      account.value,
      password.value,
      emailVerificationRequired.value ? code.value : undefined,
    )
    portalStore.setSession(res.token, { id: res.user_id, email: res.email })
    await router.push('/portal')
  } catch (e) {
    error.value = formatError(e, t('auth.registerFailed'))
  }
}

async function onSubmit() {
  if (!account.value || !password.value) return
  loading.value = true
  error.value = null
  try {
    if (mode.value === 'login') {
      await doLogin()
    } else {
      await doRegister()
    }
  } finally {
    loading.value = false
  }
}

function switchMode(next: Mode) {
  if (mode.value === next) return
  mode.value = next
  error.value = null
  passwordConfirm.value = ''
  code.value = ''
}
</script>

<template>
  <AuthLayout
    :card-title="mode === 'login' ? $t('auth.welcomeBack') : $t('auth.createAccount')"
    :card-subtitle="mode === 'login' ? $t('auth.signInSubtitle') : $t('auth.registerSubtitle')"
  >
    <!-- Mode tabs: 登录 / 注册. Admin vs portal is auto-detected at submit. -->
    <div class="mb-5 flex items-center gap-0.5 rounded-xl border border-surface-200 bg-surface-100/60 p-1 text-sm dark:border-surface-700 dark:bg-surface-800/40">
      <button
        type="button"
        class="flex flex-1 items-center justify-center rounded-lg px-3 py-2 font-medium transition-all duration-150 ease-brand"
        :class="mode === 'login'
          ? 'bg-surface-0 text-ink-900 shadow-card dark:bg-surface-900 dark:text-surface-50'
          : 'text-surface-500 hover:text-ink-900 dark:hover:text-surface-50'"
        @click="switchMode('login')"
      >
        {{ $t('auth.loginTab') }}
      </button>
      <button
        type="button"
        class="flex flex-1 items-center justify-center rounded-lg px-3 py-2 font-medium transition-all duration-150 ease-brand"
        :class="mode === 'register'
          ? 'bg-surface-0 text-ink-900 shadow-card dark:bg-surface-900 dark:text-surface-50'
          : 'text-surface-500 hover:text-ink-900 dark:hover:text-surface-50'"
        @click="switchMode('register')"
      >
        {{ $t('auth.registerTab') }}
      </button>
    </div>

    <form class="space-y-4" @submit.prevent="onSubmit">
      <div>
        <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('auth.email') }}</label>
        <div class="relative">
          <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" />
          </svg>
          <input
            v-model="account"
            type="email"
            autocomplete="email"
            required
            placeholder="you@example.com"
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
            :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
            required
            placeholder="••••••••"
            class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
          />
        </div>
      </div>
      <div v-if="mode === 'register' && emailVerificationRequired">
        <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('auth.confirmPassword') }}</label>
        <div class="relative">
          <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          <input
            v-model="passwordConfirm"
            type="password"
            autocomplete="new-password"
            required
            :placeholder="$t('auth.confirmPasswordPlaceholder')"
            class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-sm transition-colors placeholder:text-surface-400 focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
          />
        </div>
      </div>
      <!-- Email verification code (register only).
           User clicks "发送验证码" → backend mails a 6-digit code → user types
           it in here. Send button is rate-limited 60s client-side to mirror
           server-side ErrRateLimited. -->
      <div v-if="mode === 'register'">
        <label class="mb-1.5 block text-xs font-medium text-surface-600 dark:text-surface-300">{{ $t('auth.verificationCode') }}</label>
        <div class="flex items-stretch gap-2">
          <div class="relative flex-1">
            <svg class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14M22 4 12 14.01l-3-3" />
            </svg>
            <input
              v-model="code"
              type="text"
              inputmode="numeric"
              pattern="\d{6}"
              maxlength="6"
              autocomplete="one-time-code"
              required
              :placeholder="$t('auth.codePlaceholder')"
              class="block w-full rounded-xl border border-surface-200 bg-surface-0 py-2.5 pl-10 pr-3.5 text-center text-sm font-medium tabular-nums tracking-[0.4em] transition-colors placeholder:text-surface-400 placeholder:tracking-normal focus:border-accent-500 focus:outline-none focus:ring-4 focus:ring-accent-500/15 dark:border-surface-700 dark:bg-surface-900"
            />
          </div>
          <button
            type="button"
            :disabled="codeSending || codeCooldown > 0"
            class="inline-flex h-auto min-w-[120px] items-center justify-center rounded-xl border border-surface-200 px-3 text-sm font-medium text-surface-700 transition-colors hover:border-accent-300 hover:bg-accent-50 hover:text-accent-700 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:border-surface-200 disabled:hover:bg-transparent disabled:hover:text-surface-700 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-accent-950/40 dark:hover:text-accent-300"
            @click="sendCode"
          >
            <span v-if="codeSending">{{ $t('auth.sending') }}</span>
            <span v-else-if="codeCooldown > 0">{{ $t('auth.codeRetry', { n: codeCooldown }) }}</span>
            <span v-else>{{ $t('auth.sendCode') }}</span>
          </button>
        </div>
        <p class="mt-1.5 text-2xs text-surface-400">{{ $t('auth.codeValidHint') }}</p>
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
        {{ loading
          ? (mode === 'login' ? $t('auth.loggingIn') : $t('auth.registering'))
          : (mode === 'login' ? $t('auth.submit') : $t('auth.createAccount')) }}
      </button>
      <p v-if="error" class="flex items-start gap-2 rounded-xl bg-red-50 px-3.5 py-2.5 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800">
        <svg class="mt-0.5 h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="M12 8v4M12 16h.01" /></svg>
        <span>{{ error }}</span>
      </p>
    </form>

    <!-- OIDC providers — visible on login tab only, when backend reports any.
         Empty list (no OIDC configured) keeps the whole block hidden. -->
    <div v-if="mode === 'login' && oidcProviders.length" class="mt-6">
      <div class="relative my-4 flex items-center">
        <span class="h-px flex-1 bg-surface-200 dark:bg-surface-700"></span>
        <span class="px-3 text-2xs text-surface-400">{{ $t('auth.orSignInWith') }}</span>
        <span class="h-px flex-1 bg-surface-200 dark:bg-surface-700"></span>
      </div>
      <div class="space-y-2">
        <button
          v-for="p in oidcProviders"
          :key="p.name"
          type="button"
          class="group inline-flex h-11 w-full items-center justify-center gap-2.5 rounded-xl border border-surface-200 bg-surface-0 px-4 text-sm font-medium text-ink-900 transition-all ease-brand hover:border-accent-300 hover:bg-accent-50 active:scale-[0.98] dark:border-surface-700 dark:bg-surface-900 dark:text-surface-50 dark:hover:bg-accent-950/40"
          @click="startOIDC(p)"
        >
          <img v-if="p.icon" :src="p.icon" :alt="p.name" class="h-5 w-5 rounded" />
          <svg v-else class="h-5 w-5 text-surface-500 group-hover:text-accent-600 dark:group-hover:text-accent-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10" />
            <path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20" />
          </svg>
          <span>{{ $t('auth.signInWith', { name: p.name }) }}</span>
        </button>
      </div>
    </div>
  </AuthLayout>
</template>
