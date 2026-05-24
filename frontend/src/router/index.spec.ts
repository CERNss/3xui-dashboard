import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { router } from './index'

beforeAll(() => {
  const mem: Record<string, string> = {}
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => (k in mem ? mem[k] : null),
    setItem: (k: string, v: string) => { mem[k] = v },
    removeItem: (k: string) => { delete mem[k] },
    clear: () => { for (const k of Object.keys(mem)) delete mem[k] },
  })
})

describe('router unified login redirects', () => {
  beforeEach(async () => {
    setActivePinia(createPinia())
    localStorage.clear()
    if (router.currentRoute.value.path !== '/') {
      await router.push('/')
      await router.isReady()
    }
  })

  it('redirects protected admin pages to /login with next', async () => {
    await router.push('/admin/status')
    await router.isReady()
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.next).toBe('/admin/status')
  })

  it('redirects protected portal pages to /login with next', async () => {
    await router.push('/portal/subscription')
    await router.isReady()
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.next).toBe('/portal/subscription')
  })

  it('does not register old role-specific entry URLs', async () => {
    for (const path of ['/admin/login', '/portal/login', '/admin/dashboard', '/portal/dashboard', '/portal/register']) {
      await router.push(path)
      await router.isReady()
      expect(router.currentRoute.value.name).toBe('notFound')
    }
  })
})
