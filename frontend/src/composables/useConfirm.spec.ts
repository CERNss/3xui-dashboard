import { describe, it, expect } from 'vitest'

import { useConfirm } from './useConfirm'

describe('useConfirm', () => {
  it('ask() returns a promise that resolves true on settle(true)', async () => {
    const { state, ask, settle } = useConfirm()
    const p = ask({ title: 'X' })
    expect(state.value).not.toBeNull()
    expect(state.value!.open).toBe(true)
    expect(state.value!.title).toBe('X')
    settle(true)
    await expect(p).resolves.toBe(true)
    // After settle, state clears so the modal hides
    expect(state.value).toBeNull()
  })

  it('settle(false) resolves false', async () => {
    const { ask, settle } = useConfirm()
    const p = ask({ title: 'X' })
    settle(false)
    await expect(p).resolves.toBe(false)
  })

  it('setBusy mutates state.busy', () => {
    const { state, ask, setBusy } = useConfirm()
    ask({ title: 'X' })
    expect(state.value!.busy).toBe(false)
    setBusy(true)
    expect(state.value!.busy).toBe(true)
    setBusy(false)
    expect(state.value!.busy).toBe(false)
  })

  it('multiple sequential asks each get their own resolution', async () => {
    const { ask, settle } = useConfirm()
    const p1 = ask({ title: 'A' })
    settle(true)
    await expect(p1).resolves.toBe(true)

    const p2 = ask({ title: 'B' })
    settle(false)
    await expect(p2).resolves.toBe(false)
  })

  it('forwards all opts into state', () => {
    const { state, ask } = useConfirm()
    ask({
      title: 'T',
      message: 'M',
      variant: 'danger',
      confirmLabel: 'CL',
      cancelLabel: 'XL',
    })
    expect(state.value).toMatchObject({
      title: 'T',
      message: 'M',
      variant: 'danger',
      confirmLabel: 'CL',
      cancelLabel: 'XL',
      open: true,
      busy: false,
    })
  })
})
