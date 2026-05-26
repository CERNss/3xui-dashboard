import { Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

export function StreamSettingsForm() {
  const { t } = useTranslation()
  const network = Form.useWatch('network')
  const security = Form.useWatch('security')
  const httpHeader = Form.useWatch('httpHeader')
  const quicSecurity = Form.useWatch('quicSecurity')

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
        <Space align="start" wrap>
          <Form.Item name="tlsServerName" label={t('admin.inboundEditor.stream.serverName')}>
            <Input placeholder="example.com" />
          </Form.Item>
          <Form.Item name="tlsAlpn" label="ALPN">
            <Select
              mode="multiple"
              style={{ minWidth: 220 }}
              options={['h2', 'http/1.1'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item name="tlsFingerprint" label="Fingerprint">
            <Select
              style={{ width: 160 }}
              options={['', 'chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized'].map((value) => ({
                label: value || t('admin.inboundEditor.stream.fingerprintNone'),
                value,
              }))}
            />
          </Form.Item>
          <Form.Item name="tlsAllowInsecure" label={t('admin.inboundEditor.stream.allowInsecure')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="tlsCertificateFile" label={t('admin.inboundEditor.stream.certFile')}>
            <Input placeholder="/etc/letsencrypt/live/example.com/fullchain.pem" />
          </Form.Item>
          <Form.Item name="tlsKeyFile" label={t('admin.inboundEditor.stream.keyFile')}>
            <Input placeholder="/etc/letsencrypt/live/example.com/privkey.pem" />
          </Form.Item>
        </Space>
      ) : null}

      {security === 'reality' ? (
        <Space align="start" wrap>
          <Form.Item name="realityDest" label="Dest">
            <Input placeholder="www.cloudflare.com:443" />
          </Form.Item>
          <Form.Item name="realityServerNames" label={t('admin.inboundEditor.stream.serverNames')}>
            <Input placeholder={t('admin.inboundEditor.stream.serverNamesPlaceholder')} />
          </Form.Item>
          <Form.Item name="realityPrivateKey" label={t('admin.inboundEditor.stream.privateKey')}>
            <Input />
          </Form.Item>
          <Form.Item name="realityPublicKey" label={t('admin.inboundEditor.stream.publicKey')}>
            <Input />
          </Form.Item>
          <Form.Item name="realityShortIds" label={t('admin.inboundEditor.stream.shortIDs')}>
            <Input placeholder={t('admin.inboundEditor.stream.shortIDsPlaceholder')} />
          </Form.Item>
          <Form.Item name="realityFingerprint" label={t('admin.inboundEditor.stream.fingerprint')}>
            <Select
              style={{ width: 160 }}
              options={['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
        </Space>
      ) : null}
    </Space>
  )
}
