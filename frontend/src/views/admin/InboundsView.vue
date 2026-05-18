<template>
  <div class="space-y-4 animate-fade-in">
    <!-- Toolbar -->
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div>
        <h2 class="text-lg font-semibold text-gray-900">Inbounds</h2>
        <p class="text-sm text-gray-500">{{ filteredInbounds.length }} total</p>
      </div>
      <div class="flex gap-2 w-full sm:w-auto">
        <input
          v-model="search"
          class="input flex-1 sm:w-48"
          placeholder="Search remark..."
        />
        <button class="btn-primary whitespace-nowrap" @click="openCreate">+ Add Inbound</button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="card flex justify-center py-12">
      <LoadingSpinner size="lg" label="Loading inbounds..." />
    </div>

    <!-- Table -->
    <div v-else class="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">ID</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Remark</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Protocol</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Port</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Status</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Traffic (↑/↓)</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Expiry</th>
              <th class="px-4 py-3 text-right font-semibold text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-if="filteredInbounds.length === 0">
              <td colspan="8" class="px-4 py-8 text-center text-gray-400">No inbounds found</td>
            </tr>
            <tr
              v-for="inbound in paginatedInbounds"
              :key="inbound.id"
              class="hover:bg-gray-50 transition-colors"
            >
              <td class="px-4 py-3 font-mono text-gray-500">{{ inbound.id }}</td>
              <td class="px-4 py-3 font-medium text-gray-900">{{ inbound.remark || '—' }}</td>
              <td class="px-4 py-3">
                <span class="badge badge-blue">{{ inbound.protocol }}</span>
              </td>
              <td class="px-4 py-3 font-mono text-gray-700">{{ inbound.port }}</td>
              <td class="px-4 py-3">
                <span :class="inbound.enable ? 'badge-green' : 'badge-red'" class="badge">
                  {{ inbound.enable ? 'Enabled' : 'Disabled' }}
                </span>
              </td>
              <td class="px-4 py-3 text-gray-600 whitespace-nowrap">
                {{ formatBytes(inbound.up) }} / {{ formatBytes(inbound.down) }}
              </td>
              <td class="px-4 py-3 text-gray-600">
                {{ inbound.expiryTime ? formatDate(inbound.expiryTime) : '∞' }}
              </td>
              <td class="px-4 py-3 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary btn-sm" @click="openEdit(inbound)">Edit</button>
                  <button class="btn-danger btn-sm" @click="confirmDelete(inbound)">Delete</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="px-4 py-3 border-t border-gray-100 flex items-center justify-between text-sm text-gray-600">
        <span>Page {{ currentPage }} of {{ totalPages }}</span>
        <div class="flex gap-2">
          <button class="btn-secondary btn-sm" :disabled="currentPage === 1" @click="currentPage--">Prev</button>
          <button class="btn-secondary btn-sm" :disabled="currentPage === totalPages" @click="currentPage++">Next</button>
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="showModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
          @click.self="showModal = false"
        >
          <div class="bg-white rounded-2xl shadow-xl w-full max-w-lg p-6 animate-slide-up max-h-[90vh] overflow-y-auto">
            <h3 class="text-lg font-semibold text-gray-900 mb-5">
              {{ editingInbound ? 'Edit Inbound' : 'Add Inbound' }}
            </h3>
            <form @submit.prevent="saveInbound" class="space-y-4">
              <div>
                <label class="label">Remark</label>
                <input v-model="form.remark" class="input" type="text" placeholder="My inbound" />
              </div>
              <div class="grid grid-cols-2 gap-4">
                <div>
                  <label class="label">Protocol</label>
                  <select v-model="form.protocol" class="input">
                    <option value="vmess">vmess</option>
                    <option value="vless">vless</option>
                    <option value="trojan">trojan</option>
                    <option value="shadowsocks">shadowsocks</option>
                    <option value="hysteria2">hysteria2</option>
                  </select>
                </div>
                <div>
                  <label class="label">Port</label>
                  <input v-model.number="form.port" class="input" type="number" min="1" max="65535" />
                </div>
              </div>
              <div>
                <label class="label">Listen Address</label>
                <input v-model="form.listen" class="input" type="text" placeholder="0.0.0.0" />
              </div>
              <div class="flex items-center gap-2">
                <input id="enabled" v-model="form.enable" type="checkbox" class="rounded border-gray-300" />
                <label for="enabled" class="text-sm text-gray-700">Enabled</label>
              </div>
              <div v-if="formError" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">{{ formError }}</div>
              <div class="flex justify-end gap-3 pt-2">
                <button type="button" class="btn-secondary" @click="showModal = false">Cancel</button>
                <button type="submit" class="btn-primary" :disabled="saving">
                  <LoadingSpinner v-if="saving" size="sm" />
                  {{ editingInbound ? 'Update' : 'Create' }}
                </button>
              </div>
            </form>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Confirm Delete -->
    <ConfirmDialog
      v-model="showDeleteDialog"
      title="Delete Inbound"
      :message="`Delete inbound '${deleteTarget?.remark || deleteTarget?.id}'? This cannot be undone.`"
      confirm-text="Delete"
      :danger-mode="true"
      :loading="deleting"
      @confirm="doDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { adminInboundsApi } from '@/api'
import type { Inbound } from '@/types'
import { useAppStore } from '@/stores/app'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const appStore = useAppStore()
const inbounds = ref<Inbound[]>([])
const loading = ref(true)
const search = ref('')
const currentPage = ref(1)
const pageSize = 15

const showModal = ref(false)
const editingInbound = ref<Inbound | null>(null)
const form = ref({ remark: '', protocol: 'vless', port: 443, listen: '', enable: true })
const formError = ref('')
const saving = ref(false)

const showDeleteDialog = ref(false)
const deleteTarget = ref<Inbound | null>(null)
const deleting = ref(false)

const filteredInbounds = computed(() =>
  search.value
    ? inbounds.value.filter(
        (i) =>
          i.remark?.toLowerCase().includes(search.value.toLowerCase()) ||
          String(i.port).includes(search.value)
      )
    : inbounds.value
)

const totalPages = computed(() => Math.ceil(filteredInbounds.value.length / pageSize))

const paginatedInbounds = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredInbounds.value.slice(start, start + pageSize)
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(ms: number): string {
  if (!ms) return '∞'
  return new Date(ms).toLocaleDateString()
}

function openCreate() {
  editingInbound.value = null
  form.value = { remark: '', protocol: 'vless', port: 443, listen: '', enable: true }
  formError.value = ''
  showModal.value = true
}

function openEdit(inbound: Inbound) {
  editingInbound.value = inbound
  form.value = {
    remark: inbound.remark,
    protocol: inbound.protocol,
    port: inbound.port,
    listen: inbound.listen ?? '',
    enable: inbound.enable
  }
  formError.value = ''
  showModal.value = true
}

async function saveInbound() {
  saving.value = true
  formError.value = ''
  try {
    if (editingInbound.value) {
      await adminInboundsApi.update(editingInbound.value.id, form.value)
      appStore.success('Inbound updated successfully')
    } else {
      await adminInboundsApi.create(form.value)
      appStore.success('Inbound created successfully')
    }
    showModal.value = false
    await loadInbounds()
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    formError.value = err.response?.data?.error ?? 'Operation failed'
  } finally {
    saving.value = false
  }
}

function confirmDelete(inbound: Inbound) {
  deleteTarget.value = inbound
  showDeleteDialog.value = true
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await adminInboundsApi.delete(deleteTarget.value.id)
    appStore.success('Inbound deleted')
    showDeleteDialog.value = false
    await loadInbounds()
  } catch {
    appStore.error('Failed to delete inbound')
  } finally {
    deleting.value = false
  }
}

async function loadInbounds() {
  try {
    inbounds.value = await adminInboundsApi.list()
  } catch {
    appStore.error('Failed to load inbounds')
  }
}

onMounted(async () => {
  await loadInbounds()
  loading.value = false
})
</script>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
