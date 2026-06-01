import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ListUsersResponse, AdminUser } from '@/api/admin/users'
import { renderWithProviders } from '@/test-utils/renderWithProviders'
import Users from './Users'

const mocks = vi.hoisted(() => ({
  listUsers: vi.fn(),
  balanceLogs: vi.fn(),
  createMutateAsync: vi.fn(),
  updateMutateAsync: vi.fn(),
  suspendMutateAsync: vi.fn(),
  unsuspendMutateAsync: vi.fn(),
  removeMutateAsync: vi.fn(),
  adjustBalanceMutateAsync: vi.fn(),
}))

vi.mock('@/api/admin/users', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/users')>()
  return {
    ...actual,
    adminUsersApi: {
      ...actual.adminUsersApi,
      list: mocks.listUsers,
      balanceLogs: mocks.balanceLogs,
    },
  }
})

vi.mock('@/hooks/queries/admin/users', () => ({
  useCreateUser: () => ({ error: null, isPending: false, mutateAsync: mocks.createMutateAsync }),
  useUpdateUser: () => ({ error: null, isPending: false, mutateAsync: mocks.updateMutateAsync }),
  useSuspendUser: () => ({ error: null, isPending: false, mutateAsync: mocks.suspendMutateAsync }),
  useUnsuspendUser: () => ({ error: null, isPending: false, mutateAsync: mocks.unsuspendMutateAsync }),
  useRemoveUser: () => ({ error: null, isPending: false, mutateAsync: mocks.removeMutateAsync }),
  useAdjustUserBalance: () => ({ error: null, isPending: false, mutateAsync: mocks.adjustBalanceMutateAsync }),
}))

function makeUser(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: 1,
    email: 'alice@example.com',
    email_verified: true,
    status: 'active',
    balance_cents: 1500,
    auto_renew: false,
    sub_id: 'sub-abc123def456',
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    last_active_at: null,
    ...overrides,
  }
}

function makeResponse(users: AdminUser[]): ListUsersResponse {
  return { users, limit: 200, offset: 0 }
}

function renderUsers() {
  return renderWithProviders(<Users />)
}

beforeEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  mocks.listUsers.mockResolvedValue(
    makeResponse([
      makeUser(),
      makeUser({
        id: 2,
        email: 'bob@example.com',
        email_verified: false,
        status: 'suspended',
        balance_cents: 0,
        auto_renew: true,
        sub_id: 'sub-bob987654321',
        created_at: '2026-05-02T00:00:00Z',
        oidc_linked: true,
      }),
    ]),
  )
  mocks.balanceLogs.mockResolvedValue({
    logs: [
      {
        id: 1,
        user_id: 1,
        delta_cents: 1000,
        balance_after_cents: 1500,
        reason: 'admin_adjust',
        note: 'manual top-up',
        created_at: '2026-05-03T00:00:00Z',
      },
      {
        id: 2,
        user_id: 1,
        delta_cents: -250,
        balance_after_cents: 1250,
        reason: 'order_charge',
        order_id: 9,
        note: 'Basic plan',
        created_at: '2026-05-04T00:00:00Z',
      },
    ],
    limit: 80,
    offset: 0,
  })
  mocks.createMutateAsync.mockResolvedValue(makeUser({ id: 3, email: 'carol@example.com' }))
  mocks.updateMutateAsync.mockImplementation(({ id, fields }) => Promise.resolve(makeUser({ id, ...fields })))
  mocks.suspendMutateAsync.mockResolvedValue({})
  mocks.unsuspendMutateAsync.mockResolvedValue({})
  mocks.removeMutateAsync.mockResolvedValue({})
  mocks.adjustBalanceMutateAsync.mockResolvedValue({})
})

describe('Users', () => {
  it('renders user controls and the API list through ResponsiveListTable', async () => {
    renderUsers()

    expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New User' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Filters' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Columns' })).toBeInTheDocument()
    expect(screen.queryByRole('switch', { name: 'Auto refresh' })).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('Search email, id, or subscription id')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(await screen.findByRole('row', { name: /alice@example.com/i })).toBeInTheDocument()
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '¥15.00 Top up' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'Suspended' })).toBeInTheDocument()
  })

  it('searches and filters the visible rows by status and register method', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.type(screen.getByPlaceholderText('Search email, id, or subscription id'), 'alice')
    expect(screen.getByRole('row', { name: /alice@example.com/i })).toBeInTheDocument()
    expect(screen.queryByRole('row', { name: /bob@example.com/i })).not.toBeInTheDocument()

    await user.clear(screen.getByPlaceholderText('Search email, id, or subscription id'))
    await user.click(screen.getByRole('button', { name: 'Filters' }))
    await user.click(within(screen.getByRole('radiogroup', { name: 'Status filter' })).getByText('Suspended'))
    expect(screen.queryByRole('row', { name: /alice@example.com/i })).not.toBeInTheDocument()
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()

    await user.click(within(screen.getByRole('radiogroup', { name: 'Register method filter' })).getByText('Any method'))
    await user.click(within(screen.getByRole('radiogroup', { name: 'Register method filter' })).getByText('OIDC'))
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()
  })

  it('toggles optional columns from the toolbar column menu', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    expect(screen.getByRole('columnheader', { name: 'Balance' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Columns' }))
    await user.click(screen.getByRole('checkbox', { name: 'Balance' }))

    expect(screen.queryByRole('columnheader', { name: 'Balance' })).not.toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Actions' })).toBeInTheDocument()
  })

  it('creates users and converts initial balance yuan to cents', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.click(screen.getByRole('button', { name: 'New User' }))
    const dialog = screen.getByRole('dialog', { name: 'New User' })
    await user.type(within(dialog).getByLabelText('Email'), 'carol@example.com')
    await user.type(within(dialog).getByLabelText('Password'), 'testpass1234')
    await user.clear(within(dialog).getByLabelText('Initial balance'))
    await user.type(within(dialog).getByLabelText('Initial balance'), '12.34')
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))

    await waitFor(() =>
      expect(mocks.createMutateAsync).toHaveBeenCalledWith({
        email: 'carol@example.com',
        password: 'testpass1234',
        initial_balance_cents: 1234,
      }),
    )
    expect(await screen.findByText('Created carol@example.com.')).toBeInTheDocument()
  })

  it('updates email, password, and verified state from the edit dialog', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.click(screen.getByRole('button', { name: 'Edit alice@example.com' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit user #1' })
    expect(within(dialog).getByLabelText('Email')).toHaveValue('alice@example.com')
    expect(within(dialog).getByLabelText('Email verified')).toBeChecked()
    await user.clear(within(dialog).getByLabelText('Email'))
    await user.type(within(dialog).getByLabelText('Email'), 'alice2@example.com')
    await user.click(within(dialog).getByLabelText('Email verified'))
    await user.type(within(dialog).getByLabelText('Password'), 'newpass123')
    expect(within(dialog).queryByLabelText('Balance')).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))

    await waitFor(() =>
      expect(mocks.updateMutateAsync).toHaveBeenCalledWith({
        id: 1,
        fields: {
          email: 'alice2@example.com',
          email_verified: false,
          password: 'newpass123',
        },
      }),
    )
  })

  it('opens balance ledger and posts top-ups or refunds', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    const aliceRow = screen.getByRole('row', { name: /alice@example.com/i })
    await user.click(within(aliceRow).getByRole('button', { name: 'Top up' }))
    const dialog = await screen.findByRole('dialog', { name: 'User top-ups and balance changes' })

    expect(within(dialog).getByText('manual top-up')).toBeInTheDocument()
    expect(within(dialog).getByText('Plan purchase charge')).toBeInTheDocument()

    await user.clear(within(dialog).getByLabelText('Top-up amount'))
    await user.type(within(dialog).getByLabelText('Top-up amount'), '2.5')
    await user.type(within(dialog).getByLabelText('Reason'), 'manual credit')
    await user.type(within(dialog).getByLabelText('Note'), 'support request')
    await user.click(within(dialog).getByRole('button', { name: 'Confirm top-up' }))

    await waitFor(() =>
      expect(mocks.adjustBalanceMutateAsync).toHaveBeenCalledWith({
        id: 1,
        deltaCents: 250,
        reason: 'manual credit',
        note: 'support request',
      }),
    )

    await user.click(within(dialog).getByText('Refund'))
    await user.clear(within(dialog).getByLabelText('Refund amount'))
    await user.type(within(dialog).getByLabelText('Refund amount'), '1')
    await user.clear(within(dialog).getByLabelText('Reason'))
    await user.type(within(dialog).getByLabelText('Reason'), 'manual refund')
    await user.click(within(dialog).getByRole('button', { name: 'Confirm refund' }))

    await waitFor(() =>
      expect(mocks.adjustBalanceMutateAsync).toHaveBeenCalledWith({
        id: 1,
        deltaCents: -100,
        reason: 'manual refund',
        note: '',
      }),
    )
  })

  it('enables batch actions after row selection and operates on exactly selected ids', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    expect(screen.queryByText('Suspend selected')).not.toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: 'Select alice@example.com' }))
    await user.click(screen.getByRole('checkbox', { name: 'Select bob@example.com' }))

    expect(screen.getByText('2 selected')).toBeInTheDocument()
    expect(screen.getByText('Suspend selected').closest('button')).toBeEnabled()

    await user.click(screen.getByText('Suspend selected'))
    await waitFor(() => {
      expect(mocks.suspendMutateAsync).toHaveBeenCalledWith(1)
      expect(mocks.suspendMutateAsync).toHaveBeenCalledWith(2)
    })
    expect(mocks.removeMutateAsync).not.toHaveBeenCalled()
  })

  it('confirms single and batch delete via selected ids', async () => {
    const confirmSpy = vi.spyOn(Modal, 'confirm').mockImplementation((config) => {
      act(() => {
        void config.onOk?.()
      })
      return { destroy: vi.fn(), update: vi.fn() }
    })
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.click(screen.getByRole('button', { name: 'Delete alice@example.com' }))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete user' }))
    await waitFor(() => expect(mocks.removeMutateAsync).toHaveBeenCalledWith(1))

    mocks.removeMutateAsync.mockClear()
    await user.click(screen.getByRole('checkbox', { name: 'Select alice@example.com' }))
    await user.click(screen.getByText('Delete selected'))
    expect(confirmSpy).toHaveBeenCalledWith(expect.objectContaining({ title: 'Delete selected users' }))
    await waitFor(() => expect(mocks.removeMutateAsync).toHaveBeenCalledWith(1))
    expect(mocks.removeMutateAsync).not.toHaveBeenCalledWith(2)
  })
})
