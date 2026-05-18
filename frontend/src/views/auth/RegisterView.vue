<template>
  <AuthLayout>
    <div class="bg-white rounded-2xl shadow-xl p-8">
      <h2 class="text-xl font-bold text-gray-900 mb-6">Create Account</h2>
      <form @submit.prevent="handleRegister" class="space-y-5">
        <div>
          <label class="label">Username</label>
          <input
            v-model="form.username"
            class="input"
            type="text"
            placeholder="Choose a username (min 3 chars)"
            autocomplete="username"
            minlength="3"
            required
          />
        </div>
        <div>
          <label class="label">Email</label>
          <input
            v-model="form.email"
            class="input"
            type="email"
            placeholder="your@email.com"
            autocomplete="email"
            required
          />
        </div>
        <div>
          <label class="label">Password</label>
          <input
            v-model="form.password"
            class="input"
            type="password"
            placeholder="Min 6 characters"
            autocomplete="new-password"
            minlength="6"
            required
          />
        </div>
        <div v-if="errorMsg" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
          {{ errorMsg }}
        </div>
        <button type="submit" class="btn-primary w-full" :disabled="loading">
          <LoadingSpinner v-if="loading" size="sm" />
          Create Account
        </button>
      </form>
      <p class="text-center text-sm text-gray-600 mt-6">
        Already have an account?
        <router-link to="/login" class="text-primary-600 hover:underline font-medium">Sign In</router-link>
      </p>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const auth = useAuthStore()
const router = useRouter()

const form = ref({ username: '', email: '', password: '' })
const loading = ref(false)
const errorMsg = ref('')

async function handleRegister() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await authApi.register(form.value)
    auth.setAuth(res.token, res.user)
    await router.push(res.user.role === 'admin' ? '/admin/dashboard' : '/user/dashboard')
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    errorMsg.value = err.response?.data?.error ?? 'Registration failed. Username or email may already be taken.'
  } finally {
    loading.value = false
  }
}
</script>
