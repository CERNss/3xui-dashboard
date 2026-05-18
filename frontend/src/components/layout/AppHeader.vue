<template>
  <header class="h-16 bg-white border-b border-gray-200 flex items-center justify-between px-4 lg:px-6 shrink-0">
    <!-- Mobile hamburger -->
    <button
      class="lg:hidden p-2 rounded-lg text-gray-500 hover:text-gray-700 hover:bg-gray-100"
      @click="appStore.toggleSidebar"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </button>

    <!-- Page title -->
    <h1 class="text-base font-semibold text-gray-800 lg:text-lg">{{ pageTitle }}</h1>

    <!-- Right: user avatar -->
    <div class="flex items-center gap-3">
      <div class="text-right hidden sm:block">
        <p class="text-sm font-medium text-gray-800">{{ auth.user?.username }}</p>
        <p class="text-xs text-gray-500 capitalize">{{ auth.user?.role }}</p>
      </div>
      <div
        class="w-8 h-8 rounded-full bg-primary-600 text-white text-sm font-semibold flex items-center justify-center uppercase"
      >
        {{ auth.user?.username?.charAt(0) ?? '?' }}
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const auth = useAuthStore()
const appStore = useAppStore()
const route = useRoute()

const titleMap: Record<string, string> = {
  AdminDashboard: 'Dashboard',
  AdminInbounds: 'Inbounds',
  AdminClients: 'Clients',
  AdminNodes: 'Nodes',
  AdminUsers: 'Users',
  UserDashboard: 'Dashboard',
  UserSubscription: 'Subscription',
  UserProfile: 'Profile'
}

const pageTitle = computed(() => titleMap[route.name as string] ?? '3x-ui Dashboard')
</script>
