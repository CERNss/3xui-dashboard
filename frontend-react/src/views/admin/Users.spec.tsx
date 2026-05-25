import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from 'antd'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ListUsersResponse, AdminUser } from '@/api/admin/users'
import Users from './Users'

const mocks = vi.hoisted(() => ({
  listUsers: vi.fn(),
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
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <Users />
    </QueryClientProvider>,
  )
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
        oidc_subject: 'oidc|bob',
      }),
    ]),
  )
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
    expect(screen.getByPlaceholderText('Search email, id, or subscription id')).toBeInTheDocument()
    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(await screen.findByRole('row', { name: /alice@example.com/i })).toBeInTheDocument()
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '¥15.00 Adjust' })).toBeInTheDocument()
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
    await user.click(within(screen.getByRole('radiogroup', { name: 'Status filter' })).getByText('Suspended'))
    expect(screen.queryByRole('row', { name: /alice@example.com/i })).not.toBeInTheDocument()
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()

    await user.click(within(screen.getByRole('radiogroup', { name: 'Register method filter' })).getByText('Any method'))
    await user.click(within(screen.getByRole('radiogroup', { name: 'Register method filter' })).getByText('OIDC'))
    expect(screen.getByRole('row', { name: /bob@example.com/i })).toBeInTheDocument()
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

  it('updates email, password, verified state, and balance from the edit dialog', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.click(screen.getByRole('button', { name: 'Edit alice@example.com' }))
    const dialog = screen.getByRole('dialog', { name: 'Edit user #1' })
    await user.clear(within(dialog).getByLabelText('Email'))
    await user.type(within(dialog).getByLabelText('Email'), 'alice2@example.com')
    await user.click(within(dialog).getByLabelText('Email verified'))
    await user.type(within(dialog).getByLabelText('Password'), 'newpass123')
    await user.clear(within(dialog).getByLabelText('Balance'))
    await user.type(within(dialog).getByLabelText('Balance'), '45.67')
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))

    await waitFor(() =>
      expect(mocks.updateMutateAsync).toHaveBeenCalledWith({
        id: 1,
        fields: {
          email: 'alice2@example.com',
          email_verified: false,
          password: 'newpass123',
          balance_cents: 4567,
        },
      }),
    )
  })

  it('toggles auto renew and adjusts balance', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    await user.click(screen.getByRole('switch', { name: 'Enable auto renew for alice@example.com' }))
    await waitFor(() => expect(mocks.updateMutateAsync).toHaveBeenCalledWith({ id: 1, fields: { auto_renew: true } }))

    const aliceRow = screen.getByRole('row', { name: /alice@example.com/i })
    await user.click(within(aliceRow).getByRole('button', { name: 'Adjust' }))
    const dialog = screen.getByRole('dialog', { name: 'Adjust alice@example.com' })
    await user.clear(within(dialog).getByLabelText('Amount'))
    await user.type(within(dialog).getByLabelText('Amount'), '-2.5')
    await user.type(within(dialog).getByLabelText('Reason'), 'manual debit')
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))

    await waitFor(() =>
      expect(mocks.adjustBalanceMutateAsync).toHaveBeenCalledWith({
        id: 1,
        deltaCents: -250,
        reason: 'manual debit',
      }),
    )
  })

  it('enables batch actions after row selection and operates on exactly selected ids', async () => {
    const user = userEvent.setup()
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })

    expect(screen.getByText('Suspend selected').closest('button')).toBeDisabled()
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

  it('uses TanStack Query refetchInterval when auto refresh is enabled', async () => {
    renderUsers()
    await screen.findByRole('row', { name: /alice@example.com/i })
    expect(mocks.listUsers).toHaveBeenCalledTimes(1)

    vi.useFakeTimers()
    fireEvent.click(screen.getByRole('switch', { name: 'Auto refresh' }))
    act(() => {
      vi.advanceTimersByTime(15_000)
    })

    expect(mocks.listUsers).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  })
})
