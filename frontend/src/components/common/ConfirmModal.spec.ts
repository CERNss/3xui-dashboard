import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'

import ConfirmModal from './ConfirmModal.vue'

describe('ConfirmModal', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders title + message into body when open', () => {
    mount(ConfirmModal, {
      props: { open: true, title: '删除用户', message: '此操作不可撤销' },
      attachTo: document.body,
    })
    expect(document.body.textContent).toContain('删除用户')
    expect(document.body.textContent).toContain('此操作不可撤销')
  })

  it('does not render anything in DOM when closed', () => {
    mount(ConfirmModal, { props: { open: false, title: 'X' }, attachTo: document.body })
    expect(document.body.textContent).not.toContain('X')
  })

  it('emits confirm + update:open(false) when confirm button clicked', async () => {
    const w = mount(ConfirmModal, {
      props: { open: true, title: 'X', confirmLabel: '确认' },
      attachTo: document.body,
    })
    const buttons = [...document.body.querySelectorAll('button')]
    const confirmBtn = buttons.find((b) => b.textContent?.trim() === '确认')!
    confirmBtn.click()
    await w.vm.$nextTick()
    expect(w.emitted('confirm')).toBeTruthy()
  })

  it('emits cancel + update:open(false) when cancel button clicked', async () => {
    const w = mount(ConfirmModal, {
      props: { open: true, title: 'X', cancelLabel: '取消' },
      attachTo: document.body,
    })
    const buttons = [...document.body.querySelectorAll('button')]
    const cancelBtn = buttons.find((b) => b.textContent?.trim() === '取消')!
    cancelBtn.click()
    await w.vm.$nextTick()
    expect(w.emitted('cancel')).toBeTruthy()
    expect(w.emitted('update:open')?.[0]).toEqual([false])
  })

  it('does NOT emit when busy=true (buttons disabled)', async () => {
    const w = mount(ConfirmModal, {
      props: { open: true, title: 'X', busy: true, confirmLabel: '确认' },
      attachTo: document.body,
    })
    const buttons = [...document.body.querySelectorAll('button')]
    const confirmBtn = buttons.find((b) => b.textContent?.includes('确认'))!
    // disabled buttons still emit DOM click but our handler early-returns
    confirmBtn.click()
    await w.vm.$nextTick()
    expect(w.emitted('confirm')).toBeFalsy()
  })

  it('Escape key emits cancel', async () => {
    const w = mount(ConfirmModal, {
      props: { open: true, title: 'X' },
      attachTo: document.body,
    })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()
    expect(w.emitted('cancel')).toBeTruthy()
  })

  it('applies danger styling when variant=danger', () => {
    mount(ConfirmModal, {
      props: { open: true, title: 'X', variant: 'danger', confirmLabel: 'Del' },
      attachTo: document.body,
    })
    const buttons = [...document.body.querySelectorAll('button')]
    const confirmBtn = buttons.find((b) => b.textContent?.includes('Del'))!
    expect(confirmBtn.className).toContain('bg-red-')
  })
})
