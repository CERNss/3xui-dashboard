import { Form, Input, InputNumber, Select, Space, Switch } from 'antd'

export function StreamSettingsForm() {
  const network = Form.useWatch('network')
  const security = Form.useWatch('security')
  const httpHeader = Form.useWatch('httpHeader')
  const quicSecurity = Form.useWatch('quicSecurity')

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Form.Item name="network" label="Transmission">
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
          <Form.Item name="proxyProtocol" label="Proxy protocol" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="httpHeader" label="HTTP fake header" valuePropName="checked">
            <Switch />
          </Form.Item>
          {httpHeader ? (
            <>
              <Form.Item name="httpHeaderHost" label="Fake host">
                <Input placeholder="example.com" />
              </Form.Item>
              <Form.Item name="httpHeaderPath" label="Fake path">
                <Input />
              </Form.Item>
            </>
          ) : null}
        </Space>
      ) : null}

      {network === 'ws' ? (
        <Space align="start" wrap>
          <Form.Item name="wsPath" label="Path">
            <Input />
          </Form.Item>
          <Form.Item name="wsHost" label="Host">
            <Input placeholder="optional host" />
          </Form.Item>
          <Form.Item name="proxyProtocol" label="Proxy protocol" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'grpc' ? (
        <Space align="start" wrap>
          <Form.Item name="grpcServiceName" label="Service name">
            <Input />
          </Form.Item>
          <Form.Item name="grpcMultiMode" label="Multi mode" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'httpupgrade' ? (
        <Space align="start" wrap>
          <Form.Item name="httpupgradePath" label="Path">
            <Input />
          </Form.Item>
          <Form.Item name="httpupgradeHost" label="Host">
            <Input />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'h2' ? (
        <Space align="start" wrap>
          <Form.Item name="h2Path" label="Path">
            <Input />
          </Form.Item>
          <Form.Item name="h2Host" label="Host">
            <Input placeholder="comma separated hosts" />
          </Form.Item>
        </Space>
      ) : null}

      {network === 'xhttp' ? (
        <Space align="start" wrap>
          <Form.Item name="xhttpPath" label="Path">
            <Input />
          </Form.Item>
          <Form.Item name="xhttpHost" label="Host">
            <Input />
          </Form.Item>
          <Form.Item name="xhttpMode" label="Mode">
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
          <Form.Item name="kcpUpCap" label="Up cap">
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpDownCap" label="Down cap">
            <InputNumber />
          </Form.Item>
          <Form.Item name="kcpCongestion" label="Congestion" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="kcpHeader" label="Header">
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
          <Form.Item name="quicSecurity" label="QUIC security">
            <Select
              style={{ width: 200 }}
              options={['none', 'aes-128-gcm', 'chacha20-poly1305'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          {quicSecurity !== 'none' ? (
            <Form.Item name="quicKey" label="QUIC key">
              <Input />
            </Form.Item>
          ) : null}
          <Form.Item name="quicHeader" label="QUIC header">
            <Select
              style={{ width: 180 }}
              options={['none', 'srtp', 'utp', 'wechat-video', 'dtls', 'wireguard'].map((value) => ({ label: value, value }))}
            />
          </Form.Item>
        </Space>
      ) : null}

      <Form.Item name="security" label="Security">
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
          <Form.Item name="tlsServerName" label="Server name">
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
                label: value || 'none',
                value,
              }))}
            />
          </Form.Item>
          <Form.Item name="tlsAllowInsecure" label="Allow insecure" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="tlsCertificateFile" label="Certificate file">
            <Input placeholder="/etc/letsencrypt/live/example.com/fullchain.pem" />
          </Form.Item>
          <Form.Item name="tlsKeyFile" label="Key file">
            <Input placeholder="/etc/letsencrypt/live/example.com/privkey.pem" />
          </Form.Item>
        </Space>
      ) : null}

      {security === 'reality' ? (
        <Space align="start" wrap>
          <Form.Item name="realityDest" label="Dest">
            <Input placeholder="www.cloudflare.com:443" />
          </Form.Item>
          <Form.Item name="realityServerNames" label="Server names">
            <Input placeholder="comma separated names" />
          </Form.Item>
          <Form.Item name="realityPrivateKey" label="Private key">
            <Input />
          </Form.Item>
          <Form.Item name="realityPublicKey" label="Public key">
            <Input />
          </Form.Item>
          <Form.Item name="realityShortIds" label="Short IDs">
            <Input placeholder="comma separated short ids" />
          </Form.Item>
          <Form.Item name="realityFingerprint" label="Fingerprint">
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
