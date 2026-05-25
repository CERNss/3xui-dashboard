import { adminClient } from '../client/admin'

export interface Node {
  id: number
  name: string
  scheme: 'http' | 'https'
  host: string
  port: number
  base_path: string
  enabled: boolean
  status: 'online' | 'offline' | 'unknown'
  cpu_pct: number
  mem_pct: number
  xray_version: string
  uptime_s: number
  last_seen_at?: string | null
  created_at: string
  updated_at: string
}

export interface NodeMetricPoint {
  time: string
  cpu: number
  mem: number
}

export interface NodeMetricsResult {
  id: number
  from: number
  to: number
  bucket: string
  points: NodeMetricPoint[]
}

export interface NodeInput {
  name: string
  scheme: 'http' | 'https'
  host: string
  port: number
  base_path: string
  api_token?: string
  enabled: boolean
}

export const nodesApi = {
  list: () => adminClient.get<{ nodes: Node[] }>('/nodes').then((r) => r.data.nodes),
  get: (id: number) => adminClient.get<Node>(`/nodes/${id}`).then((r) => r.data),
  create: (body: NodeInput) => adminClient.post<Node>('/nodes', body).then((r) => r.data),
  update: (id: number, body: NodeInput) =>
    adminClient.put<Node>(`/nodes/${id}`, body).then((r) => r.data),
  remove: (id: number) => adminClient.delete<void>(`/nodes/${id}`).then((r) => r.data),
  enable: (id: number) => adminClient.post<void>(`/nodes/${id}/enable`).then((r) => r.data),
  disable: (id: number) => adminClient.post<void>(`/nodes/${id}/disable`).then((r) => r.data),
  probe: (id: number) =>
    adminClient.post<{ id: number; prior_status: string; status: unknown }>(`/nodes/${id}/probe`).then((r) => r.data),
  metrics: (id: number, params?: { from?: number; to?: number; bucket?: string }) =>
    adminClient
      .get<NodeMetricsResult>(`/nodes/${id}/metrics`, { params })
      .then((r) => r.data),
}
