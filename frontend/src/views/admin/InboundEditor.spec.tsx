import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import InboundEditor from './InboundEditor'
import type { Inbound } from '@/api/admin/inbounds'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const createMutateAsync = vi.fn()
const updateMutateAsync = vi.fn()

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useCreateInbound: () => ({ error: null, isPending: false, mutateAsync: createMutateAsync }),
  useUpdateInbound: () => ({ error: null, isPending: false, mutateAsync: updateMutateAsync }),
}))

const inboundTemplatesData = [
  { id: 11, name: 'VLESS TCP plain', enabled: true, protocol: 'vless' },
  { id: 12, name: 'Trojan WS TLS', enabled: true, protocol: 'trojan' },
  { id: 13, name: 'Disabled template', enabled: false, protocol: 'vless' },
]
vi.mock('@/hooks/queries/admin/inboundTemplates', () => ({
  useInboundTemplatesList: () => ({
    data: inboundTemplatesData,
    isLoading: false,
    isFetching: false,
    error: null,
  }),
}))

function renderEditor(
  source?: Inbound | null,
  options: {
    nodeID?: number | null
    nodes?: Array<{ id: number; name: string; enabled: boolean }>
    onClose?: () => void
  } = {},
) {
  return renderWithProviders(
    <InboundEditor
      open
      mode={source ? 'edit' : 'create'}
      nodeID={options.nodeID ?? 1}
      tag={source?.tag ?? ''}
      source={source}
      nodes={options.nodes ?? [{ id: 1, name: 'Tokyo Node', enabled: true }]}
      onClose={options.onClose ?? vi.fn()}
    />,
  )
}

function makeInbound(overrides: Partial<Inbound> = {}): Inbound {
  return {
    id: 10,
    up: 0,
    down: 0,
    total: 0,
    allTime: 0,
    remark: 'Main inbound',
    enable: true,
    expiryTime: 0,
    trafficReset: 'never',
    clientStats: [],
    listen: '',
    port: 443,
    protocol: 'vless',
    settings: JSON.stringify({ clients: [{ id: 'uuid-1', flow: 'xtls-rprx-vision', email: 'alice@example.com', expiryTime: 0, enable: true }], decryption: 'none' }),
    streamSettings: JSON.stringify({ network: 'ws', security: 'tls', wsSettings: { path: '/ws' }, tlsSettings: { serverName: 'edge.example.com' } }),
    sniffing: JSON.stringify({ enabled: true, destOverride: ['http', 'tls'] }),
    tag: 'inbound-443',
    ...overrides,
  }
}

beforeEach(() => {
  createMutateAsync.mockClear()
  updateMutateAsync.mockClear()
  createMutateAsync.mockResolvedValue(makeInbound())
  updateMutateAsync.mockResolvedValue(makeInbound())
})

describe('InboundEditor', () => {
  it('hydrates full inbound payload and saves through update mutation', async () => {
    const user = userEvent.setup()
    renderEditor(makeInbound())

    expect(screen.getByRole('dialog', { name: 'Edit inbound inbound-443' })).toBeInTheDocument()
    expect(screen.getByLabelText('Inbound name')).toHaveValue('Main inbound')
    expect(screen.getByLabelText('Port')).toHaveValue('443')
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByLabelText('Decryption')).toHaveValue('none')
    expect(screen.getByLabelText('Encryption')).toHaveValue('none')
    // Clients are not editable inside the inbound editor — they're
    // auto-created at customer purchase time and browsable from the
    // Inbounds page row expansion. The existing settings.clients[]
    // values are preserved through the form round-trip though.

    await user.clear(screen.getByLabelText('Inbound name'))
    await user.type(screen.getByLabelText('Inbound name'), 'Updated inbound')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({
        nodeID: 1,
        tag: 'inbound-443',
        body: expect.objectContaining({
          remark: 'Updated inbound',
          port: 443,
          protocol: 'vless',
        }),
      }),
    )
    const body = updateMutateAsync.mock.calls[0][0].body
    expect(JSON.parse(body.settings).clients[0].email).toBe('alice@example.com')
    expect(JSON.parse(body.streamSettings).wsSettings.path).toBe('/ws')
  })

  it('switches protocols and exposes key fields for all protocol components', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByLabelText('Decryption')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'vmess', settings: JSON.stringify({ clients: [], disableInsecureEncryption: true }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByLabelText('Disable insecure encryption')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'trojan', settings: JSON.stringify({ clients: [{ password: 'secret', email: 'trojan@example.com' }] }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    // Trojan client list hidden; the "Configure fallbacks" button is
    // a stable anchor that the protocol tab actually renders.
    expect(screen.getByRole('button', { name: /Configure fallbacks/i })).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'shadowsocks', settings: JSON.stringify({ clients: [], method: '2022-blake3-aes-256-gcm', password: 'global-password' }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByLabelText('Method')).toBeInTheDocument()
    expect(screen.getByLabelText('Global password')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'hysteria', settings: JSON.stringify({ auth: 'auth-token', obfs: 'obfs-token' }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByLabelText('Obfuscation')).toBeInTheDocument()
    expect(screen.getByLabelText('Auth string')).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Stream' })).not.toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'wireguard', settings: JSON.stringify({ peers: [], secretKey: 'wg-secret' }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    // WG peer list hidden; the Secret key Input remains as a stable anchor.
    expect(screen.getByLabelText('Secret key')).toBeInTheDocument()
  })

  it('creates with stream and sniffing fields in the payload', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.type(screen.getByLabelText('Inbound name'), 'Create inbound')
    await user.clear(screen.getByLabelText('Port'))
    await user.type(screen.getByLabelText('Port'), '8443')
    await user.click(screen.getByRole('tab', { name: 'Stream' }))
    await user.click(screen.getByLabelText('Transmission'))
    await user.click(await screen.findByText('WebSocket'))
    await user.clear(screen.getByLabelText('Path'))
    await user.type(screen.getByLabelText('Path'), '/socket')
    await user.click(screen.getByRole('tab', { name: 'Sniffing' }))
    expect(within(screen.getByRole('tabpanel')).getByText('http')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalled())
    const payload = createMutateAsync.mock.calls[0][0]
    expect(payload.nodeID).toBe(1)
    expect(payload.body).toEqual(expect.objectContaining({ remark: 'Create inbound', port: 8443 }))
    expect(JSON.parse(payload.body.streamSettings).wsSettings.path).toBe('/socket')
    expect(JSON.parse(payload.body.sniffing).destOverride).toContain('http')
  })

  it('validates required remark and port before creating', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.clear(screen.getByLabelText('Port'))
    await user.click(screen.getByRole('button', { name: 'Create' }))

    expect(await screen.findByText('Inbound name is required')).toBeInTheDocument()
    expect(await screen.findByText('Port is required')).toBeInTheDocument()
    expect(createMutateAsync).not.toHaveBeenCalled()
  })

  it('hides Stream and Sniffing tabs for WireGuard and Hysteria with protocol info', () => {
    renderEditor(makeInbound({ protocol: 'wireguard', settings: JSON.stringify({ peers: [], secretKey: 'wg-secret' }) }))

    expect(screen.getByText('WireGuard hides Stream and Sniffing because those settings do not apply.')).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Stream' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Sniffing' })).not.toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'hysteria', settings: JSON.stringify({ auth: 'auth-token', obfs: 'obfs-token' }) }))

    expect(screen.getByText('Hysteria uses mandatory TLS and fixed hysteria stream settings.')).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Stream' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Sniffing' })).not.toBeInTheDocument()
  })

  it('submits raw Advanced JSON overrides when enabled', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.type(screen.getByLabelText('Inbound name'), 'Advanced inbound')
    await user.click(screen.getByRole('tab', { name: 'Advanced' }))
    // Advanced tab defaults to a read-only assembled JSON preview; the
    // raw-override checkboxes only appear after flipping Edit mode.
    await user.click(within(screen.getByRole('tabpanel', { name: 'Advanced' })).getByRole('switch'))
    await user.click(screen.getByLabelText('Override settings JSON'))
    await user.click(screen.getByLabelText('Override streamSettings JSON'))
    await user.click(screen.getByLabelText('Override sniffing JSON'))

    const rawSettings = { raw: true, clients: [{ email: 'raw@example.com' }] }
    const rawStream = { network: 'grpc', security: 'tls', grpcSettings: { serviceName: 'raw-grpc' } }
    const rawSniffing = { enabled: false, destOverride: ['quic'], metadataOnly: true, routeOnly: false }
    fireEvent.change(screen.getByLabelText('settings'), { target: { value: JSON.stringify(rawSettings) } })
    fireEvent.change(screen.getByLabelText('streamSettings'), { target: { value: JSON.stringify(rawStream) } })
    fireEvent.change(screen.getByLabelText('sniffing'), { target: { value: JSON.stringify(rawSniffing) } })

    await user.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalled())
    const body = createMutateAsync.mock.calls[0][0].body
    expect(JSON.parse(body.settings)).toEqual(rawSettings)
    expect(JSON.parse(body.streamSettings)).toEqual(rawStream)
    expect(JSON.parse(body.sniffing)).toEqual(rawSniffing)
  })

  it('marks disabled node options as disabled', async () => {
    const user = userEvent.setup()
    renderEditor(null, {
      nodes: [
        { id: 1, name: 'Tokyo Node', enabled: true },
        { id: 2, name: 'Osaka Node', enabled: false },
      ],
    })

    await user.click(screen.getByLabelText('Node'))

    const disabledOption = await screen.findByText('Osaka Node (disabled)')
    expect(disabledOption.closest('.ant-select-item-option')).toHaveClass('ant-select-item-option-disabled')
  })

  it('exposes only enabled templates in the "from template" select and passes template_id on create', async () => {
    const user = userEvent.setup()
    renderEditor()

    fireEvent.mouseDown(screen.getByRole('combobox', { name: /Fill from template/ }))
    // Enabled templates appear.
    expect(await screen.findByText('VLESS TCP plain · vless')).toBeInTheDocument()
    expect(screen.getByText('Trojan WS TLS · trojan')).toBeInTheDocument()
    // Disabled one is filtered out.
    expect(screen.queryByText('Disabled template · vless')).not.toBeInTheDocument()

    await user.click(screen.getByText('VLESS TCP plain · vless'))
    await user.type(screen.getByLabelText('Inbound name'), 'tpl-inbound')
    await user.clear(screen.getByLabelText('Port'))
    await user.type(screen.getByLabelText('Port'), '18081')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalled())
    const payload = createMutateAsync.mock.calls[0][0]
    expect(payload.nodeID).toBe(1)
    expect(payload.body).toEqual(
      expect.objectContaining({
        template_id: 11,
        port: 18081,
        remark: 'tpl-inbound',
      }),
    )
  })

  it('omits template_id when no template is chosen', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.type(screen.getByLabelText('Inbound name'), 'plain-inbound')
    await user.clear(screen.getByLabelText('Port'))
    await user.type(screen.getByLabelText('Port'), '18082')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(createMutateAsync).toHaveBeenCalled())
    const body = createMutateAsync.mock.calls[0][0].body
    expect(body).not.toHaveProperty('template_id')
  })

  it('does not render the "from template" select in edit mode', () => {
    renderEditor(makeInbound())

    expect(screen.queryByRole('combobox', { name: /Fill from template/ })).not.toBeInTheDocument()
  })

  it('closes without running a create mutation', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    renderEditor(null, { onClose })

    const closeButton = screen.getAllByRole('button', { name: 'Close' }).find((button) => button.textContent === 'Close')
    expect(closeButton).toBeDefined()
    await user.click(closeButton!)

    expect(onClose).toHaveBeenCalledTimes(1)
    expect(createMutateAsync).not.toHaveBeenCalled()
    expect(updateMutateAsync).not.toHaveBeenCalled()
  })
})
