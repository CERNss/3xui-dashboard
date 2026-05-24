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
  {
    path: '/admin',
    component: () => import('@/components/layout/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    children: [
      { path: '', redirect: { name: 'admin.status' } },
      {
        path: 'status',
        name: 'admin.status',
        component: () => import('@/views/admin/Overview.vue'),
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
        path: 'provisioning-pools',
        name: 'admin.provisioningPools',
        component: () => import('@/views/admin/ProvisioningPools.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.provisioningPools' },
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
        component: () => import('@/views/admin/Overview.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.stats' },
      },
      {
        path: 'audit-log',
        name: 'admin.audit',
        component: () => import('@/views/admin/AuditLog.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.audit' },
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
  {
    path: '/portal',
    component: () => import('@/components/layout/PortalLayout.vue'),
    meta: { requiresUser: true },
    children: [
      { path: '', redirect: { name: 'portal.subscription' } },
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
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { titleKey: 'auth.login' },
  },
  // OIDC callback — IDP redirects here with ?code=&state= once the
  // user finishes the IDP login. The view immediately calls
  // /api/user/auth/oidc/callback to exchange them for a JWT.
  {
    path: '/oidc/callback',
    name: 'oidc.callback',
    component: () => import('@/views/OIDCCallback.vue'),
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

type AuthArea = 'admin' | 'portal'

const defaultAuthEntryPaths = {
  admin: new Set(['/admin', '/admin/', '/admin/status']),
  portal: new Set(['/portal', '/portal/', '/portal/subscription']),
}

function loginLocationFor(to: { path: string; fullPath: string }, area: AuthArea) {
  if (defaultAuthEntryPaths[area].has(to.path)) {
    return { name: 'login', query: { next: to.path } }
  }

  return { name: 'login', query: { next: to.fullPath } }
}

const authGuard: NavigationGuardWithThis<undefined> = (to) => {
  if (to.meta.requiresAdmin) {
    const adminAuth = useAdminAuthStore()
    if (!adminAuth.isAuthenticated) {
      return loginLocationFor(to, 'admin')
    }
  }
  if (to.meta.requiresUser) {
    const portalAuth = usePortalAuthStore()
    if (!portalAuth.isAuthenticated) {
      return loginLocationFor(to, 'portal')
    }
  }
  return true
}

router.beforeEach(authGuard)
