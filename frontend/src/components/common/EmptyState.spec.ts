import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import EmptyState from './EmptyState.vue'

describe('EmptyState', () => {
  it('renders title + description', () => {
    const w = mount(EmptyState, {
      props: { title: '没有数据', description: '稍后再来' },
    })
    expect(w.text()).toContain('没有数据')
    expect(w.text()).toContain('稍后再来')
  })

  it('does not render action button without actionLabel', () => {
    const w = mount(EmptyState, { props: { title: 'X' } })
    expect(w.find('button').exists()).toBe(false)
  })

  it('emits action when button clicked', async () => {
    const w = mount(EmptyState, {
      props: { title: 'X', actionLabel: '添加' },
    })
    await w.find('button').trigger('click')
    expect(w.emitted('action')).toBeTruthy()
  })

  it('uses custom icon path when provided', () => {
    const w = mount(EmptyState, {
      props: { title: 'X', icon: 'M1 2L3 4' },
    })
    expect(w.html()).toContain('d="M1 2L3 4"')
  })
})
