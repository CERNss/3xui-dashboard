<template>
  <div class="space-y-4 animate-fade-in">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">Dashboard Users</h2>
      <p class="text-sm text-gray-500">Manage accounts that can log into this dashboard</p>
    </div>

    <div v-if="loading" class="card flex justify-center py-12">
      <LoadingSpinner size="lg" label="Loading users..." />
    </div>

    <div v-else class="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">ID</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Username</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Email</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Role</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">XUI Email</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Balance</th>
              <th class="px-4 py-3 text-left font-semibold text-gray-600">Created</th>
              <th class="px-4 py-3 text-right font-semibold text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100">
            <tr v-if="users.length === 0">
              <td colspan="8" class="px-4 py-8 text-center text-gray-400">No users found</td>
            </tr>
            <tr
              v-for="user in users"
              :key="user.id"
              class="hover:bg-gray-50 transition-colors"
            >
              <td class="px-4 py-3 text-gray-500 font-mono">{{ user.id }}</td>
              <td class="px-4 py-3 font-medium text-gray-900">{{ user.username }}</td>
              <td class="px-4 py-3 text-gray-600">{{ user.email }}</td>
              <td class="px-4 py-3">
                <span :class="user.role === 'admin' ? 'badge-blue' : 'badge-gray'" class="badge">
                  {{ user.role }}
                </span>
              </td>
              <td class="px-4 py-3 text-gray-600">{{ user.xuiClientEmail || '—' }}</td>
              <td class="px-4 py-3 text-gray-600">${{ user.balance.toFixed(2) }}</td>
              <td class="px-4 py-3 text-gray-500">{{ formatDate(user.createdAt) }}</td>
              <td class="px-4 py-3 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary btn-sm" @click="openEdit(user)">Edit</button>
                  <button class="btn-danger btn-sm" @click="confirmDelete(user)">Delete</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Edit Modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="showModal"
          class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
          @click.self="showModal = false"
        >
          <div class="bg-white rounded-2xl shadow-xl w-full max-w-md p-6 animate-slide-up">
            <h3 class="text-lg font-semibold text-gray-900 mb-5">Edit User</h3>
            <form @submit.prevent="saveUser" class="space-y-4">
              <div>
                <label class="label">Username</label>
                <input v-model="form.username" class="input" type="text" required />
              </div>
              <div>
                <label class="label">Email</label>
                <input v-model="form.email" class="input" type="email" required />
              </div>
              <div>
                <label class="label">Role</label>
                <select v-model="form.role" class="input">
                  <option value="user">user</option>
                  <option value="admin">admin</option>
                </select>
              </div>
              <div>
                <label class="label">XUI Client Email</label>
                <input v-model="form.xuiClientEmail" class="input" type="text" placeholder="client@xui" />
              </div>
              <div>
                <label class="label">XUI Sub ID</label>
                <input v-model="form.xuiSubId" class="input" type="text" placeholder="subscription-uuid" />
              </div>
              <div v-if="formError" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">{{ formError }}</div>
              <div class="flex justify-end gap-3 pt-2">
                <button type="button" class="btn-secondary" @click="showModal = false">Cancel</button>
                <button type="submit" class="btn-primary" :disabled="saving">
                  <LoadingSpinner v-if="saving" size="sm" />
                  Save Changes
                </button>
              </div>
            </form>
          </div>
        </div>
      </Transition>
    </Teleport>

    <ConfirmDialog
      v-model="showDeleteDialog"
      title="Delete User"
      :message="`Permanently delete user '${deleteTarget?.username}'?`"
      confirm-text="Delete"
      :danger-mode="true"
      :loading="deleting"
      @confirm="doDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { adminUsersApi, type UpdateUserPayload } from '@/api'
import type { User } from '@/types'
import { useAppStore } from '@/stores/app'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const appStore = useAppStore()
const users = ref<User[]>([])
const loading = ref(true)

const showModal = ref(false)
const editingUser = ref<User | null>(null)
const form = ref<UpdateUserPayload>({ username: '', email: '', role: 'user', xuiClientEmail: '', xuiSubId: '' })
const formError = ref('')
const saving = ref(false)

const showDeleteDialog = ref(false)
const deleteTarget = ref<User | null>(null)
const deleting = ref(false)

function formatDate(d: string): string {
  return new Date(d).toLocaleDateString()
}

function openEdit(user: User) {
  editingUser.value = user
  form.value = {
    username: user.username,
    email: user.email,
    role: user.role,
    xuiClientEmail: user.xuiClientEmail ?? '',
    xuiSubId: user.xuiSubId ?? ''
  }
  formError.value = ''
  showModal.value = true
}

async function saveUser() {
  if (!editingUser.value) return
  saving.value = true
  formError.value = ''
  try {
    const updated = await adminUsersApi.update(editingUser.value.id, form.value)
    const idx = users.value.findIndex((u) => u.id === editingUser.value!.id)
    if (idx !== -1) users.value[idx] = updated
    appStore.success('User updated')
    showModal.value = false
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    formError.value = err.response?.data?.error ?? 'Update failed'
  } finally {
    saving.value = false
  }
}

function confirmDelete(user: User) {
  deleteTarget.value = user
  showDeleteDialog.value = true
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await adminUsersApi.delete(deleteTarget.value.id)
    users.value = users.value.filter((u) => u.id !== deleteTarget.value!.id)
    appStore.success('User deleted')
    showDeleteDialog.value = false
  } catch {
    appStore.error('Failed to delete user')
  } finally {
    deleting.value = false
  }
}

onMounted(async () => {
  try {
    users.value = await adminUsersApi.list()
  } catch {
    appStore.error('Failed to load users')
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.modal-enter-active, .modal-leave-active { transition: opacity 0.2s ease; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
</style>
