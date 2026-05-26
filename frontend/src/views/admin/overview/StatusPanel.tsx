import { Alert, Card, Col, Row, Space, Tag, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import type { TFunction } from 'i18next'
import { forwardRef, useEffect, useImperativeHandle, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
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

function nodeLabel(node: Node, t: TFunction) {
  const tone = nodeTone(node)
  return t(`admin.status.nodeState.${tone}`)
}

function hasProbeData(node: Node) {
  return Boolean(node.last_seen_at || node.xray_version || node.cpu_pct > 0 || node.mem_pct > 0)
}

function nodeDetail(node: Node, t: TFunction) {
  const tone = nodeTone(node)
  if (tone === 'online') return t('admin.status.nodeState.onlineHint')
  if (tone === 'disabled') return t('admin.status.nodeState.disabledHint')
  if (tone === 'unknown') return t('admin.status.nodeState.unknownHint')
  if (node.last_seen_at) return t('admin.status.nodeState.offlineLastSeen', { time: formatDateTime(node.last_seen_at) })
  return t('admin.status.nodeState.offlineNeverSeen')
}

function nodeMetrics(node: Node) {
  if (!hasProbeData(node) && node.status !== 'online') return '—'
  return `${node.cpu_pct.toFixed(1)}% / ${node.mem_pct.toFixed(1)}%`
}

interface StatusPanelProps {
  onFetchingChange?: (fetching: boolean) => void
}

export const StatusPanel = forwardRef<OverviewPanelHandle, StatusPanelProps>(function StatusPanel({ onFetchingChange }, ref) {
  const { t } = useTranslation()
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
      title: t('admin.status.column.name'),
      dataIndex: 'name',
      render: (_value, node) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{node.name}</Typography.Text>
          {!node.enabled ? <Typography.Text type="secondary">{t('admin.status.nodeState.disabled')}</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: t('admin.status.column.status'),
      dataIndex: 'status',
      render: (_value, node) => {
        const tone = nodeTone(node)
        return (
          <Space direction="vertical" size={2}>
            <Tag color={statusColor(tone)}>{nodeLabel(node, t)}</Tag>
            <Typography.Text type="secondary">{nodeDetail(node, t)}</Typography.Text>
          </Space>
        )
      },
    },
    {
      title: t('admin.status.column.cpuMem'),
      key: 'cpuMem',
      render: (_value, node) => <Typography.Text>{nodeMetrics(node)}</Typography.Text>,
    },
    {
      title: 'Xray',
      dataIndex: 'xray_version',
      render: (value: string) => <Typography.Text code>{value || '—'}</Typography.Text>,
    },
    {
      title: t('admin.status.column.lastSeen'),
      dataIndex: 'last_seen_at',
      render: (value?: string | null) => formatDateTime(value),
    },
  ]

  return (
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      {error ? <Alert type="error" showIcon message={t('admin.status.loadFailed')} /> : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} lg={6}>
          <KpiCard
            title={t('admin.status.kpi.nodes')}
            value={stats.nodes}
            extra={`${t('admin.status.kpi.online', { n: stats.online })} · ${t('admin.status.kpi.offline', { n: stats.offline })} · ${t('admin.status.kpi.unknown', { n: stats.unknown })}`}
          />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard title={t('admin.status.kpi.inbounds')} value={stats.inbounds} extra={t('admin.status.kpi.inboundsHint', { n: stats.nodes })} />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard title={t('admin.status.kpi.clients')} value={stats.clients} extra={t('admin.status.kpi.clientsHint')} />
        </Col>
        <Col xs={24} md={12} lg={6}>
          <KpiCard
            title={t('admin.status.kpi.attention')}
            value={stats.attention}
            extra={
              stats.attention > 0
                ? t('admin.status.kpi.attentionDetail', { disabled: stats.disabled, offline: stats.offline, unknown: stats.unknown })
                : t('admin.status.kpi.attentionClear')
            }
          />
        </Col>
      </Row>

      <Card
        title={t('admin.status.nodeHealth')}
        extra={<Link to="/admin/nodes">{t('admin.status.manageNodes')}</Link>}
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
                    <Tag color={statusColor(nodeTone(node))}>{nodeLabel(node, t)}</Tag>
                  </Space>
                  <Typography.Text type="secondary">{nodeDetail(node, t)}</Typography.Text>
                  <Typography.Text>{t('admin.status.column.cpuMem')}: {nodeMetrics(node)}</Typography.Text>
                  <Typography.Text>{t('admin.nodes.column.xray')}: {node.xray_version || '—'}</Typography.Text>
                  <Typography.Text>{t('admin.status.column.lastSeen')}: {formatDateTime(node.last_seen_at)}</Typography.Text>
                </Space>
              </Card>
            )}
          />
        ) : (
          <EmptyState
            title={t('admin.status.empty')}
            description={t('admin.status.emptyDescription')}
          />
        )}
      </Card>
    </Space>
  )
})
