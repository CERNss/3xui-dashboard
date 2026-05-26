import { act, fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminAuditApi, type AdminAction } from '@/api/admin/audit'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import AuditLog from './AuditLog'

vi.mock('@/api/admin/audit', () => ({
  adminAuditApi: {
    list: vi.fn(),
  },
}))

const auditListMock = vi.mocked(adminAuditApi.list)

function makeAction(overrides: Partial<AdminAction> = {}): AdminAction {
  return {
    id: 1,
    admin_username: 'root',
    method: 'POST',
    path: '/api/admin/users',
    target_resource: 'user',
    target_id: '42',
    query_string: '',
    ip: '127.0.0.1',
    user_agent: 'vitest',
    status_code: 200,
    error_msg: '',
    created_at: '2026-05-20T10:00:00Z',
    ...overrides,
  }
}

function renderAuditLog() {
  return renderWithProviders(<AuditLog />)
}

beforeEach(() => {
  vi.useRealTimers()
  auditListMock.mockReset()
  auditListMock.mockResolvedValue({
    actions: [
      makeAction({ id: 1, admin_username: 'root', method: 'POST', status_code: 200, created_at: '2026-05-20T10:00:00Z' }),
      makeAction({ id: 2, admin_username: 'alice', method: 'DELETE', status_code: 404, path: '/api/admin/orders/9', target_resource: 'order', target_id: '9', created_at: '2026-05-21T10:00:00Z' }),
      makeAction({ id: 3, admin_username: 'ops', method: 'PATCH', status_code: 500, path: '/api/admin/nodes/1', target_resource: 'node', target_id: '1', error_msg: 'boom', created_at: '2026-05-19T10:00:00Z' }),
    ],
    limit: 100,
    offset: 0,
  })
})

describe('AuditLog', () => {
  it('loads the latest 100 rows and renders tags through ResponsiveListTable', async () => {
    renderAuditLog()

    expect(await screen.findByText('/api/admin/users')).toBeInTheDocument()
    expect(screen.getByText('/api/admin/orders/9')).toBeInTheDocument()
    expect(screen.getByText('DELETE')).toBeInTheDocument()
    expect(screen.getByText('404')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(auditListMock).toHaveBeenCalledWith({ limit: 100 })
  })

  it('debounces username, resource, and method filters by 300ms', async () => {
    renderAuditLog()

    await screen.findByText('/api/admin/users')
    vi.useFakeTimers()
    fireEvent.change(screen.getByLabelText('Filter username'), { target: { value: 'root' } })
    fireEvent.change(screen.getByLabelText('Filter resource'), { target: { value: 'user' } })
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Filter method' }))
    const deleteOptions = screen.getAllByText('DELETE')
    fireEvent.click(deleteOptions[deleteOptions.length - 1])

    expect(auditListMock).toHaveBeenCalledTimes(1)
    act(() => vi.advanceTimersByTime(299))
    expect(auditListMock).toHaveBeenCalledTimes(1)
    act(() => vi.advanceTimersByTime(1))
    vi.useRealTimers()

    await waitFor(() =>
      expect(auditListMock).toHaveBeenLastCalledWith({ limit: 100, username: 'root', resource: 'user', method: 'DELETE' }),
    )
  })

  it('sorts client-side by admin, method, status, and time', async () => {
    renderAuditLog()

    await screen.findByText('/api/admin/users')

    await userEvent.click(screen.getByRole('columnheader', { name: /admin/i }))
    const firstAdminSortedRow = screen.getAllByRole('row')[1]
    expect(firstAdminSortedRow).toHaveTextContent('alice')

    await userEvent.click(screen.getByRole('columnheader', { name: /status/i }))
    const firstStatusSortedRow = screen.getAllByRole('row')[1]
    expect(firstStatusSortedRow).toHaveTextContent('200')

    await userEvent.click(screen.getByRole('columnheader', { name: /time/i }))
    await userEvent.click(screen.getByRole('columnheader', { name: /time/i }))
    const firstTimeSortedRow = screen.getAllByRole('row')[1]
    expect(firstTimeSortedRow).toHaveTextContent('/api/admin/orders/9')
  })

  it('shows the filtered empty state', async () => {
    auditListMock.mockResolvedValue({ actions: [], limit: 100, offset: 0 })
    renderAuditLog()

    expect(await screen.findByText('No audit entries yet.')).toBeInTheDocument()

    await userEvent.type(screen.getByLabelText('Filter username'), 'missing')

    await waitFor(() => expect(screen.getByText('No audit entries match the current filters.')).toBeInTheDocument(), {
      timeout: 1000,
    })
  })
})
