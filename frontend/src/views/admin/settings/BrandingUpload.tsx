import { DeleteOutlined, UploadOutlined } from '@ant-design/icons'
import { Button, Input, Space, Typography, message } from 'antd'
import type { ChangeEvent } from 'react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { useUploadBrandIcon } from '@/hooks/queries/admin/settings'
import { BRAND_ICON_KEY } from './settingHelpers'
import type { Drafts } from './types'

const BRAND_FIELDS = [
  'brand_title',
  'brand_subtitle',
  'brand_description',
  'brand_footer',
  'brand_docs_url',
  'brand_homepage_content',
  BRAND_ICON_KEY,
] as const

export interface BrandingUploadProps {
  items: SettingItem[]
  drafts: Drafts
  savingKey?: string | null
  onDraftChange: (key: string, value: string) => void
  onSave: (item: SettingItem) => Promise<void> | void
}

function brandItem(items: SettingItem[], key: string) {
  return items.find((item) => item.key === key)
}

function draftValue(items: SettingItem[], drafts: Drafts, key: string) {
  const item = brandItem(items, key)
  return drafts[key] ?? item?.value ?? ''
}

function changedBrandItems(items: SettingItem[], drafts: Drafts) {
  return BRAND_FIELDS.flatMap((key) => {
    const item = brandItem(items, key)
    if (!item) return []
    return (drafts[item.key] ?? item.value ?? '') !== (item.value ?? '') ? [item] : []
  })
}

export function BrandingUpload({
  drafts,
  items,
  onDraftChange,
  onSave,
  savingKey,
}: BrandingUploadProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const upload = useUploadBrandIcon()
  const changed = changedBrandItems(items, drafts)
  const iconItem = brandItem(items, BRAND_ICON_KEY)
  const iconURL = draftValue(items, drafts, BRAND_ICON_KEY)
  const title = draftValue(items, drafts, 'brand_title')
  const subtitle = draftValue(items, drafts, 'brand_subtitle')
  const hasIconOverride = Boolean(iconItem?.has_override || iconURL)
  const saving = Boolean(savingKey && BRAND_FIELDS.includes(savingKey as (typeof BRAND_FIELDS)[number]))

  const submit = async () => {
    for (const item of changed) {
      await onSave(item)
    }
    if (changed.length > 0) {
      message.success(t('admin.settings.branding.saved'))
    }
  }

  const uploadIcon = async () => {
    if (!file) return
    const result = await upload.mutateAsync(file)
    onDraftChange(BRAND_ICON_KEY, result.url)
    setFile(null)
    if (inputRef.current) inputRef.current.value = ''
    message.success(t('admin.settings.branding.uploaded'))
  }

  const removeIcon = async () => {
    if (iconItem) {
      onDraftChange(BRAND_ICON_KEY, '')
    }
    message.success(t('admin.settings.branding.iconRemoved'))
  }

  const selectFile = (event: ChangeEvent<HTMLInputElement>) => {
    setFile(event.target.files?.[0] ?? null)
  }

  return (
    <section className="settings-brand-panel" aria-labelledby="settings-brand-title">
      <div className="settings-brand-heading">
        <div>
          <Typography.Title id="settings-brand-title" level={4}>
            {t('admin.settings.branding.title')}
          </Typography.Title>
          <Typography.Text type="secondary">{t('admin.settings.branding.desc')}</Typography.Text>
        </div>
        <Button type="primary" disabled={changed.length === 0} loading={saving} onClick={submit}>
          {t('admin.settings.branding.save')}
        </Button>
      </div>

      <div className="settings-brand-grid">
        <label className="settings-brand-field" data-setting-key="brand_title">
          <span>{t('admin.settings.branding.siteName')}</span>
          <Input
            aria-label={t('admin.settings.branding.siteName')}
            maxLength={80}
            value={title}
            onChange={(event) => onDraftChange('brand_title', event.target.value)}
          />
          <small>{t('admin.settings.branding.siteNameDesc')}</small>
        </label>
        <label className="settings-brand-field" data-setting-key="brand_subtitle">
          <span>{t('admin.settings.branding.siteSubtitle')}</span>
          <Input
            aria-label={t('admin.settings.branding.siteSubtitle')}
            maxLength={120}
            value={subtitle}
            onChange={(event) => onDraftChange('brand_subtitle', event.target.value)}
          />
          <small>{t('admin.settings.branding.siteSubtitleDesc')}</small>
        </label>
      </div>

      <label className="settings-brand-field" data-setting-key="brand_docs_url">
        <span>{t('admin.settings.branding.docsLink')}</span>
        <Input
          aria-label={t('admin.settings.branding.docsLink')}
          inputMode="url"
          placeholder="https://docs.example.com"
          value={draftValue(items, drafts, 'brand_docs_url')}
          onChange={(event) => onDraftChange('brand_docs_url', event.target.value)}
        />
        <small>{t('admin.settings.branding.docsLinkDesc')}</small>
      </label>

      <div className="settings-brand-logo-row" data-setting-key={BRAND_ICON_KEY}>
        <div>
          <Typography.Text className="settings-brand-label">{t('admin.settings.branding.logo')}</Typography.Text>
          <div className="settings-brand-logo-actions">
            <div className="settings-brand-logo-preview" aria-label={t('admin.settings.branding.logoPreview')}>
              {iconURL ? <img alt="" src={iconURL} /> : <span>{(title || '3x').slice(0, 2)}</span>}
            </div>
            <Space wrap>
              <input
                ref={inputRef}
                aria-label={t('admin.settings.branding.fileLabel')}
                accept="image/png,image/jpeg,image/webp,image/svg+xml"
                className="settings-brand-file-input"
                type="file"
                onChange={selectFile}
              />
              <Button
                aria-label={t('admin.settings.branding.uploadFavicon')}
                icon={<UploadOutlined />}
                disabled={!file}
                loading={upload.isPending}
                onClick={uploadIcon}
              >
                {t('admin.settings.branding.upload')}
              </Button>
              <Button danger icon={<DeleteOutlined />} disabled={!hasIconOverride} onClick={removeIcon}>
                {t('admin.settings.branding.remove')}
              </Button>
            </Space>
          </div>
          <Typography.Text className="settings-brand-hint" type="secondary">
            {t('admin.settings.branding.logoDesc')}
          </Typography.Text>
        </div>
      </div>

      <label className="settings-brand-field" data-setting-key="brand_homepage_content">
        <span>{t('admin.settings.branding.homepageContent')}</span>
        <Input.TextArea
          aria-label={t('admin.settings.branding.homepageContent')}
          className="settings-brand-home-content"
          maxLength={4000}
          rows={6}
          value={draftValue(items, drafts, 'brand_homepage_content')}
          onChange={(event) => onDraftChange('brand_homepage_content', event.target.value)}
        />
        <small>{t('admin.settings.branding.homepageContentDesc')}</small>
      </label>

      <label className="settings-brand-field" data-setting-key="brand_description">
        <span>{t('admin.settings.branding.loginDescription')}</span>
        <Input.TextArea
          aria-label={t('admin.settings.branding.loginDescription')}
          maxLength={240}
          rows={2}
          value={draftValue(items, drafts, 'brand_description')}
          onChange={(event) => onDraftChange('brand_description', event.target.value)}
        />
        <small>{t('admin.settings.branding.loginDescriptionDesc')}</small>
      </label>

      <label className="settings-brand-field" data-setting-key="brand_footer">
        <span>{t('admin.settings.branding.footer')}</span>
        <Input
          aria-label={t('admin.settings.branding.footer')}
          maxLength={240}
          value={draftValue(items, drafts, 'brand_footer')}
          onChange={(event) => onDraftChange('brand_footer', event.target.value)}
        />
        <small>{t('admin.settings.branding.footerDesc')}</small>
      </label>
    </section>
  )
}
