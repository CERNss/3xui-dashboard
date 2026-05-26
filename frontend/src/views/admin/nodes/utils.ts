import type { Node, NodeInput } from '@/api/admin/nodes'

export const AREA_OPTIONS = [
  { key: 'jp', code: 'JP', label: 'Japan' },
  { key: 'sg', code: 'SG', label: 'Singapore' },
  { key: 'hk', code: 'HK', label: 'Hong Kong' },
  { key: 'tw', code: 'TW', label: 'Taiwan' },
  { key: 'us', code: 'US', label: 'United States' },
  { key: 'gb', code: 'UK', label: 'United Kingdom' },
  { key: 'de', code: 'DE', label: 'Germany' },
  { key: 'fr', code: 'FR', label: 'France' },
  { key: 'nl', code: 'NL', label: 'Netherlands' },
  { key: 'ca', code: 'CA', label: 'Canada' },
  { key: 'au', code: 'AU', label: 'Australia' },
  { key: 'kr', code: 'KR', label: 'Korea' },
  { key: 'in', code: 'IN', label: 'India' },
  { key: 'th', code: 'TH', label: 'Thailand' },
  { key: 'vn', code: 'VN', label: 'Vietnam' },
  { key: 'unknown', code: 'UN', label: 'Unknown' },
] as const

export type NodeAreaKey = (typeof AREA_OPTIONS)[number]['key']
export type NodeDisplayStatus = 'online' | 'offline' | 'unknown' | 'disabled'

export interface NodeFormValues {
  name: string
  area: NodeAreaKey
  province: string
  scheme: 'http' | 'https'
  host: string
  port: number
  base_path: string
  api_token?: string
  enabled: boolean
}

export function blankNodeForm(): NodeFormValues {
  return {
    name: '',
    area: 'unknown',
    province: 'unknown',
    scheme: 'https',
    host: '',
    port: 2053,
    base_path: '',
    api_token: '',
    enabled: true,
  }
}

export function normalizeNodeArea(area?: string | null): NodeAreaKey {
  const raw = (area || '').trim().toLowerCase()
  if (raw === 'other') return 'unknown'
  return AREA_OPTIONS.some((item) => item.key === raw) ? (raw as NodeAreaKey) : 'unknown'
}

export function normalizeNodeProvince(province?: string | null): string {
  const raw = (province || '').trim()
  return raw || 'unknown'
}

export function nodeToForm(node: Node): NodeFormValues {
  return {
    name: node.name,
    area: normalizeNodeArea(node.area),
    province: normalizeNodeProvince(node.province),
    scheme: node.scheme,
    host: node.host,
    port: node.port,
    base_path: node.base_path ?? '',
    api_token: '',
    enabled: node.enabled,
  }
}

export function formToPayload(values: NodeFormValues): NodeInput {
  return {
    name: values.name.trim(),
    area: normalizeNodeArea(values.area),
    province: normalizeNodeProvince(values.province),
    scheme: values.scheme,
    host: values.host.trim(),
    port: Math.max(1, Math.min(65535, Math.round(values.port || 0))),
    base_path: values.base_path?.trim() ?? '',
    api_token: values.api_token?.trim() ?? '',
    enabled: values.enabled,
  }
}

export function nodeDisplayStatus(node: Node): NodeDisplayStatus {
  if (!node.enabled) return 'disabled'
  if (node.status === 'online' || node.status === 'offline' || node.status === 'unknown') return node.status
  return 'unknown'
}

export function statusLabel(status: NodeDisplayStatus): string {
  if (status === 'disabled') return 'Disabled'
  return status[0].toUpperCase() + status.slice(1)
}

export function statusColor(status: NodeDisplayStatus): string {
  if (status === 'online') return 'green'
  if (status === 'offline') return 'red'
  if (status === 'disabled') return 'default'
  return 'gold'
}

export function normalizedPanelPath(basePath?: string | null): string {
  const raw = (basePath ?? '').trim()
  if (!raw || raw === '/') return '/panel'
  const withLeading = raw.startsWith('/') ? raw : `/${raw}`
  return withLeading.endsWith('/') ? withLeading.slice(0, -1) : withLeading
}

export function nodeConnectionURL(node: Node): string {
  return `${node.scheme}://${node.host}:${node.port}${normalizedPanelPath(node.base_path)}`
}

export function panelInboundURL(node: Node): string {
  return `${nodeConnectionURL(node)}/inbounds`
}

export function nodeLocationText(node: Node): string {
  const area = AREA_OPTIONS.find((item) => item.key === normalizeNodeArea(node.area))?.label ?? 'Unknown'
  return `${area} / ${normalizeNodeProvince(node.province)}`
}

export function formatLastSeen(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

export function parsePanelURL(value: string): Pick<NodeFormValues, 'scheme' | 'host' | 'port' | 'base_path'> | null {
  const raw = value.trim()
  if (!raw) return null
  const normalized = raw.startsWith('//') ? `https:${raw}` : /^[a-z][a-z\d+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`
  let url: URL
  try {
    url = new URL(normalized)
  } catch {
    return null
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
  const host = url.hostname.replace(/^\[(.*)]$/, '$1')
  if (!host) return null
  const scheme = url.protocol === 'http:' ? 'http' : 'https'
  const port = url.port ? Number(url.port) : scheme === 'http' ? 80 : 443
  if (!Number.isInteger(port) || port < 1 || port > 65535) return null
  return {
    scheme,
    host,
    port,
    base_path: panelPathFromURLPath(url.pathname),
  }
}

export function panelPathFromURLPath(pathname: string): string {
  const segments = pathname.split('/').filter(Boolean)
  if (segments.length === 0) return '/panel/'
  const panelIndex = segments.findIndex((item) => item.toLowerCase() === 'panel')
  if (panelIndex >= 0) return `/${segments.slice(0, panelIndex + 1).join('/')}/`
  const apiIndex = segments.findIndex((item) => item.toLowerCase() === 'api')
  if (apiIndex > 0) return `/${segments.slice(0, apiIndex).join('/')}/`
  return `/${[...segments, 'panel'].join('/')}/`
}

export function nodeExportRows(nodes: Node[]): NodeInput[] {
  return nodes.map((node) => ({
    name: node.name,
    area: normalizeNodeArea(node.area),
    province: normalizeNodeProvince(node.province),
    scheme: node.scheme,
    host: node.host,
    port: node.port,
    base_path: node.base_path,
    api_token: '',
    enabled: node.enabled,
  }))
}
