import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProvisioningPools from './ProvisioningPools'
import type { FleetResult } from '@/api/admin/inbounds'
import type { ProvisioningPool } from '@/api/admin/provisioningPools'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const createPoolMutateAsync = vi.fn()
const updatePoolMutateAsync = vi.fn()
const removePoolMutateAsync = vi.fn()
const addTargetMutateAsync = vi.fn()
const updateTargetMutateAsync = vi.fn()
const removeTargetMutateAsync = vi.fn()
const poolsRefetch = vi.fn()
const fleetRefetch = vi.fn()

let pools: ProvisioningPool[] = []
let fleet: FleetResult = { inbounds: [] }
let loading = false

vi.mock('@/hooks/queries/admin/provisioningPools', () => ({
  useProvisioningPoolsList: () => ({
    data: pools,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: poolsRefetch,
  }),
  useCreateProvisioningPool: () => ({ error: null, isPending: false, mutateAsync: createPoolMutateAsync }),
  useUpdateProvisioningPool: () => ({ error: null, isPending: false, mutateAsync: updatePoolMutateAsync }),
  useRemoveProvisioningPool: () => ({ error: null, isPending: false, mutateAsync: removePoolMutateAsync }),
  useAddProvisioningPoolTarget: () => ({ error: null, isPending: false, mutateAsync: addTargetMutateAsync }),
  useUpdateProvisioningPoolTarget: () => ({ error: null, isPending: false, mutateAsync: updateTargetMutateAsync }),
  useRemoveProvisioningPoolTarget: () => ({ error: null, isPending: false, mutateAsync: removeTargetMutateAsync }),
}))

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useInboundsFleet: () => ({
    data: fleet,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: fleetRefetch,
  }),
}))

function renderPools() {
  return renderWithProviders(<ProvisioningPools />)
}

beforeEach(() => {
  pools = [
    {
      id: 2,
      name: 'Default Pool',
      description: 'Primary pool',
      enabled: true,
      auto_create: false,
      port_min: 10000,
      port_max: 20000,
      allowed_protocols: ['vless', 'vmess'],
      targets: [
        {
          id: 9,
          pool_id: 2,
          node_id: 4,
          node_name: 'Node A',
          inbound_tag: 'vless-100',
          protocol: 'vless',
          max_clients: 20,
          used_clients: 7,
          priority: 50,
          enabled: true,
        },
      ],
    },
  ]
  fleet = {
    inbounds: [
      {
        node_id: 4,
        node_name: 'Node A',
        inbound: {
          id: 1,
          up: 0,
          down: 0,
          total: 0,
          allTime: 0,
          remark: 'Public vless',
          enable: true,
          expiryTime: 0,
          trafficReset: '',
          clientStats: [],
          listen: '',
          port: 443,
          protocol: 'vless',
          settings: '{}',
          streamSettings: '{}',
          tag: 'vless-100',
          sniffing: '{}',
        },
      },
      {
        node_id: 5,
        node_name: 'Node B',
        inbound: {
          id: 2,
          up: 0,
          down: 0,
          total: 0,
          allTime: 0,
          remark: 'Disabled vmess',
          enable: false,
          expiryTime: 0,
          trafficReset: '',
          clientStats: [],
          listen: '',
          port: 10086,
          protocol: 'vmess',
          settings: '{}',
          streamSettings: '{}',
          tag: 'vmess-disabled',
          sniffing: '{}',
        },
      },
    ],
  }
  loading = false
  createPoolMutateAsync.mockResolvedValue({})
  updatePoolMutateAsync.mockResolvedValue({})
  removePoolMutateAsync.mockResolvedValue({})
  addTargetMutateAsync.mockResolvedValue({})
  updateTargetMutateAsync.mockResolvedValue({})
  removeTargetMutateAsync.mockResolvedValue({})
  poolsRefetch.mockReset()
  fleetRefetch.mockReset()
  vi.restoreAllMocks()
})

describe('ProvisioningPools', () => {
  it('renders pools and target tables through ResponsiveListTable', () => {
    renderPools()

    expect(screen.getByRole('heading', { name: 'Provisioning Pools' })).toBeInTheDocument()
    expect(screen.getByText('Default Pool')).toBeInTheDocument()
    expect(screen.getByText('Primary pool')).toBeInTheDocument()
    expect(screen.getByText('Ports: 10000-20000')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Node A')).toBeInTheDocument()
    expect(screen.getByText('7 / 20')).toBeInTheDocument()
  })

  it('creates pools with comma protocol parsing and validates port ranges', async () => {
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('button', { name: 'New Pool' }))
    await user.type(screen.getByLabelText('Name'), 'Fast Pool')
    await user.type(screen.getByLabelText('Port min'), '30000')
    await user.type(screen.getByLabelText('Port max'), '20000')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.getAllByText('Port minimum must not exceed maximum')).toHaveLength(2))
    expect(createPoolMutateAsync).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('Port max'))
    await user.type(screen.getByLabelText('Port max'), '40000')
    await user.type(screen.getByLabelText('Allowed protocols'), ' VLESS, vmess, , Trojan ')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createPoolMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Fast Pool',
          port_min: 30000,
          port_max: 40000,
          allowed_protocols: ['vless', 'vmess', 'trojan'],
        }),
      ),
    )
  })

  it('edits and deletes pools', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('button', { name: 'Edit Default Pool' }))
    await user.clear(screen.getByLabelText('Name'))
    await user.type(screen.getByLabelText('Name'), 'Edited Pool')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updatePoolMutateAsync).toHaveBeenCalledWith({
        id: 2,
        input: expect.objectContaining({ name: 'Edited Pool' }),
      }),
    )

    await user.click(screen.getByRole('button', { name: 'Delete Default Pool' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete provisioning pool' }))
    await waitFor(() => expect(removePoolMutateAsync).toHaveBeenCalledWith(2))
  })

  it('adds targets from enabled fleet inbound only', async () => {
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('button', { name: 'Add Target' }))
    const modal = screen.getByRole('dialog', { name: 'Add Target' })
    expect(within(modal).getByText('Node A · Public vless · :443 · vless')).toBeInTheDocument()
    expect(screen.queryByText('Disabled vmess')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText('Max clients'))
    await user.type(screen.getByLabelText('Max clients'), '30')
    await user.clear(screen.getByLabelText('Priority'))
    await user.type(screen.getByLabelText('Priority'), '10')
    await user.click(within(modal).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(addTargetMutateAsync).toHaveBeenCalledWith({
        poolID: 2,
        input: {
          node_id: 4,
          inbound_tag: 'vless-100',
          protocol: 'vless',
          max_clients: 30,
          priority: 10,
          enabled: true,
        },
      }),
    )
  })

  it('toggles and deletes targets, then refreshes pools plus fleet', async () => {
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('switch', { name: 'Disable target vless-100' }))
    expect(updateTargetMutateAsync).toHaveBeenCalledWith({ targetID: 9, input: { enabled: false } })

    await user.click(screen.getByRole('button', { name: 'Delete target vless-100' }))
    expect(removeTargetMutateAsync).toHaveBeenCalledWith(9)

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(poolsRefetch).toHaveBeenCalledTimes(1)
    expect(fleetRefetch).toHaveBeenCalledTimes(1)
  })
})
