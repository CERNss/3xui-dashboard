import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'

import Skeleton from './Skeleton.vue'

describe('Skeleton', () => {
  it('renders default 5 table rows when no props', () => {
    const w = mount(Skeleton)
    // 5 shimmer rows in the table variant
    expect(w.html().split('animate-shimmer').length - 1).toBe(5)
  })

  it('honors custom row count', () => {
    const w = mount(Skeleton, { props: { rows: 3 } })
    expect(w.html().split('animate-shimmer').length - 1).toBe(3)
  })

  it('renders kpi variant grid', () => {
    const w = mount(Skeleton, { props: { variant: 'kpi', rows: 4 } })
    expect(w.html()).toContain('grid-cols-1')
    expect(w.html()).toContain('lg:grid-cols-4')
  })

  it('renders card variant', () => {
    const w = mount(Skeleton, { props: { variant: 'card' } })
    // card variant wraps rows in a single panel
    expect(w.html()).toContain('p-6')
  })
})
