import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  QrcodeOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Input, Modal, QRCode, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo, useState } from 'react'
import type { Client, FleetInbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import {
  useInboundsFleet,
  useRemoveInbound,
  useResetInboundTraffic,
  useSetInboundEnable,
} from '@/hooks/queries/admin/inbounds'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import InboundEditor from './InboundEditor'
import {
  buildClientLink,
  filterInbounds,
  formatBytes,
  formatLimit,
  parseClients,
  parseJSON,
  PROTOCOL_OPTIONS,
  rowKey,
  type ProtocolFilter,
} from './inbounds/utils'

interface EditorState {
  open: boolean
  mode: 'create' | 'edit'
  nodeID: number | null
  tag: string
  source: FleetInbound['inbound'] | null
}

function transportText(row: FleetInbound) {
  const stream = parseJSON(row.inbound.streamSettings)
  return `${row.inbound.protocol} / ${stream.network || 'tcp'} / ${stream.security || 'none'}`
}

export default function Inbounds() {
  const [query, setQuery] = useState('')
  const [protocols, setProtocols] = useState<ProtocolFilter[]>([...PROTOCOL_OPTIONS])
  const [expandedRowKeys, setExpandedRowKeys] = useState<React.Key[]>([])
  const [qr, setQr] = useState<{ title: string; url: string } | null>(null)
  const [editor, setEditor] = useState<EditorState>({ open: false, mode: 'create', nodeID: null, tag: '', source: null })

  const fleetQuery = useInboundsFleet()
  const nodesQuery = useNodesList()
  const setEnable = useSetInboundEnable()
  const removeInbound = useRemoveInbound()
  const resetInboundTraffic = useResetInboundTraffic()

  const rows = useMemo(() => fleetQuery.data?.inbounds ?? [], [fleetQuery.data])
  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data])
  const filtered = useMemo(() => filterInbounds(rows, query, protocols), [protocols, query, rows])
  const loading = fleetQuery.isLoading || nodesQuery.isLoading
  const error = fleetQuery.error ?? nodesQuery.error ?? setEnable.error ?? removeInbound.error ?? resetInboundTraffic.error

  const stats = useMemo(() => {
    const inbounds = rows.map((row) => row.inbound)
    return {
      up: inbounds.reduce((sum, inbound) => sum + (inbound.up || 0), 0),
      down: inbounds.reduce((sum, inbound) => sum + (inbound.down || 0), 0),
      enabled: inbounds.filter((inbound) => inbound.enable).length,
      clients: inbounds.reduce((sum, inbound) => sum + parseClients(inbound).length, 0),
    }
  }, [rows])

  const refresh = () => {
    fleetQuery.refetch()
    nodesQuery.refetch()
  }

  const openCreate = () => {
    setEditor({
      open: true,
      mode: 'create',
      nodeID: nodes.find((node) => node.enabled)?.id ?? null,
      tag: '',
      source: null,
    })
  }

  const openEdit = (row: FleetInbound) => {
    setEditor({ open: true, mode: 'edit', nodeID: row.node_id, tag: row.inbound.tag, source: row.inbound })
  }

  const confirmDelete = (row: FleetInbound) => {
    Modal.confirm({
      title: 'Delete inbound',
      content: `Delete ${row.inbound.tag} on ${row.node_name}?`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => removeInbound.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag }),
    })
  }

  const confirmReset = (row: FleetInbound) => {
    Modal.confirm({
      title: 'Reset inbound traffic',
      content: `Reset traffic for ${row.inbound.tag}?`,
      okText: 'Reset',
      onOk: () => resetInboundTraffic.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag }),
    })
  }

  const showQr = (row: FleetInbound, client: Client) => {
    const url = buildClientLink(row, client, nodes as Node[])
    if (!url) return
    setQr({ title: `${row.inbound.tag} · ${client.email}`, url })
  }

  const columns: ColumnsType<FleetInbound> = [
    {
      title: 'Remark',
      dataIndex: ['inbound', 'remark'],
      render: (_value, row) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{row.inbound.remark || row.inbound.tag}</Typography.Text>
          <Typography.Text type="secondary">
            {row.node_name} #{row.node_id} · tag {row.inbound.tag} · port {row.inbound.port}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Protocol',
      key: 'protocol',
      render: (_value, row) => <Tag>{transportText(row)}</Tag>,
    },
    {
      title: 'Clients',
      key: 'clients',
      align: 'right',
      render: (_value, row) => parseClients(row.inbound).length,
    },
    {
      title: 'Traffic',
      key: 'traffic',
      render: (_value, row) => `${formatBytes(row.inbound.up + row.inbound.down)} / ${formatLimit(row.inbound.total)}`,
    },
    {
      title: 'Enabled',
      dataIndex: ['inbound', 'enable'],
      render: (_value, row) => (
        <Switch
          checked={row.inbound.enable}
          aria-label={`${row.inbound.enable ? 'Disable' : 'Enable'} ${row.inbound.tag}`}
          loading={setEnable.isPending}
          onChange={(enable) => setEnable.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag, enable })}
        />
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, row) => (
        <Space>
          <Button aria-label={`Edit ${row.inbound.tag}`} icon={<EditOutlined />} onClick={() => openEdit(row)} />
          <Button aria-label={`Reset traffic ${row.inbound.tag}`} icon={<ReloadOutlined />} onClick={() => confirmReset(row)} />
          <Button danger aria-label={`Delete ${row.inbound.tag}`} icon={<DeleteOutlined />} onClick={() => confirmDelete(row)} />
        </Space>
      ),
    },
  ]

  const expandedRowRender = (row: FleetInbound) => {
    const clients = parseClients(row.inbound)
    return (
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Typography.Text type="secondary">Clients in {row.inbound.tag}</Typography.Text>
        {clients.length === 0 ? <Typography.Text type="secondary">No clients configured.</Typography.Text> : null}
        {clients.map((client) => {
          const link = buildClientLink(row, client, nodes as Node[])
          return (
            <Card key={client.email} size="small">
              <Space wrap style={{ justifyContent: 'space-between', width: '100%' }}>
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{client.email}</Typography.Text>
                  <Typography.Text type="secondary">{client.enable === false ? 'Disabled' : 'Enabled'} · {formatLimit(client.totalGB ?? 0)}</Typography.Text>
                  {link ? <Typography.Text code>{link}</Typography.Text> : <Typography.Text type="secondary">Link not supported for this protocol.</Typography.Text>}
                </Space>
                <Button aria-label={`Show QR ${client.email}`} icon={<QrcodeOutlined />} disabled={!link} onClick={() => showQr(row, client)}>
                  QR
                </Button>
              </Space>
            </Card>
          )
        })}
      </Space>
    )
  }

  return (
    <div>
      <PageHeader
        title="Inbounds"
        subtitle="Manage inbound listeners and client subscription links."
        actions={
          <>
            <Button type="primary" aria-label="New Inbound" icon={<PlusOutlined />} onClick={openCreate}>
              New Inbound
            </Button>
            <RefreshButton loading={fleetQuery.isFetching || nodesQuery.isFetching} onClick={refresh} />
          </>
        }
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Input.Search allowClear aria-label="Search inbounds" placeholder="Search inbounds" style={{ width: 260 }} onChange={(event) => setQuery(event.target.value)} />
        <Select
          mode="multiple"
          aria-label="Filter protocols"
          value={protocols}
          style={{ minWidth: 320 }}
          options={PROTOCOL_OPTIONS.map((protocol) => ({ label: protocol, value: protocol }))}
          onChange={(value) => setProtocols(value)}
        />
        <Tag>{filtered.length} / {rows.length}</Tag>
      </Space>

      <Space wrap style={{ marginBottom: 16 }}>
        <Card size="small">Sent / Received: {formatBytes(stats.up)} / {formatBytes(stats.down)}</Card>
        <Card size="small">Inbounds: {rows.length} ({stats.enabled} enabled)</Card>
        <Card size="small">Clients: {stats.clients}</Card>
      </Space>

      {error ? <Alert type="error" showIcon message="Inbound operation failed" style={{ marginBottom: 16 }} /> : null}
      {fleetQuery.data?.node_errors && Object.keys(fleetQuery.data.node_errors).length ? (
        <Alert
          type="warning"
          showIcon
          message="Node errors"
          description={Object.entries(fleetQuery.data.node_errors).map(([id, message]) => `node ${id}: ${message}`).join('\n')}
          style={{ marginBottom: 16 }}
        />
      ) : null}

      {filtered.length > 0 || loading ? (
        <ResponsiveListTable
          rowKey={rowKey}
          columns={columns}
          dataSource={filtered}
          loading={loading}
          pagination={false}
          expandable={{
            expandedRowKeys,
            onExpandedRowsChange: (keys) => setExpandedRowKeys([...keys]),
            expandedRowRender,
          }}
          mobileCard={(row) => (
            <Card size="small" style={{ width: '100%' }}>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                  <Typography.Text strong>{row.inbound.remark || row.inbound.tag}</Typography.Text>
                  <Tag>{row.inbound.protocol}</Tag>
                </Space>
                <Typography.Text type="secondary">{row.node_name} #{row.node_id} · {row.inbound.port}</Typography.Text>
                <Typography.Text>Clients: {parseClients(row.inbound).length}</Typography.Text>
                <Typography.Text>Traffic: {formatBytes(row.inbound.up + row.inbound.down)} / {formatLimit(row.inbound.total)}</Typography.Text>
                <Space wrap>
                  <Switch
                    checked={row.inbound.enable}
                    aria-label={`${row.inbound.enable ? 'Disable' : 'Enable'} ${row.inbound.tag}`}
                    onChange={(enable) => setEnable.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag, enable })}
                  />
                  <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
                    Edit
                  </Button>
                  <Button size="small" icon={<ReloadOutlined />} onClick={() => confirmReset(row)}>
                    Reset
                  </Button>
                  <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(row)}>
                    Delete
                  </Button>
                </Space>
              </Space>
            </Card>
          )}
        />
      ) : (
        <EmptyState title="No inbounds" description="Create an inbound listener to start provisioning clients." actionLabel="New Inbound" onAction={openCreate} />
      )}

      <InboundEditor
        open={editor.open}
        mode={editor.mode}
        nodeID={editor.nodeID}
        tag={editor.tag}
        source={editor.source}
        nodes={nodes}
        onClose={() => setEditor((prev) => ({ ...prev, open: false }))}
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
