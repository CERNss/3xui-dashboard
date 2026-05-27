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
import { useTranslation } from 'react-i18next'
import type { Node, NodeInput } from '@/api/admin/nodes'
import { ConfigListPage, RefreshButton } from '@/components/common'
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
  nodeToForm,
  normalizeNodeArea,
  normalizeNodeProvince,
  panelInboundURL,
  statusColor,
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
  const { t } = useTranslation()
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
  const labelForStatus = (status: NodeDisplayStatus) =>
    status === 'disabled' ? t('admin.status.nodeState.disabled') : t(`admin.nodes.status.${status}`)
  const locationText = (node: Node) => `${t(`admin.nodes.area.${normalizeNodeArea(node.area)}`)} / ${normalizeNodeProvince(node.province)}`
  const statusOptions: Array<{ label: string; value: NodeDisplayStatus }> = [
    { label: labelForStatus('online'), value: 'online' },
    { label: labelForStatus('offline'), value: 'offline' },
    { label: labelForStatus('unknown'), value: 'unknown' },
    { label: labelForStatus('disabled'), value: 'disabled' },
  ]

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
      title: t('admin.nodes.confirmDelete'),
      content: t('admin.nodes.confirmDeleteMsg', { name: node.name }),
      okText: t('admin.nodes.delete'),
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
      title: t('admin.nodes.batch.deleteTitle'),
      content: t('admin.nodes.batch.deleteMsg', { n: selectedRowKeys.length }),
      okText: t('admin.nodes.delete'),
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
        setLocalError(t('admin.nodes.importMissingNodes'))
        return
      }
      await Promise.all(rows.map((row) => createNode.mutateAsync(nodeInputFromImport(row))))
      await nodesQuery.refetch()
    } catch {
      setLocalError(t('admin.nodes.importInvalid'))
    } finally {
      setImporting(false)
    }
  }

  const columns: ColumnsType<Node> = [
    {
      title: t('admin.nodes.column.name'),
      dataIndex: 'name',
      width: 240,
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{node.name}</Typography.Text>
          <Typography.Text type="secondary">#{node.id} · {locationText(node)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('admin.nodes.column.status'),
      dataIndex: 'status',
      align: 'center',
      width: 120,
      sorter: (a, b) => labelForStatus(nodeDisplayStatus(a)).localeCompare(labelForStatus(nodeDisplayStatus(b))),
      render: (_value, node) => {
        const status = nodeDisplayStatus(node)
        return <Tag color={statusColor(status)}>{labelForStatus(status)}</Tag>
      },
    },
    {
      title: t('admin.nodes.column.cpuMem'),
      key: 'cpuMem',
      align: 'center',
      className: 'table-cell-number',
      width: 140,
      render: (_value, node) => `${node.cpu_pct.toFixed(1)}% / ${node.mem_pct.toFixed(1)}%`,
    },
    {
      title: 'Xray',
      dataIndex: 'xray_version',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 160,
      render: (value: string) => value || '-',
    },
    {
      title: t('admin.nodes.column.lastSeen'),
      dataIndex: 'last_seen_at',
      align: 'center',
      className: 'table-cell-nowrap',
      width: 190,
      render: (value?: string | null) => formatLastSeen(value),
    },
    {
      title: t('admin.nodes.column.connection'),
      key: 'endpoint',
      width: 280,
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text code>{nodeConnectionURL(node)}</Typography.Text>
          <a href={panelInboundURL(node)} target="_blank" rel="noreferrer">
            {t('admin.nodes.openPanel')}
          </a>
        </Space>
      ),
    },
    {
      title: t('admin.nodes.enable'),
      dataIndex: 'enabled',
      align: 'center',
      width: 96,
      render: (_value, node) => (
        <Switch
          checked={node.enabled}
          aria-label={`${node.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${node.name}`}
          loading={enableNode.isPending || disableNode.isPending}
          onChange={() => toggleEnable(node)}
        />
      ),
    },
    {
      title: t('admin.users.column.actions'),
      key: 'actions',
      align: 'center',
      className: 'table-cell-actions',
      width: 144,
      render: (_value, node) => (
        <Space>
          <Button aria-label={`${t('admin.nodes.probe')} ${node.name}`} icon={<ThunderboltOutlined />} onClick={() => probeNode.mutateAsync(node.id)} />
          <Button aria-label={`${t('admin.nodes.edit')} ${node.name}`} icon={<EditOutlined />} onClick={() => openEdit(node)} />
          <Button danger aria-label={`${t('admin.nodes.delete')} ${node.name}`} icon={<DeleteOutlined />} onClick={() => confirmDelete(node)} />
        </Space>
      ),
    },
  ]

  return (
    <div>
      <ConfigListPage
        title={t('admin.nodes.title')}
        subtitle={t('admin.nodes.subtitle')}
        actions={
          <>
            <input
              ref={importInputRef}
              hidden
              type="file"
              accept="application/json,.json"
              aria-label={t('admin.nodes.importFile')}
              onChange={importJson}
            />
            <Button aria-label={t('admin.nodes.batch.qualityCheck')} icon={<ThunderboltOutlined />} disabled={busy || nodes.length === 0} onClick={batchProbe}>
              {t('admin.nodes.batch.qualityCheck')}
            </Button>
            <Button aria-label={t('admin.nodes.batch.delete')} danger icon={<DeleteOutlined />} disabled={busy || selectedRowKeys.length === 0} onClick={batchDelete}>
              {t('admin.nodes.batch.delete')}
            </Button>
            <Button aria-label={t('admin.nodes.import')} icon={<ImportOutlined />} loading={importing} onClick={() => importInputRef.current?.click()}>
              {t('admin.nodes.import')}
            </Button>
            <Button aria-label={t('admin.nodes.export')} icon={<DownloadOutlined />} disabled={nodes.length === 0} onClick={exportJson}>
              {t('admin.nodes.export')}
            </Button>
            <Button aria-label={t('admin.nodes.addNode')} type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              {t('admin.nodes.addNode')}
            </Button>
            <RefreshButton loading={nodesQuery.isFetching} onClick={refresh} label={t('admin.nodes.reload')} />
          </>
        }
        filters={
          <Space wrap>
            <Input.Search
              allowClear
              aria-label={t('admin.nodes.searchPlaceholder')}
              placeholder={t('admin.nodes.searchPlaceholder')}
              style={{ width: 240 }}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, query: event.target.value }))}
            />
            <Select
              allowClear
              aria-label={t('admin.nodes.filterArea')}
              placeholder={t('admin.nodes.filterAreaAll')}
              style={{ width: 180 }}
              options={AREA_OPTIONS.map((area) => ({ label: t(`admin.nodes.area.${area.key}`), value: area.key }))}
              onChange={(area) => setDraftFilters((prev) => ({ ...prev, area }))}
            />
            <Input
              allowClear
              aria-label={t('admin.nodes.filterProvince')}
              placeholder={t('admin.nodes.filterProvincePlaceholder')}
              style={{ width: 160 }}
              onChange={(event) => setDraftFilters((prev) => ({ ...prev, province: event.target.value }))}
            />
            <Select
              allowClear
              aria-label={t('admin.nodes.filterProtocol')}
              placeholder={t('admin.nodes.scheme')}
              style={{ width: 140 }}
              options={[
                { label: 'HTTPS', value: 'https' },
                { label: 'HTTP', value: 'http' },
              ]}
              onChange={(scheme) => setDraftFilters((prev) => ({ ...prev, scheme }))}
            />
            <Select
              allowClear
              aria-label={t('admin.nodes.filterStatus')}
              placeholder={t('admin.nodes.filterStatusAll')}
              style={{ width: 150 }}
              options={statusOptions}
              onChange={(status) => setDraftFilters((prev) => ({ ...prev, status }))}
            />
          </Space>
        }
        alerts={
          error ? <Alert type="error" showIcon message={typeof error === 'string' ? error : t('admin.nodes.loadFailed')} /> : null
        }
        rowKey="id"
        columns={columns}
        dataSource={nodes}
        loading={nodesQuery.isLoading}
        pagination={false}
        rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
        emptyState={{
          title: t('admin.nodes.empty'),
          description: t('admin.nodes.emptyDescription'),
          actionLabel: t('admin.nodes.addNode'),
          onAction: openCreate,
        }}
        mobileCard={(node) => {
            const status = nodeDisplayStatus(node)
            return (
              <Card size="small" style={{ width: '100%' }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Typography.Text strong>{node.name}</Typography.Text>
                    <Tag color={statusColor(status)}>{labelForStatus(status)}</Tag>
                  </Space>
                  <Typography.Text type="secondary">{locationText(node)}</Typography.Text>
                  <Typography.Text>{t('admin.nodes.column.cpuMem')}: {node.cpu_pct.toFixed(1)}% / {node.mem_pct.toFixed(1)}%</Typography.Text>
                  <Typography.Text>{t('admin.nodes.column.xray')}: {node.xray_version || '-'}</Typography.Text>
                  <Typography.Text>{t('admin.nodes.column.lastSeen')}: {formatLastSeen(node.last_seen_at)}</Typography.Text>
                  <Typography.Text code>{nodeConnectionURL(node)}</Typography.Text>
                  <Space wrap>
                    <Switch
                      checked={node.enabled}
                      aria-label={`${node.enabled ? t('admin.nodes.disable') : t('admin.nodes.enable')} ${node.name}`}
                      onChange={() => toggleEnable(node)}
                    />
                    <Button size="small" icon={<ThunderboltOutlined />} onClick={() => probeNode.mutateAsync(node.id)}>
                      {t('admin.nodes.probe')}
                    </Button>
                    <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(node)}>
                      {t('admin.nodes.edit')}
                    </Button>
                    <Button size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(node)}>
                      {t('admin.nodes.delete')}
                    </Button>
                  </Space>
                </Space>
              </Card>
            )
          }}
        footer={
          <Space wrap>
            <Typography.Text type="secondary">{t('admin.nodes.resultCount', { n: nodes.length })}</Typography.Text>
            <Tag color="green">{t('admin.nodes.footerOnline', { n: counts.online })}</Tag>
            <Tag color="red">{t('admin.nodes.footerOffline', { n: counts.offline })}</Tag>
            <Tag>{t('admin.nodes.footerDisabled', { n: counts.disabled })}</Tag>
          </Space>
        }
      />

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
