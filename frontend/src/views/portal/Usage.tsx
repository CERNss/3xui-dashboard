import { ArrowDownOutlined, ArrowUpOutlined, CalendarOutlined, TeamOutlined } from '@ant-design/icons'
import { Alert, Card, Col, Progress, Row, Skeleton, Space, Table, Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import type { ClientUsage } from '@/api/portal/traffic'
import { EmptyState, PageHeader, RefreshButton } from '@/components/common'
import { useProfile } from '@/hooks/queries/portal/profile'
import { useOwnTraffic } from '@/hooks/queries/portal/traffic'
import { formatError } from '@/utils/format'
import { formatBytes, formatDateTime, formatYuan, trafficPercent } from './_shared/format'

function daysToExpiry(clients: ClientUsage[]): number | null {
  const now = Date.now()
  let soonest = Infinity
  for (const client of clients) {
    if (!client.expires_at) continue
    const delta = new Date(client.expires_at).getTime() - now
    if (delta < soonest) soonest = delta
  }
  return Number.isFinite(soonest) ? Math.max(0, Math.floor(soonest / 86_400_000)) : null
}

export function Usage() {
  const { t } = useTranslation()
  const profile = useProfile()
  const traffic = useOwnTraffic()
  const clients = useMemo(() => traffic.data ?? [], [traffic.data])
  const totalUp = clients.reduce((sum, client) => sum + (client.up || 0), 0)
  const totalDown = clients.reduce((sum, client) => sum + (client.down || 0), 0)
  const totalUsed = totalUp + totalDown
  const totalLimit = clients.reduce((sum, client) => sum + (client.limit || 0), 0)
  const percent = trafficPercent(totalUsed, totalLimit)
  const expiryDays = daysToExpiry(clients)
  const loading = profile.isLoading || traffic.isLoading
  const refreshing = profile.isFetching || traffic.isFetching
  const error = profile.error ?? traffic.error
  const nodeCount = useMemo(() => new Set(clients.map((client) => client.node_id)).size, [clients])
  const subUrl = profile.data ? `${window.location.origin}/sub/${profile.data.sub_id}` : ''

  const columns: ColumnsType<ClientUsage> = [
    {
      title: t('portal.dashboard.column.node'),
      dataIndex: 'node_id',
      render: (nodeId: number) => `#${nodeId}`,
    },
    {
      title: t('portal.dashboard.column.inbound'),
      dataIndex: 'inbound_tag',
      render: (tag: string) => <Typography.Text code>{tag}</Typography.Text>,
    },
    {
      title: t('portal.dashboard.column.upload'),
      dataIndex: 'up',
      align: 'right',
      render: (value: number) => formatBytes(value),
    },
    {
      title: t('portal.dashboard.column.download'),
      dataIndex: 'down',
      align: 'right',
      render: (value: number) => formatBytes(value),
    },
    {
      title: t('portal.dashboard.column.usageLimit'),
      align: 'right',
      render: (_, client) => (
        <Space size={4}>
          <span>{formatBytes(client.total)}</span>
          {client.limit && client.limit > 0 ? <Typography.Text type="secondary">/ {formatBytes(client.limit)}</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: t('portal.dashboard.column.expires'),
      dataIndex: 'expires_at',
      render: (value: string | null | undefined) => formatDateTime(value),
    },
  ]

  function reload() {
    void Promise.all([profile.refetch(), traffic.refetch()])
  }

  return (
    <>
      <PageHeader
        title={
          profile.data?.email
            ? t('portal.dashboard.hi', { name: profile.data.email.split('@')[0] })
            : t('portal.dashboard.welcome')
        }
        subtitle={t('portal.dashboard.subtitle')}
        actions={<RefreshButton loading={refreshing} onClick={reload} label={t('portal.dashboard.refresh')} />}
      />

      {error ? (
        <Alert showIcon type="error" style={{ marginBottom: 16 }} message={formatError(error, t('portal.dashboard.loadFailed'))} />
      ) : null}

      {loading ? (
        <Skeleton active />
      ) : (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Row gutter={[16, 16]}>
            <Col xs={24} md={12} xl={6}>
              <Card>
                <Space direction="vertical" size={10} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">{t('portal.dashboard.usedTraffic')}</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {formatBytes(totalUsed)}
                  </Typography.Title>
                  {totalLimit > 0 ? (
                    <>
                      <Progress percent={percent} status={percent >= 85 ? 'exception' : 'active'} />
                      <Typography.Text type="secondary">
                        {t('portal.dashboard.usagePct', { pct: percent, total: formatBytes(totalLimit) })}
                      </Typography.Text>
                    </>
                  ) : (
                    <Typography.Text type="secondary">{t('portal.dashboard.unlimited')}</Typography.Text>
                  )}
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12} xl={6}>
              <Card>
                <Space direction="vertical" size={10}>
                  <Typography.Text type="secondary">
                    <CalendarOutlined /> {t('portal.dashboard.planExpiry')}
                  </Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {expiryDays === null ? '—' : expiryDays}
                  </Typography.Title>
                  <Typography.Text type="secondary">
                    {expiryDays === null ? t('portal.dashboard.noOrders') : t('portal.dashboard.days')}
                  </Typography.Text>
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12} xl={6}>
              <Card>
                <Space direction="vertical" size={10}>
                  <Typography.Text type="secondary">{t('portal.dashboard.balance')}</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {formatYuan(profile.data?.balance_cents)}
                  </Typography.Title>
                  <Typography.Text type="secondary">{t('portal.dashboard.balanceHint')}</Typography.Text>
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12} xl={6}>
              <Card>
                <Space direction="vertical" size={10}>
                  <Typography.Text type="secondary">
                    <TeamOutlined /> {t('portal.dashboard.activeClients')}
                  </Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {clients.length}
                  </Typography.Title>
                  <Typography.Text type="secondary">{t('portal.dashboard.acrossNodes', { n: nodeCount })}</Typography.Text>
                </Space>
              </Card>
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} md={12}>
              <Card>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    <ArrowUpOutlined /> {t('portal.dashboard.column.upload')}
                  </Typography.Text>
                  <Typography.Title level={4} style={{ margin: 0 }}>
                    {formatBytes(totalUp)}
                  </Typography.Title>
                </Space>
              </Card>
            </Col>
            <Col xs={24} md={12}>
              <Card>
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    <ArrowDownOutlined /> {t('portal.dashboard.column.download')}
                  </Typography.Text>
                  <Typography.Title level={4} style={{ margin: 0 }}>
                    {formatBytes(totalDown)}
                  </Typography.Title>
                </Space>
              </Card>
            </Col>
          </Row>

          {profile.data ? (
            <Card>
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                <Typography.Title level={4} style={{ margin: 0 }}>
                  {t('portal.dashboard.sub')}
                </Typography.Title>
                <Typography.Text type="secondary">{t('portal.dashboard.subHint')}</Typography.Text>
                <Typography.Text code copyable>
                  {subUrl}
                </Typography.Text>
                <Link to="/portal/subscription">{t('portal.dashboard.viewQR')}</Link>
              </Space>
            </Card>
          ) : null}

          <Card>
            <Typography.Title level={4} style={{ marginTop: 0 }}>
              {t('portal.dashboard.tableHeader')}
            </Typography.Title>
            <Typography.Paragraph type="secondary">{t('portal.dashboard.tableHeaderHint')}</Typography.Paragraph>
            {clients.length > 0 ? (
              <Table
                rowKey={(client) => `${client.node_id}:${client.inbound_tag}:${client.client_email}`}
                columns={columns}
                dataSource={clients}
                pagination={false}
                scroll={{ x: true }}
              />
            ) : (
              <EmptyState
                title={t('portal.dashboard.empty')}
                description={t('portal.dashboard.emptyDescription')}
                actionLabel={t('portal.dashboard.goToPlans')}
                onAction={() => undefined}
              />
            )}
          </Card>
        </Space>
      )}
    </>
  )
}

export default Usage
