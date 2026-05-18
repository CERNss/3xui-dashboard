import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  // ---- Public ----
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: { requiresAuth: false }
  },

  // ---- Admin ----
  {
    path: '/admin',
    component: () => import('@/components/layout/AppLayout.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
    children: [
      {
        path: '',
        redirect: '/admin/dashboard'
      },
      {
        path: 'dashboard',
        name: 'AdminDashboard',
        component: () => import('@/views/admin/DashboardView.vue')
      },
      {
        path: 'inbounds',
        name: 'AdminInbounds',
        component: () => import('@/views/admin/InboundsView.vue')
      },
      {
        path: 'clients',
        name: 'AdminClients',
        component: () => import('@/views/admin/ClientsView.vue')
      },
      {
        path: 'nodes',
        name: 'AdminNodes',
        component: () => import('@/views/admin/NodesView.vue')
      },
      {
        path: 'users',
        name: 'AdminUsers',
        component: () => import('@/views/admin/UsersView.vue')
      }
    ]
  },

  // ---- User ----
  {
    path: '/user',
    component: () => import('@/components/layout/AppLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/user/dashboard'
      },
      {
        path: 'dashboard',
        name: 'UserDashboard',
        component: () => import('@/views/user/DashboardView.vue')
      },
      {
        path: 'subscription',
        name: 'UserSubscription',
        component: () => import('@/views/user/SubscriptionView.vue')
      },
      {
        path: 'profile',
        name: 'UserProfile',
        component: () => import('@/views/user/ProfileView.vue')
      }
    ]
  },

  // ---- Root redirect ----
  {
    path: '/',
    redirect: () => {
      // Will be resolved after auth check in guard
      return '/login'
    }
  },

  // ---- 404 ----
  {
    path: '/:pathMatch(.*)*',
    redirect: '/login'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  // Root: redirect based on role
  if (to.path === '/') {
    if (!auth.isAuthenticated) return '/login'
    return auth.isAdmin ? '/admin/dashboard' : '/user/dashboard'
  }

  // Public routes
  if (to.meta.requiresAuth === false) {
    // Redirect already-authenticated users away from login/register
    if (auth.isAuthenticated && (to.name === 'Login' || to.name === 'Register')) {
      return auth.isAdmin ? '/admin/dashboard' : '/user/dashboard'
    }
    return true
  }

  // Protected routes
  if (!auth.isAuthenticated) {
    return { name: 'Login', query: { redirect: to.fullPath } }
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return '/user/dashboard'
  }

  return true
})

export default router
