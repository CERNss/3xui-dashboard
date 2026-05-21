<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { formatError } from '@/utils/format'

import { settingsApi, type SettingItem } from '@/api/admin/settings'

const items = ref<SettingItem[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const savingKey = ref<string | null>(null)
const flash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

// Mutable copy of values keyed by setting key — keeps the form
// editable without re-fetching after each Save.
const drafts = ref<Record<string, string>>({})

async function load() {
  loading.value = true
  error.value = null
  try {
    items.value = await settingsApi.list()
    items.value.forEach((it) => {
      drafts.value[it.key] = it.value
    })
  } catch (e: any) {
    error.value = formatError(e, '加载设置失败')
  } finally {
    loading.value = false
  }
}

async function save(it: SettingItem) {
  savingKey.value = it.key
  flash.value = null
  try {
    const draft = drafts.value[it.key] ?? ''
    await settingsApi.set(it.key, draft)
    flash.value = { kind: 'ok', text: `${it.label} saved` }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, '保存失败') }
  } finally {
    savingKey.value = null
  }
}

async function clearOverride(it: SettingItem) {
  if (!it.has_override) return
  savingKey.value = it.key
  flash.value = null
  try {
    await settingsApi.clear(it.key)
    flash.value = { kind: 'ok', text: `${it.label} reverted to default` }
    await load()
  } catch (e: any) {
    flash.value = { kind: 'err', text: formatError(e, '重置默认值失败') }
  } finally {
    savingKey.value = null
  }
}

const grouped = computed(() => {
  const buckets: Record<string, SettingItem[]> = {}
  for (const it of items.value) {
    const g = it.group || 'other'
    buckets[g] = buckets[g] || []
    buckets[g].push(it)
  }
  return buckets
})

function groupLabel(g: string): string {
  return ({
    registration: 'Registration',
    subscription: 'Subscription',
    traffic: 'Traffic thresholds',
    other: 'Other',
  } as Record<string, string>)[g] ?? g
}

// SMTP test bench state — ops affordance, not part of the settings
// repo (no DB row, just a one-shot send).
const smtpTo = ref('')
const smtpBusy = ref(false)
const smtpFlash = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

async function sendSMTPTest() {
  smtpBusy.value = true
  smtpFlash.value = null
  try {
    await settingsApi.smtpTest(smtpTo.value)
    smtpFlash.value = { kind: 'ok', text: '已发送到 ' + smtpTo.value }
  } catch (e: any) {
    smtpFlash.value = { kind: 'err', text: formatError(e, '发送失败') }
  } finally {
    smtpBusy.value = false
  }
}

// Tab state — three surfaces for the messages/notifications split:
// general runtime settings, user-facing messages (SMTP), and
// ops-facing notifications (multi-channel + admin webhooks).
type Tab = 'general' | 'messages' | 'notifications'
const tab = ref<Tab>('general')

onMounted(load)
</script>

<template>
  <div>
    <header class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-xl font-semibold">Settings</h1>
        <p class="mt-1 text-sm text-surface-500">
          Runtime-mutable overrides. Empty value = use env default / hardcoded default.
        </p>
      </div>
      <button
        class="rounded-md border border-surface-300 px-3 py-1.5 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
        @click="load"
      >
        Refresh
      </button>
    </header>

    <p v-if="error" class="mb-4 rounded bg-red-50 px-3 py-2 text-sm text-red-700">{{ error }}</p>
    <p
      v-if="flash"
      :class="[
        'mb-4 rounded px-3 py-2 text-sm',
        flash.kind === 'ok' ? 'bg-accent-50 text-accent-800' : 'bg-red-50 text-red-700',
      ]"
    >{{ flash.text }}</p>

    <div v-if="loading" class="text-sm text-surface-500">Loading…</div>

    <div v-else class="space-y-6">
      <!-- Surface tabs. The messages/notifications split is a
           conceptual one: 消息 = user-facing SMTP-only (codes,
           low-balance, password reset), 通知 = ops-facing multi-
           channel (email/discord/feishu/telegram + admin-
           configured webhooks). Runtime overrides live under
           通用 since they aren't surface-specific. -->
      <nav
        class="flex items-center gap-1 border-b border-surface-200 dark:border-surface-800"
        role="tablist"
        aria-label="Settings surfaces"
      >
        <button
          v-for="t in (['general', 'messages', 'notifications'] as Tab[])"
          :key="t"
          type="button"
          role="tab"
          :aria-selected="tab === t"
          :class="[
            'border-b-2 px-4 py-2 text-sm font-medium transition-colors',
            tab === t
              ? 'border-primary-600 text-primary-600 dark:text-primary-300'
              : 'border-transparent text-surface-500 hover:text-surface-700 dark:hover:text-surface-200',
          ]"
          @click="tab = t"
        >
          {{ t === 'general' ? '通用' : t === 'messages' ? '消息' : '通知' }}
        </button>
      </nav>

      <!-- 通用 tab — runtime overrides grouped by category. -->
      <section
        v-for="(rows, group) in grouped"
        v-show="tab === 'general'"
        :key="group"
        class="rounded-lg border border-surface-200 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900"
      >
        <header class="border-b border-surface-200 px-4 py-3 text-sm font-semibold dark:border-surface-800">
          {{ groupLabel(group as string) }}
        </header>
        <div class="divide-y divide-surface-200 dark:divide-surface-800">
          <div v-for="it in rows" :key="it.key" class="px-4 py-4">
            <div class="flex items-start gap-4">
              <div class="flex-1">
                <label class="block text-sm font-medium" :for="'setting-' + it.key">{{ it.label }}</label>
                <p class="mt-1 text-xs text-surface-500">{{ it.description }}</p>
                <p class="mt-1 font-mono text-xs text-surface-400">
                  key=<code>{{ it.key }}</code> · type=<code>{{ it.type }}</code>
                  <span v-if="!it.has_override && it.env_fallback">
                    · env fallback=<code>{{ it.env_fallback }}</code>
                  </span>
                  <span v-if="!it.has_override && it.default">
                    · default=<code>{{ it.default }}</code>
                  </span>
                  <span v-if="!it.has_override" class="ml-1 rounded bg-surface-100 px-1 text-surface-500 dark:bg-surface-800">no override</span>
                  <span v-else class="ml-1 rounded bg-primary-50 px-1 text-primary-700 dark:bg-surface-800 dark:text-primary-300">override active</span>
                </p>
              </div>
              <div class="flex flex-col items-end gap-2">
                <select
                  v-if="it.type === 'bool'"
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  class="w-40 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                >
                  <option value="">(empty — use default)</option>
                  <option value="true">true</option>
                  <option value="false">false</option>
                </select>
                <input
                  v-else-if="it.type === 'int'"
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  type="number"
                  class="w-40 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                />
                <input
                  v-else
                  :id="'setting-' + it.key"
                  v-model="drafts[it.key]"
                  type="text"
                  class="w-60 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
                />
                <div class="flex gap-2">
                  <button
                    class="rounded bg-primary-600 px-3 py-1 text-xs font-medium text-white hover:bg-primary-700 disabled:opacity-60"
                    :disabled="savingKey === it.key"
                    @click="save(it)"
                  >
                    {{ savingKey === it.key ? 'Saving…' : 'Save' }}
                  </button>
                  <button
                    v-if="it.has_override"
                    class="rounded border border-surface-300 px-3 py-1 text-xs hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
                    :disabled="savingKey === it.key"
                    @click="clearOverride(it)"
                  >
                    Clear
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 消息 tab — user-facing SMTP. The only channel is mail to
           the user's verified email. -->
      <section
        v-show="tab === 'messages'"
        class="rounded-lg border border-surface-200 bg-surface-0 p-4 dark:border-surface-700 dark:bg-surface-900"
      >
        <h3 class="text-base font-semibold text-ink-900 dark:text-surface-50">用户消息（SMTP）</h3>
        <p class="mt-2 text-sm text-surface-600 dark:text-surface-300">
          用户向的事务性邮件：注册验证码、密码重置、余额不足提醒等。仅 SMTP 单通道，收件人为用户绑定邮箱。
          其他渠道（飞书 / Discord / Telegram / admin 配置的 webhook）只用于运营通知，不会发到用户。
        </p>
        <p class="mt-2 text-xs text-surface-500">
          SMTP 主机、账号密码、TLS 模式等通过环境变量 <code class="font-mono">SMTP_*</code> 配置；下方为发送测试。
        </p>
        <div class="mt-4 border-t border-surface-200 pt-4 dark:border-surface-700">
          <h4 class="text-sm font-semibold text-ink-900 dark:text-surface-50">SMTP 测试</h4>
          <p class="mt-1 text-xs text-surface-500">填一个收件邮箱，发一封测试邮件。SMTP 未配置时返回 503。</p>
          <div class="mt-3 flex flex-wrap items-center gap-2">
            <input
              v-model="smtpTo"
              type="email"
              placeholder="admin@example.com"
              class="w-72 rounded-md border border-surface-300 bg-surface-0 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-200 dark:border-surface-700 dark:bg-surface-900"
            />
            <button
              class="rounded bg-primary-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-primary-700 disabled:opacity-60"
              :disabled="smtpBusy || !smtpTo"
              @click="sendSMTPTest"
            >
              {{ smtpBusy ? '发送中…' : '发送测试' }}
            </button>
            <span v-if="smtpFlash" class="text-xs" :class="smtpFlash.kind === 'ok' ? 'text-accent-600' : 'text-red-600'">{{ smtpFlash.text }}</span>
          </div>
        </div>
      </section>

      <!-- 通知 tab — ops-facing fanout. 4 env-configured channels
           (email/discord/feishu/telegram) plus admin-configured
           webhooks under /admin/webhooks. -->
      <section
        v-show="tab === 'notifications'"
        class="rounded-lg border border-surface-200 bg-surface-0 p-4 dark:border-surface-700 dark:bg-surface-900"
      >
        <h3 class="text-base font-semibold text-ink-900 dark:text-surface-50">运营通知</h3>
        <p class="mt-2 text-sm text-surface-600 dark:text-surface-300">
          面向运维 / 管理员的事件通知。包含两类后端：
        </p>
        <ul class="mt-3 space-y-2 text-sm text-surface-600 dark:text-surface-300">
          <li class="flex gap-2">
            <span class="font-mono text-xs text-surface-400">[env]</span>
            <div>
              <strong>内置通道</strong>：email（ops 收件人）、Discord、飞书、Telegram，通过环境变量配置。
              事件路由由 <code class="font-mono">NOTIFY_ROUTES</code> 决定（如 <code class="font-mono">node.offline:feishu,telegram</code>）。
            </div>
          </li>
          <li class="flex gap-2">
            <span class="font-mono text-xs text-surface-400">[ui]</span>
            <div>
              <strong>Admin 配置的 Webhook</strong>：自定义 URL、模板、HMAC 签名、重试。
              <RouterLink to="/admin/webhooks" class="text-primary-600 hover:underline dark:text-primary-300">前往 Webhooks →</RouterLink>
            </div>
          </li>
        </ul>
        <p class="mt-4 text-xs text-surface-500">
          dedup 与「消息」surface 独立 —— 同名事件（例如「low_balance」）发给用户的邮件不会阻塞发给 ops 的告警。
        </p>
      </section>
    </div>
  </div>
</template>
