import { Alert, Card, Col, Row, Skeleton, Space, Tag, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import type { NodeMetricPoint } from '@/api/admin/nodes'
import { nodesApi } from '@/api/admin/nodes'
import { BarsPanel, DonutGauge, DotsGrid, TrendLine } from '@/components/charts'
import { useInboundsFleet } from '@/hooks/queries/admin/inbounds'
import { useNodesList } from '@/hooks/queries/admin/nodes'
import { avg, buildTrendPoints, formatPercent, hasProbeData, safeFleet } from './ops-monitor/calculations'

type Tone = 'live' | 'warn' | 'muted'

interface MonitorCard {
  label: string
  value: string
  hint: string
  tone: Tone
}

function cardBorder(tone: Tone) {
  if (tone === 'warn') return '#f59e0b'
  if (tone === 'live') return '#10b981'
  return '#cbd5e1'
}

function formatRatio(value: number, total: number) {
  return `${value}/${total}`
}

function lastRefreshText(date: Date | null) {
  if (!date) return 'Waiting for refresh'
  return `Last refresh ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

function MetricCard({ card }: { card: MonitorCard }) {
  return (
    <Card size="small" style={{ borderColor: cardBorder(card.tone), minHeight: 112 }}>
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Space style={{ justifyContent: 'space-between', width: '100%' }}>
          <Typography.Text strong>{card.label}</Typography.Text>
          <span
            aria-hidden
            style={{ background: cardBorder(card.tone), borderRadius: 999, display: 'inline-block', height: 8, width: 8 }}
          />
        </Space>
        <Typography.Title level={3} style={{ margin: 0 }}>
          {card.value}
        </Typography.Title>
        <Typography.Text type="secondary">{card.hint}</Typography.Text>
      </Space>
    </Card>
  )
}

export default function OpsMonitor() {
  const nodesQuery = useNodesList()
  const fleetQuery = useInboundsFleet()
  const [metricSeries, setMetricSeries] = useState<Record<number, NodeMetricPoint[]>>({})
  const [metricError, setMetricError] = useState<string | null>(null)
  const [metricsLoading, setMetricsLoading] = useState(false)
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)

  const nodes = useMemo(() => nodesQuery.data ?? [], [nodesQuery.data])
  const fleet = safeFleet(fleetQuery.data)
  const loading = nodesQuery.isLoading || fleetQuery.isLoading
  const error = nodesQuery.error ?? fleetQuery.error

  useEffect(() => {
    if (loading) return
    setLastRefresh(new Date())
  }, [loading, nodes.length, fleet.inbounds.length])

  useEffect(() => {
    const enabled = nodes.filter((node) => node.enabled)
    if (nodesQuery.isLoading || enabled.length === 0) {
      setMetricSeries({})
      setMetricError(null)
      return
    }

    let cancelled = false
    const to = Math.floor(Date.now() / 1000)
    const from = to - 3 * 60 * 60

    setMetricsLoading(true)
    setMetricError(null)
    Promise.allSettled(enabled.map((node) => nodesApi.metrics(node.id, { from, to, bucket: '10m' })))
      .then((results) => {
        if (cancelled) return
        const next: Record<number, NodeMetricPoint[]> = {}
        let failures = 0
        results.forEach((result, index) => {
          const nodeID = enabled[index].id
          if (result.status === 'fulfilled') {
            next[nodeID] = Array.isArray(result.value.points) ? result.value.points : []
          } else {
            failures += 1
            next[nodeID] = []
          }
        })
        setMetricSeries(next)
        setMetricError(failures > 0 ? `${failures} node metric request failed; healthy series are still shown.` : null)
      })
      .finally(() => {
        if (!cancelled) setMetricsLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [nodes, nodesQuery.isLoading])

  const enabledNodes = useMemo(() => nodes.filter((node) => node.enabled), [nodes])
  const onlineNodes = enabledNodes.filter((node) => node.status === 'online')
  const offlineNodes = enabledNodes.filter((node) => node.status === 'offline')
  const unknownNodes = enabledNodes.filter((node) => node.status !== 'online' && node.status !== 'offline')
  const disabledNodes = nodes.filter((node) => !node.enabled)
  const attentionCount = offlineNodes.length + unknownNodes.length + disabledNodes.length
  const healthScore = enabledNodes.length === 0 ? 0 : Math.round((onlineNodes.length / enabledNodes.length) * 100)
  const healthTone: Tone = enabledNodes.length === 0 ? 'muted' : attentionCount > 0 ? 'warn' : 'live'
  const healthLabel = enabledNodes.length === 0 ? 'No enabled nodes' : attentionCount > 0 ? 'Needs attention' : 'Healthy'

  const inbounds = fleet.inbounds.map((row) => row.inbound)
  const activeInbounds = inbounds.filter((row) => row.enable).length
  const clientCount = inbounds.reduce((sum, row) => sum + (row.clientStats?.length ?? 0), 0)
  const enabledClientCount = inbounds.reduce(
    (sum, row) => sum + (row.clientStats?.filter((client) => client.enable !== false).length ?? 0),
    0,
  )
  const nodeErrorCount = Object.keys(fleet.node_errors ?? {}).length
  const resourceNodes = enabledNodes.filter(hasProbeData)
  const cpuAvg = avg(resourceNodes.map((node) => node.cpu_pct))
  const memAvg = avg(resourceNodes.map((node) => node.mem_pct))
  const trendPoints = buildTrendPoints(metricSeries)
  const loadedNodes = [...resourceNodes]
    .sort((left, right) => Math.max(right.cpu_pct, right.mem_pct) - Math.max(left.cpu_pct, left.mem_pct))
    .slice(0, 6)

  const realtimeCards: MonitorCard[] = [
    { label: 'QPS', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'TPS', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'Requests', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'Tokens', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    {
      label: 'Active inbounds',
      value: formatRatio(activeInbounds, inbounds.length),
      hint: 'From fleet inventory.',
      tone: activeInbounds > 0 ? 'live' : 'muted',
    },
    {
      label: 'Clients',
      value: enabledClientCount.toLocaleString(),
      hint: `${clientCount.toLocaleString()} total clients.`,
      tone: enabledClientCount > 0 ? 'live' : 'muted',
    },
  ]
  const qualityCards: MonitorCard[] = [
    { label: 'SLA', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'Request errors', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'Latency P99', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    { label: 'TTFT P99', value: 'Unavailable', hint: 'Business telemetry is not connected.', tone: 'muted' },
    {
      label: 'Upstream errors',
      value: nodeErrorCount.toLocaleString(),
      hint: 'Fleet node errors.',
      tone: nodeErrorCount > 0 ? 'warn' : 'live',
    },
  ]
  const infraCards: MonitorCard[] = [
    { label: 'CPU', value: formatPercent(cpuAvg), hint: 'Average node probe.', tone: cpuAvg === null ? 'muted' : cpuAvg >= 85 ? 'warn' : 'live' },
    { label: 'Memory', value: formatPercent(memAvg), hint: 'Average node probe.', tone: memAvg === null ? 'muted' : memAvg >= 85 ? 'warn' : 'live' },
    { label: 'DB', value: 'Unavailable', hint: 'Infrastructure telemetry is not connected.', tone: 'muted' },
    { label: 'Redis', value: 'Unavailable', hint: 'Infrastructure telemetry is not connected.', tone: 'muted' },
    { label: 'Queue', value: 'Unavailable', hint: 'Infrastructure telemetry is not connected.', tone: 'muted' },
    { label: 'Background tasks', value: 'Unavailable', hint: 'Infrastructure telemetry is not connected.', tone: 'muted' },
  ]

  if (loading) {
    return (
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Skeleton active />
        <Skeleton active />
        <Skeleton active />
      </Space>
    )
  }

  return (
    <Space direction="vertical" size={20} style={{ width: '100%' }}>
      {error ? <Alert type="error" showIcon message="Ops monitor failed to load" /> : null}
      <Space wrap style={{ justifyContent: 'flex-end', width: '100%' }}>
        <Tag color={healthTone === 'warn' ? 'gold' : healthTone === 'live' ? 'green' : 'default'}>{healthLabel}</Tag>
        <Tag>{lastRefreshText(lastRefresh)}</Tag>
      </Space>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={8}>
          <Card title="Fleet health" extra={formatRatio(onlineNodes.length, enabledNodes.length)} style={{ height: '100%' }}>
            <DonutGauge value={healthScore} label="Health score" detail={healthLabel} tone={healthTone} ariaLabel="Fleet health score" />
            <Row gutter={8} style={{ textAlign: 'center' }}>
              <Col span={6}><Typography.Text strong>{onlineNodes.length}</Typography.Text><br /><Typography.Text type="secondary">Online</Typography.Text></Col>
              <Col span={6}><Typography.Text strong>{offlineNodes.length}</Typography.Text><br /><Typography.Text type="secondary">Offline</Typography.Text></Col>
              <Col span={6}><Typography.Text strong>{unknownNodes.length}</Typography.Text><br /><Typography.Text type="secondary">Unknown</Typography.Text></Col>
              <Col span={6}><Typography.Text strong>{disabledNodes.length}</Typography.Text><br /><Typography.Text type="secondary">Disabled</Typography.Text></Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} xl={16}>
          <Card title="Realtime">
            <Row gutter={[12, 12]}>{realtimeCards.map((card) => <Col key={card.label} xs={24} sm={12} lg={8}><MetricCard card={card} /></Col>)}</Row>
            <Typography.Title level={5}>Quality</Typography.Title>
            <Row gutter={[12, 12]}>{qualityCards.map((card) => <Col key={card.label} xs={24} sm={12} xl={8}><MetricCard card={card} /></Col>)}</Row>
          </Card>
        </Col>
      </Row>

      {metricError ? <Alert type="warning" showIcon message={metricError} /> : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={14}>
          <Card
            title="Resource trend"
            extra={metricsLoading ? <Tag>Loading metrics</Tag> : <Space><Tag color="green">CPU</Tag><Tag color="blue">Memory</Tag></Space>}
          >
            {trendPoints.length >= 2 ? (
              <TrendLine
                ariaLabel="Resource trend"
                height={176}
                series={[
                  { points: trendPoints.map((point) => point.mem), color: '#0ea5e9', strokeWidth: 1.6 },
                  { points: trendPoints.map((point) => point.cpu), color: '#10b981', strokeWidth: 1.8 },
                ]}
              />
            ) : (
              <EmptyText title="No trend data" text="Enabled nodes have not reported enough metric points yet." />
            )}
            <Row gutter={12}>
              <Col span={12}><MetricCard card={{ label: 'CPU', value: formatPercent(cpuAvg), hint: 'Average node probe.', tone: 'live' }} /></Col>
              <Col span={12}><MetricCard card={{ label: 'Memory', value: formatPercent(memAvg), hint: 'Average node probe.', tone: 'live' }} /></Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} xl={10}>
          <Card title="Infrastructure">
            <Row gutter={[12, 12]}>{infraCards.map((card) => <Col key={card.label} xs={24} sm={12}><MetricCard card={card} /></Col>)}</Row>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <Card title="Node load">
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              {loadedNodes.map((node) => (
                <Card key={node.id} size="small">
                  <Space direction="vertical" size={6} style={{ width: '100%' }}>
                    <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                      <Typography.Text strong>{node.name}</Typography.Text>
                      <Typography.Text>{node.cpu_pct.toFixed(1)}% / {node.mem_pct.toFixed(1)}%</Typography.Text>
                    </Space>
                    <div style={{ background: '#e2e8f0', borderRadius: 999, height: 8, overflow: 'hidden' }}>
                      <div style={{ background: '#10b981', height: 8, width: `${Math.min(100, Math.max(0, node.cpu_pct))}%` }} />
                    </div>
                    <div style={{ background: '#e2e8f0', borderRadius: 999, height: 8, overflow: 'hidden' }}>
                      <div style={{ background: '#0ea5e9', height: 8, width: `${Math.min(100, Math.max(0, node.mem_pct))}%` }} />
                    </div>
                  </Space>
                </Card>
              ))}
              {loadedNodes.length === 0 ? <EmptyText title="No node metrics" text="No enabled node has probe data yet." /> : null}
            </Space>
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="Node errors">
            {nodeErrorCount === 0 ? (
              <EmptyText title="No node errors" text="Fleet inventory returned without node errors." />
            ) : (
              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                {Object.entries(fleet.node_errors ?? {}).map(([id, text]) => (
                  <Alert key={id} type="warning" showIcon message={`Node ${id}`} description={text} />
                ))}
              </Space>
            )}
          </Card>
        </Col>
      </Row>

      <Card title="Business telemetry" extra={<Tag>Not connected</Tag>}>
        <Row gutter={[12, 12]}>
          <AnalysisPanel title="Concurrency queue" kind="bars" />
          <AnalysisPanel title="Account switch" kind="line" />
          <AnalysisPanel title="Throughput" kind="line" />
          <AnalysisPanel title="Duration distribution" kind="stack" />
          <AnalysisPanel title="Error distribution" kind="dots" />
          <AnalysisPanel title="Error trend" kind="line" />
        </Row>
      </Card>
    </Space>
  )
}

function EmptyText({ title, text }: { title: string; text: string }) {
  return (
    <div style={{ padding: 24, textAlign: 'center' }}>
      <Typography.Text strong>{title}</Typography.Text>
      <br />
      <Typography.Text type="secondary">{text}</Typography.Text>
    </div>
  )
}

function AnalysisPanel({ title, kind }: { title: string; kind: 'bars' | 'line' | 'stack' | 'dots' }) {
  return (
    <Col xs={24} md={12} xl={8}>
      <Card size="small" title={title} extra={<span aria-hidden style={{ background: '#94a3b8', borderRadius: 999, display: 'block', height: 8, width: 8 }} />}>
        <Typography.Text type="secondary">Business telemetry is not connected.</Typography.Text>
        <div style={{ marginTop: 16 }}>
          {kind === 'bars' ? <BarsPanel ariaLabel={`${title} bars`} values={[28, 44, 22, 52, 36]} /> : null}
          {kind === 'line' ? <TrendLine ariaLabel={`${title} line`} height={64} showGrid={false} series={[{ points: [25, 38, 30, 52, 46, 68, 60], color: '#cbd5e1' }]} /> : null}
          {kind === 'stack' ? (
            <svg aria-label={`${title} stack`} role="img" viewBox="0 0 120 48" style={{ height: 64, width: '100%' }}>
              <rect x="4" y="18" width="56" height="12" rx="6" fill="#cbd5e1" />
              <rect x="62" y="18" width="36" height="12" rx="6" fill="#cbd5e1" opacity="0.75" />
              <rect x="100" y="18" width="16" height="12" rx="6" fill="#cbd5e1" opacity="0.5" />
            </svg>
          ) : null}
          {kind === 'dots' ? <DotsGrid ariaLabel={`${title} dots`} values={[12, 8, 16, 10, 14, 8]} /> : null}
        </div>
        <Typography.Text type="secondary">Not connected</Typography.Text>
      </Card>
    </Col>
  )
}
