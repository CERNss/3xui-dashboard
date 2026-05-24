import { afterEach, describe, expect, it } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import AccountMenu from './AccountMenu.vue'

async function mountMenu() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/profile', component: { template: '<div />' } },
    ],
  })
  await router.push('/')
  await router.isReady()
  return mount(AccountMenu, {
    props: {
      name: 'AI',
      subtitle: 'dev-ai@example.com',
      roleLabel: 'Admin',
      openLabel: '打开账户菜单',
      logoutLabel: '退出登录',
      items: [
        {
          label: '个人资料',
          to: '/profile',
          icon: 'M4 4h16v16H4z',
        },
        {
          label: 'GitHub',
          href: 'https://github.com/cern/3xui-dashboard',
          icon: 'M4 4h16v16H4z',
        },
      ],
    },
    global: { plugins: [router] },
    attachTo: document.body,
  })
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('common/AccountMenu.vue', () => {
  it('opens the menu and emits logout', async () => {
    const wrapper = await mountMenu()

    expect(wrapper.text()).not.toContain('dev-ai@example.com')

    const trigger = wrapper.get('button[aria-label="打开账户菜单"]')
    expect(trigger.attributes('aria-expanded')).toBe('false')

    await trigger.trigger('click')
    await flushPromises()

    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.text()).toContain('dev-ai@example.com')
    expect(wrapper.text()).toContain('个人资料')
    expect(wrapper.text()).toContain('GitHub')
    expect(wrapper.text()).toContain('退出登录')

    const logout = wrapper.findAll('button').find((button) => button.text().includes('退出登录'))
    expect(logout).toBeTruthy()
    await logout!.trigger('click')

    expect(wrapper.emitted('logout')).toHaveLength(1)
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('renders items in the order supplied (router-link and href interleaved)', async () => {
    const wrapper = await mountMenu()
    await wrapper.get('button[aria-label="打开账户菜单"]').trigger('click')
    await flushPromises()

    const itemTexts = wrapper.findAll('[role="menuitem"]').map((n) => n.text())
    // Caller passed: 个人资料 (to), GitHub (href). Order must be preserved
    // and logout is appended last by the component itself.
    expect(itemTexts).toEqual(['个人资料', 'GitHub', '退出登录'])
  })

  it('sets target=_blank and rel="noopener noreferrer" on external links', async () => {
    const wrapper = await mountMenu()
    await wrapper.get('button[aria-label="打开账户菜单"]').trigger('click')
    await flushPromises()

    const link = wrapper.get('a[href="https://github.com/cern/3xui-dashboard"]')
    expect(link.attributes('target')).toBe('_blank')
    const rel = link.attributes('rel') ?? ''
    expect(rel).toContain('noopener')
    expect(rel).toContain('noreferrer')
  })

  it('closes the menu on Escape', async () => {
    const wrapper = await mountMenu()
    const trigger = wrapper.get('button[aria-label="打开账户菜单"]')
    await trigger.trigger('click')
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('true')

    await wrapper.get('div.relative').trigger('keydown.escape')
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('closes the menu on outside pointerdown', async () => {
    const wrapper = await mountMenu()
    const trigger = wrapper.get('button[aria-label="打开账户菜单"]')
    await trigger.trigger('click')
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('true')

    // jsdom lacks PointerEvent; fake-init an Event with the right type so
    // the document-level pointerdown listener fires with bubbles=true.
    const event = new Event('pointerdown', { bubbles: true })
    document.body.dispatchEvent(event)
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('closes the menu when a router-link item is clicked', async () => {
    const wrapper = await mountMenu()
    const trigger = wrapper.get('button[aria-label="打开账户菜单"]')
    await trigger.trigger('click')
    await flushPromises()

    const item = wrapper.findAll('a[role="menuitem"]').find((n) => n.text().includes('个人资料'))
    expect(item).toBeTruthy()
    await item!.trigger('click')
    await flushPromises()

    expect(trigger.attributes('aria-expanded')).toBe('false')
  })
})
