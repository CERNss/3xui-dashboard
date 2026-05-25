import type { Client, FleetInbound, Inbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'

export const PROTOCOL_OPTIONS = ['vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'hysteria'] as const
export type ProtocolFilter = (typeof PROTOCOL_OPTIONS)[number]
// Inbound settings are backend-owned JSON blobs whose nested shape varies by protocol.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type LooseJson = Record<string, any>

export function rowKey(row: FleetInbound) {
  return `${row.node_id}|${row.inbound.tag}`
}

export function parseJSON(value: string) {
  try {
    return JSON.parse(value || '{}') as LooseJson
  } catch {
    return {}
  }
}

export function parseClients(inbound: Inbound): Client[] {
  const settings = parseJSON(inbound.settings)
  return Array.isArray(settings.clients) ? settings.clients : []
}

export function formatBytes(value: number): string {
  if (!value) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let size = Math.abs(value)
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${unit === 0 ? size.toFixed(0) : size.toFixed(2)} ${units[unit]}`
}

export function formatLimit(value: number) {
  return value === 0 ? 'Unlimited' : formatBytes(value)
}

export function protocolKey(protocol: string): ProtocolFilter | null {
  const normalized = protocol.toLowerCase()
  if (normalized === 'hysteria2') return 'hysteria'
  return (PROTOCOL_OPTIONS as readonly string[]).includes(normalized) ? (normalized as ProtocolFilter) : null
}

export function filterInbounds(rows: FleetInbound[], query: string, protocols: ProtocolFilter[]) {
  const q = query.trim().toLowerCase()
  const selected = new Set(protocols)
  return rows.filter((row) => {
    const protocol = protocolKey(row.inbound.protocol)
    if (selected.size === 0) return false
    if (protocol ? !selected.has(protocol) : selected.size !== PROTOCOL_OPTIONS.length) return false
    if (!q) return true
    return (
      row.node_name.toLowerCase().includes(q) ||
      row.inbound.tag.toLowerCase().includes(q) ||
      row.inbound.remark.toLowerCase().includes(q) ||
      row.inbound.protocol.toLowerCase().includes(q) ||
      String(row.inbound.port).includes(q)
    )
  })
}

export function buildClientLink(row: FleetInbound, client: Client, nodes: Node[]): string {
  const inbound = row.inbound
  const node = nodes.find((item) => item.id === row.node_id)
  const host = node?.host ?? '127.0.0.1'
  const stream = parseJSON(inbound.streamSettings)
  const network = stream.network || 'tcp'
  const security = stream.security || 'none'
  const remark = `${inbound.remark || inbound.tag} - ${client.email}`
  const enc = encodeURIComponent

  if (inbound.protocol === 'vless') {
    const q = new URLSearchParams()
    q.set('type', network)
    q.set('security', security)
    if (client.flow) q.set('flow', client.flow)
    if (security === 'tls' && stream.tlsSettings?.serverName) q.set('sni', stream.tlsSettings.serverName)
    if (security === 'reality' && stream.realitySettings) {
      const reality = stream.realitySettings
      if (reality.publicKey) q.set('pbk', reality.publicKey)
      if (Array.isArray(reality.shortIds) && reality.shortIds.length) q.set('sid', reality.shortIds[0])
      if (Array.isArray(reality.serverNames) && reality.serverNames.length) q.set('sni', reality.serverNames[0])
      if (reality.fingerprint) q.set('fp', reality.fingerprint)
    }
    if (network === 'ws' && stream.wsSettings?.path) q.set('path', stream.wsSettings.path)
    if (network === 'grpc' && stream.grpcSettings?.serviceName) q.set('serviceName', stream.grpcSettings.serviceName)
    return `vless://${client.id}@${host}:${inbound.port}?${q.toString()}#${enc(remark)}`
  }
  if (inbound.protocol === 'trojan') {
    const q = new URLSearchParams()
    q.set('security', security || 'tls')
    if (stream.tlsSettings?.serverName) q.set('sni', stream.tlsSettings.serverName)
    return `trojan://${client.password}@${host}:${inbound.port}?${q.toString()}#${enc(remark)}`
  }
  if (inbound.protocol === 'shadowsocks') {
    const settings = parseJSON(inbound.settings)
    const method = settings.method || 'chacha20-ietf-poly1305'
    const encoded = btoa(`${method}:${client.password}`).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')
    return `ss://${encoded}@${host}:${inbound.port}#${enc(remark)}`
  }
  if (inbound.protocol === 'vmess') {
    const obj: Record<string, unknown> = {
      v: '2',
      ps: remark,
      add: host,
      port: inbound.port,
      id: client.id,
      aid: 0,
      scy: client.security || 'auto',
      net: network,
      type: 'none',
      tls: security,
    }
    if (network === 'ws' && stream.wsSettings?.path) obj.path = stream.wsSettings.path
    if (security === 'tls' && stream.tlsSettings?.serverName) obj.sni = stream.tlsSettings.serverName
    return `vmess://${btoa(JSON.stringify(obj)).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')}`
  }
  return ''
}
