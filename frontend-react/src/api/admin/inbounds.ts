import { adminClient } from '../client/admin'

// Mirrors backend runtime.Inbound + runtime.ClientTraffic.
export interface ClientTraffic {
  id: number
  inboundId: number
  enable: boolean
  email: string
  up: number
  down: number
  allTime: number
  expiryTime: number // unix ms
  total: number      // bytes
  reset: number
}

export interface Inbound {
  id: number
  up: number
  down: number
  total: number       // bytes; 0 = unlimited
  allTime: number
  remark: string
  enable: boolean
  expiryTime: number  // unix ms; 0 = never
  trafficReset: string
  clientStats: ClientTraffic[]
  listen: string
  port: number
  protocol: string
  settings: string         // stringified JSON
  streamSettings: string   // stringified JSON
  tag: string
  sniffing: string         // stringified JSON
}

export interface FleetInbound {
  node_id: number
  node_name: string
  inbound: Inbound
}

export interface FleetResult {
  inbounds: FleetInbound[]
  node_errors?: Record<number, string>
}

export const inboundsApi = {
  fleet: () =>
    adminClient.get<FleetResult>('/inbounds').then((r) => r.data),

  get: (nodeID: number, tag: string) =>
    adminClient
      .get<Inbound>(`/inbounds/nodes/${nodeID}/${encodeURIComponent(tag)}`)
      .then((r) => r.data),

  create: (nodeID: number, body: Partial<Inbound>) =>
    adminClient.post<Inbound>(`/inbounds/nodes/${nodeID}`, body).then((r) => r.data),

  update: (nodeID: number, tag: string, body: Partial<Inbound>) =>
    adminClient
      .put<Inbound>(`/inbounds/nodes/${nodeID}/${encodeURIComponent(tag)}`, body)
      .then((r) => r.data),

  setEnable: (nodeID: number, tag: string, enable: boolean) =>
    adminClient.post<void>(
      `/inbounds/nodes/${nodeID}/${encodeURIComponent(tag)}/${enable ? 'enable' : 'disable'}`,
    ),

  remove: (nodeID: number, tag: string) =>
    adminClient.delete<void>(`/inbounds/nodes/${nodeID}/${encodeURIComponent(tag)}`),
}

export interface NodeSnapshot {
  Inbounds: Inbound[] | null
  OnlineEmails: string[] | null
  LastOnlineByEmail: Record<string, number> | null
}

export interface Client {
  id?: string
  password?: string
  auth?: string
  security?: string
  flow?: string
  email: string
  limitIp?: number
  totalGB?: number
  expiryTime?: number
  enable: boolean
  tgId?: number
  subId?: string
  comment?: string
  reset?: number
}

export const clientsApi = {
  snapshot: (nodeID: number) =>
    adminClient.get<NodeSnapshot>(`/clients/nodes/${nodeID}/snapshot`).then((r) => r.data),

  add: (nodeID: number, tag: string, client: Partial<Client>, userID = 0) =>
    adminClient
      .post(`/clients/nodes/${nodeID}/inbounds/${encodeURIComponent(tag)}/add`, {
        client,
        user_id: userID,
      })
      .then((r) => r.data),

  update: (nodeID: number, tag: string, email: string, client: Partial<Client>) =>
    adminClient
      .put(
        `/clients/nodes/${nodeID}/inbounds/${encodeURIComponent(tag)}/clients/${encodeURIComponent(email)}`,
        { ...client, email },
      )
      .then((r) => r.data),

  remove: (nodeID: number, tag: string, email: string) =>
    adminClient.delete(
      `/clients/nodes/${nodeID}/inbounds/${encodeURIComponent(tag)}/clients/${encodeURIComponent(email)}`,
    ),
}

export const trafficApi = {
  resetClient: (nodeID: number, tag: string, email: string) =>
    adminClient.post<void>(
      `/traffic/reset/node/${nodeID}/inbound/${encodeURIComponent(tag)}/client/${encodeURIComponent(email)}`,
    ),
  resetInbound: (nodeID: number, tag: string) =>
    adminClient.post<void>(`/traffic/reset/node/${nodeID}/inbound/${encodeURIComponent(tag)}`),
  resetNode: (nodeID: number) => adminClient.post<void>(`/traffic/reset/node/${nodeID}`),
}

// Simplified input shape the modal binds to.
export interface InboundFormInput {
  remark: string
  port: number
  protocol: 'vless' | 'vmess' | 'trojan' | 'shadowsocks'
  network: 'tcp' | 'ws' | 'grpc'
  security: 'none' | 'tls'
  sniffing: boolean
  listen?: string
  tlsServerName?: string
  wsPath?: string
  grpcServiceName?: string
}

// composeInboundBody turns the form input into the wire-format
// runtime.Inbound payload (settings / streamSettings / sniffing are
// stringified JSON per the 3x-ui contract).
export function composeInboundBody(input: InboundFormInput): Partial<Inbound> {
  const settings = (() => {
    if (input.protocol === 'vless') {
      return JSON.stringify({ clients: [], decryption: 'none', fallbacks: [] })
    }
    return JSON.stringify({ clients: [] })
  })()

  const stream: Record<string, unknown> = {
    network: input.network,
    security: input.security,
  }
  if (input.security === 'tls' && input.tlsServerName) {
    stream.tlsSettings = { serverName: input.tlsServerName, alpn: ['h2', 'http/1.1'] }
  }
  if (input.network === 'ws') {
    stream.wsSettings = { path: input.wsPath || '/', headers: {} }
  }
  if (input.network === 'grpc') {
    stream.grpcSettings = { serviceName: input.grpcServiceName || 'grpc' }
  }

  const sniffing = input.sniffing
    ? JSON.stringify({ enabled: true, destOverride: ['http', 'tls'] })
    : JSON.stringify({ enabled: false })

  return {
    remark: input.remark,
    enable: true,
    listen: input.listen || '',
    port: input.port,
    protocol: input.protocol,
    expiryTime: 0,
    total: 0,
    settings,
    streamSettings: JSON.stringify(stream),
    sniffing,
    tag: '',
  }
}

// streamSettings → { network, security } summary chips
export function parseTransport(stream: string): { network: string; security: string } {
  try {
    const obj = JSON.parse(stream)
    return {
      network: typeof obj?.network === 'string' ? obj.network : 'tcp',
      security: typeof obj?.security === 'string' ? obj.security : 'none',
    }
  } catch {
    return { network: 'tcp', security: 'none' }
  }
}
