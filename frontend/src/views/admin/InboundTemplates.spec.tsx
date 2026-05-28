import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { InboundTemplate } from '@/api/admin/inboundTemplates'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import InboundTemplates from './InboundTemplates'

const createTemplateMutateAsync = vi.fn()
const updateTemplateMutateAsync = vi.fn()
const removeTemplateMutateAsync = vi.fn()
const templatesRefetch = vi.fn()

let templates: InboundTemplate[] = []
let loading = false

vi.mock('@/hooks/queries/admin/inboundTemplates', () => ({
  useInboundTemplatesList: () => ({
    data: templates,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: templatesRefetch,
  }),
  useCreateInboundTemplate: () => ({ error: null, isPending: false, mutateAsync: createTemplateMutateAsync }),
  useUpdateInboundTemplate: () => ({ error: null, isPending: false, mutateAsync: updateTemplateMutateAsync }),
  useRemoveInboundTemplate: () => ({ error: null, isPending: false, mutateAsync: removeTemplateMutateAsync }),
}))

function renderTemplates() {
  return renderWithProviders(<InboundTemplates />)
}

beforeEach(() => {
  templates = [
    {
      id: 3,
      name: 'Basic VLESS',
      description: 'Default pool template',
      enabled: true,
      protocol: 'vless',
      remark: 'basic-vless',
      listen: '',
      total: 0,
      expiryTime: 0,
      trafficReset: 'never',
      settings: JSON.stringify({ clients: [], decryption: 'none', fallbacks: [] }),
      streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
      sniffing: JSON.stringify({ enabled: true, destOverride: ['http', 'tls'] }),
      created_at: '',
      updated_at: '',
    },
    {
      id: 4,
      name: 'Trojan TLS',
      description: '',
      enabled: false,
      protocol: 'trojan',
      remark: 'tls-template',
      listen: '0.0.0.0',
      total: 1024,
      expiryTime: 1710000000000,
      trafficReset: 'monthly',
      settings: JSON.stringify({ clients: [], fallbacks: [] }),
      streamSettings: JSON.stringify({ network: 'tcp', security: 'tls' }),
      sniffing: JSON.stringify({ enabled: false }),
      created_at: '',
      updated_at: '',
    },
  ]
  loading = false
  createTemplateMutateAsync.mockResolvedValue({})
  updateTemplateMutateAsync.mockResolvedValue({})
  removeTemplateMutateAsync.mockResolvedValue({})
  templatesRefetch.mockReset()
  vi.restoreAllMocks()
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query: string) =>
      ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  )
})

describe('InboundTemplates', () => {
  it('renders templates through the ConfigListPage table shell', () => {
    renderTemplates()

    expect(screen.getByRole('heading', { name: 'Inbound Templates' })).toBeInTheDocument()
    expect(document.querySelector('[data-component="config-list-page"]')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Basic VLESS')).toBeInTheDocument()
    expect(screen.getByText('Default pool template')).toBeInTheDocument()
    expect(screen.getByText('tcp / none')).toBeInTheDocument()
    expect(screen.getByText('1.00 KiB')).toBeInTheDocument()
    expect(screen.getByText('1 of 2 enabled')).toBeInTheDocument()
  })

  it('creates templates with protocol select defaults and clients array JSON', async () => {
    const user = userEvent.setup()
    renderTemplates()

    await user.click(screen.getByRole('button', { name: 'New Template' }))
    const modal = screen.getByRole('dialog', { name: 'New inbound template' })
    expect(within(modal).queryByRole('textbox', { name: 'Protocol' })).not.toBeInTheDocument()

    const settingsBox = within(modal).getByLabelText('settings') as HTMLTextAreaElement
    expect(JSON.parse(settingsBox.value).clients).toEqual([])

    fireEvent.mouseDown(within(modal).getByRole('combobox', { name: 'Protocol' }))
    const trojanOption = (await screen.findAllByTitle('Trojan')).find((item) =>
      item.classList.contains('ant-select-item-option'),
    )
    expect(trojanOption).toBeTruthy()
    await user.click(trojanOption as HTMLElement)
    expect(JSON.parse(settingsBox.value).clients).toEqual([])
    expect(JSON.parse(settingsBox.value).fallbacks).toEqual([])

    await user.type(within(modal).getByLabelText('Name'), 'Edge Trojan')
    await user.type(within(modal).getByLabelText('Remark'), 'edge-trojan')
    await user.clear(within(modal).getByLabelText('Total bytes (0 = unlimited)'))
    await user.type(within(modal).getByLabelText('Total bytes (0 = unlimited)'), '2048')
    await user.click(within(modal).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createTemplateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Edge Trojan',
          enabled: true,
          protocol: 'trojan',
          remark: 'edge-trojan',
          total: 2048,
          expiryTime: 0,
          trafficReset: 'never',
          settings: JSON.stringify({ clients: [], fallbacks: [] }),
          streamSettings: JSON.stringify({ network: 'tcp', security: 'none' }),
        }),
      ),
    )
  })

  it('rejects settings JSON without clients array before creating', async () => {
    const user = userEvent.setup()
    renderTemplates()

    await user.click(screen.getByRole('button', { name: 'New Template' }))
    const modal = screen.getByRole('dialog', { name: 'New inbound template' })

    await user.type(within(modal).getByLabelText('Name'), 'Broken Template')
    await user.clear(within(modal).getByLabelText('settings'))
    fireEvent.change(within(modal).getByLabelText('settings'), { target: { value: '{"fallbacks":[]}' } })
    await user.click(within(modal).getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('settings must include a clients array')).toBeInTheDocument()
    expect(createTemplateMutateAsync).not.toHaveBeenCalled()
  })

  it('edits, toggles, deletes, filters, and refreshes templates', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderTemplates()

    await user.type(screen.getByLabelText('Search templates'), 'trojan')
    expect(screen.getByText('Trojan TLS')).toBeInTheDocument()
    expect(screen.queryByText('Basic VLESS')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(templatesRefetch).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('switch', { name: 'Enable Trojan TLS' }))
    expect(updateTemplateMutateAsync).toHaveBeenCalledWith({ id: 4, input: { enabled: true } })

    await user.click(screen.getByRole('button', { name: 'Edit Trojan TLS' }))
    const modal = screen.getByRole('dialog', { name: 'Edit template #4' })
    expect(within(modal).getByDisplayValue('Trojan TLS')).toBeInTheDocument()
    expect(within(modal).getByDisplayValue('tls-template')).toBeInTheDocument()
    expect(within(modal).getByText('Trojan')).toBeInTheDocument()

    await user.clear(within(modal).getByLabelText('Description'))
    await user.type(within(modal).getByLabelText('Description'), 'TLS edited')
    await user.click(within(modal).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateTemplateMutateAsync).toHaveBeenCalledWith({
        id: 4,
        input: expect.objectContaining({ description: 'TLS edited', protocol: 'trojan' }),
      }),
    )

    await user.click(screen.getByRole('button', { name: 'Delete Trojan TLS' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete inbound template' }))
    await waitFor(() => expect(removeTemplateMutateAsync).toHaveBeenCalledWith(4))
  })
})
