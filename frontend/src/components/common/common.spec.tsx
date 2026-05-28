import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import {
  AccountMenu,
  ConfigListPage,
  EmptyState,
  LocaleSwitcher,
  PageHeader,
  RefreshButton,
  ResponsiveListTable,
  Skeleton,
} from './index'

const changeLanguageMock = vi.hoisted(() => vi.fn())

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    i18n: {
      changeLanguage: changeLanguageMock,
    },
    t: (key: string) =>
      ({
        'language.chinese': 'Chinese',
        'language.english': 'English',
        'language.label': 'Language',
      })[key] ?? key,
  }),
}))

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

  it('renders LocaleSwitcher trigger', () => {
    render(<LocaleSwitcher />)

    expect(screen.getByText('EN')).toBeInTheDocument()
    expect(screen.getByText('中')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: /Language/i })).toBeInTheDocument()
  })

  it('changes i18n language when LocaleSwitcher changes', async () => {
    changeLanguageMock.mockResolvedValue(undefined)
    render(<LocaleSwitcher />)

    await userEvent.click(screen.getByRole('switch', { name: /Language/i }))

    expect(changeLanguageMock).toHaveBeenCalledWith('zh')
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

  it('renders ConfigListPage shell with filters, stats, table, footer, and empty state', () => {
    const { rerender } = render(
      <ConfigListPage
        title="Nodes"
        subtitle="Manage nodes"
        actions={<button type="button">New</button>}
        filters={<label htmlFor="search">Search</label>}
        stats={<span>2 online</span>}
        footer={<span>Showing 2</span>}
        rowKey="id"
        columns={[{ dataIndex: 'name', title: 'Name' }]}
        dataSource={[{ id: 1, name: 'Node A' }]}
        emptyState={{ title: 'No nodes', description: 'Create one' }}
        mobileCard={(record) => <span>{record.name}</span>}
      />,
    )

    expect(document.querySelector('[data-component="config-list-page"]')).toBeInTheDocument()
    expect(screen.getByText('Nodes')).toBeInTheDocument()
    expect(screen.getByText('Search')).toBeInTheDocument()
    expect(screen.getByText('2 online')).toBeInTheDocument()
    expect(screen.getByText('Node A')).toBeInTheDocument()
    expect(screen.getByText('Showing 2')).toBeInTheDocument()

    rerender(
      <ConfigListPage
        title="Nodes"
        rowKey="id"
        columns={[{ dataIndex: 'name', title: 'Name' }]}
        dataSource={[]}
        emptyState={{ title: 'No nodes', description: 'Create one' }}
        mobileCard={(record: { id: number; name: string }) => <span>{record.name}</span>}
      />,
    )

    expect(screen.getByText('No nodes')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('keeps table pagination controls in the ConfigListPage footer', () => {
    render(
      <ConfigListPage
        title="Nodes"
        rowKey="id"
        columns={[{ dataIndex: 'name', title: 'Name' }]}
        dataSource={[
          { id: 1, name: 'Node A' },
          { id: 2, name: 'Node B' },
        ]}
        pagination={{ pageSize: 1 }}
        mobileCard={(record) => <span>{record.name}</span>}
      />,
    )

    const list = document.querySelector('.config-list-page-list')
    const footer = document.querySelector('.config-list-page-footer')

    expect(list?.querySelector('.ant-table-pagination')).not.toBeInTheDocument()
    expect(footer?.querySelector('.ant-pagination')).toBeInTheDocument()
    expect(screen.getByText('Node A')).toBeInTheDocument()
    expect(screen.queryByText('Node B')).not.toBeInTheDocument()
  })
})
