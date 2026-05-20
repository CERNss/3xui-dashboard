import { ref, type Ref } from 'vue'

export interface ConfirmOpts {
  title: string
  message?: string
  variant?: 'default' | 'danger'
  confirmLabel?: string
  cancelLabel?: string
}

interface ConfirmState extends ConfirmOpts {
  open: boolean
  busy: boolean
}

interface ConfirmAPI {
  state: Ref<ConfirmState | null>
  /** Open the modal and resolve to true (confirm) / false (cancel/Escape). */
  ask: (opts: ConfirmOpts) => Promise<boolean>
  /** Toggle the spinner on the confirm button while async work runs. */
  setBusy: (busy: boolean) => void
  /** Resolve the outstanding ask() and close the modal. */
  settle: (result: boolean) => void
}

/**
 * Replaces native `confirm()` with a styled, a11y-friendly modal.
 *
 * Usage:
 *
 *   const { state: confirmState, ask: askConfirm, settle: settleConfirm } = useConfirm()
 *
 *   async function destroy() {
 *     if (!(await askConfirm({ title: '删除？', variant: 'danger' }))) return
 *     // do work
 *   }
 *
 * Template:
 *
 *   <ConfirmModal
 *     v-if="confirmState"
 *     :open="confirmState.open"
 *     :title="confirmState.title"
 *     :message="confirmState.message"
 *     :variant="confirmState.variant"
 *     :busy="confirmState.busy"
 *     @confirm="settleConfirm(true)"
 *     @cancel="settleConfirm(false)"
 *   />
 *
 * Per-view scope (not global) — each consumer gets its own state, so
 * two views can't race each other's modals.
 */
export function useConfirm(): ConfirmAPI {
  const state = ref<ConfirmState | null>(null)
  let pending: ((v: boolean) => void) | null = null

  function ask(opts: ConfirmOpts): Promise<boolean> {
    return new Promise((resolve) => {
      pending = resolve
      state.value = { ...opts, open: true, busy: false }
    })
  }
  function setBusy(busy: boolean) {
    if (state.value) state.value.busy = busy
  }
  function settle(result: boolean) {
    pending?.(result)
    pending = null
    state.value = null
  }
  return { state, ask, setBusy, settle }
}
