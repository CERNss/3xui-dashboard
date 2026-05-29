import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProvisioningPools from './ProvisioningPools'
import type { InboundTemplate } from '@/api/admin/inboundTemplates'
import type { Node } from '@/api/admin/nodes'
import type { ProvisioningPool } from '@/api/admin/provisioningPools'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const createPoolMutateAsync = vi.fn()
const updatePoolMutateAsync = vi.fn()
const removePoolMutateAsync = vi.fn()
const addTargetMutateAsync = vi.fn()
const updateTargetMutateAsync = vi.fn()
const removeTargetMutateAsync = vi.fn()
const poolsRefetch = vi.fn()
const templatesRefetch = vi.fn()
const nodesRefetch = vi.fn()

let pools: ProvisioningPool[] = []
let templates: InboundTemplate[] = []
let nodes: Node[] = []
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
  useInboundsFleet: () => ({ data: { inbounds: [] }, error: null, isLoading: false, isFetching: false }),
}))

vi.mock('@/hooks/queries/admin/inboundTemplates', () => ({
  useInboundTemplatesList: () => ({
    data: templates,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: templatesRefetch,
  }),
}))

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: () => ({
    data: nodes,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: nodesRefetch,
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
  templates = [
    {
      id: 1,
      name: 'Basic VLESS',
      description: 'Default template',
      enabled: true,
      protocol: 'vless',
      remark: 'basic-vless',
      listen: '',
      total: 0,
      expiryTime: 0,
      trafficReset: 'never',
      settings: JSON.stringify({ clients: [], decryption: 'none' }),
      streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
      sniffing: JSON.stringify({ enabled: true, destOverride: ['http', 'tls'] }),
    },
  ]
  nodes = [
    {
      id: 4,
      name: 'Node A',
      area: 'us',
      province: 'CA',
      scheme: 'https',
      host: 'node-a.example.com',
      port: 2053,
      base_path: '/panel/',
      enabled: true,
      status: 'online',
      cpu_pct: 0,
      mem_pct: 0,
      xray_version: '1.8.24',
      uptime_s: 0,
      created_at: '',
      updated_at: '',
    },
  ]
  loading = false
  createPoolMutateAsync.mockResolvedValue({})
  updatePoolMutateAsync.mockResolvedValue({})
  removePoolMutateAsync.mockResolvedValue({})
  updateTargetMutateAsync.mockResolvedValue({})
  removeTargetMutateAsync.mockResolvedValue({})
  poolsRefetch.mockReset()
  templatesRefetch.mockReset()
  nodesRefetch.mockReset()
  vi.restoreAllMocks()
})

describe('ProvisioningPools', () => {
  it('renders pools and target tables through ResponsiveListTable', () => {
    renderPools()

    expect(screen.getByRole('heading', { name: 'Provisioning Pools' })).toBeInTheDocument()
    expect(screen.getByText('Default Pool')).toBeInTheDocument()
    expect(screen.getByText('Primary pool')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Node A')).toBeInTheDocument()
    expect(screen.getByText('7 / 20')).toBeInTheDocument()
  })

  it('creates template-driven pools and validates port ranges', async () => {
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('button', { name: 'New Pool' }))
    const modal = screen.getByRole('dialog', { name: 'New provisioning pool' })
    await user.type(within(modal).getByLabelText('Name'), 'Fast Pool')
    fireEvent.mouseDown(within(modal).getByRole('combobox', { name: 'Allowed protocols' }))
    await user.click(await screen.findByTitle('VLESS'))
    await user.click(await screen.findByTitle('VMess'))
    await user.click(within(modal).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createPoolMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Fast Pool',
          enabled: true,
          allowed_protocols: ['vless', 'vmess'],
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
    const modal = screen.getByRole('dialog', { name: 'Edit pool #2' })
    expect(within(modal).getByDisplayValue('Default Pool')).toBeInTheDocument()
    expect(within(modal).getByDisplayValue('Primary pool')).toBeInTheDocument()
    expect(within(modal).getByText('VLESS')).toBeInTheDocument()
    expect(within(modal).getByText('VMess')).toBeInTheDocument()

    await user.clear(within(modal).getByLabelText('Name'))
    await user.type(within(modal).getByLabelText('Name'), 'Edited Pool')
    await user.click(within(modal).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updatePoolMutateAsync).toHaveBeenCalledWith({
        id: 2,
        input: expect.objectContaining({ name: 'Edited Pool', allowed_protocols: ['vless', 'vmess'] }),
      }),
    )

    await user.click(screen.getByRole('button', { name: 'Delete Default Pool' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete provisioning pool' }))
    await waitFor(() => expect(removePoolMutateAsync).toHaveBeenCalledWith(2))
  })

  it('does not expose manual target creation as the primary flow', () => {
    renderPools()

    expect(screen.queryByRole('button', { name: 'Add Target' })).not.toBeInTheDocument()
  })

  it('toggles and deletes generated targets, then refreshes pool dependencies', async () => {
    const user = userEvent.setup()
    renderPools()

    await user.click(screen.getByRole('switch', { name: 'Disable target vless-100' }))
    expect(updateTargetMutateAsync).toHaveBeenCalledWith({ targetID: 9, input: { enabled: false } })

    await user.click(screen.getByRole('button', { name: 'Delete target vless-100' }))
    expect(removeTargetMutateAsync).toHaveBeenCalledWith(9)

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(poolsRefetch).toHaveBeenCalledTimes(1)
    expect(nodesRefetch).toHaveBeenCalledTimes(1)
  })
})
