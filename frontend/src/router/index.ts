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
    path: '/admin/login',
    name: 'admin.login',
    component: () => import('@/views/admin/Login.vue'),
    meta: { titleKey: 'auth.login' },
  },
  {
    path: '/admin',
    component: () => import('@/components/layout/AdminLayout.vue'),
    meta: { requiresAdmin: true },
    children: [
      { path: '', redirect: { name: 'admin.dashboard' } },
      {
        path: 'dashboard',
        name: 'admin.dashboard',
        component: () => import('@/views/admin/Dashboard.vue'),
        meta: { requiresAdmin: true, titleKey: 'nav.dashboard' },
      },
    ],
  },
]

// ---- Portal route tree -----------------------------------------------------
const portalRoutes: RouteRecordRaw[] = [
  {
    path: '/portal/login',
    name: 'portal.login',
    component: () => import('@/views/portal/Login.vue'),
    meta: { titleKey: 'auth.login' },
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
      { path: '', redirect: { name: 'portal.dashboard' } },
      {
        path: 'dashboard',
        name: 'portal.dashboard',
        component: () => import('@/views/portal/Dashboard.vue'),
        meta: { requiresUser: true, titleKey: 'nav.dashboard' },
      },
    ],
  },
]

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/portal' },
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
      return { name: 'admin.login', query: { next: to.fullPath } }
    }
  }
  if (to.meta.requiresUser) {
    const portalAuth = usePortalAuthStore()
    if (!portalAuth.isAuthenticated) {
      return { name: 'portal.login', query: { next: to.fullPath } }
    }
  }
  return true
}

router.beforeEach(authGuard)
