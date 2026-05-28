import { Alert, Form, Input, Radio, Space, Tag } from 'antd'
import { useTranslation } from 'react-i18next'
import { ProtocolClients } from '../ProtocolClients'

type AuthMode = 'none' | 'x25519' | 'mlkem768'

const AUTH_LABEL: Record<AuthMode, string> = {
  none: 'None',
  x25519: 'X25519',
  mlkem768: 'ML-KEM-768',
}

export function VlessProtocol({ hideClients }: { hideClients?: boolean } = {}) {
  const { t } = useTranslation()
  const form = Form.useFormInstance()
  const authMode = Form.useWatch<AuthMode>('vlessAuthMode', form) ?? 'none'

  const setAuthMode = (mode: AuthMode) => {
    form.setFieldValue('vlessAuthMode', mode)
    if (mode === 'none') {
      form.setFieldsValue({ decryption: 'none', encryption: 'none' })
    } else {
      // For non-None modes the actual key strings get filled in by the
      // backend at create time (node panel /getNewVlessEnc). In the
      // editor we leave a sentinel that makes it visually clear nothing
      // is pre-generated yet.
      form.setFieldsValue({ decryption: `auto:${mode}`, encryption: `auto:${mode}` })
    }
  }

  return (
    <ProtocolClients
      hideClients={hideClients}
      title={t('admin.inboundEditor.clients.vlessTitle')}
      fields={[
        { name: 'id', label: 'UUID', placeholder: t('admin.inboundEditor.clients.uuidPlaceholder') },
        { name: 'flow', label: 'Flow', placeholder: 'xtls-rprx-vision' },
        { name: 'email', label: 'Email', placeholder: 'alice@example.com' },
        { name: 'expiryTime', label: t('admin.inboundEditor.basicExpiry'), numeric: true },
        { name: 'enable', label: t('admin.inboundEditor.basicEnable'), switch: true, defaultValue: true },
      ]}
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Space align="start" wrap>
          <Form.Item name="decryption" label={t('admin.inboundEditor.decryption')}>
            <Input style={{ width: 220 }} disabled={authMode !== 'none'} />
          </Form.Item>
          <Form.Item name="encryption" label={t('admin.inboundEditor.encryption')}>
            <Input style={{ width: 220 }} disabled={authMode !== 'none'} />
          </Form.Item>
        </Space>
        <Form.Item name="vlessAuthMode" label={t('admin.inboundEditor.vlessAuthMode')}>
          <Radio.Group
            optionType="button"
            buttonStyle="solid"
            onChange={(event) => setAuthMode(event.target.value as AuthMode)}
            options={[
              { label: t('admin.inboundEditor.vlessAuthNone'), value: 'none' },
              { label: t('admin.inboundEditor.vlessAuthX25519'), value: 'x25519' },
              { label: t('admin.inboundEditor.vlessAuthMlkem768'), value: 'mlkem768' },
            ]}
          />
        </Form.Item>
        <Space size={6}>
          <span style={{ color: '#888' }}>{t('admin.inboundEditor.vlessAuthSelected')}</span>
          <Tag color={authMode === 'none' ? 'default' : 'blue'}>{AUTH_LABEL[authMode]}</Tag>
        </Space>
        {authMode !== 'none' ? (
          <Alert
            type="info"
            showIcon
            message={t('admin.inboundEditor.vlessAuthHint')}
          />
        ) : null}
      </Space>
    </ProtocolClients>
  )
}
