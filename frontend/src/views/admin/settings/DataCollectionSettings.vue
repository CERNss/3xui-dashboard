<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { SettingItem } from '@/api/admin/settings'

const props = defineProps<{
  items: SettingItem[]
  drafts: Record<string, string>
  savingKey: string | null
}>()

const emit = defineEmits<{
  save: [item: SettingItem]
  clear: [item: SettingItem]
}>()

const { t, locale } = useI18n()

const helpPaths: Record<string, string> = {
  ops_collect_enabled: 'admin.settings.settingHelp.opsCollectEnabled',
  ops_collect_interval_seconds: 'admin.settings.settingHelp.opsCollectInterval',
  ops_collect_concurrency: 'admin.settings.settingHelp.opsCollectConcurrency',
  ops_collect_timeout_seconds: 'admin.settings.settingHelp.opsCollectTimeout',
  ops_collect_retry_attempts: 'admin.settings.settingHelp.opsCollectRetry',
  ops_retention_seconds: 'admin.settings.settingHelp.opsRetention',
  traffic_collect_enabled: 'admin.settings.settingHelp.trafficCollectEnabled',
  traffic_collect_interval_seconds: 'admin.settings.settingHelp.trafficCollectInterval',
  traffic_collect_concurrency: 'admin.settings.settingHelp.trafficCollectConcurrency',
  traffic_collect_timeout_seconds: 'admin.settings.settingHelp.trafficCollectTimeout',
  traffic_collect_retry_attempts: 'admin.settings.settingHelp.trafficCollectRetry',
  traffic_retention_seconds: 'admin.settings.settingHelp.trafficRetention',
}

function localizedLabel(it: SettingItem): string {
  return locale.value === 'zh' && it.label_zh ? it.label_zh : it.label
}

function settingHelp(it: SettingItem): string {
  const path = helpPaths[it.key]
  return path ? t(path) : ''
}

function inputMin(key: string): string {
  if (key.endsWith('_interval_seconds')) return '5'
  if (key.endsWith('_concurrency') || key.endsWith('_timeout_seconds')) return '1'
  return '0'
}

function inputMax(key: string): string | undefined {
  if (key.endsWith('_concurrency')) return '64'
  if (key.endsWith('_timeout_seconds')) {
    return String(Math.min(300, Number(props.drafts[intervalKeyForTimeout(key)] || 300)))
  }
  if (key.endsWith('_retry_attempts')) return '5'
  return undefined
}

function intervalKeyForTimeout(key: string): string {
  if (key.startsWith('ops_collect_')) return 'ops_collect_interval_seconds'
  if (key.startsWith('traffic_collect_')) return 'traffic_collect_interval_seconds'
  return key
}
</script>

<template>
  <section class="overflow-hidden rounded-xl border border-surface-100 bg-surface-0 shadow-card dark:border-surface-800 dark:bg-surface-900">
    <header class="flex flex-col gap-2 px-4 py-4 lg:px-5">
      <div>
        <h2 class="text-lg font-semibold tracking-tight text-ink-900 dark:text-surface-50">
          {{ $t('admin.settings.dataCollectionTitle') }}
        </h2>
        <p class="mt-1 text-sm text-surface-500 dark:text-surface-400">
          {{ $t('admin.settings.dataCollectionDesc') }}
        </p>
      </div>
    </header>

    <div class="border-t border-surface-100 dark:border-surface-800">
      <div class="px-4 py-3 text-xs font-semibold uppercase tracking-caps text-surface-500 dark:text-surface-400 lg:px-5">
        {{ $t('admin.settings.groupDataCollection') }}
      </div>
      <div
        v-for="it in props.items"
        :key="it.key"
        :data-setting-key="it.key"
        class="grid gap-3 border-t border-surface-100 px-4 py-4 dark:border-surface-800 lg:grid-cols-[minmax(220px,0.36fr),minmax(0,1fr)] lg:items-start lg:px-5"
      >
        <div class="max-w-2xl">
          <label class="text-sm font-semibold text-ink-900 dark:text-surface-50" :for="'setting-' + it.key">
            {{ localizedLabel(it) }}
          </label>
          <p v-if="settingHelp(it)" class="mt-1 text-sm text-surface-500 dark:text-surface-400">
            {{ settingHelp(it) }}
          </p>
        </div>
        <div class="flex min-w-0 flex-col gap-2 lg:items-end">
          <select
            v-if="it.type === 'bool'"
            :id="'setting-' + it.key"
            v-model="props.drafts[it.key]"
            class="h-10 w-full rounded-lg border border-surface-200 bg-surface-50 px-3 text-sm text-ink-900 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-50 lg:w-44"
          >
            <option value="">{{ $t('admin.settings.useDefaultEmpty') }}</option>
            <option value="true">{{ $t('admin.settings.values.true') }}</option>
            <option value="false">{{ $t('admin.settings.values.false') }}</option>
          </select>
          <input
            v-else-if="it.type === 'int'"
            :id="'setting-' + it.key"
            v-model="props.drafts[it.key]"
            type="number"
            :min="inputMin(it.key)"
            :max="inputMax(it.key)"
            class="h-10 w-full rounded-lg border border-surface-200 bg-surface-50 px-3 text-sm text-ink-900 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-50 lg:w-44"
          />
          <input
            v-else
            :id="'setting-' + it.key"
            v-model="props.drafts[it.key]"
            type="text"
            class="h-10 w-full rounded-lg border border-surface-200 bg-surface-50 px-3 text-sm text-ink-900 transition-colors focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-500/30 dark:border-surface-700 dark:bg-surface-800/70 dark:text-surface-50 lg:w-80"
          />
          <div class="flex flex-wrap justify-end gap-2">
            <button
              class="inline-flex h-8 items-center rounded-lg bg-ink-900 px-3 text-xs font-semibold text-white transition-colors hover:bg-ink-800 disabled:opacity-60 dark:bg-accent-600 dark:hover:bg-accent-500"
              type="button"
              :disabled="props.savingKey === it.key"
              @click="emit('save', it)"
            >
              {{ props.savingKey === it.key ? $t('admin.settings.saving') : $t('admin.settings.save') }}
            </button>
            <button
              v-if="it.has_override"
              class="inline-flex h-8 items-center rounded-lg border border-surface-200 px-3 text-xs font-semibold text-surface-700 transition-colors hover:bg-surface-50 disabled:opacity-60 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
              type="button"
              :disabled="props.savingKey === it.key"
              @click="emit('clear', it)"
            >
              {{ $t('admin.settings.reset') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
