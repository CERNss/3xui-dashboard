import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import {
  AccountMenu,
  EmptyState,
  LocaleSwitcher,
  PageHeader,
  RefreshButton,
  ResponsiveListTable,
  Skeleton,
} from './index'

describe('common components', () => {
  it('renders EmptyState and fires the action callback', async () => {
    const onAction = vi.fn()
    render(<EmptyState title="Nothing here" description="Try again" actionLabel="Retry" onAction={onAction} />)

    expect(screen.getByText('Nothing here')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onAction).toHaveBeenCalledTimes(1)
  })

  it('renders Skeleton variants without crashing', () => {
    const { rerender, container } = render(<Skeleton variant="kpi" />)
    expect(container.querySelector('.ant-skeleton')).toBeInTheDocument()

    rerender(<Skeleton variant="table" rows={2} />)
    expect(container.querySelectorAll('.ant-skeleton-input')).toHaveLength(2)
  })

  it('renders PageHeader title, subtitle, and actions', () => {
    render(<PageHeader title="Plans" subtitle="Manage products" actions={<button type="button">Create</button>} />)

    expect(screen.getByText('Plans')).toBeInTheDocument()
    expect(screen.getByText('Manage products')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Create' })).toBeInTheDocument()
  })

  it('renders RefreshButton with the shared icon button shape', async () => {
    const onClick = vi.fn()
    render(<RefreshButton label="Refresh now" onClick={onClick} />)

    await userEvent.click(screen.getByRole('button', { name: 'Refresh now' }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('renders AccountMenu trigger', () => {
    render(
      <MemoryRouter future={{ v7_relativeSplatPath: true, v7_startTransition: true }}>
        <AccountMenu items={[{ label: 'Profile', to: '/portal/profile' }]}>Open account</AccountMenu>
      </MemoryRouter>,
    )

    expect(screen.getByText('Open account')).toBeInTheDocument()
  })

  it('renders LocaleSwitcher with segmented options', () => {
    render(<LocaleSwitcher />)

    expect(screen.getByText('EN')).toBeInTheDocument()
    expect(screen.getByText('中文')).toBeInTheDocument()
  })

  it('renders ResponsiveListTable root marker and desktop table by default', () => {
    render(
      <ResponsiveListTable
        rowKey="id"
        columns={[{ dataIndex: 'name', title: 'Name' }]}
        dataSource={[{ id: 1, name: 'Node A' }]}
        mobileCard={(record) => <span>{record.name}</span>}
      />,
    )

    expect(document.querySelector('[data-component="responsive-list-table"]')).toBeInTheDocument()
    expect(screen.getByText('Node A')).toBeInTheDocument()
  })
})
