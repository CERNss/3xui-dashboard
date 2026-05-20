import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import type { AdminUser } from '@/api/admin/users'

const apiStubs = vi.hoisted(() => ({
  list: vi.fn(),
  suspend: vi.fn(),
  unsuspend: vi.fn(),
  remove: vi.fn(),
  adjustBalance: vi.fn(),
}))
vi.mock('@/api/admin/users', () => ({
  adminUsersApi: apiStubs,
}))

import Users from './Users.vue'

function makeUser(over: Partial<AdminUser> = {}): AdminUser {
  return {
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1500,
    sub_id: 'sub-abc123def456',
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...over,
  }
}

async function mountUsers() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/admin/users', component: { template: '<div/>' } }],
  })
  await router.push('/admin/users')
  await router.isReady()
  const w = mount(Users, {
    global: { plugins: [router], mocks: { $t: (k: string) => k } },
    attachTo: document.body,
  })
  await flushPromises()
  return w
}

beforeEach(() => {
  apiStubs.list.mockResolvedValue({
    users: [
      makeUser(),
      makeUser({ id: 2, email: 'bob@example.com', status: 'suspended', balance_cents: 0 }),
    ],
    limit: 200,
    offset: 0,
  })
})
afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('Users.vue smoke', () => {
  it('mounts and renders the page title', async () => {
    const w = await mountUsers()
    expect(w.text()).toContain('用户管理')
  })

  it('renders the user list from the API', async () => {
    const w = await mountUsers()
    expect(w.text()).toContain('alice@example.com')
    expect(w.text()).toContain('bob@example.com')
    expect(w.text()).toContain('¥15.00') // 1500 cents formatted
  })

  it('shows status badges with localized labels', async () => {
    const w = await mountUsers()
    expect(w.text()).toContain('正常')
    expect(w.text()).toContain('已停用')
  })

  it('search filters the visible rows by email', async () => {
    const w = await mountUsers()
    const search = w.find('input[type="text"]')
    await search.setValue('alice')
    await flushPromises()
    expect(w.text()).toContain('alice@example.com')
    expect(w.text()).not.toContain('bob@example.com')
  })
})
