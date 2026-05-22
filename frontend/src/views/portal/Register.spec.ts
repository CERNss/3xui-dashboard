import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'

// Same localStorage stub as Login.spec.ts — Pinia stores
// initialize `state()` with localStorage.getItem at instantiation.
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
  register: vi.fn(),
}))
vi.mock('@/api/portal/auth', () => ({
  portalAuthApi: { register: apiStubs.register },
}))

import Register from './Register.vue'

async function mountRegister() {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/register', name: 'portal.register', component: { template: '<div/>' } },
      { path: '/portal', name: 'portal.subscription', component: { template: '<div/>' } },
    ],
  })
  await router.push('/register')
  await router.isReady()
  const w = mount(Register, {
    global: {
      plugins: [router, createPinia()],
    },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.register.mockResolvedValue({ token: 'fake-jwt', user: { id: 1, email: 'u@x.com' } })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('portal/Register.vue smoke', () => {
  it('mounts and renders the signup form', async () => {
    const w = await mountRegister()
    expect(w.exists()).toBe(true)
    expect(w.findAll('input').length).toBeGreaterThanOrEqual(2)
  })

  it('client-side rejects passwords shorter than 8 chars', async () => {
    const w = await mountRegister()
    const inputs = w.findAll('input')
    await inputs[0].setValue('a@b.com')
    await inputs[1].setValue('short') // < 8
    await w.find('form').trigger('submit.prevent')
    await flushPromises()
    expect(apiStubs.register).not.toHaveBeenCalled()
    // Error message surfaces (any non-empty error chip on the page).
    expect(w.text().toLowerCase()).toMatch(/8 characters|至少 8/)
  })

  it('calls portalAuthApi.register with email + password on submit', async () => {
    const w = await mountRegister()
    const inputs = w.findAll('input')
    await inputs[0].setValue('alice@example.com')
    await inputs[1].setValue('passw0rd-ok')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()
    expect(apiStubs.register).toHaveBeenCalledTimes(1)
    expect(apiStubs.register).toHaveBeenCalledWith('alice@example.com', 'passw0rd-ok')
  })
})
