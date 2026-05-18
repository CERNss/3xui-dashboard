<template>
  <!-- Mobile backdrop -->
  <div
    v-if="appStore.sidebarOpen"
    class="fixed inset-0 z-20 bg-black/50 lg:hidden"
    @click="appStore.closeSidebar"
  />

  <!-- Sidebar -->
  <aside
    class="fixed inset-y-0 left-0 z-30 w-64 bg-gray-900 text-white flex flex-col transition-transform duration-300 lg:translate-x-0"
    :class="appStore.sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-6 h-16 border-b border-gray-700 shrink-0">
      <div class="w-8 h-8 rounded-lg bg-primary-600 flex items-center justify-center text-white font-bold text-sm">
        3X
      </div>
      <span class="font-semibold text-lg">3x-ui Dashboard</span>
    </div>

    <!-- Role badge -->
    <div class="px-6 py-3 border-b border-gray-700">
      <span
        class="text-xs font-medium px-2 py-1 rounded-full"
        :class="auth.isAdmin ? 'bg-primary-900 text-primary-300' : 'bg-gray-700 text-gray-300'"
      >
        {{ auth.isAdmin ? 'Administrator' : 'User' }}
      </span>
      <p class="text-gray-400 text-sm mt-1 truncate">{{ auth.user?.username }}</p>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 overflow-y-auto px-3 py-4">
      <!-- Admin nav -->
      <template v-if="auth.isAdmin">
        <p class="px-3 mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">Management</p>
        <NavItem to="/admin/dashboard" icon="grid">Dashboard</NavItem>
        <NavItem to="/admin/inbounds" icon="server">Inbounds</NavItem>
        <NavItem to="/admin/clients" icon="users">Clients</NavItem>
        <NavItem to="/admin/nodes" icon="globe">Nodes</NavItem>
        <p class="px-3 mt-6 mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">Settings</p>
        <NavItem to="/admin/users" icon="user-cog">Users</NavItem>
      </template>

      <!-- User nav -->
      <template v-else>
        <p class="px-3 mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500">My Account</p>
        <NavItem to="/user/dashboard" icon="home">Dashboard</NavItem>
        <NavItem to="/user/subscription" icon="link">Subscription</NavItem>
        <NavItem to="/user/profile" icon="user">Profile</NavItem>
      </template>
    </nav>

    <!-- Footer: logout -->
    <div class="px-3 py-4 border-t border-gray-700">
      <button
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors text-sm"
        @click="logout"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        Sign Out
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useRouter } from 'vue-router'
import NavItem from './NavItem.vue'

const auth = useAuthStore()
const appStore = useAppStore()
const router = useRouter()

function logout() {
  auth.clearAuth()
  router.push('/login')
}
</script>
