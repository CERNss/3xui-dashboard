import { Button, Drawer, Form, Input, InputNumber, Select, Space, Switch } from 'antd'
import type { FormInstance } from 'antd'
import { useTranslation } from 'react-i18next'
import { AREA_OPTIONS, parsePanelURL, type NodeFormValues } from './utils'

interface NodeDrawerProps {
  editingName?: string
  form: FormInstance<NodeFormValues>
  open: boolean
  saving?: boolean
  onClose: () => void
  onSubmit: () => void
}

export function NodeDrawer({ editingName, form, open, saving, onClose, onSubmit }: NodeDrawerProps) {
  const { t } = useTranslation()
  const isEditing = Boolean(editingName)

  const applyQuickURL = (value: string) => {
    const parsed = parsePanelURL(value)
    if (!parsed) return
    form.setFieldsValue(parsed)
  }

  return (
    <Drawer
      title={isEditing ? t('admin.nodes.editTitle', { name: editingName }) : t('admin.nodes.createTitle')}
      open={open}
      width={560}
      onClose={onClose}
      destroyOnClose
      extra={
        <Space>
          <Button onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="primary" loading={saving} onClick={onSubmit}>
            {saving ? t('admin.nodes.updating') : t('admin.nodes.save')}
          </Button>
        </Space>
      }
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item label={t('admin.nodes.quickImportLabel')} tooltip={t('admin.nodes.quickImportHint')}>
          <Input
            aria-label={t('admin.nodes.quickImportLabel')}
            placeholder={t('admin.nodes.quickImportPlaceholder')}
            onChange={(event) => applyQuickURL(event.target.value)}
          />
        </Form.Item>

        <Form.Item name="name" label={t('admin.nodes.name')} rules={[{ required: true, whitespace: true, message: t('admin.nodes.nameRequired') }]}>
          <Input placeholder={t('admin.nodes.namePlaceholder')} />
        </Form.Item>

        <Space align="start" style={{ width: '100%' }} wrap>
          <Form.Item name="area" label={t('admin.nodes.areaLabel')} rules={[{ required: true, message: t('admin.nodes.areaRequired') }]}>
            <Select
              style={{ minWidth: 180 }}
              options={AREA_OPTIONS.map((area) => ({ label: t(`admin.nodes.area.${area.key}`), value: area.key }))}
            />
          </Form.Item>
          <Form.Item
            name="province"
            label={t('admin.nodes.provinceLabel')}
            rules={[{ required: true, whitespace: true, message: t('admin.nodes.provinceRequired') }]}
          >
            <Input placeholder={t('admin.nodes.provincePlaceholder')} />
          </Form.Item>
        </Space>

        <Space align="start" style={{ width: '100%' }} wrap>
          <Form.Item name="scheme" label={t('admin.nodes.scheme')} rules={[{ required: true, message: t('admin.nodes.schemeRequired') }]}>
            <Select
              style={{ width: 120 }}
              options={[
                { label: 'https', value: 'https' },
                { label: 'http', value: 'http' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="port"
            label={t('admin.nodes.port')}
            rules={[
              { required: true, type: 'number', message: t('admin.nodes.portRequired') },
              { type: 'number', min: 1, max: 65535, message: t('admin.nodes.portRangeInvalid') },
            ]}
          >
            <InputNumber precision={0} />
          </Form.Item>
        </Space>

        <Form.Item name="host" label={t('admin.nodes.host')} rules={[{ required: true, whitespace: true, message: t('admin.nodes.hostRequired') }]}>
          <Input placeholder={t('admin.nodes.hostPlaceholder')} />
        </Form.Item>

        <Form.Item name="base_path" label={t('admin.nodes.basePath')}>
          <Input placeholder={t('admin.nodes.basePathPlaceholder')} />
        </Form.Item>

        <Form.Item
          name="api_token"
          label={t('admin.nodes.apiToken')}
          rules={isEditing ? [] : [{ required: true, whitespace: true, message: t('admin.nodes.apiTokenRequired') }]}
          extra={isEditing ? t('admin.nodes.apiTokenKeepHint') : undefined}
        >
          <Input placeholder={isEditing ? t('admin.nodes.apiTokenEditPlaceholder') : t('admin.nodes.apiTokenPlaceholder')} />
        </Form.Item>

        <Form.Item name="enabled" label={t('admin.nodes.enableDefaultLabel')} valuePropName="checked">
          <Switch />
        </Form.Item>
      </Form>
    </Drawer>
  )
}
