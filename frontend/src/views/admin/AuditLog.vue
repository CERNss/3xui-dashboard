<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAuditApi, type AdminAction, type ListAuditParams } from '@/api/admin/audit'
import EmptyState from '@/components/common/EmptyState.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import { formatError } from '@/utils/format'

const { t } = useI18n()

const rows = ref<AdminAction[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

const filters = ref<ListAuditParams>({ limit: 100 })

async function reload() {
  loading.value = true
  error.value = null
  try {
    const { actions } = await adminAuditApi.list(filters.value)
    rows.value = actions
  } catch (e: any) {
    error.value = formatError(e, t('admin.auditLog.loadFailed'))
  } finally {
    loading.value = false
  }
}

// Debounced refresh on filter typing so we don't fire a request
// per keystroke.
let debounceTimer: ReturnType<typeof setTimeout> | null = null
watch(
  () => [filters.value.username, filters.value.resource, filters.value.method],
  () => {
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(reload, 300)
  },
)

function statusChip(code: number): string {
  if (code >= 200 && code < 300)
    return 'bg-accent-50 text-accent-700 ring-accent-100 dark:bg-accent-950/40 dark:text-accent-300 dark:ring-accent-800'
  if (code >= 400 && code < 500)
    return 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800'
  return 'bg-red-50 text-red-700 ring-red-100 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800'
}

function methodChip(m: string): string {
  return ({
    GET: 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-950/40 dark:text-sky-300 dark:ring-sky-800',
    POST: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:ring-emerald-800',
    PUT: 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:ring-amber-800',
    DELETE: 'bg-red-50 text-red-700 ring-red-200 dark:bg-red-950/40 dark:text-red-300 dark:ring-red-800',
    PATCH: 'bg-violet-50 text-violet-700 ring-violet-200 dark:bg-violet-950/40 dark:text-violet-300 dark:ring-violet-800',
  } as Record<string, string>)[m] ?? ''
}

const hasFilters = computed(() => !!(filters.value.username || filters.value.resource || filters.value.method))

onMounted(reload)
</script>

<template>
  <div>
    <header class="mb-7 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ $t('admin.auditLog.title') }}</h1>
        <p class="mt-1.5 text-sm text-surface-500">{{ $t('admin.auditLog.subtitle') }}</p>
      </div>
    </header>

    <!-- Filters -->
    <div class="mb-4 grid grid-cols-1 gap-2 sm:grid-cols-4">
      <input
        v-model="filters.username"
        type="text"
        :placeholder="$t('admin.auditLog.filterUsername')"
        class="rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900"
      />
      <input
        v-model="filters.resource"
        type="text"
        :placeholder="$t('admin.auditLog.filterResource')"
        class="rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900"
      />
      <select
        v-model="filters.method"
        class="rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900"
      >
        <option value="">{{ $t('admin.auditLog.anyMethod') }}</option>
        <option value="POST">POST</option>
        <option value="PUT">PUT</option>
        <option value="DELETE">DELETE</option>
        <option value="PATCH">PATCH</option>
      </select>
      <button
        type="button"
        class="rounded-lg border border-surface-200 px-3 py-2 text-sm font-medium text-surface-700 transition-colors hover:bg-surface-50 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
        @click="reload"
      >
        {{ $t('admin.users.reload') }}
      </button>
    </div>

    <p v-if="error" class="mb-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 ring-1 ring-inset ring-red-100 dark:bg-red-950/40 dark:text-red-300">{{ error }}</p>

    <Skeleton v-if="loading" :rows="6" />

    <div v-else-if="rows.length > 0" class="overflow-x-auto rounded-2xl border border-surface-100 bg-surface-0 dark:border-surface-800 dark:bg-surface-900">
      <table class="min-w-full text-sm">
        <thead class="text-left text-2xs font-medium uppercase tracking-wider text-surface-400 dark:text-surface-500">
          <tr class="border-b border-surface-100 dark:border-surface-800">
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.time') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.admin') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.method') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.path') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.target') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.status') }}</th>
            <th class="px-6 py-3 font-medium">{{ $t('admin.auditLog.column.ip') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-surface-100 dark:divide-surface-800">
          <tr v-for="r in rows" :key="r.id" class="transition-colors hover:bg-surface-50/60 dark:hover:bg-surface-800/40">
            <td class="px-6 py-3.5 font-mono text-xs text-surface-500 whitespace-nowrap">{{ new Date(r.created_at).toLocaleString() }}</td>
            <td class="px-6 py-3.5 font-medium text-ink-900 dark:text-surface-50">{{ r.admin_username || $t('admin.auditLog.unknownAdmin') }}</td>
            <td class="px-6 py-3.5">
              <span class="inline-flex items-center rounded-md px-2 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="methodChip(r.method)">{{ r.method }}</span>
            </td>
            <td class="px-6 py-3.5 font-mono text-xs text-surface-600 dark:text-surface-300 break-all">{{ r.path }}<span v-if="r.query_string" class="text-surface-400">?{{ r.query_string }}</span></td>
            <td class="px-6 py-3.5 text-xs">
              <span v-if="r.target_resource" class="font-mono">{{ r.target_resource }}{{ r.target_id ? ' #' + r.target_id : '' }}</span>
              <span v-else class="text-surface-400">—</span>
            </td>
            <td class="px-6 py-3.5">
              <span class="inline-flex items-center rounded-full px-2 py-0.5 text-2xs font-medium ring-1 ring-inset" :class="statusChip(r.status_code)">{{ r.status_code }}</span>
              <div v-if="r.error_msg" class="mt-1 max-w-xs truncate text-2xs text-red-600">{{ r.error_msg }}</div>
            </td>
            <td class="px-6 py-3.5 font-mono text-2xs text-surface-500">{{ r.ip || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <EmptyState
      v-else
      icon="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4"
      :title="$t('admin.auditLog.emptyTitle')"
      :description="hasFilters ? $t('admin.auditLog.emptyFiltered') : $t('admin.auditLog.emptyTotal')"
    />
  </div>
</template>
