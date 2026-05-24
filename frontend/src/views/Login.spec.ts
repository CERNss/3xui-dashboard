import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'

beforeAll(() => {
  const mem: Record<string, string> = {}
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => (k in mem ? mem[k] : null),
    setItem: (k: string, v: string) => { mem[k] = v },
    removeItem: (k: string) => { delete mem[k] },
    clear: () => { for (const k of Object.keys(mem)) delete mem[k] },
  })
})

const apiStubs = vi.hoisted(() => ({
  adminLogin: vi.fn(),
  portalLogin: vi.fn(),
  register: vi.fn(),
  sendCode: vi.fn(),
  registrationPolicy: vi.fn(),
  oidcProviders: vi.fn(),
  oidcStart: vi.fn(),
}))

vi.mock('@/api/admin/auth', () => ({
  adminAuthApi: { login: apiStubs.adminLogin },
}))

vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: {
    login: apiStubs.portalLogin,
    register: apiStubs.register,
    sendCode: apiStubs.sendCode,
    registrationPolicy: apiStubs.registrationPolicy,
    oidcProviders: apiStubs.oidcProviders,
    oidcStart: apiStubs.oidcStart,
  },
}))

import Login from './Login.vue'

async function mountLogin(initialPath = '/login') {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'login', component: Login },
      { path: '/admin', name: 'admin.status', component: { template: '<div />' } },
      { path: '/portal', name: 'portal.subscription', component: { template: '<div />' } },
    ],
  })
  await router.push(initialPath)
  await router.isReady()
  const w = mount(Login, {
    global: { plugins: [router, createPinia()] },
    attachTo: document.body,
  })
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  localStorage.clear()
  apiStubs.adminLogin.mockRejectedValue({ status: 401 })
  apiStubs.portalLogin.mockRejectedValue({ status: 401 })
  apiStubs.register.mockResolvedValue({ token: 'portal-jwt', user_id: 1, email: 'u@example.com' })
  apiStubs.sendCode.mockResolvedValue(undefined)
  apiStubs.registrationPolicy.mockResolvedValue({ email_verification_required: false })
  apiStubs.oidcProviders.mockResolvedValue([])
  apiStubs.oidcStart.mockResolvedValue({ authorize_url: '/oidc' })
})

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('Login.vue unified auth', () => {
  it('tries admin first on the neutral login entry, then portal', async () => {
    apiStubs.portalLogin.mockResolvedValue({ token: 'portal-jwt', user_id: 1, email: 'u@example.com' })
    const { w, router } = await mountLogin('/login')
    const inputs = w.findAll('input')
    await inputs[0].setValue('u@example.com')
    await inputs[1].setValue('passw0rd-ok')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(apiStubs.adminLogin).toHaveBeenCalledWith('u@example.com', 'passw0rd-ok')
    expect(apiStubs.portalLogin).toHaveBeenCalledWith('u@example.com', 'passw0rd-ok')
    expect(router.currentRoute.value.path).toBe('/portal')
  })

  it('prefers portal auth when next points at the portal', async () => {
    apiStubs.portalLogin.mockResolvedValue({ token: 'portal-jwt', user_id: 1, email: 'u@example.com' })
    const { w } = await mountLogin('/login?next=/portal')
    const inputs = w.findAll('input')
    await inputs[0].setValue('u@example.com')
    await inputs[1].setValue('passw0rd-ok')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(apiStubs.portalLogin).toHaveBeenCalledTimes(1)
    expect(apiStubs.adminLogin).not.toHaveBeenCalled()
  })

  it('uses the same login page for registration mode', async () => {
    const { w, router } = await mountLogin('/login?mode=register')
    const inputs = w.findAll('input')
    await inputs[0].setValue('alice@example.com')
    await inputs[1].setValue('passw0rd-ok')
    await inputs[2].setValue('passw0rd-ok')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(apiStubs.register).toHaveBeenCalledWith('alice@example.com', 'passw0rd-ok', undefined)
    expect(router.currentRoute.value.path).toBe('/portal')
  })
})
