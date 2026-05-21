import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'

// jsdom in our vitest config doesn't expose localStorage to module
// initializers in some import orders — stub it before the store
// imports.
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
  login: vi.fn(),
}))
vi.mock('@/api/admin/auth', () => ({
  adminAuthApi: { login: apiStubs.login },
}))

import Login from './Login.vue'

async function mountLogin() {
  setActivePinia(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/login', name: 'admin.login', component: { template: '<div/>' } },
      { path: '/admin', name: 'admin.status', component: { template: '<div/>' } },
    ],
  })
  await router.push('/login')
  await router.isReady()
  const w = mount(Login, {
    global: {
      plugins: [router, createPinia()],
      mocks: { $t: (k: string) => k },
    },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.login.mockResolvedValue({ token: 'fake-jwt', username: 'admin' })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('admin/Login.vue smoke', () => {
  it('mounts and renders the credential form', async () => {
    const w = await mountLogin()
    expect(w.exists()).toBe(true)
    // Two input fields (username, password) + 1 submit button.
    const inputs = w.findAll('input')
    expect(inputs.length).toBeGreaterThanOrEqual(2)
    expect(w.findAll('button').length).toBeGreaterThan(0)
  })

  it('skips the API when fields are empty (early return)', async () => {
    const w = await mountLogin()
    const form = w.find('form')
    await form.trigger('submit.prevent')
    await flushPromises()
    expect(apiStubs.login).not.toHaveBeenCalled()
  })

  it('calls adminAuthApi.login with the filled credentials', async () => {
    const w = await mountLogin()
    const inputs = w.findAll('input')
    await inputs[0].setValue('admin')
    await inputs[1].setValue('hunter2')
    await w.find('form').trigger('submit.prevent')
    await flushPromises()
    expect(apiStubs.login).toHaveBeenCalledTimes(1)
    expect(apiStubs.login).toHaveBeenCalledWith('admin', 'hunter2')
  })
})
