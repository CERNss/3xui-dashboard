import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Dropdown,
  Form,
  Input,
  InputNumber,
  Modal,
  Segmented,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { MenuProps } from 'antd'
import type { CheckboxProps } from 'antd/es/checkbox'
import type { Key } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { adminUsersApi, type AdminUser, type UserStatus } from '@/api/admin/users'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import {
  useAdjustUserBalance,
  useCreateUser,
  useRemoveUser,
  useSuspendUser,
  useUnsuspendUser,
  useUpdateUser,
} from '@/hooks/queries/admin/users'
import { useQueryErrorReporter } from '@/hooks/queries/error'
import { queryKeys } from '@/hooks/queries/keys'

type StatusFilter = 'all' | UserStatus
type VerifiedFilter = 'all' | 'verified' | 'unverified'
type RegisterFilter = 'all' | 'email' | 'oidc'
type SortKey = 'created_at:desc' | 'created_at:asc' | 'balance:desc' | 'balance:asc' | 'id:desc' | 'email:asc' | 'email:desc'

interface CreateFormValues {
  email: string
  password: string
  initial_balance_yuan?: number
}

interface EditFormValues {
  email: string
  email_verified: boolean
  password?: string
  balance_yuan: number
}

interface BalanceFormValues {
  delta_yuan: number
  reason: string
}

const AUTO_REFRESH_MS = 15_000
const PAGE_SIZE = 20
const userKeys = queryKeys('admin', 'users')
const listParams = { limit: 200 }

function formatYuan(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}

function userLabel(user: AdminUser) {
  return user.email || `#${user.id}`
}

function statusTag(status: UserStatus) {
  return <Tag color={status === 'active' ? 'green' : 'red'}>{status === 'active' ? 'Active' : 'Suspended'}</Tag>
}

function verifiedTag(user: AdminUser) {
  if (!user.email) return null
  return <Tag color={user.email_verified ? 'green' : 'gold'}>{user.email_verified ? 'Verified' : 'Unverified'}</Tag>
}

function sortUsers(users: AdminUser[], sort: SortKey) {
  const out = [...users]
  switch (sort) {
    case 'created_at:asc':
      return out.sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id - b.id)
    case 'created_at:desc':
      return out.sort((a, b) => b.created_at.localeCompare(a.created_at) || b.id - a.id)
    case 'balance:asc':
      return out.sort((a, b) => a.balance_cents - b.balance_cents || a.id - b.id)
    case 'balance:desc':
      return out.sort((a, b) => b.balance_cents - a.balance_cents || b.id - a.id)
    case 'id:desc':
      return out.sort((a, b) => b.id - a.id)
    case 'email:asc':
      return out.sort((a, b) => (a.email || '').localeCompare(b.email || '') || a.id - b.id)
    case 'email:desc':
      return out.sort((a, b) => (b.email || '').localeCompare(a.email || '') || b.id - a.id)
  }
}

function flashText(action: string, ok: number, fail: number) {
  return fail === 0 ? `${action} completed for ${ok} users.` : `${action} completed for ${ok} users; ${fail} failed.`
}

export default function Users() {
  const [createForm] = Form.useForm<CreateFormValues>()
  const [editForm] = Form.useForm<EditFormValues>()
  const [balanceForm] = Form.useForm<BalanceFormValues>()
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [verifiedFilter, setVerifiedFilter] = useState<VerifiedFilter>('all')
  const [registerFilter, setRegisterFilter] = useState<RegisterFilter>('all')
  const [sort, setSort] = useState<SortKey>('created_at:desc')
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([])
  const [autoRefresh, setAutoRefresh] = useState(false)
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<AdminUser | null>(null)
  const [balanceUser, setBalanceUser] = useState<AdminUser | null>(null)

  const usersQuery = useQuery({
    queryKey: userKeys.list(listParams),
    queryFn: () => adminUsersApi.list(listParams),
    refetchInterval: autoRefresh ? AUTO_REFRESH_MS : false,
    refetchOnWindowFocus: true,
  })
  useQueryErrorReporter(usersQuery.error, usersQuery.isError)

  const createUser = useCreateUser()
  const updateUser = useUpdateUser()
  const suspendUser = useSuspendUser()
  const unsuspendUser = useUnsuspendUser()
  const removeUser = useRemoveUser()
  const adjustBalance = useAdjustUserBalance()

  const users = useMemo(() => usersQuery.data?.users ?? [], [usersQuery.data])
  const loading = usersQuery.isLoading
  const error =
    usersQuery.error ??
    createUser.error ??
    updateUser.error ??
    suspendUser.error ??
    unsuspendUser.error ??
    removeUser.error ??
    adjustBalance.error

  useEffect(() => {
    if (!flash) return undefined
    const timer = window.setTimeout(() => setFlash(null), 2400)
    return () => window.clearTimeout(timer)
  }, [flash])

  const filteredUsers = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    const visible = users.filter((user) => {
      if (statusFilter !== 'all' && user.status !== statusFilter) return false
      if (verifiedFilter === 'verified' && !user.email_verified) return false
      if (verifiedFilter === 'unverified' && user.email_verified) return false
      if (registerFilter === 'oidc' && !user.oidc_subject) return false
      if (registerFilter === 'email' && user.oidc_subject) return false
      if (!normalized) return true
      return (
        (user.email || '').toLowerCase().includes(normalized) ||
        String(user.id).includes(normalized) ||
        user.sub_id.toLowerCase().includes(normalized)
      )
    })
    return sortUsers(visible, sort)
  }, [query, registerFilter, sort, statusFilter, users, verifiedFilter])

  useEffect(() => {
    const visibleIds = new Set(filteredUsers.map((user) => user.id))
    setSelectedRowKeys((current) => {
      const next = current.filter((id) => visibleIds.has(Number(id)))
      return next.length === current.length ? current : next
    })
  }, [filteredUsers])

  const selectedIds = selectedRowKeys.map(Number)

  const refresh = () => {
    void usersQuery.refetch()
  }

  const openCreate = () => {
    createForm.setFieldsValue({ email: '', password: '', initial_balance_yuan: 0 })
    setCreateOpen(true)
  }

  const openEdit = (user: AdminUser) => {
    setEditing(user)
    editForm.setFieldsValue({
      email: user.email || '',
      email_verified: user.email_verified,
      password: '',
      balance_yuan: user.balance_cents / 100,
    })
  }

  const openBalance = (user: AdminUser) => {
    setBalanceUser(user)
    balanceForm.setFieldsValue({ delta_yuan: 0, reason: '' })
  }

  const closeCreate = () => {
    setCreateOpen(false)
    createForm.resetFields()
  }

  const closeEdit = () => {
    setEditing(null)
    editForm.resetFields()
  }

  const closeBalance = () => {
    setBalanceUser(null)
    balanceForm.resetFields()
  }

  const saveCreate = async () => {
    const values = await createForm.validateFields().catch(() => null)
    if (!values) return
    const created = await createUser.mutateAsync({
      email: values.email.trim(),
      password: values.password,
      initial_balance_cents: Math.round((values.initial_balance_yuan ?? 0) * 100),
    })
    closeCreate()
    setFlash({ type: 'success', text: `Created ${userLabel(created)}.` })
  }

  const saveEdit = async () => {
    if (!editing) return
    const values = await editForm.validateFields().catch(() => null)
    if (!values) return
    const fields: Parameters<typeof updateUser.mutateAsync>[0]['fields'] = {
      email: values.email.trim(),
      email_verified: values.email_verified,
      balance_cents: Math.round(values.balance_yuan * 100),
    }
    if (values.password) fields.password = values.password
    const updated = await updateUser.mutateAsync({ id: editing.id, fields })
    closeEdit()
    setFlash({ type: 'success', text: `Updated ${userLabel(updated)}.` })
  }

  const saveBalance = async () => {
    if (!balanceUser) return
    const values = await balanceForm.validateFields().catch(() => null)
    if (!values) return
    await adjustBalance.mutateAsync({
      id: balanceUser.id,
      deltaCents: Math.round(values.delta_yuan * 100),
      reason: values.reason.trim(),
    })
    closeBalance()
    setFlash({ type: 'success', text: `Adjusted ${userLabel(balanceUser)} balance.` })
  }

  const toggleAutoRenew = async (user: AdminUser) => {
    await updateUser.mutateAsync({ id: user.id, fields: { auto_renew: !user.auto_renew } })
    setFlash({ type: 'success', text: `Auto renew ${user.auto_renew ? 'disabled' : 'enabled'} for ${userLabel(user)}.` })
  }

  const toggleSuspend = async (user: AdminUser) => {
    if (user.status === 'suspended') {
      await unsuspendUser.mutateAsync(user.id)
      setFlash({ type: 'success', text: `Unsuspended ${userLabel(user)}.` })
    } else {
      await suspendUser.mutateAsync(user.id)
      setFlash({ type: 'success', text: `Suspended ${userLabel(user)}.` })
    }
  }

  const confirmDelete = (user: AdminUser) => {
    Modal.confirm({
      title: 'Delete user',
      content: `Delete ${userLabel(user)}? This cannot be undone.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: async () => {
        await removeUser.mutateAsync(user.id)
        setFlash({ type: 'success', text: `Deleted ${userLabel(user)}.` })
      },
    })
  }

  const runBatch = async (label: string, runner: (id: number) => Promise<unknown>) => {
    if (selectedIds.length === 0) return
    const ids = [...selectedIds]
    const results = await Promise.allSettled(ids.map((id) => runner(id)))
    const ok = results.filter((result) => result.status === 'fulfilled').length
    const fail = results.length - ok
    setSelectedRowKeys([])
    setFlash({ type: fail === 0 ? 'success' : 'error', text: flashText(label, ok, fail) })
    await usersQuery.refetch()
  }

  const batchDelete = () => {
    if (selectedIds.length === 0) return
    Modal.confirm({
      title: 'Delete selected users',
      content: `Delete ${selectedIds.length} selected users? This cannot be undone.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => runBatch('Delete', (id) => removeUser.mutateAsync(id)),
    })
  }

  const exportCsv = () => {
    const headers = ['id', 'email', 'status', 'balance_cents', 'email_verified', 'created_at']
    const rows = filteredUsers.map((user) =>
      [user.id, user.email || '', user.status, user.balance_cents, user.email_verified ? 'true' : 'false', user.created_at]
        .map((value) => `"${String(value).replace(/"/g, '""')}"`)
        .join(','),
    )
    const blob = new Blob([[headers.join(','), ...rows].join('\n')], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `users-${new Date().toISOString().slice(0, 10)}.csv`
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    URL.revokeObjectURL(url)
    setFlash({ type: 'success', text: `Exported ${filteredUsers.length} users.` })
  }

  const moreItems: MenuProps['items'] = [
    { key: 'csv', icon: <DownloadOutlined />, label: 'Export CSV', onClick: exportCsv },
    {
      key: 'purge',
      label: 'Purge client cache',
      onClick: () => setFlash({ type: 'error', text: 'Client cache purge is not implemented yet.' }),
    },
  ]

  const columns: ColumnsType<AdminUser> = [
    {
      title: 'User',
      dataIndex: 'email',
      sorter: (a, b) => (a.email || '').localeCompare(b.email || ''),
      render: (_value, user) => (
        <Space direction="vertical" size={2}>
          <Space wrap>
            <Typography.Text strong>{user.email || '—'}</Typography.Text>
            {verifiedTag(user)}
            {user.oidc_subject ? <Tag color="blue">OIDC</Tag> : <Tag>Email</Tag>}
          </Space>
          <Typography.Text type="secondary">{user.sub_id.slice(0, 12)}...</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
      render: (id: number) => <Typography.Text code>#{id}</Typography.Text>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      filters: [
        { text: 'Active', value: 'active' },
        { text: 'Suspended', value: 'suspended' },
      ],
      onFilter: (value, user) => user.status === value,
      render: statusTag,
    },
    {
      title: 'Balance',
      dataIndex: 'balance_cents',
      align: 'right',
      sorter: (a, b) => a.balance_cents - b.balance_cents,
      render: (_value, user) => (
        <Space>
          <Typography.Text strong>{formatYuan(user.balance_cents)}</Typography.Text>
          <Button size="small" onClick={() => openBalance(user)}>
            Adjust
          </Button>
        </Space>
      ),
    },
    {
      title: 'Auto renew',
      dataIndex: 'auto_renew',
      render: (_value, user) => (
        <Switch
          checked={user.auto_renew}
          aria-label={`${user.auto_renew ? 'Disable' : 'Enable'} auto renew for ${userLabel(user)}`}
          loading={updateUser.isPending}
          onChange={() => toggleAutoRenew(user)}
        />
      ),
    },
    {
      title: 'Registered',
      dataIndex: 'created_at',
      sorter: (a, b) => a.created_at.localeCompare(b.created_at),
      render: formatDate,
    },
    {
      title: 'Last active',
      dataIndex: 'last_active_at',
      render: formatDate,
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, user) => (
        <Space>
          <Button aria-label={`Edit ${userLabel(user)}`} icon={<EditOutlined />} onClick={() => openEdit(user)} />
          <Button
            aria-label={`${user.status === 'suspended' ? 'Unsuspend' : 'Suspend'} ${userLabel(user)}`}
            icon={user.status === 'suspended' ? <PlayCircleOutlined /> : <PauseCircleOutlined />}
            onClick={() => toggleSuspend(user)}
          />
          <Button
            danger
            aria-label={`Delete ${userLabel(user)}`}
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(user)}
          />
        </Space>
      ),
    },
  ]

  return (
    <section>
      <PageHeader
        title="Users"
        subtitle="Create accounts, review balances, and manage account status."
        actions={
          <>
            <Button type="primary" aria-label="New User" icon={<PlusOutlined />} onClick={openCreate}>
              New User
            </Button>
            <RefreshButton loading={usersQuery.isFetching} onClick={refresh} />
          </>
        }
      />

      {error ? <Alert type="error" showIcon message="User operation failed" style={{ marginBottom: 16 }} /> : null}
      {flash ? (
        <Alert
          data-show={Boolean(flash)}
          type={flash.type}
          showIcon
          message={flash.text}
          style={{ marginBottom: 16, transition: 'opacity 0.2s ease, transform 0.2s ease' }}
        />
      ) : null}

      <Card size="small" style={{ marginBottom: 16 }}>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Input.Search
            allowClear
            value={query}
            placeholder="Search email, id, or subscription id"
            onChange={(event) => setQuery(event.target.value)}
          />
          <Space wrap>
            <Segmented
              aria-label="Status filter"
              value={statusFilter}
              onChange={(value) => setStatusFilter(value as StatusFilter)}
              options={[
                { label: 'All', value: 'all' },
                { label: 'Active', value: 'active' },
                { label: 'Suspended', value: 'suspended' },
              ]}
            />
            <Segmented
              aria-label="Verified filter"
              value={verifiedFilter}
              onChange={(value) => setVerifiedFilter(value as VerifiedFilter)}
              options={[
                { label: 'Any verification', value: 'all' },
                { label: 'Verified', value: 'verified' },
                { label: 'Unverified', value: 'unverified' },
              ]}
            />
            <Segmented
              aria-label="Register method filter"
              value={registerFilter}
              onChange={(value) => setRegisterFilter(value as RegisterFilter)}
              options={[
                { label: 'Any method', value: 'all' },
                { label: 'Email', value: 'email' },
                { label: 'OIDC', value: 'oidc' },
              ]}
            />
            <Select
              aria-label="Sort users"
              value={sort}
              onChange={(value) => setSort(value)}
              style={{ minWidth: 190 }}
              options={[
                { label: 'Newest first', value: 'created_at:desc' },
                { label: 'Oldest first', value: 'created_at:asc' },
                { label: 'Highest balance', value: 'balance:desc' },
                { label: 'Lowest balance', value: 'balance:asc' },
                { label: 'Highest id', value: 'id:desc' },
                { label: 'Email A-Z', value: 'email:asc' },
                { label: 'Email Z-A', value: 'email:desc' },
              ]}
            />
            <Switch
              checked={autoRefresh}
              aria-label="Auto refresh"
              checkedChildren="Auto"
              unCheckedChildren="Manual"
              onChange={setAutoRefresh}
            />
            <Dropdown menu={{ items: moreItems }}>
              <Button>More</Button>
            </Dropdown>
          </Space>
        </Space>
      </Card>

      <div
        data-show={selectedRowKeys.length > 0}
        style={{
          display: selectedRowKeys.length > 0 ? 'flex' : 'none',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 16,
          padding: 12,
          border: '1px solid #d9d9d9',
          borderRadius: 8,
          transition: 'opacity 0.18s ease, transform 0.18s ease',
        }}
      >
        <Typography.Text strong>{selectedRowKeys.length} selected</Typography.Text>
        <Space wrap>
          <Button
            disabled={selectedRowKeys.length === 0}
            onClick={() => runBatch('Suspend', (id) => suspendUser.mutateAsync(id))}
          >
            Suspend selected
          </Button>
          <Button
            disabled={selectedRowKeys.length === 0}
            onClick={() => runBatch('Unsuspend', (id) => unsuspendUser.mutateAsync(id))}
          >
            Unsuspend selected
          </Button>
          <Button danger disabled={selectedRowKeys.length === 0} onClick={batchDelete}>
            Delete selected
          </Button>
          <Button onClick={() => setSelectedRowKeys([])}>Clear</Button>
        </Space>
      </div>

      {filteredUsers.length > 0 || loading ? (
        <ResponsiveListTable<AdminUser>
          rowKey="id"
          columns={columns}
          dataSource={filteredUsers}
          loading={loading}
          pagination={{ pageSize: PAGE_SIZE, showSizeChanger: false }}
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
            getCheckboxProps: (user) =>
              ({ 'aria-label': `Select ${userLabel(user)}` }) as Partial<
                Omit<CheckboxProps, 'checked' | 'defaultChecked'>
              >,
          }}
          locale={{
            emptyText: users.length === 0 ? 'No users yet.' : 'No users match the current filters.',
          }}
          mobileCard={(user) => (
            <Card size="small" style={{ width: '100%' }}>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                  <Checkbox
                    checked={selectedRowKeys.includes(user.id)}
                    aria-label={`Select ${userLabel(user)}`}
                    onChange={(event) =>
                      setSelectedRowKeys((current) =>
                        event.target.checked ? [...current, user.id] : current.filter((id) => id !== user.id),
                      )
                    }
                  />
                  {statusTag(user.status)}
                </Space>
                <Space wrap>
                  <Typography.Text strong>{user.email || '—'}</Typography.Text>
                  {verifiedTag(user)}
                  {user.oidc_subject ? <Tag color="blue">OIDC</Tag> : <Tag>Email</Tag>}
                </Space>
                <Typography.Text type="secondary">#{user.id} · {user.sub_id.slice(0, 12)}...</Typography.Text>
                <Typography.Text>Balance: {formatYuan(user.balance_cents)}</Typography.Text>
                <Typography.Text>Auto renew: {user.auto_renew ? 'On' : 'Off'}</Typography.Text>
                <Typography.Text>Registered: {formatDate(user.created_at)}</Typography.Text>
                <Typography.Text>Last active: {formatDate(user.last_active_at)}</Typography.Text>
                <Space wrap>
                  <Button size="small" onClick={() => openBalance(user)}>
                    Adjust
                  </Button>
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(user)}>
                    Edit
                  </Button>
                  <Button size="small" onClick={() => toggleSuspend(user)}>
                    {user.status === 'suspended' ? 'Unsuspend' : 'Suspend'}
                  </Button>
                  <Button size="small" danger onClick={() => confirmDelete(user)}>
                    Delete
                  </Button>
                </Space>
              </Space>
            </Card>
          )}
        />
      ) : (
        <EmptyState title="No users" description="Create a user to start managing accounts." actionLabel="New User" onAction={openCreate} />
      )}

      <Modal title="New User" open={createOpen} onCancel={closeCreate} onOk={saveCreate} confirmLoading={createUser.isPending} destroyOnHidden>
        <Form form={createForm} layout="vertical" preserve={false}>
          <Form.Item name="email" label="Email" rules={[{ required: true, type: 'email', message: 'Valid email is required' }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="password" label="Password" rules={[{ required: true, min: 8, message: 'Password must be at least 8 characters' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="initial_balance_yuan" label="Initial balance" rules={[{ type: 'number', min: 0, message: 'Initial balance must be zero or greater' }]}>
            <InputNumber min={0} step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={editing ? `Edit user #${editing.id}` : 'Edit user'} open={Boolean(editing)} onCancel={closeEdit} onOk={saveEdit} confirmLoading={updateUser.isPending} destroyOnHidden>
        <Form form={editForm} layout="vertical" preserve={false}>
          <Form.Item name="email" label="Email" rules={[{ required: true, type: 'email', message: 'Valid email is required' }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="email_verified" label="Email verified" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="password" label="Password" rules={[{ min: 8, message: 'Password must be at least 8 characters' }]}>
            <Input.Password autoComplete="new-password" placeholder="Leave blank to keep current password" />
          </Form.Item>
          <Form.Item name="balance_yuan" label="Balance" rules={[{ required: true, type: 'number', min: 0, message: 'Balance must be zero or greater' }]}>
            <InputNumber min={0} step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={balanceUser ? `Adjust ${userLabel(balanceUser)}` : 'Adjust balance'} open={Boolean(balanceUser)} onCancel={closeBalance} onOk={saveBalance} confirmLoading={adjustBalance.isPending} destroyOnHidden>
        <Form form={balanceForm} layout="vertical" preserve={false}>
          <Form.Item
            name="delta_yuan"
            label="Amount"
            rules={[
              { required: true, type: 'number', message: 'Amount is required' },
              {
                validator: (_, value) =>
                  value === 0 ? Promise.reject(new Error('Amount must be non-zero')) : Promise.resolve(),
              },
            ]}
          >
            <InputNumber step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="reason" label="Reason" rules={[{ required: true, whitespace: true, message: 'Reason is required' }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
