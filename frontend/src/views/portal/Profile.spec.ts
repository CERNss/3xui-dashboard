import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { LoginMethodsResponse, UserProfile } from '@/api/portal/profile'

const apiStubs = vi.hoisted(() => ({
  get: vi.fn<() => Promise<UserProfile>>(),
  loginMethods: vi.fn<() => Promise<LoginMethodsResponse>>(),
  changePassword: vi.fn<(oldPassword: string, newPassword: string) => Promise<unknown>>(),
  bindEmail: vi.fn<(email: string) => Promise<unknown>>(),
  startOIDCLink: vi.fn<(redirectAfter?: string) => Promise<{ authorize_url: string }>>(),
  rotateSubID: vi.fn<() => Promise<{ sub_id: string }>>(),
}))

vi.mock('@/api/portal/profile', () => ({
  portalProfileApi: apiStubs,
}))

import Profile from './Profile.vue'

function makeProfile(over: Partial<UserProfile> = {}): UserProfile {
  return {
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1000,
    sub_id: 'sub-1',
    created_at: '2026-05-01T00:00:00Z',
    ...over,
  }
}

function makeMethods(over: Partial<LoginMethodsResponse> = {}): LoginMethodsResponse {
  return {
    email: { bound: true, email: 'alice@example.com', verified: true },
    oidc: { enabled: true, bound: false, name: '集换社认证系统' },
    ...over,
  }
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/portal/profile', component: { template: '<div />' } },
      { path: '/portal/orders', component: { template: '<div />' } },
    ],
  })
}

async function mountProfile(path = '/portal/profile') {
  const router = makeRouter()
  await router.push(path)
  await router.isReady()
  const w = mount(Profile, {
    global: { plugins: [router] },
    attachTo: document.body,
  })
  await flushPromises()
  return { w, router }
}

beforeEach(() => {
  apiStubs.get.mockResolvedValue(makeProfile())
  apiStubs.loginMethods.mockResolvedValue(makeMethods())
  apiStubs.changePassword.mockResolvedValue({ status: 'ok' })
  apiStubs.bindEmail.mockResolvedValue({ status: 'ok' })
  apiStubs.startOIDCLink.mockResolvedValue({ authorize_url: 'https://idp.example.com/authorize' })
  apiStubs.rotateSubID.mockResolvedValue({ sub_id: 'sub-2' })
})

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('Profile.vue login methods', () => {
  it('renders email and configured OIDC binding state', async () => {
    const { w } = await mountProfile()

    expect(w.text()).toContain('登录方式绑定')
    expect(w.text()).toContain('邮箱登录')
    expect(w.text()).toContain('alice@example.com')
    expect(w.text()).toContain('集换社认证系统')
    expect(w.text()).toContain('绑定 集换社认证系统')
  })

  it('starts OIDC linking from the profile page', async () => {
    const originalLocation = window.location
    Object.defineProperty(window, 'location', {
      value: { ...originalLocation, href: '' },
      configurable: true,
    })

    const { w } = await mountProfile()
    const button = w.findAll('button').find((b) => b.text().includes('绑定 集换社认证系统'))
    expect(button).toBeDefined()
    await button!.trigger('click')
    await flushPromises()

    expect(apiStubs.startOIDCLink).toHaveBeenCalledWith('/portal/profile?linked=oidc')
    expect(window.location.href).toBe('https://idp.example.com/authorize')

    Object.defineProperty(window, 'location', {
      value: originalLocation,
      configurable: true,
    })
  })

  it('does not show hard-coded providers when OIDC is unavailable', async () => {
    apiStubs.loginMethods.mockResolvedValue(makeMethods({
      oidc: { enabled: false, bound: false },
    }))

    const { w } = await mountProfile()
    expect(w.text()).toContain('第三方认证')
    expect(w.text()).toContain('未开启')
    expect(w.text()).not.toContain('LinuxDo')
    expect(w.text()).not.toContain('微信')
  })

  it('shows success flash and clears ?linked=oidc when returning from IDP', async () => {
    apiStubs.loginMethods.mockResolvedValue(
      makeMethods({ oidc: { enabled: true, bound: true, name: '集换社认证系统' } }),
    )

    const { w, router } = await mountProfile('/portal/profile?linked=oidc')

    expect(w.text()).toContain('集换社认证系统 已绑定')
    expect(router.currentRoute.value.query.linked).toBeUndefined()
  })

  it('surfaces an error flash when startOIDCLink fails', async () => {
    apiStubs.startOIDCLink.mockRejectedValue(new Error('idp unreachable'))

    const { w } = await mountProfile()
    const button = w.findAll('button').find((b) => b.text().includes('绑定 集换社认证系统'))
    await button!.trigger('click')
    await flushPromises()

    // formatError surfaces Error.message verbatim, so the raw message
    // should be rendered as the err-flash body.
    expect(w.text()).toContain('idp unreachable')
  })
})
