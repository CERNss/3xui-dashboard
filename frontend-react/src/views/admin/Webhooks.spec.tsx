import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Webhook, WebhookDelivery } from '@/api/admin/webhooks'
import { adminWebhooksApi } from '@/api/admin/webhooks'
import Webhooks from './Webhooks'

const createMutateAsync = vi.fn()
const updateMutateAsync = vi.fn()
const removeMutateAsync = vi.fn()
const testMutateAsync = vi.fn()
const replayMutateAsync = vi.fn()
const webhooksRefetch = vi.fn()

let webhooks: Webhook[] = []
let deliveries: WebhookDelivery[] = []
let expandedDeliveries: WebhookDelivery[] | undefined

vi.mock('@/hooks/queries/admin/webhooks', () => ({
  useWebhooksList: () => ({
    data: webhooks,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch: webhooksRefetch,
  }),
  useWebhookDeliveries: (id: number, enabled: boolean) => ({
    data: enabled && id === 7 ? expandedDeliveries : undefined,
    error: null,
    isFetching: false,
    isLoading: false,
    refetch: vi.fn().mockResolvedValue({ data: expandedDeliveries }),
  }),
  useCreateWebhook: () => ({ error: null, isPending: false, mutateAsync: createMutateAsync }),
  useUpdateWebhook: () => ({ error: null, isPending: false, mutateAsync: updateMutateAsync }),
  useRemoveWebhook: () => ({ error: null, isPending: false, mutateAsync: removeMutateAsync }),
  useTestWebhook: () => ({ error: null, isPending: false, mutateAsync: testMutateAsync }),
  useReplayWebhookDelivery: () => ({ error: null, isPending: false, mutateAsync: replayMutateAsync }),
}))

vi.mock('@/api/admin/webhooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/webhooks')>()
  return {
    ...actual,
    adminWebhooksApi: {
      ...actual.adminWebhooksApi,
      deliveries: vi.fn(() => Promise.resolve(deliveries)),
    },
  }
})

function renderWebhooks(props?: { embedded?: boolean }) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <Webhooks {...props} />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  webhooks = [
    {
      id: 7,
      name: 'Receiver',
      url: 'https://example.test/hook',
      events: ['user.created', 'order.paid'],
      enabled: true,
      allow_private: false,
      method: 'POST',
      headers: { Authorization: 'Bearer old' },
      body_template: '{"event":"{{event}}"}',
      template_format: 'json',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
  ]
  deliveries = [
    {
      id: 44,
      webhook_id: 7,
      event_type: 'test.fire',
      status: 'success',
      http_status: 200,
      attempt: 1,
      scheduled_at: '2026-01-01T00:00:00Z',
      next_attempt_at: '2026-01-01T00:00:00Z',
      delivered_at: '2026-01-01T00:00:01Z',
    },
  ]
  expandedDeliveries = undefined
  createMutateAsync.mockResolvedValue({})
  updateMutateAsync.mockResolvedValue({})
  removeMutateAsync.mockResolvedValue({})
  testMutateAsync.mockResolvedValue({})
  replayMutateAsync.mockResolvedValue({})
  webhooksRefetch.mockReset()
  vi.mocked(adminWebhooksApi.deliveries).mockClear()
  vi.restoreAllMocks()
})

describe('Webhooks', () => {
  it('renders standalone header and hides PageHeader when embedded', () => {
    const { rerender } = renderWebhooks()
    expect(screen.getByRole('heading', { name: 'Webhooks', level: 2 })).toBeInTheDocument()

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <Webhooks embedded />
      </QueryClientProvider>,
    )
    expect(screen.queryByRole('button', { name: 'Refresh' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Webhooks', level: 4 })).toBeInTheDocument()
  })

  it('creates with parsed headers and comma-separated events', async () => {
    const user = userEvent.setup()
    renderWebhooks()

    await user.click(screen.getByRole('button', { name: 'New Webhook' }))
    await user.type(screen.getByLabelText('Name'), 'PagerDuty')
    await user.type(screen.getByLabelText('URL'), 'https://example.test/pager')
    await user.clear(screen.getByLabelText('Events'))
    await user.type(screen.getByLabelText('Events'), ' user.created, order.paid ')
    await user.type(screen.getByLabelText('Headers'), 'Authorization: Bearer token\nX-Team: Ops')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(createMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          events: ['user.created', 'order.paid'],
          headers: { Authorization: 'Bearer token', 'X-Team': 'Ops' },
          name: 'PagerDuty',
        }),
      ),
    )
  })

  it('edits, lazily expands deliveries, tests, replays, refreshes, and deletes', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderWebhooks()

    await user.click(screen.getByRole('button', { name: 'Edit Receiver' }))
    await user.clear(screen.getByLabelText('Headers'))
    await user.type(screen.getByLabelText('Headers'), 'X-Edited: yes')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateMutateAsync).toHaveBeenCalledWith({
        id: 7,
        patch: expect.objectContaining({ headers: { 'X-Edited': 'yes' } }),
      }),
    )

    expandedDeliveries = deliveries
    await user.click(screen.getByRole('button', { name: 'Deliveries' }))
    expect(await screen.findByText('test.fire')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Test' }))
    await waitFor(() => expect(testMutateAsync).toHaveBeenCalledWith(7))
    await waitFor(() => expect(adminWebhooksApi.deliveries).toHaveBeenCalledWith(7))

    await user.click(screen.getByRole('button', { name: 'Replay' }))
    await waitFor(() => expect(replayMutateAsync).toHaveBeenCalledWith(44))

    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(webhooksRefetch).toHaveBeenCalledTimes(1)

    await user.click(screen.getByRole('button', { name: 'Delete Receiver' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete webhook' }))
    await waitFor(() => expect(removeMutateAsync).toHaveBeenCalledWith(7))
  })
})
