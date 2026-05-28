import type { Inbound } from '@/api/admin/inbounds'
import type { InboundTemplate, InboundTemplateInput } from '@/api/admin/inboundTemplates'
import type { InboundEditorValues, ProtocolName, SecurityName, TransmissionName } from './types'

const BYTES_PER_GB = 1024 * 1024 * 1024
// Inbound settings are backend-owned JSON blobs whose nested shape varies by protocol.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type LooseJson = Record<string, any>

function parseJSON(value: string, fallback: LooseJson = {}) {
  try {
    return JSON.parse(value || '{}') as LooseJson
  } catch {
    return fallback
  }
}

function stringify(value: unknown) {
  return JSON.stringify(value)
}

function msToInput(value?: number) {
  if (!value) return ''
  const date = new Date(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function inputToMs(value?: string) {
  return value ? new Date(value).getTime() : 0
}

export function blankInboundValues(nodeID: number | null): InboundEditorValues {
  return {
    node_id: nodeID,
    enable: true,
    remark: '',
    protocol: 'vless',
    listen: '',
    port: 44400,
    total_gb: 0,
    trafficReset: 'never',
    expiryTime: '',

    clients: [],
    decryption: 'none',
    disableInsecureEncryption: false,
    ssMethod: 'chacha20-ietf-poly1305',
    ssNetwork: 'tcp,udp',
    ssPassword: '',
    wireguardMtu: 1420,
    wireguardSecretKey: '',
    wireguardNoKernelTun: false,
    hysteriaSni: '',
    hysteriaAuth: '',
    hysteriaObfs: '',
    hysteriaUpMbps: 100,
    hysteriaDownMbps: 100,

    network: 'tcp',
    security: 'none',
    proxyProtocol: false,
    httpHeader: false,
    httpHeaderHost: '',
    httpHeaderPath: '/',
    wsPath: '/',
    wsHost: '',
    grpcServiceName: 'grpc',
    grpcMultiMode: false,
    httpupgradePath: '/',
    httpupgradeHost: '',
    h2Path: '/',
    h2Host: '',
    xhttpPath: '/',
    xhttpHost: '',
    xhttpMode: 'auto',
    kcpMtu: 1350,
    kcpTti: 50,
    kcpUpCap: 5,
    kcpDownCap: 20,
    kcpCongestion: false,
    kcpHeader: 'none',
    kcpSeed: '',
    quicSecurity: 'none',
    quicKey: '',
    quicHeader: 'none',

    tlsServerName: '',
    tlsAlpn: ['h2', 'http/1.1'],
    tlsFingerprint: 'chrome',
    tlsAllowInsecure: false,
    tlsCertificateFile: '',
    tlsKeyFile: '',
    realityDest: 'www.cloudflare.com:443',
    realityServerNames: 'www.cloudflare.com',
    realityPublicKey: '',
    realityPrivateKey: '',
    realityShortIds: '',
    realityFingerprint: 'chrome',

    sniffEnabled: true,
    sniffHttp: true,
    sniffTls: true,
    sniffQuic: false,
    sniffFakedns: false,
    sniffMetadataOnly: false,
    sniffRouteOnly: false,

    advSettingsOverride: false,
    advSettings: '',
    advStreamOverride: false,
    advStream: '',
    advSniffingOverride: false,
    advSniffing: '',
  }
}

export function inboundToValues(inbound: Inbound, nodeID: number | null): InboundEditorValues {
  const values = blankInboundValues(nodeID)
  values.enable = inbound.enable
  values.remark = inbound.remark
  values.protocol = (inbound.protocol === 'hysteria2' ? 'hysteria' : inbound.protocol) as ProtocolName
  values.listen = inbound.listen
  values.port = inbound.port
  values.total_gb = inbound.total ? inbound.total / BYTES_PER_GB : 0
  values.trafficReset = (inbound.trafficReset as InboundEditorValues['trafficReset']) || 'never'
  values.expiryTime = msToInput(inbound.expiryTime)

  const settings = parseJSON(inbound.settings)
  values.clients = Array.isArray(settings.clients) ? settings.clients : []
  values.decryption = settings.decryption ?? 'none'
  values.disableInsecureEncryption = Boolean(settings.disableInsecureEncryption)
  values.ssMethod = settings.method ?? values.ssMethod
  values.ssNetwork = settings.network ?? values.ssNetwork
  values.ssPassword = settings.password ?? ''
  values.wireguardMtu = settings.mtu ?? 1420
  values.wireguardSecretKey = settings.secretKey ?? ''
  values.wireguardNoKernelTun = Boolean(settings.noKernelTun)
  values.hysteriaAuth = settings.auth ?? settings.auth_str ?? ''
  values.hysteriaObfs = settings.obfs ?? ''
  values.hysteriaUpMbps = settings.up_mbps ?? settings.upMbps ?? 100
  values.hysteriaDownMbps = settings.down_mbps ?? settings.downMbps ?? 100
  values.advSettings = JSON.stringify(settings, null, 2)

  const stream = parseJSON(inbound.streamSettings)
  values.network = (stream.network as TransmissionName) || 'tcp'
  values.security = (stream.security as SecurityName) || 'none'
  if (stream.network === 'hysteria') values.network = 'tcp'
  if (stream.tcpSettings) {
    values.proxyProtocol = Boolean(stream.tcpSettings.acceptProxyProtocol)
    const header = stream.tcpSettings.header
    if (header?.type === 'http') {
      values.httpHeader = true
      values.httpHeaderPath = header.request?.path?.[0] ?? '/'
      values.httpHeaderHost = header.request?.headers?.Host?.[0] ?? ''
    }
  }
  if (stream.wsSettings) {
    values.wsPath = stream.wsSettings.path ?? '/'
    values.wsHost = stream.wsSettings.host ?? stream.wsSettings.headers?.Host ?? ''
    values.proxyProtocol = Boolean(stream.wsSettings.acceptProxyProtocol)
  }
  if (stream.grpcSettings) {
    values.grpcServiceName = stream.grpcSettings.serviceName ?? 'grpc'
    values.grpcMultiMode = Boolean(stream.grpcSettings.multiMode)
  }
  if (stream.httpupgradeSettings) {
    values.httpupgradePath = stream.httpupgradeSettings.path ?? '/'
    values.httpupgradeHost = stream.httpupgradeSettings.host ?? ''
  }
  if (stream.httpSettings || stream.h2Settings) {
    const h2 = stream.httpSettings || stream.h2Settings
    values.h2Path = h2.path ?? '/'
    values.h2Host = h2.host?.[0] ?? ''
  }
  if (stream.xhttpSettings) {
    values.xhttpPath = stream.xhttpSettings.path ?? '/'
    values.xhttpHost = stream.xhttpSettings.host ?? ''
    values.xhttpMode = stream.xhttpSettings.mode ?? 'auto'
  }
  if (stream.kcpSettings) {
    values.kcpMtu = stream.kcpSettings.mtu ?? 1350
    values.kcpTti = stream.kcpSettings.tti ?? 50
    values.kcpUpCap = stream.kcpSettings.uplinkCapacity ?? 5
    values.kcpDownCap = stream.kcpSettings.downlinkCapacity ?? 20
    values.kcpCongestion = Boolean(stream.kcpSettings.congestion)
    values.kcpHeader = stream.kcpSettings.header?.type ?? 'none'
    values.kcpSeed = stream.kcpSettings.seed ?? ''
  }
  if (stream.quicSettings) {
    values.quicSecurity = stream.quicSettings.security ?? 'none'
    values.quicKey = stream.quicSettings.key ?? ''
    values.quicHeader = stream.quicSettings.header?.type ?? 'none'
  }
  if (stream.tlsSettings) {
    values.tlsServerName = stream.tlsSettings.serverName ?? ''
    values.hysteriaSni = values.tlsServerName
    values.tlsAlpn = Array.isArray(stream.tlsSettings.alpn) ? stream.tlsSettings.alpn : ['h2', 'http/1.1']
    values.tlsFingerprint = stream.tlsSettings.fingerprint ?? 'chrome'
    values.tlsAllowInsecure = Boolean(stream.tlsSettings.allowInsecure)
    const cert = Array.isArray(stream.tlsSettings.certificates) ? stream.tlsSettings.certificates[0] : null
    values.tlsCertificateFile = cert?.certificateFile ?? ''
    values.tlsKeyFile = cert?.keyFile ?? ''
  }
  if (stream.realitySettings) {
    values.realityDest = stream.realitySettings.dest ?? ''
    values.realityServerNames = (stream.realitySettings.serverNames ?? []).join(',')
    values.realityPublicKey = stream.realitySettings.publicKey ?? ''
    values.realityPrivateKey = stream.realitySettings.privateKey ?? ''
    values.realityShortIds = (stream.realitySettings.shortIds ?? []).join(',')
    values.realityFingerprint = stream.realitySettings.fingerprint ?? 'chrome'
  }
  values.advStream = JSON.stringify(stream, null, 2)

  const sniffing = parseJSON(inbound.sniffing)
  values.sniffEnabled = Boolean(sniffing.enabled)
  const dest = Array.isArray(sniffing.destOverride) ? sniffing.destOverride : []
  values.sniffHttp = dest.includes('http')
  values.sniffTls = dest.includes('tls')
  values.sniffQuic = dest.includes('quic')
  values.sniffFakedns = dest.includes('fakedns')
  values.sniffMetadataOnly = Boolean(sniffing.metadataOnly)
  values.sniffRouteOnly = Boolean(sniffing.routeOnly)
  values.advSniffing = JSON.stringify(sniffing, null, 2)

  return values
}

function settingsFromValues(values: InboundEditorValues) {
  if (values.protocol === 'vless') return { clients: values.clients, decryption: values.decryption || 'none', fallbacks: [] }
  if (values.protocol === 'vmess') return { clients: values.clients, disableInsecureEncryption: values.disableInsecureEncryption }
  if (values.protocol === 'trojan') return { clients: values.clients, fallbacks: [] }
  if (values.protocol === 'shadowsocks') {
    return { clients: values.clients, method: values.ssMethod, network: values.ssNetwork, password: values.ssPassword }
  }
  if (values.protocol === 'wireguard') {
    return {
      mtu: values.wireguardMtu,
      secretKey: values.wireguardSecretKey,
      peers: values.clients,
      noKernelTun: values.wireguardNoKernelTun,
    }
  }
  return {
    clients: values.clients,
    version: 2,
    auth: values.hysteriaAuth,
    obfs: values.hysteriaObfs,
    up_mbps: values.hysteriaUpMbps,
    down_mbps: values.hysteriaDownMbps,
  }
}

function streamFromValues(values: InboundEditorValues) {
  if (values.protocol === 'hysteria') {
    return {
      network: 'hysteria',
      security: 'tls',
      tlsSettings: {
        serverName: values.hysteriaSni || values.tlsServerName,
        alpn: ['h3'],
        fingerprint: values.tlsFingerprint || undefined,
        allowInsecure: values.tlsAllowInsecure,
        certificates:
          values.tlsCertificateFile || values.tlsKeyFile
            ? [{ certificateFile: values.tlsCertificateFile, keyFile: values.tlsKeyFile }]
            : [],
      },
      hysteriaSettings: { version: 2, udpIdleTimeout: 60 },
    }
  }

  const out: Record<string, unknown> = { network: values.network, security: values.security }
  if (values.network === 'tcp') {
    out.tcpSettings = {
      acceptProxyProtocol: values.proxyProtocol,
      header: values.httpHeader
        ? { type: 'http', request: { path: [values.httpHeaderPath || '/'], headers: values.httpHeaderHost ? { Host: [values.httpHeaderHost] } : {} } }
        : { type: 'none' },
    }
  } else if (values.network === 'ws') {
    out.wsSettings = {
      acceptProxyProtocol: values.proxyProtocol,
      path: values.wsPath || '/',
      headers: values.wsHost ? { Host: values.wsHost } : {},
    }
  } else if (values.network === 'grpc') {
    out.grpcSettings = { serviceName: values.grpcServiceName || 'grpc', multiMode: values.grpcMultiMode }
  } else if (values.network === 'httpupgrade') {
    out.httpupgradeSettings = { path: values.httpupgradePath || '/', host: values.httpupgradeHost }
  } else if (values.network === 'h2') {
    out.httpSettings = { path: values.h2Path || '/', host: values.h2Host ? [values.h2Host] : [] }
  } else if (values.network === 'xhttp') {
    out.xhttpSettings = { path: values.xhttpPath || '/', host: values.xhttpHost, mode: values.xhttpMode }
  } else if (values.network === 'kcp') {
    out.kcpSettings = {
      mtu: values.kcpMtu,
      tti: values.kcpTti,
      uplinkCapacity: values.kcpUpCap,
      downlinkCapacity: values.kcpDownCap,
      congestion: values.kcpCongestion,
      header: { type: values.kcpHeader },
      seed: values.kcpSeed,
    }
  } else if (values.network === 'quic') {
    out.quicSettings = { security: values.quicSecurity, key: values.quicKey, header: { type: values.quicHeader } }
  }

  if (values.security === 'tls') {
    out.tlsSettings = {
      serverName: values.tlsServerName,
      alpn: values.tlsAlpn,
      fingerprint: values.tlsFingerprint || undefined,
      allowInsecure: values.tlsAllowInsecure,
      certificates:
        values.tlsCertificateFile || values.tlsKeyFile
          ? [{ certificateFile: values.tlsCertificateFile, keyFile: values.tlsKeyFile }]
          : [],
    }
  } else if (values.security === 'reality') {
    out.realitySettings = {
      dest: values.realityDest,
      serverNames: values.realityServerNames.split(',').map((item) => item.trim()).filter(Boolean),
      publicKey: values.realityPublicKey,
      privateKey: values.realityPrivateKey,
      shortIds: values.realityShortIds.split(',').map((item) => item.trim()).filter(Boolean),
      fingerprint: values.realityFingerprint,
    }
  }
  return out
}

function sniffingFromValues(values: InboundEditorValues) {
  const destOverride: string[] = []
  if (values.sniffHttp) destOverride.push('http')
  if (values.sniffTls) destOverride.push('tls')
  if (values.sniffQuic) destOverride.push('quic')
  if (values.sniffFakedns) destOverride.push('fakedns')
  return {
    enabled: values.sniffEnabled,
    destOverride,
    metadataOnly: values.sniffMetadataOnly,
    routeOnly: values.sniffRouteOnly,
  }
}

function jsonOrFallback(text: string, fallback: unknown) {
  try {
    return stringify(JSON.parse(text))
  } catch {
    return stringify(fallback)
  }
}

/**
 * Hydrate an InboundTemplate into the InboundEditorValues shape so
 * the template editor can reuse the inbound-editor's form fields,
 * tabs, and protocol components. Template doesn't store port / tag /
 * node — those stay at their blank defaults.
 */
export function templateToValues(template: InboundTemplate): InboundEditorValues {
  const synthetic: Inbound = {
    id: 0,
    up: 0,
    down: 0,
    allTime: 0,
    clientStats: [],
    tag: '',
    enable: true,
    remark: template.remark,
    protocol: template.protocol,
    listen: template.listen,
    port: 0,
    total: template.total,
    expiryTime: template.expiryTime,
    trafficReset: template.trafficReset,
    settings: template.settings,
    streamSettings: template.streamSettings,
    sniffing: template.sniffing,
  }
  return inboundToValues(synthetic, null)
}

/**
 * Serialize an editor form back into the InboundTemplateInput shape.
 * Templates don't carry port / tag / node — those are runtime
 * decisions made when the operator creates a real inbound from this
 * template. Top-level template-only fields (name, description,
 * enabled) are supplied by the caller and not derived here.
 */
export function valuesToTemplateBody(values: InboundEditorValues): Omit<InboundTemplateInput, 'name' | 'description' | 'enabled'> {
  const body = valuesToInboundBody(values)
  return {
    protocol: body.protocol ?? '',
    remark: (body.remark ?? '').trim(),
    listen: (body.listen ?? '').trim(),
    total: body.total ?? 0,
    expiryTime: body.expiryTime ?? 0,
    trafficReset: body.trafficReset ?? 'never',
    settings: body.settings ?? '{}',
    streamSettings: body.streamSettings ?? '{}',
    sniffing: body.sniffing ?? '{}',
  }
}

export function valuesToInboundBody(values: InboundEditorValues): Partial<Inbound> {
  const settings = settingsFromValues(values)
  const streamSettings = streamFromValues(values)
  const sniffing = sniffingFromValues(values)
  return {
    remark: values.remark.trim(),
    enable: values.enable,
    listen: values.listen.trim(),
    port: Math.round(values.port),
    protocol: values.protocol,
    expiryTime: inputToMs(values.expiryTime),
    total: Math.max(0, Math.round((values.total_gb || 0) * BYTES_PER_GB)),
    trafficReset: values.trafficReset,
    settings: values.advSettingsOverride && values.advSettings.trim() ? jsonOrFallback(values.advSettings, settings) : stringify(settings),
    streamSettings: values.advStreamOverride && values.advStream.trim() ? jsonOrFallback(values.advStream, streamSettings) : stringify(streamSettings),
    sniffing: values.advSniffingOverride && values.advSniffing.trim() ? jsonOrFallback(values.advSniffing, sniffing) : stringify(sniffing),
    tag: '',
  }
}
