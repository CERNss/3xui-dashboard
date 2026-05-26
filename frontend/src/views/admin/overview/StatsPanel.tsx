import { Alert, Card, Col, List, Progress, Row, Space, Tag, Typography } from 'antd'
import type { TFunction } from 'i18next'
import { forwardRef, useEffect, useImperativeHandle, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import type { AdminStats, TrafficRanking } from '@/api/admin/stats'
import { usePlansList } from '@/hooks/queries/admin/plans'
import { useAdminStats } from '@/hooks/queries/admin/stats'
import { formatBytes, formatDateTime, formatYuan } from './format'
import { KpiCard } from './KpiCard'
import type { OverviewPanelHandle } from './StatusPanel'

function assertStatsPayload(value: AdminStats | undefined, t: TFunction): AdminStats | null {
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
    throw new Error(t('admin.stats.invalidPayload'))
  }

  return value
}

function monthDelta(stats: AdminStats, t: TFunction) {
  const current = stats.users.month_new
  const previous = stats.users.prev_month_new
  if (current === 0 && previous === 0) return null
  if (previous === 0) return t('admin.stats.kpiSubtitle.monthDelta', { percent: 100, sign: '+' })

  const change = ((current - previous) / previous) * 100
  const rounded = Math.round(change * 10) / 10
  const sign = rounded > 0 ? '+' : rounded < 0 ? '-' : ''
  return t('admin.stats.kpiSubtitle.monthDelta', { percent: Math.abs(rounded), sign })
}

function rankingShare(row: TrafficRanking, rows: TrafficRanking[]) {
  const max = rows.reduce((largest, item) => Math.max(largest, item.bytes), 0)
  if (max <= 0) return 0
  return Math.max(2, Math.round((row.bytes / max) * 100))
}

function orderStatusColor(status: string) {
  if (status === 'completed' || status === 'paid') return 'green'
  if (status === 'failed') return 'red'
  return 'default'
}

function orderStatusLabel(status: string, t: TFunction) {
  const label = t(`admin.stats.orderStatus.${status}`, { defaultValue: '' })
  return label || status
}

function orderUserLabel(order: { user_email?: string; user_id: number }, t: TFunction) {
  return order.user_email || t('admin.stats.unknownUser', { id: order.user_id })
}

function orderPlanLabel(order: { plan_name?: string; plan_id: number }, t: TFunction) {
  return order.plan_name || t('admin.stats.unknownPlan', { id: order.plan_id })
}

function RankingCard({ title, subtitle, rows, t }: { title: string; subtitle: string; rows: TrafficRanking[]; t: TFunction }) {
  return (
    <Card title={title} extra={<Tag>{t('admin.stats.todayWindow')}</Tag>}>
      <Typography.Paragraph type="secondary">{subtitle}</Typography.Paragraph>
      <List
        dataSource={rows}
        locale={{ emptyText: t('admin.stats.noTraffic') }}
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
  const { t } = useTranslation()
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
      assertStatsPayload(statsQuery.data, t)
      return null
    } catch (error) {
      return error instanceof Error ? error : new Error(t('admin.stats.invalidPayload'))
    }
  }, [statsQuery.data, t])
  const stats = payloadError ? null : assertStatsPayload(statsQuery.data, t)
  const plans = plansQuery.data ?? []
  const auditTotal = stats ? stats.audit.info + stats.audit.warn + stats.audit.err : 0

  if (payloadError) {
    return <Alert type="error" showIcon message={payloadError.message} />
  }

  return (
    <Space direction="vertical" size={24} style={{ width: '100%' }}>
      {statsQuery.error ? <Alert type="error" showIcon message={t('admin.stats.loadFailed')} /> : null}

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
              <KpiCard title={t('admin.stats.kpi.monthNewUsers')} value={stats.users.month_new} extra={monthDelta(stats, t)} />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard title={t('admin.stats.kpi.totalUsers')} value={stats.users.total} extra={t('admin.stats.kpiSubtitle.activeUsers', { n: stats.users.active })} />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard
                title={t('admin.stats.kpi.monthUpload')}
                value={formatBytes(stats.traffic.month_up_bytes)}
                extra={t('admin.stats.kpiSubtitle.todayDelta', { value: formatBytes(stats.traffic.today_up_bytes) })}
              />
            </Col>
            <Col xs={24} md={12} lg={6}>
              <KpiCard
                title={t('admin.stats.kpi.monthDownload')}
                value={formatBytes(stats.traffic.month_down_bytes)}
                extra={t('admin.stats.kpiSubtitle.todayDelta', { value: formatBytes(stats.traffic.today_down_bytes) })}
              />
            </Col>
          </Row>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <RankingCard
                title={t('admin.stats.nodeTrafficRanking')}
                subtitle={t('admin.stats.nodeTrafficRankingSubtitle')}
                rows={stats.top_nodes}
                t={t}
              />
            </Col>
            <Col xs={24} lg={12}>
              <RankingCard
                title={t('admin.stats.userTrafficRanking')}
                subtitle={t('admin.stats.userTrafficRankingSubtitle')}
                rows={stats.top_users}
                t={t}
              />
            </Col>
          </Row>

          <Card title={t('admin.stats.systemLog')} extra={<Link to="/admin/audit-log">{t('admin.stats.systemLogViewAll')}</Link>}>
            <Typography.Paragraph type="secondary">{t('admin.stats.systemLogSubtitle')}</Typography.Paragraph>
            <Row gutter={[12, 12]}>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">{t('admin.stats.systemLogInfo')}</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.info}
                  </Typography.Title>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">{t('admin.stats.systemLogWarn')}</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.warn}
                  </Typography.Title>
                </Card>
              </Col>
              <Col xs={24} sm={8}>
                <Card size="small">
                  <Typography.Text type="secondary">{t('admin.stats.systemLogErr')}</Typography.Text>
                  <Typography.Title level={3} style={{ margin: 0 }}>
                    {stats.audit.err}
                  </Typography.Title>
                </Card>
              </Col>
            </Row>
            <Typography.Text type="secondary">{t('admin.stats.systemLogTotal', { n: auditTotal })}</Typography.Text>
          </Card>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={8}>
              <Card title={t('admin.stats.plans')}>
                <Typography.Paragraph type="secondary">
                  {t('admin.stats.plansEnabledSummary', { disabled: stats.plans.disabled, enabled: stats.plans.enabled })}
                </Typography.Paragraph>
                <List
                  loading={plansQuery.isLoading}
                  dataSource={plans}
                  locale={{ emptyText: plansQuery.error ? t('admin.stats.plansLoadFailed') : t('admin.stats.empty') }}
                  renderItem={(plan) => (
                    <List.Item>
                      <List.Item.Meta
                        title={plan.name}
                        description={t('admin.stats.planTrafficLine', {
                          days: plan.duration_days,
                          traffic:
                          plan.traffic_limit_bytes === 0
                            ? t('admin.stats.unlimited')
                            : `${Math.round(plan.traffic_limit_bytes / 1024 / 1024 / 1024)} GB`,
                        })}
                      />
                      <Typography.Text strong>{formatYuan(plan.price_cents)}</Typography.Text>
                    </List.Item>
                  )}
                />
                {plansQuery.error ? <Alert type="warning" showIcon message={t('admin.stats.plansLoadFailed')} /> : null}
              </Card>
            </Col>
            <Col xs={24} lg={16}>
              <Card title={t('admin.stats.recentOrders')}>
                <List
                  dataSource={stats.recent_orders}
                  locale={{ emptyText: t('admin.stats.emptyOrders') }}
                  renderItem={(order) => (
                    <List.Item>
                      <List.Item.Meta
                        title={t('admin.stats.orderLine', {
                          plan: orderPlanLabel(order, t),
                          user: orderUserLabel(order, t),
                        })}
                        description={formatDateTime(order.created_at)}
                      />
                      <Space>
                        <Typography.Text strong>{formatYuan(order.price_cents)}</Typography.Text>
                        <Tag color={orderStatusColor(order.status)}>{orderStatusLabel(order.status, t)}</Tag>
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
