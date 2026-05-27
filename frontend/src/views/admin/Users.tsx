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
import { useTranslation } from 'react-i18next'
import { adminUsersApi, type AdminUser, type UserStatus } from '@/api/admin/users'
import { ConfigListPage, RefreshButton } from '@/components/common'
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

function statusTag(status: UserStatus, label: string) {
  return <Tag color={status === 'active' ? 'green' : 'red'}>{label}</Tag>
}

function verifiedTag(user: AdminUser, verified: string, unverified: string) {
  if (!user.email) return null
  return <Tag color={user.email_verified ? 'green' : 'gold'}>{user.email_verified ? verified : unverified}</Tag>
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

export default function Users() {
  const { t } = useTranslation()
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
      if (registerFilter === 'oidc' && !user.oidc_linked) return false
      if (registerFilter === 'email' && user.oidc_linked) return false
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
  const userStatusLabel = (status: UserStatus) => t(`admin.users.status.${status}`)
  const userStatusTag = (status: UserStatus) => statusTag(status, userStatusLabel(status))
  const userVerifiedTag = (user: AdminUser) => verifiedTag(user, t('admin.users.verified'), t('admin.users.unverified'))

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
    setFlash({ type: 'success', text: t('admin.users.create.success', { email: userLabel(created) }) })
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
    setFlash({ type: 'success', text: t('admin.users.edit.success', { email: userLabel(updated) }) })
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
    setFlash({ type: 'success', text: t('admin.users.balance.success', { email: userLabel(balanceUser) }) })
  }

  const toggleAutoRenew = async (user: AdminUser) => {
    await updateUser.mutateAsync({ id: user.id, fields: { auto_renew: !user.auto_renew } })
    setFlash({ type: 'success', text: t(user.auto_renew ? 'admin.users.autoRenewOff' : 'admin.users.autoRenewOn') })
  }

  const toggleSuspend = async (user: AdminUser) => {
    if (user.status === 'suspended') {
      await unsuspendUser.mutateAsync(user.id)
      setFlash({ type: 'success', text: t('admin.users.unsuspend') })
    } else {
      await suspendUser.mutateAsync(user.id)
      setFlash({ type: 'success', text: t('admin.users.suspend') })
    }
  }

  const confirmDelete = (user: AdminUser) => {
    Modal.confirm({
      title: t('admin.users.confirmDelete'),
      content: t('admin.users.confirmDeleteMsg', { email: userLabel(user) }),
      okText: t('admin.users.delete'),
      okButtonProps: { danger: true },
      onOk: async () => {
        await removeUser.mutateAsync(user.id)
        setFlash({ type: 'success', text: t('admin.users.delete') })
      },
    })
  }

  const runBatch = async (resultKey: string, runner: (id: number) => Promise<unknown>) => {
    if (selectedIds.length === 0) return
    const ids = [...selectedIds]
    const results = await Promise.allSettled(ids.map((id) => runner(id)))
    const ok = results.filter((result) => result.status === 'fulfilled').length
    const fail = results.length - ok
    setSelectedRowKeys([])
    setFlash({ type: fail === 0 ? 'success' : 'error', text: t(resultKey, { ok, fail }) })
    await usersQuery.refetch()
  }

  const batchDelete = () => {
    if (selectedIds.length === 0) return
    Modal.confirm({
      title: t('admin.users.batch.deleteTitle'),
      content: t('admin.users.batch.deleteMsg', { n: selectedIds.length }),
      okText: t('admin.users.delete'),
      okButtonProps: { danger: true },
      onOk: () => runBatch('admin.users.batch.deleteResult', (id) => removeUser.mutateAsync(id)),
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
    setFlash({ type: 'success', text: t('admin.users.more.csvExported', { n: filteredUsers.length }) })
  }

  const moreItems: MenuProps['items'] = [
    { key: 'csv', icon: <DownloadOutlined />, label: t('admin.users.more.exportCsv'), onClick: exportCsv },
    {
      key: 'purge',
      label: t('admin.users.more.purgeCache'),
      onClick: () => setFlash({ type: 'error', text: t('admin.users.more.notImplemented') }),
    },
  ]

  const columns: ColumnsType<AdminUser> = [
    {
      title: t('admin.users.column.user'),
      dataIndex: 'email',
      sorter: (a, b) => (a.email || '').localeCompare(b.email || ''),
      render: (_value, user) => (
        <Space direction="vertical" size={2}>
          <Space wrap>
            <Typography.Text strong>{user.email || '—'}</Typography.Text>
            {userVerifiedTag(user)}
            {user.oidc_linked ? <Tag color="blue">OIDC</Tag> : <Tag>{t('admin.users.auth.email')}</Tag>}
          </Space>
          <Typography.Text type="secondary">{user.sub_id.slice(0, 12)}...</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'ID',
      dataIndex: 'id',
      align: 'center',
      width: 80,
      render: (id: number) => <Typography.Text code>#{id}</Typography.Text>,
    },
    {
      title: t('admin.users.column.status'),
      dataIndex: 'status',
      align: 'center',
      width: 120,
      filters: [
        { text: t('admin.users.status.active'), value: 'active' },
        { text: t('admin.users.status.suspended'), value: 'suspended' },
      ],
      onFilter: (value, user) => user.status === value,
      render: userStatusTag,
    },
    {
      title: t('admin.users.column.balance'),
      dataIndex: 'balance_cents',
      align: 'right',
      className: 'table-cell-number',
      width: 150,
      sorter: (a, b) => a.balance_cents - b.balance_cents,
      render: (_value, user) => (
        <Space>
          <Typography.Text strong>{formatYuan(user.balance_cents)}</Typography.Text>
          <Button size="small" onClick={() => openBalance(user)}>
            {t('admin.users.balance.adjustShort')}
          </Button>
        </Space>
      ),
    },
    {
      title: t('admin.users.column.autoRenew'),
      dataIndex: 'auto_renew',
      align: 'center',
      width: 120,
      render: (_value, user) => (
        <Switch
          checked={user.auto_renew}
          aria-label={`${user.auto_renew ? t('admin.users.autoRenewOff') : t('admin.users.autoRenewOn')} ${userLabel(user)}`}
          loading={updateUser.isPending}
          onChange={() => toggleAutoRenew(user)}
        />
      ),
    },
    {
      title: t('admin.users.column.registered'),
      dataIndex: 'created_at',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 180,
      sorter: (a, b) => a.created_at.localeCompare(b.created_at),
      render: formatDate,
    },
    {
      title: t('admin.users.column.lastActive'),
      dataIndex: 'last_active_at',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 180,
      render: formatDate,
    },
    {
      title: t('admin.users.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 144,
      render: (_value, user) => (
        <Space>
          <Button aria-label={`${t('admin.users.edit.open')} ${userLabel(user)}`} icon={<EditOutlined />} onClick={() => openEdit(user)} />
          <Button
            aria-label={`${user.status === 'suspended' ? t('admin.users.unsuspend') : t('admin.users.suspend')} ${userLabel(user)}`}
            icon={user.status === 'suspended' ? <PlayCircleOutlined /> : <PauseCircleOutlined />}
            onClick={() => toggleSuspend(user)}
          />
          <Button
            danger
            aria-label={`${t('admin.users.delete')} ${userLabel(user)}`}
            icon={<DeleteOutlined />}
            onClick={() => confirmDelete(user)}
          />
        </Space>
      ),
    },
  ]

  return (
    <section>
      <ConfigListPage
        title={t('admin.users.title')}
        subtitle={t('admin.users.subtitle')}
        actions={
          <>
            <Button type="primary" aria-label={t('admin.users.addUser')} icon={<PlusOutlined />} onClick={openCreate}>
              {t('admin.users.addUser')}
            </Button>
            <RefreshButton loading={usersQuery.isFetching} onClick={refresh} label={t('admin.users.reload')} />
          </>
        }
        filters={
          <Space wrap>
            <Input.Search
              allowClear
              value={query}
              placeholder={t('admin.users.searchPlaceholder')}
              onChange={(event) => setQuery(event.target.value)}
              style={{ width: 260 }}
            />
            <Segmented
              aria-label={t('admin.users.filterStatus')}
              value={statusFilter}
              onChange={(value) => setStatusFilter(value as StatusFilter)}
              options={[
                { label: t('admin.users.filterAll'), value: 'all' },
                { label: t('admin.users.status.active'), value: 'active' },
                { label: t('admin.users.status.suspended'), value: 'suspended' },
              ]}
            />
            <Segmented
              aria-label={t('admin.users.filterVerified')}
              value={verifiedFilter}
              onChange={(value) => setVerifiedFilter(value as VerifiedFilter)}
              options={[
                { label: t('admin.users.filterVerifiedAll'), value: 'all' },
                { label: t('admin.users.verified'), value: 'verified' },
                { label: t('admin.users.unverified'), value: 'unverified' },
              ]}
            />
            <Segmented
              aria-label={t('admin.users.filterRegisterMethod')}
              value={registerFilter}
              onChange={(value) => setRegisterFilter(value as RegisterFilter)}
              options={[
                { label: t('admin.users.filterRegisterAll'), value: 'all' },
                { label: t('admin.users.filterRegisterEmail'), value: 'email' },
                { label: 'OIDC', value: 'oidc' },
              ]}
            />
            <Select
              aria-label={t('admin.users.sortLabel')}
              value={sort}
              onChange={(value) => setSort(value)}
              style={{ minWidth: 190 }}
              options={[
                { label: t('admin.users.sort.createdDesc'), value: 'created_at:desc' },
                { label: t('admin.users.sort.createdAsc'), value: 'created_at:asc' },
                { label: t('admin.users.sort.balanceDesc'), value: 'balance:desc' },
                { label: t('admin.users.sort.balanceAsc'), value: 'balance:asc' },
                { label: t('admin.users.sort.idDesc'), value: 'id:desc' },
                { label: t('admin.users.sort.emailAsc'), value: 'email:asc' },
                { label: t('admin.users.sort.emailDesc'), value: 'email:desc' },
              ]}
            />
            <Switch
              checked={autoRefresh}
              aria-label={t('admin.users.autoRefresh')}
              checkedChildren={t('admin.users.autoRefreshShort')}
              unCheckedChildren={t('admin.users.manualRefreshShort')}
              onChange={setAutoRefresh}
            />
            <Dropdown menu={{ items: moreItems }}>
              <Button>{t('admin.users.more.label')}</Button>
            </Dropdown>
          </Space>
        }
        alerts={error || flash || selectedRowKeys.length > 0 ? (
          <>
            {error ? <Alert type="error" showIcon message={t('admin.users.loadFailed')} /> : null}
            {flash ? (
              <Alert
                data-show={Boolean(flash)}
                type={flash.type}
                showIcon
                message={flash.text}
                style={{ marginTop: error ? 16 : 0, transition: 'opacity 0.2s ease, transform 0.2s ease' }}
              />
            ) : null}
            {selectedRowKeys.length > 0 ? (
              <div
                data-show
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 12,
                  marginTop: error || flash ? 16 : 0,
                  padding: 12,
                  border: '1px solid #d9d9d9',
                  borderRadius: 8,
                  transition: 'opacity 0.18s ease, transform 0.18s ease',
                }}
              >
                <Typography.Text strong>{t('admin.users.batch.selectedCount', { n: selectedRowKeys.length })}</Typography.Text>
                <Space wrap>
                  <Button
                    disabled={selectedRowKeys.length === 0}
                    onClick={() => runBatch('admin.users.batch.suspendResult', (id) => suspendUser.mutateAsync(id))}
                  >
                    {t('admin.users.batch.suspend')}
                  </Button>
                  <Button
                    disabled={selectedRowKeys.length === 0}
                    onClick={() => runBatch('admin.users.batch.unsuspendResult', (id) => unsuspendUser.mutateAsync(id))}
                  >
                    {t('admin.users.batch.unsuspend')}
                  </Button>
                  <Button danger disabled={selectedRowKeys.length === 0} onClick={batchDelete}>
                    {t('admin.users.batch.delete')}
                  </Button>
                  <Button onClick={() => setSelectedRowKeys([])}>{t('admin.users.batch.clear')}</Button>
                </Space>
              </div>
            ) : null}
          </>
        ) : null}
        rowKey="id"
        columns={columns}
        dataSource={filteredUsers}
        loading={loading}
        pagination={{ pageSize: PAGE_SIZE, showSizeChanger: false }}
        rowSelection={{
          selectedRowKeys,
          onChange: setSelectedRowKeys,
          getCheckboxProps: (user) =>
            ({ 'aria-label': t('admin.users.batch.toggleRow', { email: userLabel(user) }) }) as Partial<
              Omit<CheckboxProps, 'checked' | 'defaultChecked'>
            >,
        }}
        emptyState={{
          title: t('admin.users.empty'),
          description: t('admin.users.emptyDescription'),
          actionLabel: t('admin.users.addUser'),
          onAction: openCreate,
        }}
        mobileCard={(user) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Checkbox
                  checked={selectedRowKeys.includes(user.id)}
                  aria-label={t('admin.users.batch.toggleRow', { email: userLabel(user) })}
                  onChange={(event) =>
                    setSelectedRowKeys((current) =>
                      event.target.checked ? [...current, user.id] : current.filter((id) => id !== user.id),
                    )
                  }
                />
                {userStatusTag(user.status)}
              </Space>
              <Space wrap>
                <Typography.Text strong>{user.email || '—'}</Typography.Text>
                {userVerifiedTag(user)}
                {user.oidc_linked ? <Tag color="blue">OIDC</Tag> : <Tag>{t('admin.users.auth.email')}</Tag>}
              </Space>
              <Typography.Text type="secondary">#{user.id} · {user.sub_id.slice(0, 12)}...</Typography.Text>
              <Typography.Text>{t('admin.users.column.balance')}: {formatYuan(user.balance_cents)}</Typography.Text>
              <Typography.Text>{t('admin.users.column.autoRenew')}: {user.auto_renew ? t('admin.users.on') : t('admin.users.off')}</Typography.Text>
              <Typography.Text>{t('admin.users.column.registered')}: {formatDate(user.created_at)}</Typography.Text>
              <Typography.Text>{t('admin.users.column.lastActive')}: {formatDate(user.last_active_at)}</Typography.Text>
              <Space wrap>
                <Button size="small" onClick={() => openBalance(user)}>
                  {t('admin.users.balance.adjustShort')}
                </Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(user)}>
                  {t('admin.users.edit.open')}
                </Button>
                <Button size="small" onClick={() => toggleSuspend(user)}>
                  {user.status === 'suspended' ? t('admin.users.unsuspend') : t('admin.users.suspend')}
                </Button>
                <Button size="small" danger onClick={() => confirmDelete(user)}>
                  {t('admin.users.delete')}
                </Button>
              </Space>
            </Space>
          </Card>
        )}
      />

      <Modal title={t('admin.users.create.title')} open={createOpen} onCancel={closeCreate} onOk={saveCreate} confirmLoading={createUser.isPending} destroyOnHidden>
        <Form form={createForm} layout="vertical" preserve={false}>
          <Form.Item name="email" label={t('admin.users.create.emailLabel')} rules={[{ required: true, type: 'email', message: t('admin.users.create.emailRequired') }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="password" label={t('admin.users.create.passwordLabel')} rules={[{ required: true, min: 8, message: t('admin.users.create.passwordMin') }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="initial_balance_yuan" label={t('admin.users.create.initialBalanceLabel')} rules={[{ type: 'number', min: 0, message: t('admin.users.create.initialBalanceLabel') }]}>
            <InputNumber min={0} step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={editing ? `${t('admin.users.edit.title')} #${editing.id}` : t('admin.users.edit.title')} open={Boolean(editing)} onCancel={closeEdit} onOk={saveEdit} confirmLoading={updateUser.isPending} destroyOnHidden>
        <Form form={editForm} layout="vertical" preserve={false}>
          <Form.Item name="email" label={t('admin.users.edit.emailLabel')} rules={[{ required: true, type: 'email', message: t('admin.users.edit.emailRequired') }]}>
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="email_verified" label={t('admin.users.edit.verifiedLabel')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="password" label={t('admin.users.edit.passwordLabel')} rules={[{ min: 8, message: t('admin.users.edit.passwordMin') }]}>
            <Input.Password autoComplete="new-password" placeholder={t('admin.users.edit.passwordPlaceholder')} />
          </Form.Item>
          <Form.Item name="balance_yuan" label={t('admin.users.edit.balanceLabel')} rules={[{ required: true, type: 'number', min: 0, message: t('admin.users.edit.balanceInvalid') }]}>
            <InputNumber min={0} step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={balanceUser ? `${t('admin.users.balance.adjust')} ${userLabel(balanceUser)}` : t('admin.users.balance.title')} open={Boolean(balanceUser)} onCancel={closeBalance} onOk={saveBalance} confirmLoading={adjustBalance.isPending} destroyOnHidden>
        <Form form={balanceForm} layout="vertical" preserve={false}>
          <Form.Item
            name="delta_yuan"
            label={t('admin.users.balance.amountLabel')}
            rules={[
              { required: true, type: 'number', message: t('admin.users.balance.amountRequired') },
              {
                validator: (_, value) =>
                  value === 0 ? Promise.reject(new Error(t('admin.users.balance.deltaMustNonZero'))) : Promise.resolve(),
              },
            ]}
          >
            <InputNumber step={0.01} precision={2} prefix="¥" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="reason" label={t('admin.users.balance.reasonLabel')} rules={[{ required: true, whitespace: true, message: t('admin.users.balance.reasonRequired') }]}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
