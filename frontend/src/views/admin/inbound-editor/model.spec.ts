import { describe, expect, it } from 'vitest'
import type { Inbound } from '@/api/admin/inbounds'
import { blankInboundValues, inboundToValues, valuesToInboundBody, valuesToTemplateBody } from './model'

function realitySettingsFrom(streamSettings?: string) {
  return JSON.parse(streamSettings ?? '{}').realitySettings
}

describe('inbound editor model', () => {
  it('defaults sniffing off while preserving 3x-ui destination presets', () => {
    const body = valuesToInboundBody(blankInboundValues(1))

    expect(JSON.parse(body.sniffing ?? '{}')).toEqual({
      enabled: false,
      destOverride: ['http', 'tls', 'quic', 'fakedns'],
      metadataOnly: false,
      routeOnly: false,
    })
  })

  it('serializes Reality target for inbound payloads', () => {
    const values = {
      ...blankInboundValues(1),
      security: 'reality' as const,
      realityDest: ' www.cloudflare.com:443 ',
      realityServerNames: 'www.cloudflare.com,example.com',
      realityPublicKey: 'PUB_KEY_HERE',
      realityPrivateKey: 'PRIV_KEY_HERE',
      realityMldsa65Seed: 'SEED_HERE',
      realityMldsa65Verify: 'VERIFY_HERE',
    }

    const body = valuesToInboundBody(values)
    const realitySettings = realitySettingsFrom(body.streamSettings)

    expect(realitySettings.target).toBe('www.cloudflare.com:443')
    expect(realitySettings.dest).toBe('www.cloudflare.com:443')
    expect(realitySettings.privateKey).toBe('PRIV_KEY_HERE')
    expect(realitySettings.mldsa65Seed).toBe('SEED_HERE')
    expect(realitySettings.serverNames).toEqual(['www.cloudflare.com', 'example.com'])
    expect(realitySettings.settings).toMatchObject({
      publicKey: 'PUB_KEY_HERE',
      serverName: 'www.cloudflare.com',
      mldsa65Verify: 'VERIFY_HERE',
    })
    expect(realitySettings.publicKey).toBeUndefined()
    expect(realitySettings.mldsa65Verify).toBeUndefined()
  })

  it('serializes Reality target for template payloads', () => {
    const values = {
      ...blankInboundValues(null),
      security: 'reality' as const,
      realityDest: 'www.cloudflare.com:443',
      realityPublicKey: 'TEMPLATE_PUB_KEY',
      realityPrivateKey: 'TEMPLATE_PRIV_KEY',
    }

    const body = valuesToTemplateBody(values)
    const realitySettings = realitySettingsFrom(body.streamSettings)

    expect(realitySettings.target).toBe('www.cloudflare.com:443')
    expect(realitySettings.dest).toBe('www.cloudflare.com:443')
    expect(realitySettings.privateKey).toBe('TEMPLATE_PRIV_KEY')
    expect(realitySettings.settings.publicKey).toBe('TEMPLATE_PUB_KEY')
  })

  it('hydrates Reality target when dest is blank', () => {
    const values = inboundToValues(
      {
        id: 1,
        up: 0,
        down: 0,
        total: 0,
        allTime: 0,
        remark: 'Reality inbound',
        enable: true,
        expiryTime: 0,
        trafficReset: 'never',
        clientStats: [],
        listen: '',
        port: 443,
        protocol: 'vless',
        settings: JSON.stringify({ clients: [], decryption: 'none', fallbacks: [] }),
        streamSettings: JSON.stringify({
          network: 'tcp',
          security: 'reality',
          realitySettings: {
            target: 'target.example.com:443',
            dest: '',
            privateKey: 'EXISTING_PRIV_KEY',
            settings: {
              publicKey: 'EXISTING_PUB_KEY',
            },
          },
        }),
        tag: 'reality-443',
        sniffing: JSON.stringify({ enabled: true }),
      } satisfies Inbound,
      1,
    )

    expect(values.realityDest).toBe('target.example.com:443')
    expect(values.realityPublicKey).toBe('EXISTING_PUB_KEY')
    expect(values.realityPrivateKey).toBe('EXISTING_PRIV_KEY')
  })

  it('hydrates legacy flat Reality client fields', () => {
    const values = inboundToValues(
      {
        id: 1,
        up: 0,
        down: 0,
        total: 0,
        allTime: 0,
        remark: 'Reality inbound',
        enable: true,
        expiryTime: 0,
        trafficReset: 'never',
        clientStats: [],
        listen: '',
        port: 443,
        protocol: 'vless',
        settings: JSON.stringify({ clients: [], decryption: 'none', fallbacks: [] }),
        streamSettings: JSON.stringify({
          network: 'tcp',
          security: 'reality',
          realitySettings: {
            target: 'target.example.com:443',
            serverNames: ['target.example.com'],
            publicKey: 'LEGACY_PUB_KEY',
            privateKey: 'LEGACY_PRIV_KEY',
            fingerprint: 'firefox',
            spiderX: '/legacy',
            mldsa65Seed: 'LEGACY_SEED',
            mldsa65Verify: 'LEGACY_VERIFY',
          },
        }),
        tag: 'reality-443',
        sniffing: JSON.stringify({ enabled: false }),
      } satisfies Inbound,
      1,
    )

    expect(values.realityPublicKey).toBe('LEGACY_PUB_KEY')
    expect(values.realityPrivateKey).toBe('LEGACY_PRIV_KEY')
    expect(values.realityFingerprint).toBe('firefox')
    expect(values.realitySpiderX).toBe('/legacy')
    expect(values.realityMldsa65Seed).toBe('LEGACY_SEED')
    expect(values.realityMldsa65Verify).toBe('LEGACY_VERIFY')
  })
})
