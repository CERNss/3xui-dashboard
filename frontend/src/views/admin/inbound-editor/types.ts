import type { Inbound } from '@/api/admin/inbounds'

export type ProtocolName =
  | 'vless'
  | 'vmess'
  | 'trojan'
  | 'shadowsocks'
  | 'wireguard'
  | 'hysteria'
  | 'http'
  | 'mixed'
  | 'tunnel'
  | 'tun'
export type TransmissionName = 'tcp' | 'ws' | 'grpc' | 'httpupgrade' | 'h2' | 'xhttp' | 'kcp' | 'quic'
export type SecurityName = 'none' | 'tls' | 'reality'

export interface InboundEditorValues {
  node_id: number | null
  enable: boolean
  remark: string
  protocol: ProtocolName
  listen: string
  port: number
  total_gb: number
  trafficReset: 'never' | 'daily' | 'weekly' | 'monthly' | 'yearly'
  expiryTime?: string

  clients: Array<Record<string, unknown>>
  decryption: string
  encryption: string
  vlessAuthMode: 'none' | 'x25519' | 'mlkem768'
  disableInsecureEncryption: boolean
  fallbacks: Array<{
    name: string
    alpn: string
    path: string
    dest: string
    xver: number
  }>
  ssMethod: string
  ssNetwork: 'tcp' | 'udp' | 'tcp,udp'
  ssPassword: string
  ssIvCheck: boolean
  wireguardMtu: number
  wireguardSecretKey: string
  wireguardPublicKey: string
  wireguardNoKernelTun: boolean
  wireguardGenerateKeypair: boolean
  hysteriaVersion: 1 | 2
  hysteriaSni: string
  hysteriaAuth: string
  hysteriaObfs: string
  hysteriaUpMbps: number
  hysteriaDownMbps: number
  hysteriaUdpIdleTimeout: number
  hysteriaMasqueradeType: '' | 'proxy' | 'file' | 'string'
  hysteriaMasqueradeProxyUrl: string
  hysteriaMasqueradeProxyRewriteHost: boolean
  hysteriaMasqueradeProxyInsecure: boolean
  hysteriaMasqueradeFileDir: string
  hysteriaMasqueradeStringContent: string
  hysteriaMasqueradeStringStatusCode: number
  hysteriaMasqueradeStringHeaders: Array<{ key: string; value: string }>

  httpAllowTransparent: boolean
  mixedAuth: 'noauth' | 'password'
  mixedUdp: boolean
  mixedUdpIP: string

  tunnelRewriteAddress: string
  tunnelRewritePort: number
  tunnelAllowedNetwork: 'tcp,udp' | 'tcp' | 'udp'
  tunnelPortMap: Array<{ name: string; value: string }>
  tunnelFollowRedirect: boolean

  tunName: string
  tunMtu: number
  tunGateway: string[]
  tunDns: string[]
  tunUserLevel: number
  tunAutoSystemRoutingTable: string[]
  tunAutoOutboundsInterface: string

  network: TransmissionName
  security: SecurityName
  proxyProtocol: boolean
  httpHeader: boolean
  httpHeaderHost: string
  httpHeaderPath: string
  wsPath: string
  wsHost: string
  grpcServiceName: string
  grpcMultiMode: boolean
  httpupgradePath: string
  httpupgradeHost: string
  h2Path: string
  h2Host: string
  xhttpPath: string
  xhttpHost: string
  xhttpMode: 'auto' | 'packet-up' | 'stream-up' | 'stream-one'
  kcpMtu: number
  kcpTti: number
  kcpUpCap: number
  kcpDownCap: number
  kcpCongestion: boolean
  kcpHeader: 'none' | 'srtp' | 'utp' | 'wechat-video' | 'dtls' | 'wireguard'
  kcpSeed: string
  quicSecurity: 'none' | 'aes-128-gcm' | 'chacha20-poly1305'
  quicKey: string
  quicHeader: 'none' | 'srtp' | 'utp' | 'wechat-video' | 'dtls' | 'wireguard'

  tlsServerName: string
  tlsAlpn: string[]
  tlsFingerprint: '' | 'chrome' | 'firefox' | 'safari' | 'ios' | 'android' | 'edge' | 'random' | 'randomized'
  tlsAllowInsecure: boolean
  tlsCertificateFile: string
  tlsKeyFile: string
  realityDest: string
  realityServerNames: string
  realityPublicKey: string
  realityPrivateKey: string
  realityShortIds: string
  realityFingerprint: 'chrome' | 'firefox' | 'safari' | 'ios' | 'android' | 'edge' | 'random' | 'randomized'
  realityShow: boolean
  realityXver: number
  realityMaxTimeDiff: number
  realityMinClientVer: string
  realityMaxClientVer: string
  realitySpiderX: string
  realityMldsa65Seed: string
  realityMldsa65Verify: string
  realityGenerateKeypair: boolean
  realityGenerateMldsa65: boolean
  realityRandomizeTarget: boolean
  realityRandomizeSNI: boolean
  realityRandomizeShortIds: boolean

  sniffEnabled: boolean
  sniffHttp: boolean
  sniffTls: boolean
  sniffQuic: boolean
  sniffFakedns: boolean
  sniffMetadataOnly: boolean
  sniffRouteOnly: boolean

  advSettingsOverride: boolean
  advSettings: string
  advStreamOverride: boolean
  advStream: string
  advSniffingOverride: boolean
  advSniffing: string
}

export interface InboundEditorProps {
  open: boolean
  mode: 'create' | 'edit'
  nodeID: number | null
  tag: string
  source?: Inbound | null
  nodes: Array<{ id: number; name: string; enabled: boolean }>
  onClose: () => void
  onSaved?: (inbound: Inbound) => void
}
