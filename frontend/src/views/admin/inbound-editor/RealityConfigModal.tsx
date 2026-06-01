import { ReloadOutlined } from '@ant-design/icons'
import { Button, Divider, Form, Input, InputNumber, Modal, Select, Space, Switch, message } from 'antd'
import { useTranslation } from 'react-i18next'
import { useGenerateRealityMldsa65, useGenerateRealityX25519 } from '@/hooks/queries/admin/nodes'
import { randomRealityShortIds, randomRealityTarget } from './random'

interface RealityConfigModalProps {
  open: boolean
  onClose: () => void
  generationEnabled?: boolean
}

const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random', 'randomized']

export function RealityConfigModal({ open, onClose, generationEnabled = true }: RealityConfigModalProps) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const [messageApi, contextHolder] = message.useMessage()
  const nodeID = Form.useWatch('node_id', form)
  const generateX25519 = useGenerateRealityX25519()
  const generateMldsa65 = useGenerateRealityMldsa65()
  const canGenerateOnNode = generationEnabled && typeof nodeID === 'number'

  const generateRealityKeypair = async () => {
    if (typeof nodeID !== 'number') {
      messageApi.warning(t('admin.inboundEditor.stream.selectNodeFirst'))
      return
    }
    const cert = await generateX25519.mutateAsync(nodeID)
    form.setFieldsValue({
      realityPrivateKey: cert.privateKey,
      realityPublicKey: cert.publicKey,
      realityGenerateKeypair: false,
    })
    messageApi.success(t('admin.inboundEditor.stream.generatedKeypair'))
  }

  const generateMldsa65Seed = async () => {
    if (typeof nodeID !== 'number') {
      messageApi.warning(t('admin.inboundEditor.stream.selectNodeFirst'))
      return
    }
    const cert = await generateMldsa65.mutateAsync(nodeID)
    form.setFieldsValue({
      realityMldsa65Seed: cert.seed,
      realityMldsa65Verify: cert.verify,
      realityGenerateMldsa65: false,
    })
    messageApi.success(t('admin.inboundEditor.stream.generatedMldsa65'))
  }

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

  const fillRandomTarget = () => {
    const next = randomRealityTarget()
    form.setFieldsValue({
      realityDest: next.target,
      realityServerNames: next.sni,
      realityRandomizeTarget: false,
      realityRandomizeSNI: false,
    })
  }

  const fillRandomSNI = () => {
    form.setFieldValue('realityServerNames', randomRealityTarget().sni)
    form.setFieldValue('realityRandomizeSNI', false)
  }

  const fillRandomShortIds = () => {
    form.setFieldsValue({
      realityShortIds: randomRealityShortIds().join(','),
      realityRandomizeShortIds: false,
    })
  }

  return (
    <Modal
      title={t('admin.inboundEditor.stream.configureReality')}
      open={open}
      onCancel={onClose}
      footer={<Button type="primary" onClick={onClose}>{t('admin.inboundEditor.stream.done')}</Button>}
      width={760}
      destroyOnHidden={false}
      maskClosable
    >
      {contextHolder}
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
                <Button
                  aria-label={t('admin.inboundEditor.stream.randomizeTarget')}
                  size="small"
                  type="text"
                  icon={<ReloadOutlined />}
                  onClick={fillRandomTarget}
                />
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
                <Button
                  aria-label={t('admin.inboundEditor.stream.randomizeSNI')}
                  size="small"
                  type="text"
                  icon={<ReloadOutlined />}
                  onClick={fillRandomSNI}
                />
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
                <Button
                  aria-label={t('admin.inboundEditor.stream.randomizeShortIds')}
                  size="small"
                  type="text"
                  icon={<ReloadOutlined />}
                  onClick={fillRandomShortIds}
                />
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
            <Input style={{ width: 320 }} />
          </Form.Item>
          <Form.Item
            name="realityPrivateKey"
            label={t('admin.inboundEditor.stream.privateKey')}
            dependencies={['realityPublicKey']}
            rules={[
              ({ getFieldValue }) => ({
                validator(_, value) {
                  const publicKey = String(getFieldValue('realityPublicKey') ?? '').trim()
                  const privateKey = String(value ?? '').trim()
                  if (!publicKey && privateKey) {
                    return Promise.reject(new Error(t('admin.inboundEditor.stream.publicKeyRequired')))
                  }
                  return Promise.resolve()
                },
              }),
            ]}
          >
            <Input style={{ width: 320 }} />
          </Form.Item>
        </Space>
        <Space size={12} align="center">
          {generationEnabled ? (
            <Button
              onClick={generateRealityKeypair}
              loading={generateX25519.isPending}
              disabled={!canGenerateOnNode}
            >
              {t('admin.inboundEditor.stream.getNewCert')}
            </Button>
          ) : (
            <>
              <Form.Item name="realityGenerateKeypair" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
              <span style={{ color: '#888' }}>{t('admin.inboundEditor.stream.willGenerateKeypair')}</span>
            </>
          )}
          <Button size="small" onClick={clearRealityKeypair}>{t('admin.inboundEditor.stream.clear')}</Button>
        </Space>

        <Divider orientation="left" plain style={{ margin: '4px 0' }}>ML-DSA-65 seed</Divider>
        <Space align="start" wrap size={[12, 0]}>
          <Form.Item name="realityMldsa65Seed" label="Seed">
            <Input.TextArea rows={2} style={{ width: 320 }} />
          </Form.Item>
          <Form.Item name="realityMldsa65Verify" label="Verify">
            <Input.TextArea rows={2} style={{ width: 320 }} />
          </Form.Item>
        </Space>
        <Space size={12} align="center">
          {generationEnabled ? (
            <Button
              onClick={generateMldsa65Seed}
              loading={generateMldsa65.isPending}
              disabled={!canGenerateOnNode}
            >
              {t('admin.inboundEditor.stream.getNewSeed')}
            </Button>
          ) : (
            <>
              <Form.Item name="realityGenerateMldsa65" valuePropName="checked" noStyle>
                <Switch />
              </Form.Item>
              <span style={{ color: '#888' }}>{t('admin.inboundEditor.stream.willGenerateMldsa65')}</span>
            </>
          )}
          <Button size="small" onClick={clearMldsa65}>{t('admin.inboundEditor.stream.clear')}</Button>
        </Space>
      </Space>
    </Modal>
  )
}
