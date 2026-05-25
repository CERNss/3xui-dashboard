import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import { clearToasts, dismissToast, pushToast } from '@/composables/useToast'

import ToastHost from './ToastHost.vue'

describe('ToastHost', () => {
  afterEach(() => {
    clearToasts()
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('renders and dismisses a pushed toast', async () => {
    vi.useFakeTimers()
    mount(ToastHost, { attachTo: document.body })

    pushToast('登录成功！欢迎回来。')
    await flushPromises()

    expect(document.body.textContent).toContain('登录成功！欢迎回来。')
    const button = document.body.querySelector('button')
    expect(button).toBeTruthy()
    button!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    expect(document.body.textContent).not.toContain('登录成功！欢迎回来。')
  })

  it('auto-dismisses after the configured duration', async () => {
    vi.useFakeTimers()
    mount(ToastHost, { attachTo: document.body })

    pushToast('操作成功', 'success', 100)
    await flushPromises()
    expect(document.body.textContent).toContain('操作成功')

    vi.advanceTimersByTime(100)
    await flushPromises()
    expect(document.body.textContent).not.toContain('操作成功')
  })

  it('allows direct dismissal by id', async () => {
    vi.useFakeTimers()
    mount(ToastHost, { attachTo: document.body })

    const id = pushToast('A')
    await flushPromises()
    expect(document.body.textContent).toContain('A')

    // Current host caps to 4 toasts, so this makes sure the exported
    // dismissal path remains usable for non-UI callers too.
    dismissToast(id!)
    await flushPromises()
    expect(document.body.textContent).not.toContain('A')
  })

  it('coalesces duplicate toast text', async () => {
    vi.useFakeTimers()
    mount(ToastHost, { attachTo: document.body })

    pushToast('操作成功')
    pushToast('操作成功')
    await flushPromises()

    expect(document.body.textContent?.match(/操作成功/g)?.length).toBe(1)
  })
})
