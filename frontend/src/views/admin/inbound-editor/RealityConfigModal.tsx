import { Button, Divider, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

interface RealityConfigModalProps {
  open: boolean
  onClose: () => void
}

const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized']

export function RealityConfigModal({ open, onClose }: RealityConfigModalProps) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
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
    <Modal
      title={t('admin.inboundEditor.stream.configureReality')}
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>{t('admin.inboundEditor.stream.done')}</Button>}
      width={760}
      destroyOnClose={false}
      maskClosable
    >
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="realityShow" label="Show" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="realityXver" label="Xver">
            <InputNumber min={0} max={2} style={{ width: 80 }} />
          </Form.Item>
          <Form.Item name="realityFingerprint" label="uTLS">
            <Select
              style={{ width: 140 }}
              options={FINGERPRINTS.map((value) => ({ label: value, value }))}
            />
          </Form.Item>
          <Form.Item name="realityMaxTimeDiff" label="Max Time Diff (ms)">
            <InputNumber min={0} style={{ width: 120 }} />
          </Form.Item>
        </Space>

        <Space align="start" wrap size={[12, 0]}>
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
            <Input style={{ width: 260 }} placeholder="www.amazon.com:443" />
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
            <Input style={{ width: 260 }} placeholder="www.amazon.com" />
          </Form.Item>
          <Form.Item name="realitySpiderX" label="SpiderX">
            <Input style={{ width: 120 }} placeholder="/" />
          </Form.Item>
        </Space>

        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="realityMinClientVer" label="Min Client Ver">
            <Input placeholder="25.9.11" style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="realityMaxClientVer" label="Max Client Ver">
            <Input placeholder="25.9.11" style={{ width: 140 }} />
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
            style={{ flex: 1, minWidth: 320 }}
          >
            <Input.TextArea rows={2} placeholder={t('admin.inboundEditor.stream.shortIDsPlaceholder')} />
          </Form.Item>
        </Space>

        <Divider orientation="left" plain style={{ margin: '4px 0' }}>X25519 keypair</Divider>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="realityPublicKey" label={t('admin.inboundEditor.stream.publicKey')}>
            <Input style={{ width: 320 }} disabled={generateKeypair} />
          </Form.Item>
          <Form.Item name="realityPrivateKey" label={t('admin.inboundEditor.stream.privateKey')}>
            <Input style={{ width: 320 }} disabled={generateKeypair} />
          </Form.Item>
        </Space>
        <Space size={12} align="center">
          <Form.Item name="realityGenerateKeypair" valuePropName="checked" noStyle>
            <Switch />
          </Form.Item>
          <span style={{ color: '#888' }}>{t('admin.inboundEditor.stream.getNewCert')}</span>
          <Button size="small" onClick={clearRealityKeypair}>{t('admin.inboundEditor.stream.clear')}</Button>
        </Space>

        <Divider orientation="left" plain style={{ margin: '4px 0' }}>ML-DSA-65 seed</Divider>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="realityMldsa65Seed" label="Seed">
            <Input.TextArea rows={2} style={{ width: 320 }} disabled={generateMldsa65} />
          </Form.Item>
          <Form.Item name="realityMldsa65Verify" label="Verify">
            <Input.TextArea rows={2} style={{ width: 320 }} disabled={generateMldsa65} />
          </Form.Item>
        </Space>
        <Space size={12} align="center">
          <Form.Item name="realityGenerateMldsa65" valuePropName="checked" noStyle>
            <Switch />
          </Form.Item>
          <span style={{ color: '#888' }}>{t('admin.inboundEditor.stream.getNewSeed')}</span>
          <Button size="small" onClick={clearMldsa65}>{t('admin.inboundEditor.stream.clear')}</Button>
        </Space>
      </Space>
    </Modal>
  )
}
