import { Alert, Card, Col, Row, Space, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { forwardRef, useEffect, useImperativeHandle, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { EmptyState, ResponsiveListTable } from '@/components/common'
import type { Node } from '@/api/admin/nodes'
import { useInboundsFleet } from '@/hooks/queries/admin/inbounds'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import { formatDateTime } from './format'
import { KpiCard } from './KpiCard'

export interface OverviewPanelHandle {
  reload: () => void
  isFetching: boolean
}

type NodeTone = 'online' | 'offline' | 'unknown' | 'disabled'

function nodeTone(node: Node): NodeTone {
  if (!node.enabled) return 'disabled'
  if (node.status === 'online' || node.status === 'offline') return node.status
  return 'unknown'
}

function statusColor(tone: NodeTone) {
  if (tone === 'online') return 'green'
  if (tone === 'offline') return 'red'
  if (tone === 'disabled') return 'default'
  return 'gold'
}

function nodeLabel(node: Node) {
  const tone = nodeTone(node)
  if (tone === 'disabled') return 'Disabled'
  if (tone === 'unknown') return 'Unknown'
  return tone === 'online' ? 'Online' : 'Offline'
}

function hasProbeData(node: Node) {
  return Boolean(node.last_seen_at || node.xray_version || node.cpu_pct > 0 || node.mem_pct > 0)
}

function nodeDetail(node: Node) {
  const tone = nodeTone(node)
  if (tone === 'online') return 'Live probe data'
  if (tone === 'disabled') return 'Disabled nodes do not report probes'
  if (tone === 'unknown') return 'No successful probe has reported yet'
  if (node.last_seen_at) return `Last seen ${formatDateTime(node.last_seen_at)}`
  return 'Never seen online'
}

function nodeMetrics(node: Node) {
  if (!hasProbeData(node) && node.status !== 'online') return '—'
  return `${node.cpu_pct.toFixed(1)}% / ${node.mem_pct.toFixed(1)}%`
}

interface StatusPanelProps {
  onFetchingChange?: (fetching: boolean) => void
}

export const StatusPanel = forwardRef<OverviewPanelHandle, StatusPanelProps>(function StatusPanel({ onFetchingChange }, ref) {
  const nodesQuery = useNodesList()
  const fleetQuery = useInboundsFleet()

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data])
  const fleetInbounds = useMemo(() => fleetQuery.data?.inbounds ?? [], [fleetQuery.data])
  const loading = nodesQuery.isLoading || fleetQuery.isLoading
  const fetching = nodesQuery.isFetching || fleetQuery.isFetching
  const error = nodesQuery.error ?? fleetQuery.error

  useImperativeHandle(
    ref,
    () => ({
      isFetching: fetching,
      reload: () => {
        void nodesQuery.refetch()
        void fleetQuery.refetch()
      },
    }),
    [fetching, fleetQuery, nodesQuery],
  )

  useEffect(() => {
    onFetchingChange?.(fetching)
  }, [fetching, onFetchingChange])

  const stats = useMemo(() => {
    const enabled = nodes.filter((node) => node.enabled)
    const online = enabled.filter((node) => node.status === 'online').length
    const offline = enabled.filter((node) => node.status === 'offline').length
    const unknown = enabled.length - online - offline
    const disabled = nodes.length - enabled.length
    const clients = fleetInbounds.reduce((sum, row) => sum + (row.inbound.clientStats?.length ?? 0), 0)

    return {
      nodes: nodes.length,
      online,
      offline,
      unknown,
      disabled,
      attention: offline + unknown + disabled,
      inbounds: fleetInbounds.length,
      clients,
    }
  }, [fleetInbounds, nodes])

  const columns: ColumnsType<Node> = [
    {
      title: 'Name',
      dataIndex: 'name',
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{node.name}</Typography.Text>
          {!node.enabled ? <Typography.Text type="secondary">Disabled</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      render: (_value, node) => {
        const tone = nodeTone(node)
        return (
          <Space direction="vertical" size={2}>
            <Tag color={statusColor(tone)}>{nodeLabel(node)}</Tag>
            <Typography.Text type="secondary">{nodeDetail(node)}</Typography.Text>
          </Space>
        )
      },
    },
    {
      title: 'CPU / Mem',
      key: 'cpuMem',
      render: (_value, node) => <Typography.Text>{nodeMetrics(node)}</Typography.Text>,
    },
    {
      title: 'Xray',
      dataIndex: 'xray_version',
      render: (value: string) => <Typography.Text code>{value || '—'}</Typography.Text>,
    },
    {
      title: 'Last seen',
      dataIndex: 'last_seen_at',
      render: (value?: string | null) => formatDateTime(value),
    },
  ]

  return (
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      {error ? <Alert type="error" showIcon message="Status load failed" /> : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} lg={6}>
          <KpiCard
            title="Nodes"
            value={stats.nodes}
            extra={`${stats.online} online · ${stats.offline} offline · ${stats.unknown} unknown`}
          />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard title="Inbounds" value={stats.inbounds} extra={`${stats.nodes} nodes in fleet`} />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard title="Clients" value={stats.clients} extra="Configured inbound clients" />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard
            title="Needs attention"
            value={stats.attention}
            extra={
              stats.attention > 0
                ? `${stats.offline} offline · ${stats.unknown} unknown · ${stats.disabled} disabled`
                : 'All enabled nodes are online'
            }
          />
        </Col>
      </Row>

      <Card
        title="Node health"
        extra={<Link to="/admin/nodes">Manage nodes</Link>}
        styles={{ body: { padding: nodes.length > 0 || loading ? 0 : 24 } }}
      >
        {nodes.length > 0 || loading ? (
          <ResponsiveListTable
            rowKey="id"
            columns={columns}
            dataSource={nodes}
            loading={loading}
            pagination={false}
            mobileCard={(node) => (
              <Card size="small" style={{ width: '100%' }}>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Typography.Text strong>{node.name}</Typography.Text>
                    <Tag color={statusColor(nodeTone(node))}>{nodeLabel(node)}</Tag>
                  </Space>
                  <Typography.Text type="secondary">{nodeDetail(node)}</Typography.Text>
                  <Typography.Text>CPU / Mem: {nodeMetrics(node)}</Typography.Text>
                  <Typography.Text>Xray: {node.xray_version || '—'}</Typography.Text>
                  <Typography.Text>Last seen: {formatDateTime(node.last_seen_at)}</Typography.Text>
                </Space>
              </Card>
            )}
          />
        ) : (
          <EmptyState
            title="No nodes"
            description="Create a node before checking fleet health."
          />
        )}
      </Card>
    </Space>
  )
})
