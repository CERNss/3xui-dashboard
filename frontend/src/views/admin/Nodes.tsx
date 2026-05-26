import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  ImportOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, Modal, Select, Space, Switch, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useMemo, useRef, useState } from 'react'
import type { Node, NodeInput } from '@/api/admin/nodes'
import { EmptyState, PageHeader, RefreshButton, ResponsiveListTable } from '@/components/common'
import {
  useCreateNode,
  useDisableNode,
  useEnableNode,
  useNodesList,
  useProbeNode,
  useRemoveNode,
  useUpdateNode,
} from '@/hooks/queries/admin/nodes'
import { NodeDrawer } from './nodes/NodeDrawer'
import {
  AREA_OPTIONS,
  blankNodeForm,
  formatLastSeen,
  formToPayload,
  nodeConnectionURL,
  nodeDisplayStatus,
  nodeExportRows,
  nodeLocationText,
  nodeToForm,
  normalizeNodeArea,
  normalizeNodeProvince,
  panelInboundURL,
  statusColor,
  statusLabel,
  type NodeDisplayStatus,
  type NodeFormValues,
} from './nodes/utils'

interface NodeFilters {
  query?: string
  area?: string
  province?: string
  scheme?: string
  status?: string
}

const STATUS_OPTIONS: Array<{ label: string; value: NodeDisplayStatus }> = [
  { label: 'Online', value: 'online' },
  { label: 'Offline', value: 'offline' },
  { label: 'Unknown', value: 'unknown' },
  { label: 'Disabled', value: 'disabled' },
]

function cleanFilters(filters: NodeFilters): NodeFilters {
  return {
    query: filters.query?.trim() || undefined,
    area: filters.area || undefined,
    province: filters.province?.trim() || undefined,
    scheme: filters.scheme || undefined,
    status: filters.status || undefined,
  }
}

function nodeInputFromImport(row: Partial<NodeInput>): NodeInput {
  return {
    name: String(row.name ?? ''),
    area: normalizeNodeArea(String(row.area ?? '')),
    province: normalizeNodeProvince(String(row.province ?? '')),
    scheme: row.scheme === 'http' ? 'http' : 'https',
    host: String(row.host ?? ''),
    port: Number(row.port ?? 2053),
    base_path: String(row.base_path ?? ''),
    api_token: String(row.api_token ?? ''),
    enabled: Boolean(row.enabled ?? true),
  }
}

export default function Nodes() {
  const [form] = Form.useForm<NodeFormValues>()
  const importInputRef = useRef<HTMLInputElement | null>(null)
  const [draftFilters, setDraftFilters] = useState<NodeFilters>({})
  const [filters, setFilters] = useState<NodeFilters>({})
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editing, setEditing] = useState<Node | null>(null)
  const [importing, setImporting] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => setFilters(cleanFilters(draftFilters)), 250)
    return () => window.clearTimeout(timer)
  }, [draftFilters])

  const nodesQuery = useNodesList(filters)
  const createNode = useCreateNode()
  const updateNode = useUpdateNode()
  const removeNode = useRemoveNode()
  const enableNode = useEnableNode()
  const disableNode = useDisableNode()
  const probeNode = useProbeNode()

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data])
  const busy =
    createNode.isPending ||
    updateNode.isPending ||
    removeNode.isPending ||
    enableNode.isPending ||
    disableNode.isPending ||
    probeNode.isPending ||
    importing
  const error =
    localError ??
    nodesQuery.error ??
    createNode.error ??
    updateNode.error ??
    removeNode.error ??
    enableNode.error ??
    disableNode.error ??
    probeNode.error

  useEffect(() => {
    setSelectedRowKeys((keys) => keys.filter((key) => nodes.some((node) => node.id === key)))
  }, [nodes])

  const counts = useMemo(() => {
    return nodes.reduce(
      (acc, node) => {
        acc[nodeDisplayStatus(node)] += 1
        return acc
      },
      { disabled: 0, offline: 0, online: 0, unknown: 0 } as Record<NodeDisplayStatus, number>,
    )
  }, [nodes])

  const refresh = () => nodesQuery.refetch()

  const openCreate = () => {
    setEditing(null)
    form.setFieldsValue(blankNodeForm())
    setDrawerOpen(true)
  }

  const openEdit = (node: Node) => {
    setEditing(node)
    form.setFieldsValue(nodeToForm(node))
    setDrawerOpen(true)
  }

  const closeDrawer = () => {
    setDrawerOpen(false)
    setEditing(null)
    form.resetFields()
  }

  const submit = async () => {
    setLocalError(null)
    const values = await form.validateFields().catch(() => null)
    if (!values) return
    const payload = formToPayload(values)
    if (editing) {
      await updateNode.mutateAsync({ id: editing.id, body: payload })
    } else {
      await createNode.mutateAsync(payload)
    }
    closeDrawer()
  }

  const toggleEnable = async (node: Node) => {
    if (node.enabled) await disableNode.mutateAsync(node.id)
    else await enableNode.mutateAsync(node.id)
  }

  const confirmDelete = (node: Node) => {
    Modal.confirm({
      title: 'Delete node',
      content: `Delete ${node.name}? This only removes the dashboard registration.`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: () => removeNode.mutateAsync(node.id),
    })
  }

  const batchProbe = async () => {
    const targets = selectedRowKeys.length > 0 ? nodes.filter((node) => selectedRowKeys.includes(node.id)) : nodes
    await Promise.all(targets.map((node) => probeNode.mutateAsync(node.id)))
    setSelectedRowKeys([])
  }

  const batchDelete = () => {
    if (selectedRowKeys.length === 0) return
    Modal.confirm({
      title: 'Delete selected nodes',
      content: `Delete ${selectedRowKeys.length} selected node(s)?`,
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: async () => {
        await Promise.all(selectedRowKeys.map((id) => removeNode.mutateAsync(Number(id))))
        setSelectedRowKeys([])
      },
    })
  }

  const exportJson = () => {
    const blob = new Blob([JSON.stringify({ nodes: nodeExportRows(nodes) }, null, 2)], {
      type: 'application/json;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `nodes-${new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)}.json`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const importJson = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setImporting(true)
    setLocalError(null)
    try {
      const parsed = JSON.parse(await file.text()) as { nodes?: Partial<NodeInput>[] } | Partial<NodeInput>[]
      const rows = Array.isArray(parsed) ? parsed : parsed.nodes
      if (!Array.isArray(rows)) {
        setLocalError('Import file must contain a nodes array.')
        return
      }
      await Promise.all(rows.map((row) => createNode.mutateAsync(nodeInputFromImport(row))))
      await nodesQuery.refetch()
    } catch {
      setLocalError('Import file must be valid JSON.')
    } finally {
      setImporting(false)
    }
  }

  const columns: ColumnsType<Node> = [
    {
      title: 'Name',
      dataIndex: 'name',
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{node.name}</Typography.Text>
          <Typography.Text type="secondary">#{node.id} · {nodeLocationText(node)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      sorter: (a, b) => statusLabel(nodeDisplayStatus(a)).localeCompare(statusLabel(nodeDisplayStatus(b))),
      render: (_value, node) => {
        const status = nodeDisplayStatus(node)
        return <Tag color={statusColor(status)}>{statusLabel(status)}</Tag>
      },
    },
    {
      title: 'CPU / Mem',
      key: 'cpuMem',
      align: 'right',
      render: (_value, node) => `${node.cpu_pct.toFixed(1)}% / ${node.mem_pct.toFixed(1)}%`,
    },
    {
      title: 'Xray',
      dataIndex: 'xray_version',
      render: (value: string) => value || '-',
    },
    {
      title: 'Last seen',
      dataIndex: 'last_seen_at',
      render: (value?: string | null) => formatLastSeen(value),
    },
    {
      title: 'Endpoint',
      key: 'endpoint',
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text code>{nodeConnectionURL(node)}</Typography.Text>
          <a href={panelInboundURL(node)} target="_blank" rel="noreferrer">
            Open panel
          </a>
        </Space>
      ),
    },
    {
      title: 'Enabled',
      dataIndex: 'enabled',
      render: (_value, node) => (
        <Switch
          checked={node.enabled}
          aria-label={`${node.enabled ? 'Disable' : 'Enable'} ${node.name}`}
          loading={enableNode.isPending || disableNode.isPending}
          onChange={() => toggleEnable(node)}
        />
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right',
      render: (_value, node) => (
        <Space>
          <Button aria-label={`Probe ${node.name}`} icon={<ThunderboltOutlined />} onClick={() => probeNode.mutateAsync(node.id)} />
          <Button aria-label={`Edit ${node.name}`} icon={<EditOutlined />} onClick={() => openEdit(node)} />
          <Button danger aria-label={`Delete ${node.name}`} icon={<DeleteOutlined />} onClick={() => confirmDelete(node)} />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title="Nodes"
        subtitle="Register, probe, and maintain 3x-ui panel nodes."
        actions={
          <>
            <input
              ref={importInputRef}
              hidden
              type="file"
              accept="application/json,.json"
              aria-label="Import nodes file"
              onChange={importJson}
            />
            <Button aria-label="Probe batch" icon={<ThunderboltOutlined />} disabled={busy || nodes.length === 0} onClick={batchProbe}>
              Probe batch
            </Button>
            <Button aria-label="Delete batch" danger icon={<DeleteOutlined />} disabled={busy || selectedRowKeys.length === 0} onClick={batchDelete}>
              Delete batch
            </Button>
            <Button aria-label="Import" icon={<ImportOutlined />} loading={importing} onClick={() => importInputRef.current?.click()}>
              Import
            </Button>
            <Button aria-label="Export" icon={<DownloadOutlined />} disabled={nodes.length === 0} onClick={exportJson}>
              Export
            </Button>
            <Button aria-label="New Node" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              New Node
            </Button>
            <RefreshButton loading={nodesQuery.isFetching} onClick={refresh} />
          </>
        }
      />

      <Space wrap style={{ marginBottom: 16 }}>
        <Input.Search
          allowClear
          aria-label="Search nodes"
          placeholder="Search nodes"
          style={{ width: 240 }}
          onChange={(event) => setDraftFilters((prev) => ({ ...prev, query: event.target.value }))}
        />
        <Select
          allowClear
          aria-label="Filter area"
          placeholder="All areas"
          style={{ width: 180 }}
          options={AREA_OPTIONS.map((area) => ({ label: area.label, value: area.key }))}
          onChange={(area) => setDraftFilters((prev) => ({ ...prev, area }))}
        />
        <Input
          allowClear
          aria-label="Filter province"
          placeholder="Province"
          style={{ width: 160 }}
          onChange={(event) => setDraftFilters((prev) => ({ ...prev, province: event.target.value }))}
        />
        <Select
          allowClear
          aria-label="Filter scheme"
          placeholder="Scheme"
          style={{ width: 140 }}
          options={[
            { label: 'HTTPS', value: 'https' },
            { label: 'HTTP', value: 'http' },
          ]}
          onChange={(scheme) => setDraftFilters((prev) => ({ ...prev, scheme }))}
        />
        <Select
          allowClear
          aria-label="Filter status"
          placeholder="Status"
          style={{ width: 150 }}
          options={STATUS_OPTIONS}
          onChange={(status) => setDraftFilters((prev) => ({ ...prev, status }))}
        />
      </Space>

      {error ? <Alert type="error" showIcon message={typeof error === 'string' ? error : 'Node operation failed'} style={{ marginBottom: 16 }} /> : null}

      {nodes.length > 0 || nodesQuery.isLoading ? (
        <ResponsiveListTable
          rowKey="id"
          columns={columns}
          dataSource={nodes}
          loading={nodesQuery.isLoading}
          pagination={false}
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          mobileCard={(node) => {
            const status = nodeDisplayStatus(node)
            return (
              <Card size="small" style={{ width: '100%' }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Typography.Text strong>{node.name}</Typography.Text>
                    <Tag color={statusColor(status)}>{statusLabel(status)}</Tag>
                  </Space>
                  <Typography.Text type="secondary">{nodeLocationText(node)}</Typography.Text>
                  <Typography.Text>CPU / Mem: {node.cpu_pct.toFixed(1)}% / {node.mem_pct.toFixed(1)}%</Typography.Text>
                  <Typography.Text>Xray: {node.xray_version || '-'}</Typography.Text>
                  <Typography.Text>Last seen: {formatLastSeen(node.last_seen_at)}</Typography.Text>
                  <Typography.Text code>{nodeConnectionURL(node)}</Typography.Text>
                  <Space wrap>
                    <Switch
                      checked={node.enabled}
                      aria-label={`${node.enabled ? 'Disable' : 'Enable'} ${node.name}`}
                      onChange={() => toggleEnable(node)}
                    />
                    <Button size="small" icon={<ThunderboltOutlined />} onClick={() => probeNode.mutateAsync(node.id)}>
                      Probe
                    </Button>
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(node)}>
                      Edit
                    </Button>
                    <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(node)}>
                      Delete
                    </Button>
                  </Space>
                </Space>
              </Card>
            )
          }}
        />
      ) : (
        <EmptyState title="No nodes" description="Create a node to connect this dashboard to a 3x-ui panel." actionLabel="New Node" onAction={openCreate} />
      )}

      <Space wrap style={{ marginTop: 16 }}>
        <Typography.Text type="secondary">Showing {nodes.length} node(s)</Typography.Text>
        <Tag color="green">Online {counts.online}</Tag>
        <Tag color="red">Offline {counts.offline}</Tag>
        <Tag>Disabled {counts.disabled}</Tag>
      </Space>

      <NodeDrawer
        form={form}
        open={drawerOpen}
        editingName={editing?.name}
        saving={createNode.isPending || updateNode.isPending}
        onClose={closeDrawer}
        onSubmit={submit}
      />
    </div>
  )
}
