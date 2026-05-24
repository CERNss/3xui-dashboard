<script setup lang="ts">
import { computed, defineComponent, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatError } from '@/utils/format'

import { inboundsApi, type Inbound } from '@/api/admin/inbounds'
import type { Node } from '@/api/admin/nodes'

const { t } = useI18n()

// ---- Local UI components (declared first so template can resolve them) ----

const Row = defineComponent({
  props: { label: { type: String, required: true } },
  setup(props, { slots, attrs }) {
    return () =>
      h('div', { ...attrs, class: ['flex min-w-0 items-center gap-3', attrs.class] }, [
        h('label', { class: 'w-28 shrink-0 text-right text-xs leading-4 text-surface-600 dark:text-surface-300' }, props.label),
        h('div', { class: 'flex-1' }, slots.default?.()),
      ])
  },
})

const ToggleBtn = defineComponent({
  props: { modelValue: { type: Boolean, required: true } },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h(
        'button',
        {
          type: 'button',
          class: [
            'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
            props.modelValue ? 'bg-accent-500' : 'bg-surface-300 dark:bg-surface-700',
          ],
          onClick: () => emit('update:modelValue', !props.modelValue),
        },
        [
          h('span', {
            class: [
              'inline-block h-5 w-5 transform rounded-full bg-white shadow transition-transform',
              props.modelValue ? 'translate-x-5' : 'translate-x-0.5',
            ],
          }),
        ],
      )
  },
})

const Info = defineComponent({
  setup(_, { slots }) {
    return () =>
      h(
        'div',
        { class: 'flex items-start gap-2 rounded-lg bg-primary-50 px-3 py-2 text-xs text-primary-800 dark:bg-primary-950/40 dark:text-primary-200' },
        [
          h(
            'svg',
            {
              class: 'mt-0.5 h-4 w-4 shrink-0',
              viewBox: '0 0 24 24',
              fill: 'none',
              stroke: 'currentColor',
              'stroke-width': '2',
              'stroke-linecap': 'round',
              'stroke-linejoin': 'round',
            },
            [h('circle', { cx: '12', cy: '12', r: '10' }), h('path', { d: 'M12 16v-4M12 8h.01' })],
          ),
          h('div', { class: 'flex-1' }, slots.default?.()),
        ],
      )
  },
})

const AdvancedJSON = defineComponent({
  props: {
    label: { type: String, required: true },
    description: String,
    override: { type: Boolean, required: true },
    value: { type: String, required: true },
  },
  emits: ['update:override', 'update:value'],
  setup(props, { emit, attrs }) {
    return () =>
      h('div', {
        ...attrs,
        class: [
          'min-w-0 space-y-2 rounded-lg border border-surface-200 bg-surface-50/70 p-3 dark:border-surface-800 dark:bg-surface-950/30',
          attrs.class,
        ],
      }, [
        h('div', { class: 'flex min-w-0 items-start justify-between gap-2' }, [
          h('div', { class: 'min-w-0' }, [
            h('div', { class: 'truncate font-mono text-xs font-semibold text-ink-900 dark:text-surface-50' }, props.label),
            props.description ? h('div', { class: 'mt-0.5 truncate text-2xs text-surface-500' }, props.description) : null,
          ]),
          h('label', { class: 'flex shrink-0 items-center gap-1.5 text-2xs text-surface-600 dark:text-surface-300' }, [
            h('input', {
              type: 'checkbox',
              checked: props.override,
              onChange: (e: Event) => emit('update:override', (e.target as HTMLInputElement).checked),
              class: 'h-4 w-4 rounded border-surface-300 text-accent-600',
            }),
            t('admin.inboundEditor.advanced.overrideRaw'),
          ]),
        ]),
        h('textarea', {
          value: props.value,
          onInput: (e: Event) => emit('update:value', (e.target as HTMLTextAreaElement).value),
          spellcheck: 'false',
          class: [
            'h-64 w-full resize-none rounded-lg border bg-surface-0 px-3 py-2 font-mono text-2xs leading-relaxed transition-colors dark:bg-surface-900',
            props.override
              ? 'border-accent-300 focus:ring-2 focus:ring-accent-200 dark:border-accent-700'
              : 'border-surface-200 text-surface-500 dark:border-surface-700',
          ],
          disabled: !props.override,
          placeholder: '{ "foo": "bar" }',
        }),
      ])
  },
})

// =========================================================================
// Props / Emits
// =========================================================================
const props = defineProps<{
  open: boolean
  mode: 'create' | 'edit'
  /** Selected node for create; locked in edit mode. */
  nodeID: number | null
  /** Tag of the inbound being edited; '' for create. */
  tag: string
  /** Source inbound to prefill (edit mode). */
  source?: Inbound | null
  /** Enabled nodes for the selector. */
  nodes: Node[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', inbound: Inbound): void
}>()

// =========================================================================
// Local model — every field 3x-ui's add-inbound modal exposes
// =========================================================================

type ProtocolName = 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'wireguard' | 'hysteria'
type TransmissionName = 'tcp' | 'ws' | 'grpc' | 'httpupgrade' | 'h2' | 'xhttp' | 'kcp' | 'quic'
type SecurityName = 'none' | 'tls' | 'reality'

interface Model {
  // Basic
  enable: boolean
  remark: string
  protocol: ProtocolName
  listen: string
  port: number
  totalGB: number              // bytes; UI shows GB
  trafficReset: 'never' | 'daily' | 'weekly' | 'monthly' | 'yearly'
  expiryTime: number           // unix ms

  // Protocol-specific
  decryption: string           // vless
  ssMethod: string             // shadowsocks
  ssNetwork: 'tcp' | 'udp' | 'tcp,udp'
  disableInsecureEncryption: boolean // vmess

  // Stream / Network
  network: TransmissionName
  proxyProtocol: boolean       // tcp/ws
  httpHeader: boolean          // tcp HTTP 伪装
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

  // Security
  security: SecurityName
  tlsServerName: string
  tlsAlpn: string[]            // h2 / http/1.1
  tlsFingerprint: '' | 'chrome' | 'firefox' | 'safari' | 'ios' | 'android' | 'edge' | 'random' | 'randomized'
  tlsAllowInsecure: boolean
  tlsCertificateFile: string   // node-local file path; used by Hysteria + any TLS-bearing protocol
  tlsKeyFile: string           // node-local file path
  realityDest: string
  realityServerNames: string
  realityPublicKey: string
  realityPrivateKey: string
  realityShortIds: string
  realityFingerprint: 'chrome' | 'firefox' | 'safari' | 'ios' | 'android' | 'edge' | 'random' | 'randomized'

  // Sniffing
  sniffEnabled: boolean
  sniffHttp: boolean
  sniffTls: boolean
  sniffQuic: boolean
  sniffFakedns: boolean
  sniffMetadataOnly: boolean
  sniffRouteOnly: boolean

  // Advanced raw JSON (only used when adv* toggles are on)
  advSettingsOverride: boolean
  advSettings: string
  advStreamOverride: boolean
  advStream: string
  advSniffingOverride: boolean
  advSniffing: string
}

function blankModel(): Model {
  return {
    enable: true,
    remark: '',
    protocol: 'vless',
    listen: '',
    port: 44400,
    totalGB: 0,
    trafficReset: 'never',
    expiryTime: 0,

    decryption: 'none',
    ssMethod: 'chacha20-ietf-poly1305',
    ssNetwork: 'tcp,udp',
    disableInsecureEncryption: false,

    network: 'tcp',
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

    security: 'none',
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

const m = ref<Model>(blankModel())
const selectedNodeID = ref<number | null>(null)
const activeTab = ref<'basic' | 'protocol' | 'stream' | 'sniffing' | 'advanced'>('basic')

const busy = ref(false)
const error = ref<string | null>(null)

// Computed so labels react to locale changes.
const tabs = computed(() => [
  { key: 'basic' as const, label: t('admin.inboundEditor.tab.basic') },
  { key: 'protocol' as const, label: t('admin.inboundEditor.tab.protocol') },
  { key: 'stream' as const, label: t('admin.inboundEditor.tab.stream') },
  { key: 'sniffing' as const, label: t('admin.inboundEditor.tab.sniffing') },
  { key: 'advanced' as const, label: t('admin.inboundEditor.tab.advanced') },
])

// Protocol-specific tab visibility:
//   - WireGuard: no streamSettings / no sniffing — hide both
//   - Hysteria 2: streamSettings shape is fixed (network=hysteria,
//     TLS mandatory, ALPN locked) — Stream tab would only confuse
//     since most of its widgets don't apply. Sniffing still useful
//     for routing decisions, but the panel ignores it for UDP, so
//     hide it too.
const visibleTabs = computed(() => {
  const p = m.value.protocol
  if (p === 'wireguard') {
    return tabs.value.filter((tab) => tab.key !== 'stream' && tab.key !== 'sniffing')
  }
  if (p === 'hysteria') {
    return tabs.value.filter((tab) => tab.key !== 'stream' && tab.key !== 'sniffing')
  }
  return tabs.value
})

// If the protocol flips to a transport-free protocol while the
// user is standing on a now-hidden tab, bounce them back to basic.
watch(
  () => m.value.protocol,
  (p) => {
    const onHidden = activeTab.value === 'stream' || activeTab.value === 'sniffing'
    if ((p === 'wireguard' || p === 'hysteria') && onHidden) {
      activeTab.value = 'basic'
    }
  },
)

// =========================================================================
// Lifecycle / prefill
// =========================================================================
watch(
  () => props.open,
  (open) => {
    if (!open) return
    activeTab.value = 'basic'
    error.value = null
    if (props.mode === 'edit' && props.source) {
      hydrateFromInbound(props.source)
      selectedNodeID.value = props.nodeID
    } else {
      m.value = blankModel()
      selectedNodeID.value = props.nodeID ?? props.nodes.find((n) => n.enabled)?.id ?? null
    }
  },
  { immediate: true },
)

function hydrateFromInbound(in_: Inbound) {
  const out = blankModel()
  out.enable = in_.enable
  out.remark = in_.remark
  out.protocol = (in_.protocol as ProtocolName) || 'vless'
  out.listen = in_.listen
  out.port = in_.port
  out.totalGB = in_.total
  out.trafficReset = (in_.trafficReset as Model['trafficReset']) || 'never'
  out.expiryTime = in_.expiryTime

  // settings
  try {
    const s = JSON.parse(in_.settings || '{}')
    if (out.protocol === 'vless') out.decryption = s.decryption ?? 'none'
    if (out.protocol === 'shadowsocks') {
      out.ssMethod = s.method ?? out.ssMethod
      out.ssNetwork = (s.network as Model['ssNetwork']) ?? out.ssNetwork
    }
    if (out.protocol === 'vmess') {
      out.disableInsecureEncryption = !!s.disableInsecureEncryption
    }
    out.advSettings = JSON.stringify(s, null, 2)
  } catch {
    out.advSettings = in_.settings
  }

  // stream
  try {
    const s = JSON.parse(in_.streamSettings || '{}')
    out.network = (s.network as TransmissionName) || 'tcp'
    out.security = (s.security as SecurityName) || 'none'
    if (s.tcpSettings) {
      out.proxyProtocol = !!s.tcpSettings.acceptProxyProtocol
      const t = s.tcpSettings.header
      if (t && t.type === 'http') {
        out.httpHeader = true
        out.httpHeaderPath = t.request?.path?.[0] ?? '/'
        out.httpHeaderHost = t.request?.headers?.Host?.[0] ?? ''
      }
    }
    if (s.wsSettings) {
      out.wsPath = s.wsSettings.path ?? '/'
      out.wsHost = s.wsSettings.host ?? s.wsSettings.headers?.Host ?? ''
    }
    if (s.grpcSettings) {
      out.grpcServiceName = s.grpcSettings.serviceName ?? 'grpc'
      out.grpcMultiMode = !!s.grpcSettings.multiMode
    }
    if (s.httpupgradeSettings) {
      out.httpupgradePath = s.httpupgradeSettings.path ?? '/'
      out.httpupgradeHost = s.httpupgradeSettings.host ?? ''
    }
    if (s.httpSettings || s.h2Settings) {
      const h = s.httpSettings || s.h2Settings
      out.h2Path = h.path ?? '/'
      out.h2Host = h.host?.[0] ?? ''
    }
    if (s.xhttpSettings) {
      out.xhttpPath = s.xhttpSettings.path ?? '/'
      out.xhttpHost = s.xhttpSettings.host ?? ''
      out.xhttpMode = s.xhttpSettings.mode ?? 'auto'
    }
    if (s.kcpSettings) {
      out.kcpMtu = s.kcpSettings.mtu ?? 1350
      out.kcpTti = s.kcpSettings.tti ?? 50
      out.kcpUpCap = s.kcpSettings.uplinkCapacity ?? 5
      out.kcpDownCap = s.kcpSettings.downlinkCapacity ?? 20
      out.kcpCongestion = !!s.kcpSettings.congestion
      out.kcpHeader = s.kcpSettings.header?.type ?? 'none'
      out.kcpSeed = s.kcpSettings.seed ?? ''
    }
    if (s.quicSettings) {
      out.quicSecurity = s.quicSettings.security ?? 'none'
      out.quicKey = s.quicSettings.key ?? ''
      out.quicHeader = s.quicSettings.header?.type ?? 'none'
    }
    if (s.tlsSettings) {
      out.tlsServerName = s.tlsSettings.serverName ?? ''
      out.tlsAlpn = Array.isArray(s.tlsSettings.alpn) ? s.tlsSettings.alpn : ['h2', 'http/1.1']
      out.tlsFingerprint = s.tlsSettings.fingerprint ?? 'chrome'
      out.tlsAllowInsecure = !!s.tlsSettings.allowInsecure
      // Cert paths live in tlsSettings.certificates[0]. The fork
      // accepts an array but the dashboard only edits the first
      // entry; multi-cert SNI multiplexing stays an advanced-tab
      // raw-JSON edit.
      const cert = Array.isArray(s.tlsSettings.certificates) && s.tlsSettings.certificates[0]
      if (cert) {
        out.tlsCertificateFile = cert.certificateFile ?? ''
        out.tlsKeyFile = cert.keyFile ?? ''
      }
    }
    if (s.realitySettings) {
      out.realityDest = s.realitySettings.dest ?? ''
      out.realityServerNames = (s.realitySettings.serverNames ?? []).join(',')
      out.realityPublicKey = s.realitySettings.publicKey ?? ''
      out.realityPrivateKey = s.realitySettings.privateKey ?? ''
      out.realityShortIds = (s.realitySettings.shortIds ?? []).join(',')
      out.realityFingerprint = s.realitySettings.fingerprint ?? 'chrome'
    }
    out.advStream = JSON.stringify(s, null, 2)
  } catch {
    out.advStream = in_.streamSettings
  }

  // sniffing
  try {
    const s = JSON.parse(in_.sniffing || '{}')
    out.sniffEnabled = !!s.enabled
    const dest: string[] = Array.isArray(s.destOverride) ? s.destOverride : []
    out.sniffHttp = dest.includes('http')
    out.sniffTls = dest.includes('tls')
    out.sniffQuic = dest.includes('quic')
    out.sniffFakedns = dest.includes('fakedns')
    out.sniffMetadataOnly = !!s.metadataOnly
    out.sniffRouteOnly = !!s.routeOnly
    out.advSniffing = JSON.stringify(s, null, 2)
  } catch {
    out.advSniffing = in_.sniffing
  }

  m.value = out
}

// =========================================================================
// Compose body from model
// =========================================================================

function buildSettings(): object {
  const mv = m.value
  if (mv.protocol === 'vless') {
    return { clients: [], decryption: mv.decryption || 'none', fallbacks: [] }
  }
  if (mv.protocol === 'vmess') {
    return { clients: [], disableInsecureEncryption: mv.disableInsecureEncryption }
  }
  if (mv.protocol === 'trojan') {
    return { clients: [], fallbacks: [] }
  }
  if (mv.protocol === 'shadowsocks') {
    return { clients: [], method: mv.ssMethod, network: mv.ssNetwork }
  }
  if (mv.protocol === 'wireguard') {
    // Empty shell — node generates the server keypair and writes
    // it back into settings.secretKey on first POST. Peers are
    // managed by the dashboard's RMW provisioning flow, not here.
    return { mtu: 1420, secretKey: '', peers: [], noKernelTun: false }
  }
  if (mv.protocol === 'hysteria') {
    return { clients: [], version: 2 }
  }
  return { clients: [] }
}

function buildStream(): object {
  const mv = m.value

  // Hysteria 2 has a fixed streamSettings shape — network is
  // literally "hysteria", TLS is mandatory, ALPN is locked to
  // ["h3"], and an extra hysteriaSettings block carries the
  // version + udpIdleTimeout knobs. Bypass the generic builder
  // since most of the per-network branches don't apply.
  if (mv.protocol === 'hysteria') {
    return {
      network: 'hysteria',
      security: 'tls',
      tlsSettings: {
        serverName: mv.tlsServerName,
        alpn: ['h3'],
        fingerprint: mv.tlsFingerprint || undefined,
        allowInsecure: mv.tlsAllowInsecure,
        certificates: mv.tlsCertificateFile || mv.tlsKeyFile
          ? [{ certificateFile: mv.tlsCertificateFile, keyFile: mv.tlsKeyFile }]
          : [],
      },
      hysteriaSettings: {
        version: 2,
        udpIdleTimeout: 60,
      },
    }
  }

  const out: Record<string, unknown> = {
    network: mv.network,
    security: mv.security,
  }

  // per-network
  if (mv.network === 'tcp') {
    const tcp: Record<string, unknown> = { acceptProxyProtocol: mv.proxyProtocol }
    if (mv.httpHeader) {
      tcp.header = {
        type: 'http',
        request: { path: [mv.httpHeaderPath || '/'], headers: mv.httpHeaderHost ? { Host: [mv.httpHeaderHost] } : {} },
      }
    } else {
      tcp.header = { type: 'none' }
    }
    out.tcpSettings = tcp
  } else if (mv.network === 'ws') {
    out.wsSettings = {
      acceptProxyProtocol: mv.proxyProtocol,
      path: mv.wsPath || '/',
      headers: mv.wsHost ? { Host: mv.wsHost } : {},
    }
  } else if (mv.network === 'grpc') {
    out.grpcSettings = {
      serviceName: mv.grpcServiceName || 'grpc',
      multiMode: mv.grpcMultiMode,
    }
  } else if (mv.network === 'httpupgrade') {
    out.httpupgradeSettings = { path: mv.httpupgradePath || '/', host: mv.httpupgradeHost }
  } else if (mv.network === 'h2') {
    out.httpSettings = { path: mv.h2Path || '/', host: mv.h2Host ? [mv.h2Host] : [] }
  } else if (mv.network === 'xhttp') {
    out.xhttpSettings = { path: mv.xhttpPath || '/', host: mv.xhttpHost, mode: mv.xhttpMode }
  } else if (mv.network === 'kcp') {
    out.kcpSettings = {
      mtu: mv.kcpMtu,
      tti: mv.kcpTti,
      uplinkCapacity: mv.kcpUpCap,
      downlinkCapacity: mv.kcpDownCap,
      congestion: mv.kcpCongestion,
      header: { type: mv.kcpHeader },
      seed: mv.kcpSeed,
    }
  } else if (mv.network === 'quic') {
    out.quicSettings = { security: mv.quicSecurity, key: mv.quicKey, header: { type: mv.quicHeader } }
  }

  // per-security
  if (mv.security === 'tls') {
    out.tlsSettings = {
      serverName: mv.tlsServerName,
      alpn: mv.tlsAlpn,
      fingerprint: mv.tlsFingerprint || undefined,
      allowInsecure: mv.tlsAllowInsecure,
      certificates: mv.tlsCertificateFile || mv.tlsKeyFile
        ? [{ certificateFile: mv.tlsCertificateFile, keyFile: mv.tlsKeyFile }]
        : [],
    }
  } else if (mv.security === 'reality') {
    out.realitySettings = {
      dest: mv.realityDest,
      serverNames: mv.realityServerNames.split(',').map((s) => s.trim()).filter(Boolean),
      publicKey: mv.realityPublicKey,
      privateKey: mv.realityPrivateKey,
      shortIds: mv.realityShortIds.split(',').map((s) => s.trim()).filter(Boolean),
      fingerprint: mv.realityFingerprint,
    }
  }

  return out
}

function buildSniffing(): object {
  const mv = m.value
  const dest: string[] = []
  if (mv.sniffHttp) dest.push('http')
  if (mv.sniffTls) dest.push('tls')
  if (mv.sniffQuic) dest.push('quic')
  if (mv.sniffFakedns) dest.push('fakedns')
  return {
    enabled: mv.sniffEnabled,
    destOverride: dest,
    metadataOnly: mv.sniffMetadataOnly,
    routeOnly: mv.sniffRouteOnly,
  }
}

function jsonOrFallback(text: string, fallback: object): string {
  try {
    return JSON.stringify(JSON.parse(text))
  } catch {
    return JSON.stringify(fallback)
  }
}

function composeBody(): Partial<Inbound> {
  const mv = m.value
  const settings = mv.advSettingsOverride && mv.advSettings.trim()
    ? jsonOrFallback(mv.advSettings, buildSettings())
    : JSON.stringify(buildSettings())
  const stream = mv.advStreamOverride && mv.advStream.trim()
    ? jsonOrFallback(mv.advStream, buildStream())
    : JSON.stringify(buildStream())
  const sniffing = mv.advSniffingOverride && mv.advSniffing.trim()
    ? jsonOrFallback(mv.advSniffing, buildSniffing())
    : JSON.stringify(buildSniffing())

  return {
    remark: mv.remark,
    enable: mv.enable,
    listen: mv.listen,
    port: mv.port,
    protocol: mv.protocol,
    expiryTime: mv.expiryTime,
    total: mv.totalGB,
    trafficReset: mv.trafficReset,
    settings,
    streamSettings: stream,
    sniffing,
    tag: '',
  }
}

// =========================================================================
// Submit
// =========================================================================
async function submit() {
  error.value = null
  if (!selectedNodeID.value) {
    error.value = t('admin.inboundEditor.errSelectNode')
    return
  }
  if (!m.value.remark.trim()) {
    error.value = t('admin.inboundEditor.errRemark')
    return
  }
  if (m.value.port < 1 || m.value.port > 65535) {
    error.value = t('admin.inboundEditor.errPort')
    return
  }
  busy.value = true
  try {
    const body = composeBody()
    const result =
      props.mode === 'create'
        ? await inboundsApi.create(selectedNodeID.value, body)
        : await inboundsApi.update(selectedNodeID.value, props.tag, body)
    emit('saved', result)
    emit('close')
  } catch (e: any) {
    error.value = formatError(e, t('admin.inboundEditor.operationFailed'))
  } finally {
    busy.value = false
  }
}

// =========================================================================
// Helpers
// =========================================================================

function bytesToGB(n: number): string {
  return (n / 1024 / 1024 / 1024).toFixed(2)
}
function gbToBytes(gb: number): number {
  return Math.round(gb * 1024 * 1024 * 1024)
}
const totalGBDisplay = computed({
  get: () => bytesToGB(m.value.totalGB),
  set: (v: string) => {
    const f = parseFloat(v)
    m.value.totalGB = isFinite(f) ? gbToBytes(f) : 0
  },
})

// Convert unix ms ↔ datetime-local string
const expiryDisplay = computed({
  get: () => {
    if (!m.value.expiryTime) return ''
    const d = new Date(m.value.expiryTime)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  },
  set: (v: string) => {
    m.value.expiryTime = v ? new Date(v).getTime() : 0
  },
})
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-3 backdrop-blur-sm sm:p-4"
  >
    <div class="flex max-h-[calc(100vh-1.5rem)] w-full max-w-5xl flex-col overflow-hidden rounded-2xl bg-surface-0 shadow-elevated dark:bg-surface-900 sm:max-h-[calc(100vh-2rem)]">
      <!-- Title bar -->
      <header class="flex h-12 shrink-0 items-center justify-between border-b border-surface-200 px-5 dark:border-surface-800">
        <h2 class="text-base font-semibold tracking-tight text-ink-900 dark:text-surface-50">{{ mode === 'create' ? $t('admin.inboundEditor.createTitle') : $t('admin.inboundEditor.editTitle', { tag }) }}</h2>
        <button class="flex h-8 w-8 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-surface-700 dark:hover:bg-surface-800" @click="$emit('close')">
          <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12" /></svg>
        </button>
      </header>

      <!-- Tab bar -->
      <nav class="flex shrink-0 border-b border-surface-200 px-5 dark:border-surface-800">
        <button
          v-for="t in visibleTabs"
          :key="t.key"
          class="-mb-px border-b-2 px-3 py-2.5 text-sm font-medium transition-brand transition"
          :class="
            activeTab === t.key
              ? 'border-primary-600 text-primary-700 dark:text-primary-300'
              : 'border-transparent text-surface-500 hover:text-surface-800 dark:hover:text-surface-200'
          "
          @click="activeTab = t.key"
        >
          {{ t.label }}
        </button>
      </nav>

      <!-- Body -->
      <form class="min-h-0 flex-1 overflow-y-auto px-5 py-4" @submit.prevent="submit">
        <!-- ============ Tab: 基础配置 ============ -->
        <div v-if="activeTab === 'basic'" class="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-x-5">
          <Row :label="$t('admin.inboundEditor.basicEnable')">
            <ToggleBtn v-model="m.enable" />
          </Row>
          <Row :label="$t('admin.inboundEditor.basicNode')">
            <select
              v-model="selectedNodeID"
              :disabled="mode === 'edit'"
              class="w-full rounded-lg border border-surface-200 bg-surface-0 px-3 py-2 text-sm dark:border-surface-700 dark:bg-surface-900 disabled:opacity-60"
            >
              <option v-for="n in nodes" :key="n.id" :value="n.id" :disabled="!n.enabled">
                {{ n.name }} {{ n.enabled ? '' : $t('admin.inboundEditor.nodeDisabledSuffix') }}
              </option>
            </select>
          </Row>
          <Row :label="$t('admin.inboundEditor.basicRemark')">
            <input v-model="m.remark" type="text" class="input" :placeholder="$t('admin.inboundEditor.basicRemarkPlaceholder')" />
          </Row>
          <Row :label="$t('admin.inboundEditor.basicProtocol')">
            <select v-model="m.protocol" class="input">
              <option value="vless">vless</option>
              <option value="vmess">vmess</option>
              <option value="trojan">trojan</option>
              <option value="shadowsocks">shadowsocks</option>
              <option value="wireguard">wireguard</option>
              <option value="hysteria">hysteria</option>
            </select>
          </Row>
          <p v-if="m.protocol === 'wireguard'" class="text-xs leading-5 text-surface-500 sm:col-span-2 sm:pl-28">
            {{ $t('admin.inboundEditor.wireguardInfo') }}
          </p>
          <p v-if="m.protocol === 'hysteria'" class="text-xs leading-5 text-surface-500 sm:col-span-2 sm:pl-28">
            {{ $t('admin.inboundEditor.hysteriaInfo') }}
          </p>
          <Row :label="$t('admin.inboundEditor.basicAddress')">
            <input v-model="m.listen" type="text" class="input" :placeholder="$t('admin.inboundEditor.basicAddressPlaceholder')" />
          </Row>
          <Row :label="$t('admin.inboundEditor.basicPort')">
            <input v-model.number="m.port" type="number" min="1" max="65535" class="input" />
          </Row>
          <Row :label="$t('admin.inboundEditor.basicTotalGB')">
            <input v-model="totalGBDisplay" type="number" min="0" step="0.01" class="input" />
          </Row>
          <Row :label="$t('admin.inboundEditor.basicTrafficReset')">
            <select v-model="m.trafficReset" class="input">
              <option value="never">{{ $t('admin.inboundEditor.trafficReset.never') }}</option>
              <option value="daily">{{ $t('admin.inboundEditor.trafficReset.daily') }}</option>
              <option value="weekly">{{ $t('admin.inboundEditor.trafficReset.weekly') }}</option>
              <option value="monthly">{{ $t('admin.inboundEditor.trafficReset.monthly') }}</option>
              <option value="yearly">{{ $t('admin.inboundEditor.trafficReset.yearly') }}</option>
            </select>
          </Row>
          <Row :label="$t('admin.inboundEditor.basicExpiry')">
            <input v-model="expiryDisplay" type="datetime-local" class="input" />
          </Row>
        </div>

        <!-- ============ Tab: 协议 ============ -->
        <div v-else-if="activeTab === 'protocol'" class="space-y-4">
          <Info>
            {{ $t('admin.inboundEditor.protocolInfo') }}
          </Info>

          <template v-if="m.protocol === 'vless'">
            <Row label="Decryption">
              <select v-model="m.decryption" class="input">
                <option value="none">none</option>
              </select>
            </Row>
            <p class="text-xs text-surface-500 pl-32">{{ $t('admin.inboundEditor.protocolVLESSHint') }}</p>
          </template>

          <template v-else-if="m.protocol === 'vmess'">
            <Row :label="$t('admin.inboundEditor.vmessDisableInsecure')">
              <ToggleBtn v-model="m.disableInsecureEncryption" />
            </Row>
          </template>

          <template v-else-if="m.protocol === 'shadowsocks'">
            <Row :label="$t('admin.inboundEditor.ssMethod')">
              <select v-model="m.ssMethod" class="input">
                <option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305</option>
                <option value="aes-256-gcm">aes-256-gcm</option>
                <option value="aes-128-gcm">aes-128-gcm</option>
                <option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option>
                <option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option>
                <option value="2022-blake3-chacha20-poly1305">2022-blake3-chacha20-poly1305</option>
              </select>
            </Row>
            <Row :label="$t('admin.inboundEditor.ssNetwork')">
              <select v-model="m.ssNetwork" class="input">
                <option value="tcp,udp">tcp+udp</option>
                <option value="tcp">tcp only</option>
                <option value="udp">udp only</option>
              </select>
            </Row>
          </template>

          <template v-else-if="m.protocol === 'trojan'">
            <p class="text-xs text-surface-500">{{ $t('admin.inboundEditor.trojanHint') }}</p>
          </template>

          <template v-else-if="m.protocol === 'wireguard'">
            <p class="text-sm text-surface-700 dark:text-surface-300">
              {{ $t('admin.inboundEditor.wireguardNote1') }}
            </p>
            <p class="text-xs text-surface-500">
              {{ $t('admin.inboundEditor.wireguardNote2') }}
            </p>
          </template>

          <template v-else-if="m.protocol === 'hysteria'">
            <Row label="SNI">
              <input v-model="m.tlsServerName" type="text" class="input" :placeholder="$t('admin.inboundEditor.stream.sniPlaceholder').replace('example.com', 'vpn.example.com')" />
            </Row>
            <Row :label="$t('admin.inboundEditor.realityFingerprint')">
              <select v-model="m.tlsFingerprint" class="input">
                <option value="">none</option>
                <option value="chrome">chrome</option>
                <option value="firefox">firefox</option>
                <option value="safari">safari</option>
                <option value="ios">ios</option>
                <option value="android">android</option>
                <option value="randomized">randomized</option>
              </select>
            </Row>
            <Row :label="$t('admin.inboundEditor.stream.allowInsecure')">
              <ToggleBtn v-model="m.tlsAllowInsecure" />
            </Row>
            <Row :label="$t('admin.inboundEditor.stream.certFile')">
              <input
                v-model="m.tlsCertificateFile"
                type="text"
                class="input font-mono text-xs"
                placeholder="/etc/letsencrypt/live/example.com/fullchain.pem"
              />
            </Row>
            <Row :label="$t('admin.inboundEditor.stream.keyFile')">
              <input
                v-model="m.tlsKeyFile"
                type="text"
                class="input font-mono text-xs"
                placeholder="/etc/letsencrypt/live/example.com/privkey.pem"
              />
            </Row>
            <p class="text-xs text-surface-500">
              {{ $t('admin.inboundEditor.hysteriaTLSNote') }}
            </p>
          </template>
        </div>

        <!-- ============ Tab: Stream ============ -->
        <div v-else-if="activeTab === 'stream'" class="space-y-4">
          <Row :label="$t('admin.inboundEditor.stream.transmission')">
            <select v-model="m.network" class="input">
              <option value="tcp">TCP (RAW)</option>
              <option value="ws">WebSocket</option>
              <option value="grpc">gRPC</option>
              <option value="httpupgrade">HTTPUpgrade</option>
              <option value="h2">HTTP/2</option>
              <option value="xhttp">XHTTP</option>
              <option value="kcp">mKCP</option>
              <option value="quic">QUIC</option>
            </select>
          </Row>

          <!-- TCP -->
          <template v-if="m.network === 'tcp'">
            <Row :label="$t('admin.inboundEditor.stream.proxyProtocol')"><ToggleBtn v-model="m.proxyProtocol" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.httpHeader')"><ToggleBtn v-model="m.httpHeader" /></Row>
            <template v-if="m.httpHeader">
              <Row :label="$t('admin.inboundEditor.stream.httpFakeHost')"><input v-model="m.httpHeaderHost" class="input" :placeholder="$t('admin.inboundEditor.stream.sniPlaceholder')" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.httpFakePath')"><input v-model="m.httpHeaderPath" class="input" /></Row>
            </template>
          </template>

          <!-- WS -->
          <template v-if="m.network === 'ws'">
            <Row :label="$t('admin.inboundEditor.stream.path')"><input v-model="m.wsPath" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.host')"><input v-model="m.wsHost" class="input" :placeholder="$t('admin.inboundEditor.stream.hostOptional')" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.proxyProtocol')"><ToggleBtn v-model="m.proxyProtocol" /></Row>
          </template>

          <!-- gRPC -->
          <template v-if="m.network === 'grpc'">
            <Row :label="$t('admin.inboundEditor.stream.serviceName')"><input v-model="m.grpcServiceName" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.multiMode')"><ToggleBtn v-model="m.grpcMultiMode" /></Row>
          </template>

          <!-- httpupgrade -->
          <template v-if="m.network === 'httpupgrade'">
            <Row :label="$t('admin.inboundEditor.stream.path')"><input v-model="m.httpupgradePath" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.host')"><input v-model="m.httpupgradeHost" class="input" /></Row>
          </template>

          <!-- h2 -->
          <template v-if="m.network === 'h2'">
            <Row :label="$t('admin.inboundEditor.stream.path')"><input v-model="m.h2Path" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.host')"><input v-model="m.h2Host" class="input" :placeholder="$t('admin.inboundEditor.stream.hostsCommaSep')" /></Row>
          </template>

          <!-- xhttp -->
          <template v-if="m.network === 'xhttp'">
            <Row :label="$t('admin.inboundEditor.stream.path')"><input v-model="m.xhttpPath" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.host')"><input v-model="m.xhttpHost" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.mode')">
              <select v-model="m.xhttpMode" class="input">
                <option value="auto">auto</option>
                <option value="packet-up">packet-up</option>
                <option value="stream-up">stream-up</option>
                <option value="stream-one">stream-one</option>
              </select>
            </Row>
          </template>

          <!-- kcp -->
          <template v-if="m.network === 'kcp'">
            <Row :label="$t('admin.inboundEditor.stream.kcpMTU')"><input v-model.number="m.kcpMtu" type="number" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.kcpTTI')"><input v-model.number="m.kcpTti" type="number" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.upCap')"><input v-model.number="m.kcpUpCap" type="number" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.downCap')"><input v-model.number="m.kcpDownCap" type="number" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.congestion')"><ToggleBtn v-model="m.kcpCongestion" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.kcpHeader')">
              <select v-model="m.kcpHeader" class="input">
                <option value="none">none</option>
                <option value="srtp">srtp</option>
                <option value="utp">utp</option>
                <option value="wechat-video">wechat-video</option>
                <option value="dtls">dtls</option>
                <option value="wireguard">wireguard</option>
              </select>
            </Row>
            <Row :label="$t('admin.inboundEditor.stream.kcpSeed')"><input v-model="m.kcpSeed" class="input" /></Row>
          </template>

          <!-- quic -->
          <template v-if="m.network === 'quic'">
            <Row :label="$t('admin.inboundEditor.stream.quicSecurity')">
              <select v-model="m.quicSecurity" class="input">
                <option value="none">none</option>
                <option value="aes-128-gcm">aes-128-gcm</option>
                <option value="chacha20-poly1305">chacha20-poly1305</option>
              </select>
            </Row>
            <Row :label="$t('admin.inboundEditor.stream.quicKey')" v-if="m.quicSecurity !== 'none'"><input v-model="m.quicKey" class="input" /></Row>
            <Row :label="$t('admin.inboundEditor.stream.quicHeader')">
              <select v-model="m.quicHeader" class="input">
                <option value="none">none</option>
                <option value="srtp">srtp</option>
                <option value="utp">utp</option>
                <option value="wechat-video">wechat-video</option>
                <option value="dtls">dtls</option>
                <option value="wireguard">wireguard</option>
              </select>
            </Row>
          </template>

          <!-- Security -->
          <div class="my-4 border-t border-surface-200 pt-4 dark:border-surface-800">
            <Row :label="$t('admin.inboundEditor.stream.security')">
              <select v-model="m.security" class="input">
                <option value="none">none</option>
                <option value="tls">tls</option>
                <option value="reality">reality</option>
              </select>
            </Row>

            <template v-if="m.security === 'tls'">
              <Row :label="$t('admin.inboundEditor.stream.serverName')"><input v-model="m.tlsServerName" class="input" :placeholder="$t('admin.inboundEditor.stream.sniPlaceholder')" /></Row>
              <Row label="ALPN">
                <div class="flex gap-3">
                  <label class="flex items-center gap-1 text-sm">
                    <input type="checkbox" :checked="m.tlsAlpn.includes('h2')" @change="(e) => { const arr = m.tlsAlpn.filter(x => x !== 'h2'); if ((e.target as HTMLInputElement).checked) arr.unshift('h2'); m.tlsAlpn = arr }" /> h2
                  </label>
                  <label class="flex items-center gap-1 text-sm">
                    <input type="checkbox" :checked="m.tlsAlpn.includes('http/1.1')" @change="(e) => { const arr = m.tlsAlpn.filter(x => x !== 'http/1.1'); if ((e.target as HTMLInputElement).checked) arr.push('http/1.1'); m.tlsAlpn = arr }" /> http/1.1
                  </label>
                </div>
              </Row>
              <Row :label="$t('admin.inboundEditor.stream.fingerprint')">
                <select v-model="m.tlsFingerprint" class="input">
                  <option value="">{{ $t('admin.inboundEditor.stream.fingerprintNone') }}</option>
                  <option value="chrome">chrome</option>
                  <option value="firefox">firefox</option>
                  <option value="safari">safari</option>
                  <option value="ios">ios</option>
                  <option value="android">android</option>
                  <option value="edge">edge</option>
                  <option value="random">random</option>
                  <option value="randomized">randomized</option>
                </select>
              </Row>
              <Row :label="$t('admin.inboundEditor.stream.allowInsecure')"><ToggleBtn v-model="m.tlsAllowInsecure" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.certFile')">
                <input
                  v-model="m.tlsCertificateFile"
                  type="text"
                  class="input font-mono text-xs"
                  placeholder="/etc/letsencrypt/live/example.com/fullchain.pem"
                />
              </Row>
              <Row :label="$t('admin.inboundEditor.stream.keyFile')">
                <input
                  v-model="m.tlsKeyFile"
                  type="text"
                  class="input font-mono text-xs"
                  placeholder="/etc/letsencrypt/live/example.com/privkey.pem"
                />
              </Row>
              <p class="pl-32 text-xs text-surface-500">
                {{ $t('admin.inboundEditor.stream.certNote') }}
              </p>
            </template>

            <template v-if="m.security === 'reality'">
              <Row :label="$t('admin.inboundEditor.stream.dest')"><input v-model="m.realityDest" class="input" placeholder="www.cloudflare.com:443" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.serverNames')"><input v-model="m.realityServerNames" class="input" :placeholder="$t('admin.inboundEditor.stream.serverNamesPlaceholder')" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.privateKey')"><input v-model="m.realityPrivateKey" class="input" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.publicKey')"><input v-model="m.realityPublicKey" class="input" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.shortIDs')"><input v-model="m.realityShortIds" class="input" :placeholder="$t('admin.inboundEditor.stream.shortIDsPlaceholder')" /></Row>
              <Row :label="$t('admin.inboundEditor.stream.fingerprint')">
                <select v-model="m.realityFingerprint" class="input">
                  <option value="chrome">chrome</option>
                  <option value="firefox">firefox</option>
                  <option value="safari">safari</option>
                  <option value="ios">ios</option>
                  <option value="android">android</option>
                  <option value="edge">edge</option>
                  <option value="random">random</option>
                  <option value="randomized">randomized</option>
                </select>
              </Row>
              <Info>{{ $t('admin.inboundEditor.realityInfo') }}</Info>
            </template>
          </div>
        </div>

        <!-- ============ Tab: Sniffing ============ -->
        <div v-else-if="activeTab === 'sniffing'" class="space-y-4">
          <Row :label="$t('admin.inboundEditor.sniff.enabled')"><ToggleBtn v-model="m.sniffEnabled" /></Row>
          <template v-if="m.sniffEnabled">
            <Row :label="$t('admin.inboundEditor.sniff.destOverride')">
              <div class="flex flex-wrap gap-3">
                <label class="flex items-center gap-1.5 text-sm"><input type="checkbox" v-model="m.sniffHttp" /> http</label>
                <label class="flex items-center gap-1.5 text-sm"><input type="checkbox" v-model="m.sniffTls" /> tls</label>
                <label class="flex items-center gap-1.5 text-sm"><input type="checkbox" v-model="m.sniffQuic" /> quic</label>
                <label class="flex items-center gap-1.5 text-sm"><input type="checkbox" v-model="m.sniffFakedns" /> fakedns</label>
              </div>
            </Row>
            <Row :label="$t('admin.inboundEditor.sniff.metadataOnly')"><ToggleBtn v-model="m.sniffMetadataOnly" /></Row>
            <Row :label="$t('admin.inboundEditor.sniff.routeOnly')"><ToggleBtn v-model="m.sniffRouteOnly" /></Row>
          </template>
        </div>

        <!-- ============ Tab: 高级配置 ============ -->
        <div v-else-if="activeTab === 'advanced'" class="space-y-3">
          <Info>{{ $t('admin.inboundEditor.advanced.info') }}</Info>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <AdvancedJSON
              :label="$t('admin.inboundEditor.advanced.labelSettings')"
              :description="$t('admin.inboundEditor.advanced.labelSettingsDesc')"
              v-model:override="m.advSettingsOverride"
              v-model:value="m.advSettings"
            />
            <AdvancedJSON
              :label="$t('admin.inboundEditor.advanced.labelStream')"
              :description="$t('admin.inboundEditor.advanced.labelStreamDesc')"
              v-model:override="m.advStreamOverride"
              v-model:value="m.advStream"
            />
            <AdvancedJSON
              :label="$t('admin.inboundEditor.advanced.labelSniffing')"
              :description="$t('admin.inboundEditor.advanced.labelSniffingDesc')"
              v-model:override="m.advSniffingOverride"
              v-model:value="m.advSniffing"
            />
          </div>
        </div>
      </form>

      <!-- Footer -->
      <footer class="flex min-h-14 shrink-0 items-center justify-between gap-3 border-t border-surface-200 px-5 py-3 dark:border-surface-800">
        <p v-if="error" class="min-w-0 flex-1 truncate text-sm text-red-600">{{ error }}</p>
        <p v-else class="min-w-0 flex-1 truncate text-xs text-surface-500">{{ mode === 'create' ? $t('admin.inboundEditor.footerHintCreate') : $t('admin.inboundEditor.footerHintEdit') }}</p>
        <div class="flex gap-2">
          <button type="button" class="inline-flex h-9 items-center rounded-lg border border-surface-200 px-4 text-sm hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800" @click="$emit('close')">{{ $t('admin.inboundEditor.close') }}</button>
          <button type="button" :disabled="busy" class="inline-flex h-9 items-center rounded-lg bg-accent-600 px-5 text-sm font-medium text-white hover:bg-accent-700 disabled:opacity-60" @click="submit">
            {{ busy ? $t('admin.inboundEditor.busy') : (mode === 'create' ? $t('admin.inboundEditor.submitCreate') : $t('admin.inboundEditor.submitSave')) }}
          </button>
        </div>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.input {
  @apply h-9 w-full rounded-lg border border-surface-200 bg-surface-0 px-3 text-sm transition-brand transition focus:border-accent-500 focus:outline-none focus:ring-2 focus:ring-accent-200 dark:border-surface-700 dark:bg-surface-900;
}
</style>
