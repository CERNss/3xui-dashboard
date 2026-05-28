import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Plans from './Plans'
import type { AdminPlan } from '@/api/admin/plans'
import type { ProvisioningPool } from '@/api/admin/provisioningPools'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const createMutateAsync = vi.fn()
const updateMutateAsync = vi.fn()
const removeMutateAsync = vi.fn()
const plansRefetch = vi.fn()
const poolsRefetch = vi.fn()

let plans: AdminPlan[] = []
let pools: ProvisioningPool[] = []
let loading = false

vi.mock('@/hooks/queries/admin/plans', () => ({
  usePlansList: () => ({
    data: plans,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: plansRefetch,
  }),
  useCreatePlan: () => ({ error: null, isPending: false, mutateAsync: createMutateAsync }),
  useUpdatePlan: () => ({ error: null, isPending: false, mutateAsync: updateMutateAsync }),
  useRemovePlan: () => ({ error: null, isPending: false, mutateAsync: removeMutateAsync }),
}))

vi.mock('@/hooks/queries/admin/provisioningPools', () => ({
  useProvisioningPoolsList: () => ({
    data: pools,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: poolsRefetch,
  }),
}))

function renderPlans() {
  return renderWithProviders(<Plans />)
}

beforeEach(() => {
  plans = [
    {
      id: 7,
      name: 'Starter',
      description: 'Small team plan',
      duration_days: 30,
      traffic_limit_bytes: 100 * 1024 * 1024 * 1024,
      price_cents: 500,
      ip_limit: 2,
      provisioning_pool_id: 3,
      enabled: true,
    },
  ]
  pools = [
    {
      id: 3,
      name: 'Default Pool',
      description: '',
      enabled: true,
      auto_create: false,
      template_id: null,
      allowed_protocols: ['vless'],
      node_ids: [],
      max_clients: 0,
      targets: [],
    },
  ]
  loading = false
  createMutateAsync.mockResolvedValue({})
  updateMutateAsync.mockResolvedValue({})
  removeMutateAsync.mockResolvedValue({})
  plansRefetch.mockReset()
  poolsRefetch.mockReset()
  vi.restoreAllMocks()
})

describe('Plans', () => {
  it('renders plans through ResponsiveListTable with pool lookup text', () => {
    renderPlans()

    expect(screen.getByRole('heading', { name: 'Plans' })).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Starter')).toBeInTheDocument()
    expect(screen.getByText('Pool: Default Pool')).toBeInTheDocument()
    expect(screen.getByText('¥5.00')).toBeInTheDocument()
  })

  it('creates a plan and converts yuan plus GB to backend fields', async () => {
    const user = userEvent.setup()
    renderPlans()

    await user.click(screen.getByRole('button', { name: 'New Plan' }))
    await user.clear(screen.getByLabelText('Name'))
    await user.type(screen.getByLabelText('Name'), 'Annual')
    await user.clear(screen.getByLabelText('Price'))
    await user.type(screen.getByLabelText('Price'), '12.34')
    await user.clear(screen.getByLabelText('Duration days'))
    await user.type(screen.getByLabelText('Duration days'), '365')
    await user.clear(screen.getByLabelText('Traffic GB'))
    await user.type(screen.getByLabelText('Traffic GB'), '250')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Annual',
          duration_days: 365,
          traffic_limit_bytes: 250 * 1024 * 1024 * 1024,
          price_cents: 1234,
          enabled: true,
        }),
      ),
    )
  })

  it('blocks invalid create form before issuing a mutation', async () => {
    const user = userEvent.setup()
    renderPlans()

    await user.click(screen.getByRole('button', { name: 'New Plan' }))
    await user.clear(screen.getByLabelText('Name'))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Name is required')).toBeInTheDocument()
    expect(createMutateAsync).not.toHaveBeenCalled()
  })

  it('edits, toggles, refreshes, and deletes plans', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderPlans()

    await user.click(screen.getByRole('button', { name: 'Edit Starter' }))
    await user.clear(screen.getByLabelText('Name'))
    await user.type(screen.getByLabelText('Name'), 'Starter Plus')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({
        id: 7,
        input: expect.objectContaining({ name: 'Starter Plus' }),
      }),
    )

    await user.click(screen.getByRole('switch', { name: 'Disable Starter' }))
    expect(updateMutateAsync).toHaveBeenCalledWith({ id: 7, input: { enabled: false } })

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(plansRefetch).toHaveBeenCalledTimes(1)
    expect(poolsRefetch).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'Delete Starter' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete plan' }))
    await waitFor(() => expect(removeMutateAsync).toHaveBeenCalledWith(7))
  })
})
