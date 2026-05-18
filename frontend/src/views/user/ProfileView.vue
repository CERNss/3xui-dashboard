<template>
  <div class="space-y-6 animate-fade-in max-w-xl">
    <div>
      <h2 class="text-lg font-semibold text-gray-900">Profile</h2>
      <p class="text-sm text-gray-500">Manage your account information</p>
    </div>

    <!-- Account Info -->
    <div class="card">
      <h3 class="font-semibold text-gray-700 mb-4">Account Information</h3>
      <dl class="space-y-3">
        <div class="flex justify-between text-sm">
          <dt class="text-gray-500">Username</dt>
          <dd class="font-medium text-gray-900">{{ auth.user?.username }}</dd>
        </div>
        <div class="flex justify-between text-sm">
          <dt class="text-gray-500">Role</dt>
          <dd>
            <span :class="auth.user?.role === 'admin' ? 'badge-blue' : 'badge-gray'" class="badge">
              {{ auth.user?.role }}
            </span>
          </dd>
        </div>
        <div class="flex justify-between text-sm">
          <dt class="text-gray-500">XUI Email</dt>
          <dd class="font-medium text-gray-900">{{ auth.user?.xuiClientEmail || '—' }}</dd>
        </div>
        <div class="flex justify-between text-sm">
          <dt class="text-gray-500">Balance</dt>
          <dd class="font-medium text-gray-900">${{ auth.user?.balance?.toFixed(2) ?? '0.00' }}</dd>
        </div>
      </dl>
    </div>

    <!-- Update Email -->
    <div class="card">
      <h3 class="font-semibold text-gray-700 mb-4">Update Email</h3>
      <form @submit.prevent="updateProfile" class="space-y-4">
        <div>
          <label class="label">Email</label>
          <input v-model="profileForm.email" class="input" type="email" required />
        </div>
        <div v-if="profileSuccess" class="text-sm text-green-600 bg-green-50 rounded-lg px-3 py-2">
          {{ profileSuccess }}
        </div>
        <div v-if="profileError" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
          {{ profileError }}
        </div>
        <button type="submit" class="btn-primary" :disabled="profileSaving">
          <LoadingSpinner v-if="profileSaving" size="sm" />
          Update Email
        </button>
      </form>
    </div>

    <!-- Change Password -->
    <div class="card">
      <h3 class="font-semibold text-gray-700 mb-4">Change Password</h3>
      <form @submit.prevent="changePassword" class="space-y-4">
        <div>
          <label class="label">Current Password</label>
          <input v-model="pwForm.oldPassword" class="input" type="password" autocomplete="current-password" required />
        </div>
        <div>
          <label class="label">New Password</label>
          <input v-model="pwForm.newPassword" class="input" type="password" autocomplete="new-password" minlength="6" required />
        </div>
        <div>
          <label class="label">Confirm New Password</label>
          <input v-model="pwForm.confirmPassword" class="input" type="password" autocomplete="new-password" required />
        </div>
        <div v-if="pwSuccess" class="text-sm text-green-600 bg-green-50 rounded-lg px-3 py-2">
          {{ pwSuccess }}
        </div>
        <div v-if="pwError" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
          {{ pwError }}
        </div>
        <button type="submit" class="btn-primary" :disabled="pwSaving">
          <LoadingSpinner v-if="pwSaving" size="sm" />
          Change Password
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { userProfileApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const auth = useAuthStore()

// Profile form
const profileForm = ref({ email: auth.user?.email ?? '' })
const profileSaving = ref(false)
const profileSuccess = ref('')
const profileError = ref('')

// Password form
const pwForm = ref({ oldPassword: '', newPassword: '', confirmPassword: '' })
const pwSaving = ref(false)
const pwSuccess = ref('')
const pwError = ref('')

async function updateProfile() {
  profileSaving.value = true
  profileSuccess.value = ''
  profileError.value = ''
  try {
    const updated = await userProfileApi.update({ email: profileForm.value.email })
    auth.updateUser(updated)
    profileSuccess.value = 'Email updated successfully'
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    profileError.value = err.response?.data?.error ?? 'Update failed'
  } finally {
    profileSaving.value = false
  }
}

async function changePassword() {
  pwError.value = ''
  pwSuccess.value = ''
  if (pwForm.value.newPassword !== pwForm.value.confirmPassword) {
    pwError.value = 'New passwords do not match'
    return
  }
  pwSaving.value = true
  try {
    await userProfileApi.changePassword({
      oldPassword: pwForm.value.oldPassword,
      newPassword: pwForm.value.newPassword
    })
    pwSuccess.value = 'Password changed successfully'
    pwForm.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    pwError.value = err.response?.data?.error ?? 'Failed to change password'
  } finally {
    pwSaving.value = false
  }
}

onMounted(async () => {
  try {
    const user = await userProfileApi.get()
    auth.updateUser(user)
    profileForm.value.email = user.email
  } catch {
    // use cached data
  }
})
</script>
