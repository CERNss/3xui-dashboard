import { CheckCircleOutlined, CodeOutlined, FormatPainterOutlined } from '@ant-design/icons'
import { Alert, Button, Input, Space, Typography } from 'antd'
import { dump as dumpYAML, load as loadYAML } from 'js-yaml'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { SettingItem } from '@/api/admin/settings'
import { inputMax, inputMin, itemValue, localizedDescription, localizedLabel } from './settingHelpers'
import type { Drafts } from './types'

type TemplateFormat = 'yaml' | 'json'

const TEMPLATE_PROXIES_PLACEHOLDER = '${proxies}'
const TEMPLATE_PROXY_NAMES_PLACEHOLDER = '${proxy_names}'
const TEMPLATE_PROXY_GROUPS_PLACEHOLDER = '${proxy_groups}'
const TEMPLATE_RULE_PROVIDERS_PLACEHOLDER = '${rule_providers}'
const TEMPLATE_RULES_PLACEHOLDER = '${rules}'
const TEMPLATE_PROXIES_SENTINEL = '__3xui_template_proxies__'
const TEMPLATE_PROXY_NAMES_SENTINEL = '__3xui_template_proxy_names__'
const TEMPLATE_PROXY_GROUPS_SENTINEL = '__3xui_template_proxy_groups__'
const TEMPLATE_RULE_PROVIDERS_SENTINEL = '__3xui_template_rule_providers__'
const TEMPLATE_RULES_SENTINEL = '__3xui_template_rules__'
const CLASH_TEMPLATE_KEY = 'clash_template_yaml'
const SINGBOX_TEMPLATE_KEY = 'singbox_template_json'

interface FieldFormatHint {
  badge: string
  note: string
  tokens?: string[]
}

export interface SettingRowProps {
  item: SettingItem
  drafts: Drafts
  saving?: boolean
  onDraftChange: (key: string, value: string) => void
  onSave: (item: SettingItem) => void
  onReset: (item: SettingItem) => void
}

export function SettingRow({ item, drafts, saving, onDraftChange, onSave, onReset }: SettingRowProps) {
  const { i18n, t } = useTranslation()
  const [templateFeedback, setTemplateFeedback] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)
  const draft = drafts[item.key] ?? itemValue(item)
  const changed = draft !== itemValue(item)
  const controlID = `setting-${item.key}`
  const label = localizedLabel(item, i18n.language)
  const description = localizedDescription(item, i18n.language)
  const templateFormat = templateFormatForKey(item.key)
  const fieldFormatHint = fieldFormatHintForKey(item.key, t)
  const multiline = Boolean(templateFormat) || item.key.endsWith('_public_key')
  const rowClassName = `settings-setting-row${multiline ? ' settings-setting-row--multiline' : ''}${templateFormat ? ' settings-setting-row--template' : ''}${fieldFormatHint ? ' settings-setting-row--format-field' : ''}${changed ? ' settings-setting-row--changed' : ''}`
  const placeholder = placeholderForKey(item.key)
  const validation = useMemo(
    () => (templateFormat ? validateTemplateDraft(draft, templateFormat, t) : null),
    [draft, t, templateFormat],
  )
  const saveDisabled = !changed || (validation ? !validation.ok : false)

  const handleTemplateFormat = () => {
    if (!templateFormat) return
    try {
      const next = formatTemplateDraft(draft, templateFormat)
      onDraftChange(item.key, next)
      setTemplateFeedback({ kind: 'success', text: t('admin.settings.formatEditor.formatted', { format: templateFormatLabel(templateFormat) }) })
    } catch (error) {
      setTemplateFeedback({
        kind: 'error',
        text: t('admin.settings.formatEditor.invalid', {
          format: templateFormatLabel(templateFormat),
          message: errorMessage(error),
        }),
      })
    }
  }

  const handleTemplateValidate = () => {
    if (!templateFormat || !validation) return
    setTemplateFeedback({
      kind: validation.ok ? 'success' : 'error',
      text: validation.message,
    })
  }

  return (
    <div className={rowClassName} data-setting-key={item.key}>
      <div className="settings-setting-copy">
        <label className="settings-setting-label" htmlFor={controlID}>
          {label}
        </label>
        {description ? (
          <Typography.Text className="settings-setting-description" type="secondary">
            {description}
          </Typography.Text>
        ) : null}
        {item.env_fallback ? (
          <Typography.Text className="settings-setting-description" type="secondary">
            {t('admin.settings.fallback', { value: item.env_fallback })}
          </Typography.Text>
        ) : null}
      </div>
      <div className="settings-setting-control">
        <div className="settings-setting-field">
          {item.secret ? (
            <Input.Password
              aria-label={label}
              className="settings-setting-input"
              id={controlID}
              autoComplete="new-password"
              placeholder={item.has_override || item.env_fallback ? '********' : ''}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          ) : item.type === 'bool' ? (
            <select
              aria-label={label}
              className="settings-setting-native-control"
              id={controlID}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            >
              <option value="">{t('admin.settings.useDefault')}</option>
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
          ) : item.type === 'int' ? (
            <input
              aria-label={label}
              className="settings-setting-native-control"
              id={controlID}
              max={inputMax(item.key, drafts)}
              min={inputMin(item.key)}
              type="number"
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          ) : templateFormat ? (
            <div className="settings-template-editor">
              <div className="settings-template-toolbar">
                <span className="settings-template-language">
                  <CodeOutlined />
                  {templateFormatLabel(templateFormat)}
                </span>
                <Space size={8} wrap>
                  <Button
                    aria-label={t('admin.settings.formatEditor.format')}
                    icon={<FormatPainterOutlined />}
                    size="small"
                    onClick={handleTemplateFormat}
                  >
                    {t('admin.settings.formatEditor.format')}
                  </Button>
                  <Button
                    aria-label={t('admin.settings.formatEditor.validate')}
                    icon={<CheckCircleOutlined />}
                    size="small"
                    onClick={handleTemplateValidate}
                  >
                    {t('admin.settings.formatEditor.validate')}
                  </Button>
                </Space>
              </div>
              <Input.TextArea
                aria-label={label}
                className="settings-setting-input settings-setting-textarea settings-template-textarea"
                id={controlID}
                autoComplete="off"
                autoCorrect="off"
                placeholder={templatePlaceholder(templateFormat)}
                rows={10}
                spellCheck={false}
                value={draft}
                onChange={(event) => {
                  setTemplateFeedback(null)
                  onDraftChange(item.key, event.target.value)
                }}
              />
              <div className="settings-template-meta">
                <span>{t('admin.settings.formatEditor.placeholderHint')}</span>
                {templatePlaceholders(templateFormat).map((placeholder) => (
                  <code key={placeholder}>{placeholder}</code>
                ))}
              </div>
              {templateFeedback ? (
                <Alert className="settings-template-alert" type={templateFeedback.kind} showIcon message={templateFeedback.text} />
              ) : validation && !validation.ok ? (
                <Alert className="settings-template-alert" type="error" showIcon message={validation.message} />
              ) : null}
            </div>
          ) : multiline ? (
            <Input.TextArea
              aria-label={label}
              className="settings-setting-input settings-setting-textarea"
              id={controlID}
              rows={5}
              value={draft}
              onChange={(event) => onDraftChange(item.key, event.target.value)}
            />
          ) : (
            <div className={fieldFormatHint ? 'settings-format-field' : undefined}>
              <Input
                aria-label={label}
                className="settings-setting-input"
                id={controlID}
                placeholder={placeholder}
                prefix={fieldFormatHint ? <span className="settings-format-prefix">{fieldFormatHint.badge}</span> : undefined}
                value={draft}
                onChange={(event) => onDraftChange(item.key, event.target.value)}
              />
              {fieldFormatHint ? (
                <div className="settings-format-note">
                  <span>{fieldFormatHint.note}</span>
                  {fieldFormatHint.tokens?.map((token) => (
                    <code className="settings-format-token" key={token}>
                      {token}
                    </code>
                  ))}
                </div>
              ) : null}
            </div>
          )}
        </div>
        <Space className="settings-setting-actions" wrap>
          <Button type="primary" size="small" disabled={saveDisabled} loading={saving} onClick={() => onSave(item)}>
            {t('admin.settings.save')}
          </Button>
          {item.has_override ? (
            <Button size="small" loading={saving} onClick={() => onReset(item)}>
              {t('admin.settings.reset')}
            </Button>
          ) : null}
        </Space>
      </div>
    </div>
  )
}

function templateFormatForKey(key: string): TemplateFormat | null {
  if (key === CLASH_TEMPLATE_KEY) return 'yaml'
  if (key === SINGBOX_TEMPLATE_KEY) return 'json'
  return null
}

function placeholderForKey(key: string) {
  if (key === 'subscription_remark_model') return '/i/e/o/t'
  if (key === 'subscription_public_base_url') return 'https://sub.example.com'
  return undefined
}

function fieldFormatHintForKey(key: string, t: ReturnType<typeof useTranslation>['t']): FieldFormatHint | null {
  if (key === 'subscription_remark_model') {
    return {
      badge: t('admin.settings.fieldFormats.remarkBadge'),
      note: t('admin.settings.fieldFormats.remarkNote'),
      tokens: ['i', 'e', 'o', 't'],
    }
  }
  if (key === 'subscription_public_base_url') {
    return {
      badge: t('admin.settings.fieldFormats.urlBadge'),
      note: t('admin.settings.fieldFormats.urlNote'),
    }
  }
  return null
}

function templateFormatLabel(format: TemplateFormat) {
  return format === 'yaml' ? 'YAML' : 'JSON'
}

function templatePlaceholder(format: TemplateFormat) {
  if (format === 'yaml') {
    return `mixed-port: 7890
proxies:
${TEMPLATE_PROXIES_PLACEHOLDER}

${TEMPLATE_PROXY_GROUPS_PLACEHOLDER}
${TEMPLATE_RULE_PROVIDERS_PLACEHOLDER}
${TEMPLATE_RULES_PLACEHOLDER}`
  }
  return `{
  "outbounds": [
    {"type": "selector", "tag": "select", "outbounds": [${TEMPLATE_PROXY_NAMES_PLACEHOLDER}, "direct"]},
    ${TEMPLATE_PROXIES_PLACEHOLDER},
    {"type": "direct", "tag": "direct"}
  ],
  "route": {
    "final": "select"
  }
}`
}

function templatePlaceholders(format: TemplateFormat) {
  if (format === 'yaml') {
    return [
      TEMPLATE_PROXIES_PLACEHOLDER,
      TEMPLATE_PROXY_NAMES_PLACEHOLDER,
      TEMPLATE_PROXY_GROUPS_PLACEHOLDER,
      TEMPLATE_RULE_PROVIDERS_PLACEHOLDER,
      TEMPLATE_RULES_PLACEHOLDER,
    ]
  }
  return [TEMPLATE_PROXIES_PLACEHOLDER, TEMPLATE_PROXY_NAMES_PLACEHOLDER]
}

function validateTemplateDraft(value: string, format: TemplateFormat, t: ReturnType<typeof useTranslation>['t']) {
  const trimmed = value.trim()
  if (!trimmed) {
    return { ok: true, message: t('admin.settings.formatEditor.empty') }
  }
  if (!value.includes(TEMPLATE_PROXIES_PLACEHOLDER)) {
    return { ok: false, message: t('admin.settings.formatEditor.missingPlaceholder', { placeholder: TEMPLATE_PROXIES_PLACEHOLDER }) }
  }
  try {
    const parsed = parseProtectedTemplate(value, format)
    if (!isPlainObject(parsed)) {
      return { ok: false, message: t('admin.settings.formatEditor.objectRequired', { format: templateFormatLabel(format) }) }
    }
    return { ok: true, message: t('admin.settings.formatEditor.valid', { format: templateFormatLabel(format) }) }
  } catch (error) {
    return {
      ok: false,
      message: t('admin.settings.formatEditor.invalid', {
        format: templateFormatLabel(format),
        message: errorMessage(error),
      }),
    }
  }
}

function formatTemplateDraft(value: string, format: TemplateFormat) {
  if (!value.trim()) return ''
  const parsed = parseProtectedTemplate(value, format)
  if (!isPlainObject(parsed)) {
    throw new Error(`${templateFormatLabel(format)} must be an object`)
  }
  const formatted =
    format === 'json'
      ? JSON.stringify(parsed, null, 2)
      : dumpYAML(parsed, { flowLevel: 3, indent: 2, lineWidth: 100, noRefs: true, sortKeys: false })
  return restoreTemplatePlaceholders(formatted, format).trimEnd()
}

function parseProtectedTemplate(value: string, format: TemplateFormat) {
  const protectedValue = format === 'json' ? protectTemplateJSONPlaceholders(value) : protectTemplateYAMLPlaceholders(value)
  return format === 'json' ? JSON.parse(protectedValue) : loadYAML(protectedValue)
}

function protectTemplateJSONPlaceholders(value: string) {
  return replaceAll(
    replaceAll(value, TEMPLATE_PROXIES_PLACEHOLDER, `{"__3xui_template_placeholder":"${TEMPLATE_PROXIES_SENTINEL}"}`),
    TEMPLATE_PROXY_NAMES_PLACEHOLDER,
    `"${TEMPLATE_PROXY_NAMES_SENTINEL}"`,
  )
}

function protectTemplateYAMLPlaceholders(value: string) {
  return value
    .split('\n')
    .map((line) => {
      const trimmed = line.trim()
      const indent = line.match(/^[ \t]*/)?.[0] ?? ''
      if (trimmed === TEMPLATE_PROXIES_PLACEHOLDER) return `${indent || '  '}- __3xui_template_placeholder: ${TEMPLATE_PROXIES_SENTINEL}`
      if (trimmed === TEMPLATE_PROXY_GROUPS_PLACEHOLDER) return `${indent}${TEMPLATE_PROXY_GROUPS_SENTINEL}: []`
      if (trimmed === TEMPLATE_RULE_PROVIDERS_PLACEHOLDER) return `${indent}${TEMPLATE_RULE_PROVIDERS_SENTINEL}: []`
      if (trimmed === TEMPLATE_RULES_PLACEHOLDER) return `${indent}${TEMPLATE_RULES_SENTINEL}: []`
      return replaceTemplatePlaceholders(line, [
        [TEMPLATE_PROXIES_PLACEHOLDER, TEMPLATE_PROXIES_SENTINEL],
        [TEMPLATE_PROXY_NAMES_PLACEHOLDER, TEMPLATE_PROXY_NAMES_SENTINEL],
        [TEMPLATE_PROXY_GROUPS_PLACEHOLDER, TEMPLATE_PROXY_GROUPS_SENTINEL],
        [TEMPLATE_RULE_PROVIDERS_PLACEHOLDER, TEMPLATE_RULE_PROVIDERS_SENTINEL],
        [TEMPLATE_RULES_PLACEHOLDER, TEMPLATE_RULES_SENTINEL],
      ])
    })
    .join('\n')
}

function restoreTemplatePlaceholders(value: string, format: TemplateFormat) {
  if (format === 'json') {
    return value
      .replace(/\{\s*"__3xui_template_placeholder"\s*:\s*"__3xui_template_proxies__"\s*\}/g, () => TEMPLATE_PROXIES_PLACEHOLDER)
      .replace(/"__3xui_template_proxy_names__"/g, () => TEMPLATE_PROXY_NAMES_PLACEHOLDER)
  }
  return value
    .split('\n')
    .flatMap((line) => {
      const indent = line.match(/^[ \t]*/)?.[0] ?? ''
      const trimmed = line.trim()
      if (line.trim() === `- __3xui_template_placeholder: ${TEMPLATE_PROXIES_SENTINEL}`) {
        return `${indent}${TEMPLATE_PROXIES_PLACEHOLDER}`
      }
      if (trimmed === `${TEMPLATE_PROXY_GROUPS_SENTINEL}: []`) return `${indent}${TEMPLATE_PROXY_GROUPS_PLACEHOLDER}`
      if (trimmed === `${TEMPLATE_RULE_PROVIDERS_SENTINEL}: []`) return `${indent}${TEMPLATE_RULE_PROVIDERS_PLACEHOLDER}`
      if (trimmed === `${TEMPLATE_RULES_SENTINEL}: []`) return `${indent}${TEMPLATE_RULES_PLACEHOLDER}`
      return replaceTemplatePlaceholders(line, [
        [TEMPLATE_PROXIES_SENTINEL, TEMPLATE_PROXIES_PLACEHOLDER],
        [TEMPLATE_PROXY_NAMES_SENTINEL, TEMPLATE_PROXY_NAMES_PLACEHOLDER],
        [TEMPLATE_PROXY_GROUPS_SENTINEL, TEMPLATE_PROXY_GROUPS_PLACEHOLDER],
        [TEMPLATE_RULE_PROVIDERS_SENTINEL, TEMPLATE_RULE_PROVIDERS_PLACEHOLDER],
        [TEMPLATE_RULES_SENTINEL, TEMPLATE_RULES_PLACEHOLDER],
      ])
    })
    .join('\n')
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

function replaceAll(value: string, search: string, replacement: string) {
  return value.split(search).join(replacement)
}

function replaceTemplatePlaceholders(value: string, replacements: Array<[string, string]>) {
  return replacements.reduce((current, [search, replacement]) => replaceAll(current, search, replacement), value)
}
