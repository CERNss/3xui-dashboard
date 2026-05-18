<template>
  <AuthLayout>
    <div class="bg-white rounded-2xl shadow-xl p-8">
      <h2 class="text-xl font-bold text-gray-900 mb-6">Sign In</h2>
      <form @submit.prevent="handleLogin" class="space-y-5">
        <div>
          <label class="label">Username</label>
          <input
            v-model="form.username"
            class="input"
            type="text"
            placeholder="Enter your username"
            autocomplete="username"
            required
          />
        </div>
        <div>
          <label class="label">Password</label>
          <input
            v-model="form.password"
            class="input"
            type="password"
            placeholder="Enter your password"
            autocomplete="current-password"
            required
          />
        </div>
        <div v-if="errorMsg" class="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
          {{ errorMsg }}
        </div>
        <button type="submit" class="btn-primary w-full" :disabled="loading">
          <LoadingSpinner v-if="loading" size="sm" />
          Sign In
        </button>
      </form>
      <p class="text-center text-sm text-gray-600 mt-6">
        Don't have an account?
        <router-link to="/register" class="text-primary-600 hover:underline font-medium">Register</router-link>
      </p>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const form = ref({ username: '', password: '' })
const loading = ref(false)
const errorMsg = ref('')

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await authApi.login(form.value)
    auth.setAuth(res.token, res.user)
    const redirect = (route.query.redirect as string) || (res.user.role === 'admin' ? '/admin/dashboard' : '/user/dashboard')
    await router.push(redirect)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    errorMsg.value = err.response?.data?.error ?? 'Login failed. Please try again.'
  } finally {
    loading.value = false
  }
}
</script>
