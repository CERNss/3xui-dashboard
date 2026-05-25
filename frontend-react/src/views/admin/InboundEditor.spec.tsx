import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import InboundEditor from './InboundEditor'
import type { Inbound } from '@/api/admin/inbounds'

const createMutateAsync = vi.fn()
const updateMutateAsync = vi.fn()

vi.mock('@/hooks/queries/admin/inbounds', () => ({
  useCreateInbound: () => ({ error: null, isPending: false, mutateAsync: createMutateAsync }),
  useUpdateInbound: () => ({ error: null, isPending: false, mutateAsync: updateMutateAsync }),
}))

function renderEditor(source?: Inbound | null) {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <InboundEditor
        open
        mode={source ? 'edit' : 'create'}
        nodeID={1}
        tag={source?.tag ?? ''}
        source={source}
        nodes={[{ id: 1, name: 'Tokyo Node', enabled: true }]}
        onClose={vi.fn()}
      />
    </QueryClientProvider>,
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
  createMutateAsync.mockResolvedValue(makeInbound())
  updateMutateAsync.mockResolvedValue(makeInbound())
})

describe('InboundEditor', () => {
  it('hydrates full inbound payload and saves through update mutation', async () => {
    const user = userEvent.setup()
    renderEditor(makeInbound())

    expect(screen.getByRole('dialog', { name: 'Edit inbound inbound-443' })).toBeInTheDocument()
    expect(screen.getByLabelText('Remark')).toHaveValue('Main inbound')
    expect(screen.getByLabelText('Port')).toHaveValue('443')
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByText('VLESS clients')).toBeInTheDocument()
    expect(screen.getByTitle('none')).toBeInTheDocument()
    expect(screen.getByDisplayValue('alice@example.com')).toBeInTheDocument()

    await user.clear(screen.getByLabelText('Remark'))
    await user.type(screen.getByLabelText('Remark'), 'Updated inbound')
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
    expect(screen.getByText('VLESS clients')).toBeInTheDocument()
    expect(screen.getByLabelText('Decryption')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'vmess', settings: JSON.stringify({ clients: [], disableInsecureEncryption: true }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByText('VMess clients')).toBeInTheDocument()
    expect(screen.getByLabelText('Disable insecure encryption')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'trojan', settings: JSON.stringify({ clients: [{ password: 'secret', email: 'trojan@example.com' }] }) }))
    await user.click(screen.getByRole('tab', { name: 'Protocol' }))
    expect(screen.getByText('Trojan passwords')).toBeInTheDocument()

    cleanup()
    renderEditor(makeInbound({ protocol: 'shadowsocks', settings: JSON.stringify({ clients: [], method: 'aes-256-gcm', password: 'global-password' }) }))
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
    expect(screen.getByText('WireGuard peers')).toBeInTheDocument()
    expect(screen.getByLabelText('Secret key')).toBeInTheDocument()
  })

  it('creates with stream and sniffing fields in the payload', async () => {
    const user = userEvent.setup()
    renderEditor()

    await user.type(screen.getByLabelText('Remark'), 'Create inbound')
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
})
