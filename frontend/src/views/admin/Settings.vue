<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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
      <section
        v-for="(rows, group) in grouped"
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
    </div>
  </div>
</template>
