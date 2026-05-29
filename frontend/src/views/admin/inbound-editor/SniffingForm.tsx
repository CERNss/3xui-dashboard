import { Checkbox, Form, Select, Space, Switch } from 'antd'
import { useTranslation } from 'react-i18next'

const IP_HINT_OPTIONS = ['geoip:cn', 'geoip:private', 'geoip:!cn', 'ext:geoip.dat:cn'].map((v) => ({ value: v, label: v }))
const DOMAIN_HINT_OPTIONS = ['domain:example.com', 'geosite:cn', 'geosite:private', 'ext:geosite.dat:cn'].map((v) => ({ value: v, label: v }))

export function SniffingForm() {
  const { t } = useTranslation()
  const enabled = Form.useWatch('sniffEnabled')
  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
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
          <Form.Item
            name="sniffExcludedIPs"
            label={t('admin.inboundEditor.sniff.excludedIPs')}
            tooltip={t('admin.inboundEditor.sniff.excludedIPsHint')}
          >
            <Select
              mode="tags"
              style={{ width: '100%' }}
              placeholder="IP / CIDR / geoip:* / ext:*"
              tokenSeparators={[',', ' ']}
              options={IP_HINT_OPTIONS}
            />
          </Form.Item>
          <Form.Item
            name="sniffExcludedDomains"
            label={t('admin.inboundEditor.sniff.excludedDomains')}
            tooltip={t('admin.inboundEditor.sniff.excludedDomainsHint')}
          >
            <Select
              mode="tags"
              style={{ width: '100%' }}
              placeholder="domain:* / geosite:* / ext:*"
              tokenSeparators={[',', ' ']}
              options={DOMAIN_HINT_OPTIONS}
            />
          </Form.Item>
        </>
      ) : null}
    </Space>
  )
}
