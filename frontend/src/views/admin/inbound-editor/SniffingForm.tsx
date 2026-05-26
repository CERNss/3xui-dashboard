import { Checkbox, Form, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

export function SniffingForm() {
  const { t } = useTranslation()
  const enabled = Form.useWatch('sniffEnabled')
  return (
    <Space direction="vertical" size={12}>
      <Form.Item name="sniffEnabled" label={t('admin.inboundEditor.sniff.enabled')} valuePropName="checked">
        <Switch />
      </Form.Item>
      {enabled ? (
        <>
          <Space wrap>
            <Form.Item name="sniffHttp" valuePropName="checked">
              <Checkbox>http</Checkbox>
            </Form.Item>
            <Form.Item name="sniffTls" valuePropName="checked">
              <Checkbox>tls</Checkbox>
            </Form.Item>
            <Form.Item name="sniffQuic" valuePropName="checked">
              <Checkbox>quic</Checkbox>
            </Form.Item>
            <Form.Item name="sniffFakedns" valuePropName="checked">
              <Checkbox>fakedns</Checkbox>
            </Form.Item>
          </Space>
          <Space wrap>
            <Form.Item name="sniffMetadataOnly" label={t('admin.inboundEditor.sniff.metadataOnly')} valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="sniffRouteOnly" label={t('admin.inboundEditor.sniff.routeOnly')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
        </>
      ) : null}
    </Space>
  )
}
