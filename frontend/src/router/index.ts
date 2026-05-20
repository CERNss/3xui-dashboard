import {
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
  type NavigationGuardWithThis,
} from 'vue-router'

import { useAdminAuthStore } from '@/stores/adminAuth'
import { usePortalAuthStore } from '@/stores/portalAuth'

// ---- Admin route tree ------------------------------------------------------
const adminRoutes: RouteRecordRaw[] = [
  // Legacy alias — preserve old bookmarks. Redirects to unified /login.
  {
    path: '/admin/login',
    redirect: (to) => ({ path: '/login', query: { ...to.query, hint: 'admin' } }),
  },
  {
    path: '/admin',
    component: () => import('@/components/layout/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    children: [
      { path: '', redirect: { name: 'admin.status' } },
      { path: 'dashboard', redirect: { name: 'admin.status' } }, // legacy bookmark
      {
        path: 'status',
        name: 'admin.status',
        component: () => import('@/views/admin/Status.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.status' },
      },
      {
        path: 'nodes',
        name: 'admin.nodes',
        component: () => import('@/views/admin/Nodes.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.nodes' },
      },
      {
        path: 'inbounds',
        name: 'admin.inbounds',
        component: () => import('@/views/admin/Inbounds.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.inbounds' },
      },
      {
        path: 'users',
        name: 'admin.users',
        component: () => import('@/views/admin/Users.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.users' },
      },
      {
        path: 'plans',
        name: 'admin.plans',
        component: () => import('@/views/admin/Plans.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.plans' },
      },
      {
        path: 'orders',
        name: 'admin.orders',
        component: () => import('@/views/admin/Orders.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.orders' },
      },
      {
        path: 'stats',
        name: 'admin.stats',
        component: () => import('@/views/admin/Stats.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.stats' },
      },
      {
        path: 'webhooks',
        name: 'admin.webhooks',
        component: () => import('@/views/admin/Webhooks.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.webhooks' },
      },
      {
        path: 'settings',
        name: 'admin.settings',
        component: () => import('@/views/admin/Settings.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.settings' },
      },
    ],
  },
]

// ---- Portal route tree -----------------------------------------------------
const portalRoutes: RouteRecordRaw[] = [
  // Legacy alias — preserve old bookmarks. Redirects to unified /login.
  {
    path: '/portal/login',
    redirect: (to) => ({ path: '/login', query: { ...to.query, hint: 'portal' } }),
  },
  {
    path: '/portal/register',
    name: 'portal.register',
    component: () => import('@/views/portal/Register.vue'),
    meta: { titleKey: 'auth.register' },
  },
  {
    path: '/portal',
    component: () => import('@/components/layout/PortalLayout.vue'),
    meta: { requiresUser: true },
    children: [
      // Default landing — subscription is the primary action,
      // matching the Sub2API-style portal shape (post-sale, not
      // shopping-first).
      { path: '', redirect: { name: 'portal.subscription' } },
      // Legacy /portal/dashboard preserved as a redirect so old
      // bookmarks don't 404 after the slim-down.
      {
        path: 'dashboard',
        redirect: { name: 'portal.usage' },
      },
      {
        path: 'subscription',
        name: 'portal.subscription',
        component: () => import('@/views/portal/Subscription.vue'),
        meta: { requiresUser: true, titleKey: 'nav.subscription' },
      },
      {
        path: 'usage',
        name: 'portal.usage',
        // Reuses the Dashboard component — the traffic-stat
        // content already matches what "使用记录" should show.
        component: () => import('@/views/portal/Dashboard.vue'),
        meta: { requiresUser: true, titleKey: 'nav.usage' },
      },
      {
        path: 'plans',
        name: 'portal.plans',
        component: () => import('@/views/portal/Plans.vue'),
        meta: { requiresUser: true, titleKey: 'nav.plans' },
      },
      {
        path: 'orders',
        name: 'portal.orders',
        component: () => import('@/views/portal/Orders.vue'),
        meta: { requiresUser: true, titleKey: 'nav.orders' },
      },
      {
        path: 'profile',
        name: 'portal.profile',
        component: () => import('@/views/portal/Profile.vue'),
        meta: { requiresUser: true, titleKey: 'nav.profile' },
      },
    ],
  },
]

const routes: RouteRecordRaw[] = [
  // Unified login — single entrypoint for both admin and portal users.
  // Picks which auth endpoint to hit based on credentials + optional ?hint=.
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { titleKey: 'auth.login' },
  },
  // Root — admin store has priority since fleet operators are the heavy users.
  // Unauthenticated visitors get bounced to /login by the guard below.
  { path: '/', redirect: '/admin' },
  ...adminRoutes,
  ...portalRoutes,
  {
    path: '/:pathMatch(.*)*',
    name: 'notFound',
    component: () => import('@/views/NotFound.vue'),
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

const authGuard: NavigationGuardWithThis<undefined> = (to) => {
  if (to.meta.requiresAdmin) {
    const adminAuth = useAdminAuthStore()
    if (!adminAuth.isAuthenticated) {
      return { name: 'login', query: { next: to.fullPath, hint: 'admin' } }
    }
  }
  if (to.meta.requiresUser) {
    const portalAuth = usePortalAuthStore()
    if (!portalAuth.isAuthenticated) {
      return { name: 'login', query: { next: to.fullPath, hint: 'portal' } }
    }
  }
  return true
}

router.beforeEach(authGuard)
