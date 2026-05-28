import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

export function StreamSettingsForm() {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const network = Form.useWatch('network')
  const security = Form.useWatch('security')
  const httpHeader = Form.useWatch('httpHeader')
  const quicSecurity = Form.useWatch('quicSecurity')
  const generateKeypair = Form.useWatch('realityGenerateKeypair', form)
  const generateMldsa65 = Form.useWatch('realityGenerateMldsa65', form)

  const clearRealityKeypair = () => {
    form.setFieldsValue({
      realityPrivateKey: '',
      realityPublicKey: '',
      realityGenerateKeypair: false,
    })
  }

  const clearMldsa65 = () => {
    form.setFieldsValue({
      realityMldsa65Seed: '',
      realityMldsa65Verify: '',
      realityGenerateMldsa65: false,
    })
  }

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
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Space align="start" wrap>
            <Form.Item name="realityShow" label="Show" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="realityXver" label="Xver">
              <InputNumber min={0} max={2} />
            </Form.Item>
            <Form.Item name="realityFingerprint" label="uTLS">
              <Select
                style={{ width: 160 }}
                options={['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized'].map((value) => ({ label: value, value }))}
              />
            </Form.Item>
          </Space>
          <Space align="start" wrap>
            <Form.Item
              name="realityDest"
              label={
                <Space size={4}>
                  <span>Target</span>
                  <Form.Item name="realityRandomizeTarget" valuePropName="checked" noStyle>
                    <Switch size="small" checkedChildren="↻" unCheckedChildren="↻" />
                  </Form.Item>
                </Space>
              }
              tooltip={t('admin.inboundEditor.stream.realityRandomizeHint')}
            >
              <Input style={{ width: 280 }} placeholder="www.amazon.com:443" />
            </Form.Item>
            <Form.Item
              name="realityServerNames"
              label={
                <Space size={4}>
                  <span>SNI</span>
                  <Form.Item name="realityRandomizeSNI" valuePropName="checked" noStyle>
                    <Switch size="small" checkedChildren="↻" unCheckedChildren="↻" />
                  </Form.Item>
                </Space>
              }
            >
              <Input style={{ width: 280 }} placeholder="www.amazon.com" />
            </Form.Item>
            <Form.Item name="realityMaxTimeDiff" label="Max Time Diff (ms)">
              <InputNumber min={0} />
            </Form.Item>
          </Space>
          <Space align="start" wrap>
            <Form.Item name="realityMinClientVer" label="Min Client Ver">
              <Input placeholder="25.9.11" style={{ width: 160 }} />
            </Form.Item>
            <Form.Item name="realityMaxClientVer" label="Max Client Ver">
              <Input placeholder="25.9.11" style={{ width: 160 }} />
            </Form.Item>
            <Form.Item
              name="realityShortIds"
              label={
                <Space size={4}>
                  <span>Short IDs</span>
                  <Form.Item name="realityRandomizeShortIds" valuePropName="checked" noStyle>
                    <Switch size="small" checkedChildren="↻" unCheckedChildren="↻" />
                  </Form.Item>
                </Space>
              }
            >
              <Input.TextArea rows={2} style={{ width: 420 }} placeholder={t('admin.inboundEditor.stream.shortIDsPlaceholder')} />
            </Form.Item>
            <Form.Item name="realitySpiderX" label="SpiderX">
              <Input style={{ width: 160 }} placeholder="/" />
            </Form.Item>
          </Space>
          <Space align="start" wrap>
            <Form.Item name="realityPublicKey" label={t('admin.inboundEditor.stream.publicKey')}>
              <Input style={{ width: 360 }} disabled={generateKeypair} />
            </Form.Item>
            <Form.Item name="realityPrivateKey" label={t('admin.inboundEditor.stream.privateKey')}>
              <Input style={{ width: 360 }} disabled={generateKeypair} />
            </Form.Item>
          </Space>
          <Space align="center">
            <Form.Item name="realityGenerateKeypair" valuePropName="checked" label={t('admin.inboundEditor.stream.getNewCert')}>
              <Switch />
            </Form.Item>
            <Button onClick={clearRealityKeypair}>{t('admin.inboundEditor.stream.clear')}</Button>
          </Space>
          <Space align="start" wrap>
            <Form.Item name="realityMldsa65Seed" label="mldsa65 Seed">
              <Input.TextArea rows={2} style={{ width: 420 }} disabled={generateMldsa65} />
            </Form.Item>
            <Form.Item name="realityMldsa65Verify" label="mldsa65 Verify">
              <Input.TextArea rows={2} style={{ width: 420 }} disabled={generateMldsa65} />
            </Form.Item>
          </Space>
          <Space align="center">
            <Form.Item name="realityGenerateMldsa65" valuePropName="checked" label={t('admin.inboundEditor.stream.getNewSeed')}>
              <Switch />
            </Form.Item>
            <Button onClick={clearMldsa65}>{t('admin.inboundEditor.stream.clear')}</Button>
          </Space>
        </Space>
      ) : null}
    </Space>
  )
}
