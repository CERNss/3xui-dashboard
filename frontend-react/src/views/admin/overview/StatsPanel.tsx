import { Alert, Card, Col, List, Progress, Row, Space, Tag, Typography } from 'antd'
import { forwardRef, useEffect, useImperativeHandle, useMemo } from 'react'
import { Link } from 'react-router-dom'
import type { AdminStats, TrafficRanking } from '@/api/admin/stats'
import { usePlansList } from '@/hooks/queries/admin/plans'
import { useAdminStats } from '@/hooks/queries/admin/stats'
import { formatBytes, formatDateTime, formatYuan } from './format'
import { KpiCard } from './KpiCard'
import type { OverviewPanelHandle } from './StatusPanel'

function assertStatsPayload(value: AdminStats | undefined): AdminStats | null {
  if (!value) return null
  if (
    !value.users ||
    !value.plans ||
    !value.orders ||
    !value.traffic ||
    !Array.isArray(value.top_nodes) ||
    !Array.isArray(value.top_users) ||
    !value.audit ||
    !Array.isArray(value.recent_orders)
  ) {
    throw new Error('Stats payload is incomplete')
  }

  return value
}

function monthDelta(stats: AdminStats) {
  const current = stats.users.month_new
  const previous = stats.users.prev_month_new
  if (current === 0 && previous === 0) return null
  if (previous === 0) return '+100% vs last month'

  const change = ((current - previous) / previous) * 100
  const rounded = Math.round(change * 10) / 10
  const sign = rounded > 0 ? '+' : rounded < 0 ? '-' : ''
  return `${sign}${Math.abs(rounded)}% vs last month`
}

function rankingShare(row: TrafficRanking, rows: TrafficRanking[]) {
  const max = rows.reduce((largest, item) => Math.max(largest, item.bytes), 0)
  if (max <= 0) return 0
  return Math.max(2, Math.round((row.bytes / max) * 100))
}

function RankingCard({ title, subtitle, rows }: { title: string; subtitle: string; rows: TrafficRanking[] }) {
  return (
    <Card title={title} extra={<Tag>Today</Tag>}>
      <Typography.Paragraph type="secondary">{subtitle}</Typography.Paragraph>
      <List
        dataSource={rows}
        locale={{ emptyText: 'No traffic data' }}
        renderItem={(row) => (
          <List.Item>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                <Typography.Text strong>{row.key}</Typography.Text>
                <Typography.Text type="secondary">{formatBytes(row.bytes)}</Typography.Text>
              </Space>
              <Progress percent={rankingShare(row, rows)} showInfo={false} size="small" />
            </Space>
          </List.Item>
        )}
      />
    </Card>
  )
}

interface StatsPanelProps {
  onFetchingChange?: (fetching: boolean) => void
}

export const StatsPanel = forwardRef<OverviewPanelHandle, StatsPanelProps>(function StatsPanel({ onFetchingChange }, ref) {
  const statsQuery = useAdminStats()
  const plansQuery = usePlansList()
  const fetching = statsQuery.isFetching || plansQuery.isFetching

  useImperativeHandle(
    ref,
    () => ({
      isFetching: fetching,
      reload: () => {
        void statsQuery.refetch()
        void plansQuery.refetch()
      },
    }),
    [fetching, plansQuery, statsQuery],
  )

  useEffect(() => {
    onFetchingChange?.(fetching)
  }, [fetching, onFetchingChange])

  const payloadError = useMemo(() => {
    try {
      assertStatsPayload(statsQuery.data)
      return null
    } catch (error) {
      return error instanceof Error ? error : new Error('Stats payload is incomplete')
    }
  }, [statsQuery.data])
  const stats = payloadError ? null : assertStatsPayload(statsQuery.data)
  const plans = plansQuery.data ?? []
  const auditTotal = stats ? stats.audit.info + stats.audit.warn + stats.audit.err : 0

  if (payloadError) {
    return <Alert type="error" showIcon message={payloadError.message} />
  }

  return (
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      {statsQuery.error ? <Alert type="error" showIcon message="Stats load failed" /> : null}

      {!stats && statsQuery.isLoading ? (
        <Row gutter={[16, 16]}>
          {[0, 1, 2, 3].map((item) => (
            <Col xs={24} md={12} lg={6} key={item}>
              <Card loading />
            </Col>
          ))}
        </Row>
      ) : null}

      {stats ? (
        <>
          <Row gutter={[16, 16]}>
            <Col xs={24} md={12} lg={6}>
              <KpiCard title="Month new users" value={stats.users.month_new} extra={monthDelta(stats)} />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard title="Total users" value={stats.users.total} extra={`Active users: ${stats.users.active}`} />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard
                title="Month upload"
                value={formatBytes(stats.traffic.month_up_bytes)}
                extra={`Today: ${formatBytes(stats.traffic.today_up_bytes)}`}
              />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard
                title="Month download"
                value={formatBytes(stats.traffic.month_down_bytes)}
                extra={`Today: ${formatBytes(stats.traffic.today_down_bytes)}`}
              />
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <RankingCard
                title="Node traffic ranking"
                subtitle="Largest node consumers in today's window."
                rows={stats.top_nodes}
              />
            </Col>
            <Col xs={24} lg={12}>
              <RankingCard
                title="User traffic ranking"
                subtitle="Largest user consumers in today's window."
                rows={stats.top_users}
              />
            </Col>
          </Row>

          <Card title="System log" extra={<Link to="/admin/audit-log">View all</Link>}>
            <Typography.Paragraph type="secondary">Audit severity counts from recent system activity.</Typography.Paragraph>
            <Row gutter={[12, 12]}>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">Info</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.info}
                  </Typography.Title>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">Warn</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.warn}
                  </Typography.Title>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">Error</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.err}
                  </Typography.Title>
                </Card>
              </Col>
            </Row>
            <Typography.Text type="secondary">Total {auditTotal} entries</Typography.Text>
          </Card>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={8}>
              <Card title="Plans">
                <Typography.Paragraph type="secondary">
                  {stats.plans.enabled} enabled · {stats.plans.disabled} disabled
                </Typography.Paragraph>
                <List
                  loading={plansQuery.isLoading}
                  dataSource={plans}
                  locale={{ emptyText: plansQuery.error ? 'Plans unavailable' : 'No plans' }}
                  renderItem={(plan) => (
                    <List.Item>
                      <List.Item.Meta
                        title={plan.name}
                        description={`${plan.duration_days} days · ${
                          plan.traffic_limit_bytes === 0
                            ? 'Unlimited'
                            : `${Math.round(plan.traffic_limit_bytes / 1024 / 1024 / 1024)} GB`
                        }`}
                      />
                      <Typography.Text strong>{formatYuan(plan.price_cents)}</Typography.Text>
                    </List.Item>
                  )}
                />
                {plansQuery.error ? <Alert type="warning" showIcon message="Plans load failed" /> : null}
              </Card>
            </Col>
            <Col xs={24} lg={16}>
              <Card title="Recent orders">
                <List
                  dataSource={stats.recent_orders}
                  locale={{ emptyText: 'No recent orders' }}
                  renderItem={(order) => (
                    <List.Item>
                      <List.Item.Meta
                        title={`${order.user_email || `User #${order.user_id}`} -> ${
                          order.plan_name || `Plan #${order.plan_id}`
                        }`}
                        description={formatDateTime(order.created_at)}
                      />
                      <Space>
                        <Typography.Text strong>{formatYuan(order.price_cents)}</Typography.Text>
                        <Tag
                          color={
                            order.status === 'completed' || order.status === 'paid'
                              ? 'green'
                              : order.status === 'failed'
                                ? 'red'
                                : 'default'
                          }
                        >
                          {order.status}
                        </Tag>
                      </Space>
                    </List.Item>
                  )}
                />
              </Card>
            </Col>
          </Row>
        </>
      ) : null}
    </Space>
  )
})
