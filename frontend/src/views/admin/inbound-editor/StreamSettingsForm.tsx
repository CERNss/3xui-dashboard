import { SettingOutlined } from '@ant-design/icons'
import { Button, Form, Input, InputNumber, Select, Space, Switch, Typography } from 'antd'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RealityConfigModal } from './RealityConfigModal'
import { TLSConfigModal } from './TLSConfigModal'

export function StreamSettingsForm() {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const network = Form.useWatch('network')
  const security = Form.useWatch('security')
  const httpHeader = Form.useWatch('httpHeader')
  const quicSecurity = Form.useWatch('quicSecurity')

  const [tlsOpen, setTlsOpen] = useState(false)
  const [realityOpen, setRealityOpen] = useState(false)

  const tlsServerName = Form.useWatch('tlsServerName', form)
  const tlsAlpn = Form.useWatch('tlsAlpn', form)
  const tlsAllowInsecure = Form.useWatch('tlsAllowInsecure', form)
  const tlsCert = Form.useWatch('tlsCertificateFile', form)

  const realityDest = Form.useWatch('realityDest', form)
  const realityGenerateKeypair = Form.useWatch('realityGenerateKeypair', form)
  const realityGenerateMldsa65 = Form.useWatch('realityGenerateMldsa65', form)
  const realityHasKey = Form.useWatch('realityPrivateKey', form)

  const tlsSummary = (() => {
    const bits: string[] = []
    if (tlsServerName) bits.push(`SNI=${tlsServerName}`)
    if (Array.isArray(tlsAlpn) && tlsAlpn.length) bits.push(`ALPN=${tlsAlpn.join(',')}`)
    if (tlsAllowInsecure) bits.push(t('admin.inboundEditor.stream.allowInsecure'))
    if (tlsCert) bits.push(t('admin.inboundEditor.stream.tlsCertSet'))
    return bits.length ? bits.join(' · ') : t('admin.inboundEditor.stream.notConfigured')
  })()

  const realitySummary = (() => {
    const bits: string[] = []
    if (realityDest) bits.push(`target=${realityDest}`)
    if (realityGenerateKeypair) bits.push(t('admin.inboundEditor.stream.willGenerateKeypair'))
    else if (realityHasKey) bits.push(t('admin.inboundEditor.stream.keypairSet'))
    if (realityGenerateMldsa65) bits.push(t('admin.inboundEditor.stream.willGenerateMldsa65'))
    return bits.length ? bits.join(' · ') : t('admin.inboundEditor.stream.notConfigured')
  })()

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Form.Item name="network" label={t('admin.inboundEditor.stream.transmission')}>
        <Select
          options={[
            { label: 'TCP (RAW)', value: 'tcp' },
            { label: 'WebSocket', value: 'ws' },
            { label: 'gRPC', value: 'grpc' },
            { label: 'HTTPUpgrade', value: 'httpupgrade' },
            { label: 'HTTP/2', value: 'h2' },
            { label: 'XHTTP', value: 'xhttp' },
            { label: 'mKCP', value: 'kcp' },
            { label: 'QUIC', value: 'quic' },
          ]}
        />
      </Form.Item>

      {network === 'tcp' ? (
        <Space align="start" wrap>
          <Form.Item name="proxyProtocol" label={t('admin.inboundEditor.stream.proxyProtocol')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="httpHeader" label={t('admin.inboundEditor.stream.httpHeader')} valuePropName="checked">
            <Switch />
          </Form.Item>
          {httpHeader ? (
            <>
              <Form.Item name="httpHeaderHost" label={t('admin.inboundEditor.stream.httpFakeHost')}>
                <Input placeholder="example.com" />
              </Form.Item>
              <Form.Item name="httpHeaderPath" label={t('admin.inboundEditor.stream.httpFakePath')}>
                <Input />
              </Form.Item>
            </>
          ) : null}
        </Space>
      ) : null}

      {network === 'ws' ? (
        <Space align="start" wrap>
          <Form.Item name="wsPath" label={t('admin.inboundEditor.stream.path')}>
            <Input />
          </Form.Item>
          <Form.Item name="wsHost" label={t('admin.inboundEditor.stream.host')}>
            <Input placeholder={t('admin.inboundEditor.stream.hostOptional')} />
          </Form.Item>
          <Form.Item name="proxyProtocol" label={t('admin.inboundEditor.stream.proxyProtocol')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'grpc' ? (
        <Space align="start" wrap>
          <Form.Item name="grpcServiceName" label={t('admin.inboundEditor.stream.serviceName')}>
            <Input />
          </Form.Item>
          <Form.Item name="grpcMultiMode" label={t('admin.inboundEditor.stream.multiMode')} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'httpupgrade' ? (
        <Space align="start" wrap>
          <Form.Item name="httpupgradePath" label={t('admin.inboundEditor.stream.path')}>
            <Input />
          </Form.Item>
          <Form.Item name="httpupgradeHost" label={t('admin.inboundEditor.stream.host')}>
            <Input />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'h2' ? (
        <Space align="start" wrap>
          <Form.Item name="h2Path" label={t('admin.inboundEditor.stream.path')}>
            <Input />
          </Form.Item>
          <Form.Item name="h2Host" label={t('admin.inboundEditor.stream.host')}>
            <Input placeholder={t('admin.inboundEditor.stream.hostsCommaSep')} />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'xhttp' ? (
        <Space align="start" wrap>
          <Form.Item name="xhttpPath" label={t('admin.inboundEditor.stream.path')}>
            <Input />
          </Form.Item>
          <Form.Item name="xhttpHost" label={t('admin.inboundEditor.stream.host')}>
            <Input />
          </Form.Item>
          <Form.Item name="xhttpMode" label={t('admin.inboundEditor.stream.mode')}>
            <Select
              style={{ width: 180 }}
              options={['auto', 'packet-up', 'stream-up', 'stream-one'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'kcp' ? (
        <Space align="start" wrap>
          <Form.Item name="kcpMtu" label="MTU">
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpTti" label="TTI">
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpUpCap" label={t('admin.inboundEditor.stream.upCap')}>
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpDownCap" label={t('admin.inboundEditor.stream.downCap')}>
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpCongestion" label={t('admin.inboundEditor.stream.congestion')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="kcpHeader" label={t('admin.inboundEditor.stream.kcpHeader')}>
            <Select
              style={{ width: 180 }}
              options={['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item name="kcpSeed" label="Seed">
            <Input />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'quic' ? (
        <Space align="start" wrap>
          <Form.Item name="quicSecurity" label={t('admin.inboundEditor.stream.quicSecurity')}>
            <Select
              style={{ width: 200 }}
              options={['none', 'aes-128-gcm', 'chacha20-poly1305'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          {quicSecurity !== 'none' ? (
            <Form.Item name="quicKey" label={t('admin.inboundEditor.stream.quicKey')}>
              <Input />
            </Form.Item>
          ) : null}
          <Form.Item name="quicHeader" label={t('admin.inboundEditor.stream.quicHeader')}>
            <Select
              style={{ width: 180 }}
              options={['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
        </Space>
      ) : null}

      <Form.Item name="security" label={t('admin.inboundEditor.stream.security')}>
        <Select
          options={[
            { label: 'none', value: 'none' },
            { label: 'tls', value: 'tls' },
            { label: 'reality', value: 'reality' },
          ]}
        />
      </Form.Item>

      {security === 'tls' ? (
        <Space direction="vertical" size={4}>
          <Space size={12} align="center">
            <Button icon={<SettingOutlined />} onClick={() => setTlsOpen(true)}>
              {t('admin.inboundEditor.stream.configureTLS')}
            </Button>
          </Space>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{tlsSummary}</Typography.Text>
        </Space>
      ) : null}

      {security === 'reality' ? (
        <Space direction="vertical" size={4}>
          <Space size={12} align="center">
            <Button icon={<SettingOutlined />} onClick={() => setRealityOpen(true)}>
              {t('admin.inboundEditor.stream.configureReality')}
            </Button>
          </Space>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{realitySummary}</Typography.Text>
        </Space>
      ) : null}

      <TLSConfigModal open={tlsOpen} onClose={() => setTlsOpen(false)} />
      <RealityConfigModal open={realityOpen} onClose={() => setRealityOpen(false)} />
    </Space>
  )
}
