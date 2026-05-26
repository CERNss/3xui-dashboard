import type { FleetResult } from '@/api/admin/inbounds'
import type { Node, NodeMetricPoint } from '@/api/admin/nodes'

export interface TrendPoint {
  time: number
  cpu: number
  mem: number
}

export function safeFleet(value: FleetResult | null | undefined): FleetResult {
  return {
    ...(value ?? { inbounds: [] }),
    inbounds: Array.isArray(value?.inbounds) ? value.inbounds : [],
    node_errors: value?.node_errors ?? {},
  }
}

export function avg(values: number[]) {
  if (values.length === 0) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

export function formatPercent(value: number | null, fallback: string) {
  if (value === null || Number.isNaN(value)) return fallback
  return `${value.toFixed(1)}%`
}

export function hasProbeData(node: Node) {
  return Boolean(node.status === 'online' || node.last_seen_at || node.cpu_pct > 0 || node.mem_pct > 0)
}

export function buildTrendPoints(metricSeries: Record<number, NodeMetricPoint[]>): TrendPoint[] {
  const buckets = new Map<number, { cpu: number; mem: number; count: number }>()
  Object.values(metricSeries).forEach((points) => {
    points.forEach((point) => {
      const time = new Date(point.time).getTime()
      if (!Number.isFinite(time) || !Number.isFinite(point.cpu) || !Number.isFinite(point.mem)) return
      const bucket = buckets.get(time) ?? { cpu: 0, mem: 0, count: 0 }
      bucket.cpu += point.cpu
      bucket.mem += point.mem
      bucket.count += 1
      buckets.set(time, bucket)
    })
  })

  return [...buckets.entries()]
    .sort(([left], [right]) => left - right)
    .slice(-24)
    .map(([time, point]) => ({ time, cpu: point.cpu / point.count, mem: point.mem / point.count }))
}
