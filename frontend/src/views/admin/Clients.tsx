import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  QrcodeOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  QRCode,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Client, FleetInbound, Inbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'
import { ConfigListPage, RefreshButton } from '@/components/common'
import {
  useAddClient,
  useInboundsFleet,
  useRemoveClient,
  useUpdateClient,
} from '@/hooks/queries/admin/inbounds'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import { useUsersList } from '@/hooks/queries/admin/users'
import { buildClientLink, formatBytes, formatLimit, parseClients } from './inbounds/utils'

// Flattened "1 client per row" shape.
interface ClientRow {
  client: Client
  inbound: Inbound
  nodeID: number
  nodeName: string
  rowKey: string
}

function flatten(fleet: FleetInbound[] | undefined): ClientRow[] {
  if (!fleet) return []
  const out: ClientRow[] = []
  for (const row of fleet) {
    for (const c of parseClients(row.inbound)) {
      out.push({
        client: c,
        inbound: row.inbound,
        nodeID: row.node_id,
        nodeName: row.node_name,
        rowKey: `${row.node_id}|${row.inbound.tag}|${c.email}`,
      })
    }
  }
  return out
}

function trafficUsed(row: ClientRow): number {
  const stat = row.inbound.clientStats?.find((s) => s.email === row.client.email)
  if (!stat) return 0
  return (stat.up ?? 0) + (stat.down ?? 0)
}

function quotaBytes(client: Client): number {
  const gb = client.totalGB ?? 0
  return gb > 0 ? gb * 1024 * 1024 * 1024 : 0
}

function formatExpiry(ms: number | undefined, never: string): string {
  if (!ms || ms <= 0) return never
  return new Date(ms).toLocaleString()
}

interface EditorState {
  open: boolean
  mode: 'create' | 'edit'
  row: ClientRow | null
}

export default function Clients() {
  const { t } = useTranslation()
  const fleetQuery = useInboundsFleet()
  const nodesQuery = useNodesList()
  const usersQuery = useUsersList()
  const addClient = useAddClient()
  const updateClient = useUpdateClient()
  const removeClient = useRemoveClient()

  const [query, setQuery] = useState('')
  const [protocolFilter, setProtocolFilter] = useState<string[]>([])
  const [nodeFilter, setNodeFilter] = useState<number[]>([])
  const [statusFilter, setStatusFilter] = useState<'all' | 'enabled' | 'disabled'>('all')
  const [editor, setEditor] = useState<EditorState>({ open: false, mode: 'create', row: null })
  const [qr, setQr] = useState<{ title: string; url: string } | null>(null)

  const rows = useMemo(() => flatten(fleetQuery.data?.inbounds), [fleetQuery.data])

  const protocolOptions = useMemo(() => {
    const set = new Set(rows.map((r) => r.inbound.protocol))
    return Array.from(set).map((p) => ({ label: p, value: p }))
  }, [rows])

  const nodeOptions = useMemo(() => {
    const map = new Map<number, string>()
    for (const r of rows) map.set(r.nodeID, r.nodeName)
    return Array.from(map.entries()).map(([id, name]) => ({ label: name, value: id }))
  }, [rows])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const protoSet = new Set(protocolFilter)
    const nodeSet = new Set(nodeFilter)
    return rows.filter((r) => {
      if (protoSet.size > 0 && !protoSet.has(r.inbound.protocol)) return false
      if (nodeSet.size > 0 && !nodeSet.has(r.nodeID)) return false
      if (statusFilter === 'enabled' && r.client.enable === false) return false
      if (statusFilter === 'disabled' && r.client.enable !== false) return false
      if (!q) return true
      return (
        r.client.email.toLowerCase().includes(q) ||
        (r.client.comment ?? '').toLowerCase().includes(q) ||
        r.inbound.tag.toLowerCase().includes(q) ||
        r.nodeName.toLowerCase().includes(q) ||
        (r.client.subId ?? '').toLowerCase().includes(q)
      )
    })
  }, [rows, query, protocolFilter, nodeFilter, statusFilter])

  const users = useMemo(() => usersQuery.data?.users ?? [], [usersQuery.data])
  const nodes = useMemo(() => (nodesQuery.data ?? []) as Node[], [nodesQuery.data])

  const userById = useMemo(() => {
    const m = new Map<number, string>()
    for (const u of users) m.set(u.id, u.email ?? '')
    return m
  }, [users])

  const showQrFor = (row: ClientRow) => {
    const fleetRow: FleetInbound = { node_id: row.nodeID, node_name: row.nodeName, inbound: row.inbound }
    const url = buildClientLink(fleetRow, row.client, nodes)
    if (!url) return
    setQr({ title: `${row.inbound.tag} · ${row.client.email}`, url })
  }

  const confirmDelete = (row: ClientRow) => {
    Modal.confirm({
      title: t('admin.clients.confirmDelete'),
      content: t('admin.clients.confirmDeleteMsg', { email: row.client.email, tag: row.inbound.tag }),
      okText: t('admin.clients.delete'),
      okButtonProps: { danger: true },
      onOk: () => removeClient.mutateAsync({ nodeID: row.nodeID, tag: row.inbound.tag, email: row.client.email }),
    })
  }

  const toggleEnable = async (row: ClientRow) => {
    await updateClient.mutateAsync({
      nodeID: row.nodeID,
      tag: row.inbound.tag,
      email: row.client.email,
      client: { ...row.client, enable: row.client.enable === false },
    })
  }

  const columns: ColumnsType<ClientRow> = [
    {
      title: t('admin.clients.column.email'),
      key: 'email',
      width: 240,
      render: (_v, row) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{row.client.email}</Typography.Text>
          {row.client.comment ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>{row.client.comment}</Typography.Text>
          ) : null}
        </Space>
      ),
    },
    {
      title: t('admin.clients.column.inbound'),
      key: 'inbound',
      width: 260,
      render: (_v, row) => (
        <Space direction="vertical" size={0}>
          <Space size={6}>
            <Tag>{row.inbound.protocol}</Tag>
            <Typography.Text strong>{row.inbound.tag}</Typography.Text>
          </Space>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {row.nodeName} · :{row.inbound.port}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.clients.column.user'),
      key: 'user',
      width: 180,
      render: (_v, row) =>
        row.client.subId && userById.has(Number(row.client.subId))
          ? <Typography.Text>{userById.get(Number(row.client.subId))}</Typography.Text>
          : <Typography.Text type="secondary">{t('admin.clients.unbound')}</Typography.Text>,
    },
    {
      title: t('admin.clients.column.traffic'),
      key: 'traffic',
      width: 180,
      render: (_v, row) => {
        const used = trafficUsed(row)
        const quota = quotaBytes(row.client)
        return (
          <Space direction="vertical" size={0}>
            <Typography.Text>{formatBytes(used)} / {formatLimit(quota, t('admin.clients.unlimited'))}</Typography.Text>
            {row.client.reset && row.client.reset > 0 ? (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t('admin.clients.resetEvery', { hours: row.client.reset })}
              </Typography.Text>
            ) : null}
          </Space>
        )
      },
    },
    {
      title: t('admin.clients.column.expiry'),
      key: 'expiry',
      width: 180,
      render: (_v, row) => (
        <Typography.Text>{formatExpiry(row.client.expiryTime, t('admin.clients.never'))}</Typography.Text>
      ),
    },
    {
      title: t('admin.clients.column.enabled'),
      key: 'enabled',
      width: 90,
      align: 'center',
      render: (_v, row) => (
        <Switch
          checked={row.client.enable !== false}
          loading={updateClient.isPending}
          aria-label={`${row.client.enable !== false ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${row.client.email}`}
          onChange={() => toggleEnable(row)}
        />
      ),
    },
    {
      title: t('admin.clients.column.actions'),
      key: 'actions',
      width: 160,
      align: 'center',
      className: 'table-cell-actions',
      render: (_v, row) => {
        const fleetRow: FleetInbound = { node_id: row.nodeID, node_name: row.nodeName, inbound: row.inbound }
        const link = buildClientLink(fleetRow, row.client, nodes)
        return (
          <Space>
            <Button
              aria-label={`${t('admin.inbounds.qrInbound')} ${row.client.email}`}
              icon={<QrcodeOutlined />}
              disabled={!link}
              onClick={() => showQrFor(row)}
            />
            <Button
              aria-label={`${t('admin.clients.edit')} ${row.client.email}`}
              icon={<EditOutlined />}
              onClick={() => setEditor({ open: true, mode: 'edit', row })}
            />
            <Button
              danger
              aria-label={`${t('admin.clients.delete')} ${row.client.email}`}
              icon={<DeleteOutlined />}
              onClick={() => confirmDelete(row)}
            />
          </Space>
        )
      },
    },
  ]

  return (
    <div>
      <ConfigListPage<ClientRow>
        title={t('admin.clients.title')}
        subtitle={t('admin.clients.subtitle')}
        actions={
          <>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setEditor({ open: true, mode: 'create', row: null })}
            >
              {t('admin.clients.create')}
            </Button>
            <RefreshButton loading={fleetQuery.isFetching} onClick={() => fleetQuery.refetch()} label={t('common.refresh')} />
          </>
        }
        filters={
          <Space wrap>
            <Input.Search
              allowClear
              aria-label={t('admin.clients.searchPlaceholder')}
              placeholder={t('admin.clients.searchPlaceholder')}
              style={{ width: 260 }}
              onChange={(e) => setQuery(e.target.value)}
            />
            <Select
              mode="multiple"
              aria-label={t('admin.clients.filterNode')}
              placeholder={t('admin.clients.filterNode')}
              style={{ minWidth: 200 }}
              value={nodeFilter}
              options={nodeOptions}
              onChange={(v) => setNodeFilter(v)}
            />
            <Select
              mode="multiple"
              aria-label={t('admin.clients.filterProtocol')}
              placeholder={t('admin.clients.filterProtocol')}
              style={{ minWidth: 200 }}
              value={protocolFilter}
              options={protocolOptions}
              onChange={(v) => setProtocolFilter(v)}
            />
            <Select
              aria-label={t('admin.clients.filterStatus')}
              style={{ width: 140 }}
              value={statusFilter}
              onChange={(v) => setStatusFilter(v)}
              options={[
                { label: t('admin.clients.statusAll'), value: 'all' },
                { label: t('admin.clients.statusEnabled'), value: 'enabled' },
                { label: t('admin.clients.statusDisabled'), value: 'disabled' },
              ]}
            />
            <Tag>{filtered.length} / {rows.length}</Tag>
          </Space>
        }
        alerts={fleetQuery.error ? <Alert type="error" showIcon message={t('admin.clients.loadFailed')} /> : null}
        rowKey="rowKey"
        columns={columns}
        dataSource={filtered}
        loading={fleetQuery.isLoading}
        pagination={{ pageSize: 50, showSizeChanger: true }}
        emptyState={{
          title: t('admin.clients.empty'),
          description: t('admin.clients.emptyDescription'),
          actionLabel: t('admin.clients.create'),
          onAction: () => setEditor({ open: true, mode: 'create', row: null }),
        }}
        mobileCard={(row) => (
          <Card size="small">
            <Space direction="vertical" size={4} style={{ width: '100%' }}>
              <Typography.Text strong>{row.client.email}</Typography.Text>
              <Typography.Text type="secondary">{row.nodeName} · {row.inbound.tag} · {row.inbound.protocol}</Typography.Text>
            </Space>
          </Card>
        )}
      />

      <ClientEditorModal
        state={editor}
        rows={rows}
        users={users}
        busy={addClient.isPending || updateClient.isPending}
        onClose={() => setEditor((s) => ({ ...s, open: false }))}
        onCreate={(payload) => addClient.mutateAsync(payload)}
        onUpdate={(payload) => updateClient.mutateAsync(payload)}
      />

      <Modal title={qr?.title} open={Boolean(qr)} onCancel={() => setQr(null)} footer={null} destroyOnHidden>
        {qr ? (
          <Space direction="vertical" align="center" style={{ width: '100%' }}>
            <QRCode type="svg" value={qr.url} size={260} />
            <Input.TextArea readOnly value={qr.url} rows={4} />
          </Space>
        ) : null}
      </Modal>
    </div>
  )
}

// ---- Editor Modal ----------------------------------------------------------

interface CreatePayload {
  nodeID: number
  tag: string
  client: Partial<Client>
  userID?: number
}

interface UpdatePayload {
  nodeID: number
  tag: string
  email: string
  client: Partial<Client>
}

interface UserOption {
  id: number
  email?: string | null
}

interface ClientEditorModalProps {
  state: EditorState
  rows: ClientRow[]
  users: UserOption[]
  busy: boolean
  onClose: () => void
  onCreate: (payload: CreatePayload) => Promise<unknown>
  onUpdate: (payload: UpdatePayload) => Promise<unknown>
}

interface ClientFormValues {
  inboundKey?: string
  email: string
  password?: string
  uuid?: string
  flow?: string
  totalGB?: number
  expiryTime?: string
  comment?: string
  enable: boolean
  userID?: number
}

function clientFormDefaults(row: ClientRow | null): ClientFormValues {
  if (!row) return { email: '', enable: true }
  const c = row.client
  return {
    inboundKey: `${row.nodeID}|${row.inbound.tag}`,
    email: c.email ?? '',
    password: c.password ?? '',
    uuid: c.id ?? '',
    flow: c.flow ?? '',
    totalGB: c.totalGB ?? 0,
    expiryTime: c.expiryTime && c.expiryTime > 0 ? new Date(c.expiryTime).toISOString().slice(0, 16) : '',
    comment: c.comment ?? '',
    enable: c.enable !== false,
    userID: c.subId ? Number(c.subId) || undefined : undefined,
  }
}

function ClientEditorModal({ state, rows, users, busy, onClose, onCreate, onUpdate }: ClientEditorModalProps) {
  const { t } = useTranslation()
  const [form] = Form.useForm<ClientFormValues>()

  const inboundOptions = useMemo(() => {
    const seen = new Set<string>()
    const opts: Array<{ label: string; value: string; protocol: string }> = []
    for (const r of rows) {
      const key = `${r.nodeID}|${r.inbound.tag}`
      if (seen.has(key)) continue
      seen.add(key)
      opts.push({
        label: `${r.nodeName} · ${r.inbound.tag} (${r.inbound.protocol}:${r.inbound.port})`,
        value: key,
        protocol: r.inbound.protocol,
      })
    }
    return opts
  }, [rows])

  const inboundKey = Form.useWatch('inboundKey', form)
  const selectedProtocol = inboundOptions.find((o) => o.value === inboundKey)?.protocol ?? ''

  // Reset form values when modal opens.
  useMemo(() => {
    if (state.open) {
      form.setFieldsValue(clientFormDefaults(state.row))
    }
  }, [state.open, state.row, form])

  const save = async () => {
    const values = await form.validateFields().catch(() => null)
    if (!values) return
    const expiry = values.expiryTime ? new Date(values.expiryTime).getTime() : 0
    const clientPayload: Partial<Client> = {
      email: values.email.trim(),
      enable: values.enable,
      totalGB: values.totalGB ?? 0,
      expiryTime: expiry,
      comment: values.comment?.trim() ?? '',
    }
    // Protocol-specific identity field.
    const proto = state.mode === 'edit' ? state.row?.inbound.protocol : selectedProtocol
    if (proto === 'vless' || proto === 'vmess') {
      clientPayload.id = values.uuid?.trim() || crypto.randomUUID()
      if (proto === 'vless' && values.flow) clientPayload.flow = values.flow.trim()
    } else if (proto === 'trojan' || proto === 'shadowsocks') {
      clientPayload.password = values.password?.trim() || ''
    } else if (proto === 'hysteria') {
      clientPayload.auth = values.password?.trim() || ''
    }
    if (values.userID) clientPayload.subId = String(values.userID)

    if (state.mode === 'create') {
      if (!values.inboundKey) return
      const [nodeIDStr, tag] = values.inboundKey.split('|')
      await onCreate({
        nodeID: Number(nodeIDStr),
        tag,
        client: clientPayload,
        userID: values.userID,
      })
    } else if (state.row) {
      await onUpdate({
        nodeID: state.row.nodeID,
        tag: state.row.inbound.tag,
        email: state.row.client.email,
        client: clientPayload,
      })
    }
    onClose()
  }

  const usingPassword = selectedProtocol === 'trojan' || selectedProtocol === 'shadowsocks'
  const usingAuth = selectedProtocol === 'hysteria'
  const usingUUID = selectedProtocol === 'vless' || selectedProtocol === 'vmess'

  return (
    <Modal
      title={state.mode === 'create' ? t('admin.clients.createTitle') : t('admin.clients.editTitle', { email: state.row?.client.email ?? '' })}
      open={state.open}
      onCancel={onClose}
      onOk={save}
      confirmLoading={busy}
      okText={state.mode === 'create' ? t('common.create') : t('common.save')}
      cancelText={t('admin.inboundEditor.close')}
      destroyOnHidden
      width={680}
    >
      <Form form={form} layout="vertical">
        {state.mode === 'create' ? (
          <Form.Item
            name="inboundKey"
            label={t('admin.clients.field.inbound')}
            rules={[{ required: true, message: t('admin.clients.field.inboundRequired') }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={t('admin.clients.field.inboundPlaceholder')}
              options={inboundOptions}
            />
          </Form.Item>
        ) : null}
        <Form.Item
          name="email"
          label="Email"
          rules={[{ required: true, message: t('admin.clients.field.emailRequired') }]}
        >
          <Input placeholder="alice@example.com" disabled={state.mode === 'edit'} />
        </Form.Item>
        {usingUUID ? (
          <Form.Item name="uuid" label="UUID" tooltip={t('admin.clients.field.uuidHint')}>
            <Input placeholder={t('admin.clients.field.uuidPlaceholder')} />
          </Form.Item>
        ) : null}
        {selectedProtocol === 'vless' ? (
          <Form.Item name="flow" label="Flow">
            <Input placeholder="xtls-rprx-vision" />
          </Form.Item>
        ) : null}
        {usingPassword ? (
          <Form.Item name="password" label={t('admin.clients.field.password')}>
            <Input placeholder={t('admin.clients.field.passwordPlaceholder')} />
          </Form.Item>
        ) : null}
        {usingAuth ? (
          <Form.Item name="password" label={t('admin.clients.field.auth')}>
            <Input placeholder={t('admin.clients.field.authPlaceholder')} />
          </Form.Item>
        ) : null}
        <Space align="start" wrap>
          <Form.Item name="totalGB" label={t('admin.clients.field.totalGB')}>
            <InputNumber min={0} step={0.1} style={{ width: 160 }} />
          </Form.Item>
          <Form.Item name="expiryTime" label={t('admin.clients.field.expiry')}>
            <Input type="datetime-local" style={{ width: 220 }} />
          </Form.Item>
          <Form.Item name="enable" label={t('admin.clients.field.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
        <Form.Item name="userID" label={t('admin.clients.field.bindUser')} tooltip={t('admin.clients.field.bindUserHint')}>
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder={t('admin.clients.field.bindUserPlaceholder')}
            options={users.map((u) => ({ label: u.email ?? `#${u.id}`, value: u.id }))}
          />
        </Form.Item>
        <Form.Item name="comment" label={t('admin.clients.field.comment')}>
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  )
}
