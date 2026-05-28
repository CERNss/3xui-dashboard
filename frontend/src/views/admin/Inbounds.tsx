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
import { useTranslation } from 'react-i18next'
import type { Client, FleetInbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'
import { ConfigListPage, RefreshButton } from '@/components/common'
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
  const { t } = useTranslation()
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
  const nodeErrors = fleetQuery.data?.node_errors
  const hasNodeErrors = Boolean(nodeErrors && Object.keys(nodeErrors).length)

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
      title: t('admin.inbounds.confirmDelete'),
      content: t('admin.inbounds.confirmDeleteMsg', {
        nodeName: row.node_name,
        port: row.inbound.port,
        tag: row.inbound.tag,
      }),
      okText: t('admin.inbounds.delete'),
      okButtonProps: { danger: true },
      onOk: () => removeInbound.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag }),
    })
  }

  const confirmReset = (row: FleetInbound) => {
    Modal.confirm({
      title: t('admin.inbounds.confirmReset'),
      content: t('admin.inbounds.confirmResetMsg', { tag: row.inbound.tag }),
      okText: t('admin.inbounds.reset'),
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
      title: t('admin.inbounds.column.remark'),
      dataIndex: ['inbound', 'remark'],
      width: 280,
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
      title: t('admin.inbounds.column.protocol'),
      key: 'protocol',
      align: 'center',
      width: 120,
      render: (_value, row) => <Tag>{transportText(row)}</Tag>,
    },
    {
      title: t('admin.inbounds.column.clients'),
      key: 'clients',
      align: 'right',
      className: 'table-cell-number',
      width: 100,
      render: (_value, row) => parseClients(row.inbound).length,
    },
    {
      title: t('admin.inbounds.column.traffic'),
      key: 'traffic',
      className: 'table-cell-number',
      width: 190,
      render: (_value, row) => `${formatBytes(row.inbound.up + row.inbound.down)} / ${formatLimit(row.inbound.total, t('admin.stats.unlimited'))}`,
    },
    {
      title: t('admin.inbounds.column.enable'),
      dataIndex: ['inbound', 'enable'],
      align: 'center',
      width: 96,
      render: (_value, row) => (
        <Switch
          checked={row.inbound.enable}
          aria-label={`${row.inbound.enable ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${row.inbound.tag}`}
          loading={setEnable.isPending}
          onChange={(enable) => setEnable.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag, enable })}
        />
      ),
    },
    {
      title: t('admin.users.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 144,
      render: (_value, row) => (
        <Space>
          <Button aria-label={`${t('admin.inbounds.edit')} ${row.inbound.tag}`} icon={<EditOutlined />} onClick={() => openEdit(row)} />
          <Button aria-label={`${t('admin.inbounds.resetInboundTraffic')} ${row.inbound.tag}`} icon={<ReloadOutlined />} onClick={() => confirmReset(row)} />
          <Button danger aria-label={`${t('admin.inbounds.delete')} ${row.inbound.tag}`} icon={<DeleteOutlined />} onClick={() => confirmDelete(row)} />
        </Space>
      ),
    },
  ]

  const expandedRowRender = (row: FleetInbound) => {
    const clients = parseClients(row.inbound)
    return (
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Typography.Text type="secondary">{t('admin.inbounds.client.in')} {row.inbound.tag}</Typography.Text>
        {clients.length === 0 ? <Typography.Text type="secondary">{t('admin.inbounds.client.emptyHint')}</Typography.Text> : null}
        {clients.map((client) => {
          const link = buildClientLink(row, client, nodes as Node[])
          return (
            <Card key={client.email} size="small">
              <Space wrap style={{ justifyContent: 'space-between', width: '100%' }}>
                <Space direction="vertical" size={2}>
                  <Typography.Text strong>{client.email}</Typography.Text>
                  <Typography.Text type="secondary">{client.enable === false ? t('admin.status.nodeState.disabled') : t('admin.nodes.enable')} · {formatLimit(client.totalGB ?? 0, t('admin.stats.unlimited'))}</Typography.Text>
                  {link ? <Typography.Text code>{link}</Typography.Text> : <Typography.Text type="secondary">{t('admin.inbounds.protocolNotSupported')}</Typography.Text>}
                </Space>
                <Button aria-label={`${t('admin.inbounds.qrInbound')} ${client.email}`} icon={<QrcodeOutlined />} disabled={!link} onClick={() => showQr(row, client)}>
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
      <ConfigListPage
        title={t('admin.inbounds.title')}
        subtitle={t('admin.inbounds.subtitle')}
        actions={
          <>
            <Button type="primary" aria-label={t('admin.inbounds.addInbound')} icon={<PlusOutlined />} onClick={openCreate}>
              {t('admin.inbounds.addInbound')}
            </Button>
            <RefreshButton loading={fleetQuery.isFetching || nodesQuery.isFetching} onClick={refresh} label={t('admin.inbounds.reload')} />
          </>
        }
        filters={
          <Space wrap>
            <Input.Search allowClear aria-label={t('admin.inbounds.searchPlaceholder')} placeholder={t('admin.inbounds.searchPlaceholder')} style={{ width: 260 }} onChange={(event) => setQuery(event.target.value)} />
            <Select
              mode="multiple"
              aria-label={t('admin.inbounds.filter.protocolLabel')}
              value={protocols}
              style={{ minWidth: 320 }}
              options={PROTOCOL_OPTIONS.map((protocol) => ({ label: protocol, value: protocol }))}
              onChange={(value) => setProtocols(value)}
            />
            <Tag>{filtered.length} / {rows.length}</Tag>
          </Space>
        }
        footer={
          <div className="inbounds-list-footer">
            <span className="config-list-page-footer-summary">{t('common.resultCount', { n: filtered.length })}</span>
            <Space className="inbounds-list-footer-metrics" size={[12, 4]} wrap>
              <Typography.Text type="secondary">{t('admin.inbounds.kpi.sentReceived')}: {formatBytes(stats.up)} / {formatBytes(stats.down)}</Typography.Text>
              <Typography.Text type="secondary">{t('admin.inbounds.kpi.inbounds')}: {rows.length} ({stats.enabled} {t('admin.inbounds.kpi.enabledSuffix')})</Typography.Text>
              <Typography.Text type="secondary">{t('admin.inbounds.kpi.clients')}: {stats.clients}</Typography.Text>
            </Space>
          </div>
        }
        alerts={
          error || hasNodeErrors ? (
            <>
            {error ? <Alert type="error" showIcon message={t('admin.inbounds.operationFailed')} /> : null}
            {hasNodeErrors ? (
              <Alert
                type="warning"
                showIcon
                message={t('admin.inbounds.nodeErrorsTitle')}
                description={Object.entries(nodeErrors ?? {}).map(([id, message]) => `${t('admin.opsMonitor.nodeId', { id })}: ${message}`).join('\n')}
                style={{ marginTop: error ? 12 : 0 }}
              />
            ) : null}
            </>
          ) : null
        }
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
        emptyState={{
          title: t('admin.inbounds.empty'),
          description: t('admin.inbounds.emptyDescription'),
          actionLabel: t('admin.inbounds.addInbound'),
          onAction: openCreate,
        }}
        mobileCard={(row) => (
          <Card size="small" style={{ width: '100%' }}>
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>{row.inbound.remark || row.inbound.tag}</Typography.Text>
                <Tag>{row.inbound.protocol}</Tag>
              </Space>
              <Typography.Text type="secondary">{row.node_name} #{row.node_id} · {row.inbound.port}</Typography.Text>
              <Typography.Text>{t('admin.inbounds.column.clients')}: {parseClients(row.inbound).length}</Typography.Text>
              <Typography.Text>{t('admin.inbounds.column.traffic')}: {formatBytes(row.inbound.up + row.inbound.down)} / {formatLimit(row.inbound.total, t('admin.stats.unlimited'))}</Typography.Text>
              <Space wrap>
                <Switch
                  checked={row.inbound.enable}
                  aria-label={`${row.inbound.enable ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${row.inbound.tag}`}
                  onChange={(enable) => setEnable.mutateAsync({ nodeID: row.node_id, tag: row.inbound.tag, enable })}
                />
                <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(row)}>
                  {t('admin.inbounds.edit')}
                </Button>
                <Button size="small" icon={<ReloadOutlined />} onClick={() => confirmReset(row)}>
                  {t('admin.inbounds.reset')}
                </Button>
                <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(row)}>
                  {t('admin.inbounds.delete')}
                </Button>
              </Space>
            </Space>
          </Card>
        )}
      />

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
