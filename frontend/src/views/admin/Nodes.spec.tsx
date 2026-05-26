import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import Nodes from './Nodes'
import type { Node } from '@/api/admin/nodes'
import { renderWithProviders } from '@/test-utils/renderWithProviders'

const createMutateAsync = vi.fn()
const updateMutateAsync = vi.fn()
const removeMutateAsync = vi.fn()
const enableMutateAsync = vi.fn()
const disableMutateAsync = vi.fn()
const probeMutateAsync = vi.fn()
const nodesRefetch = vi.fn()
const useNodesListMock = vi.fn()

let nodes: Node[] = []
let loading = false

vi.mock('@/hooks/queries/admin/nodes', () => ({
  useNodesList: (params?: unknown) => useNodesListMock(params),
  useCreateNode: () => ({ error: null, isPending: false, mutateAsync: createMutateAsync }),
  useUpdateNode: () => ({ error: null, isPending: false, mutateAsync: updateMutateAsync }),
  useRemoveNode: () => ({ error: null, isPending: false, mutateAsync: removeMutateAsync }),
  useEnableNode: () => ({ error: null, isPending: false, mutateAsync: enableMutateAsync }),
  useDisableNode: () => ({ error: null, isPending: false, mutateAsync: disableMutateAsync }),
  useProbeNode: () => ({ error: null, isPending: false, mutateAsync: probeMutateAsync }),
}))

function makeNode(partial: Partial<Node>): Node {
  return {
    id: 1,
    name: 'Tokyo Edge',
    area: 'jp',
    province: 'Tokyo',
    scheme: 'https',
    host: 'tokyo.example.com',
    port: 2053,
    base_path: '/panel/',
    enabled: true,
    status: 'online',
    cpu_pct: 12.3,
    mem_pct: 45.6,
    xray_version: '1.8.24',
    uptime_s: 3600,
    last_seen_at: '2026-05-25T10:00:00Z',
    created_at: '2026-05-20T10:00:00Z',
    updated_at: '2026-05-25T10:00:00Z',
    ...partial,
  }
}

function mockMatchMedia(matches: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query: string) =>
      ({
        matches,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  )
}

function renderNodes() {
  return renderWithProviders(<Nodes />)
}

beforeEach(() => {
  vi.restoreAllMocks()
  nodes = [
    makeNode({ id: 1, name: 'Tokyo Edge', status: 'online', cpu_pct: 12.3, mem_pct: 45.6 }),
    makeNode({
      id: 2,
      name: 'Singapore Edge',
      area: 'sg',
      province: 'Singapore',
      host: 'sg.example.com',
      enabled: false,
      status: 'offline',
      cpu_pct: 2,
      mem_pct: 10,
      xray_version: '',
      last_seen_at: null,
    }),
  ]
  loading = false
  useNodesListMock.mockImplementation((params) => ({
    data: nodes,
    error: null,
    isFetching: false,
    isLoading: loading,
    refetch: nodesRefetch,
    params,
  }))
  createMutateAsync.mockResolvedValue({})
  updateMutateAsync.mockResolvedValue({})
  removeMutateAsync.mockResolvedValue({})
  enableMutateAsync.mockResolvedValue({})
  disableMutateAsync.mockResolvedValue({})
  probeMutateAsync.mockResolvedValue({})
  nodesRefetch.mockReset()
  mockMatchMedia(false)
  URL.createObjectURL = vi.fn(() => 'blob:nodes')
  URL.revokeObjectURL = vi.fn()
})

describe('Nodes', () => {
  it('renders desktop columns through ResponsiveListTable', () => {
    mockMatchMedia(false)
    renderNodes()

    expect(screen.getByRole('heading', { name: 'Nodes' })).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'CPU / Mem' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Xray' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Last seen' })).toBeInTheDocument()
    expect(screen.getByText('Tokyo Edge')).toBeInTheDocument()
    expect(screen.getByText('12.3% / 45.6%')).toBeInTheDocument()
  })

  it('renders status summary counts for online offline and disabled nodes', () => {
    renderNodes()

    expect(screen.getByText('Showing 2 node(s)')).toBeInTheDocument()
    expect(screen.getByText('Online 1')).toBeInTheDocument()
    expect(screen.getByText('Offline 0')).toBeInTheDocument()
    expect(screen.getByText('Disabled 1')).toBeInTheDocument()
  })

  it('renders the same node facts as cards on mobile', () => {
    mockMatchMedia(true)
    renderNodes()

    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByText('Tokyo Edge')).toBeInTheDocument()
    expect(screen.getByText('CPU / Mem: 12.3% / 45.6%')).toBeInTheDocument()
    expect(screen.getByText('Xray: 1.8.24')).toBeInTheDocument()
    expect(screen.getAllByText(/Last seen:/)[0]).toBeInTheDocument()
  })

  it('debounces backend filters before changing query params', () => {
    vi.useFakeTimers()
    try {
      renderNodes()

      fireEvent.change(screen.getByLabelText('Search nodes'), { target: { value: 'tokyo' } })
      expect(useNodesListMock).toHaveBeenLastCalledWith({})

      act(() => vi.advanceTimersByTime(249))
      expect(useNodesListMock).toHaveBeenLastCalledWith({})

      act(() => vi.advanceTimersByTime(1))
      expect(useNodesListMock).toHaveBeenLastCalledWith({ query: 'tokyo' })
    } finally {
      vi.useRealTimers()
    }
  })

  it('sends area province scheme and status filters after debounce', async () => {
    const user = userEvent.setup()
    renderNodes()

    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Filter area' }))
    await user.click(await screen.findByTitle('Singapore'))
    fireEvent.change(screen.getByLabelText('Filter province'), { target: { value: 'Singapore' } })
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Filter scheme' }))
    await user.click(await screen.findByTitle('HTTP'))
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Filter status' }))
    const disabledOptions = await screen.findAllByTitle('Disabled')
    await user.click(disabledOptions[disabledOptions.length - 1])

    await waitFor(() =>
      expect(useNodesListMock).toHaveBeenLastCalledWith({
        area: 'sg',
        province: 'Singapore',
        scheme: 'http',
        status: 'disabled',
      }),
    )
  })

  it('creates a node from quick panel URL and validates port range', async () => {
    const user = userEvent.setup()
    renderNodes()

    await user.click(screen.getByRole('button', { name: 'New Node' }))
    const dialog = screen.getByRole('dialog')
    await user.type(within(dialog).getByLabelText('Quick panel URL'), 'http://new.example.com:8080/root/panel/inbounds')
    await user.type(within(dialog).getByLabelText('Name'), 'New Edge')
    await user.clear(within(dialog).getByLabelText('Port'))
    await user.type(within(dialog).getByLabelText('Port'), '70000')
    await user.type(within(dialog).getByLabelText('API token'), 'secret')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Port must be between 1 and 65535')).toBeInTheDocument()
    expect(createMutateAsync).not.toHaveBeenCalled()

    await user.clear(within(dialog).getByLabelText('Port'))
    await user.type(within(dialog).getByLabelText('Port'), '8080')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'New Edge',
          scheme: 'http',
          host: 'new.example.com',
          port: 8080,
          base_path: '/root/panel/',
          api_token: 'secret',
        }),
      ),
    )
  })

  it('renders the empty state and opens create from the empty action', async () => {
    const user = userEvent.setup()
    nodes = []
    renderNodes()

    expect(screen.getByText('No nodes')).toBeInTheDocument()
    const emptyState = screen.getByText('No nodes').closest('.ant-empty')
    expect(emptyState).toBeInTheDocument()
    await user.click(within(emptyState as HTMLElement).getByRole('button', { name: 'New Node' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('edits, toggles, probes, refreshes, and deletes nodes', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderNodes()

    await user.click(screen.getByRole('button', { name: 'Edit Tokyo Edge' }))
    const dialog = screen.getByRole('dialog')
    await user.clear(within(dialog).getByLabelText('Name'))
    await user.type(within(dialog).getByLabelText('Name'), 'Tokyo Plus')
    await user.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({
        id: 1,
        body: expect.objectContaining({ name: 'Tokyo Plus' }),
      }),
    )

    await user.click(screen.getByRole('switch', { name: 'Disable Tokyo Edge' }))
    expect(disableMutateAsync).toHaveBeenCalledWith(1)

    await user.click(screen.getByRole('button', { name: 'Probe Tokyo Edge' }))
    expect(probeMutateAsync).toHaveBeenCalledWith(1)

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(nodesRefetch).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'Delete Tokyo Edge' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete node' }))
    await waitFor(() => expect(removeMutateAsync).toHaveBeenCalledWith(1))
  })

  it('runs selected batch probe/delete and imports/exports JSON', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    const user = userEvent.setup()
    renderNodes()

    const tokyoRow = screen.getByRole('row', { name: /Tokyo Edge/ })
    await user.click(within(tokyoRow).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: 'Probe batch' }))
    await waitFor(() => expect(probeMutateAsync).toHaveBeenCalledWith(1))
    expect(probeMutateAsync).not.toHaveBeenCalledWith(2)

    await user.click(within(tokyoRow).getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: 'Delete batch' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete selected nodes' }))
    await waitFor(() => expect(removeMutateAsync).toHaveBeenCalledWith(1))

    await user.click(screen.getByRole('button', { name: 'Export' }))
    expect(URL.createObjectURL).toHaveBeenCalled()
    expect(clickSpy).toHaveBeenCalled()

    const file = new File(
      [JSON.stringify({ nodes: [{ name: 'Import Edge', host: 'import.example.com', api_token: 'tok' }] })],
      'nodes.json',
      { type: 'application/json' },
    )
    Object.defineProperty(file, 'text', {
      value: vi.fn().mockResolvedValue(JSON.stringify({ nodes: [{ name: 'Import Edge', host: 'import.example.com', api_token: 'tok' }] })),
    })
    fireEvent.change(screen.getByLabelText('Import nodes file'), { target: { files: [file] } })
    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Import Edge', host: 'import.example.com', api_token: 'tok' }),
      ),
    )
  })

  it('shows import errors for invalid JSON and missing nodes array', async () => {
    renderNodes()

    const invalidFile = new File(['{not-json'], 'invalid.json', { type: 'application/json' })
    Object.defineProperty(invalidFile, 'text', {
      value: vi.fn().mockResolvedValue('{not-json'),
    })
    fireEvent.change(screen.getByLabelText('Import nodes file'), { target: { files: [invalidFile] } })

    expect(await screen.findByText('Import file must be valid JSON.')).toBeInTheDocument()
    expect(createMutateAsync).not.toHaveBeenCalled()

    const missingNodesFile = new File([JSON.stringify({ items: [] })], 'missing-nodes.json', {
      type: 'application/json',
    })
    Object.defineProperty(missingNodesFile, 'text', {
      value: vi.fn().mockResolvedValue(JSON.stringify({ items: [] })),
    })
    fireEvent.change(screen.getByLabelText('Import nodes file'), { target: { files: [missingNodesFile] } })

    expect(await screen.findByText('Import file must contain a nodes array.')).toBeInTheDocument()
    expect(createMutateAsync).not.toHaveBeenCalled()
  })
})
