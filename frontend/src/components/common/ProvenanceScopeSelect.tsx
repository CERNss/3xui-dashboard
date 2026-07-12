import { Select } from 'antd'
import { useTranslation } from 'react-i18next'

// Shared with views/admin/inbounds/utils.ts (kept literal here so the
// common component doesn't import from a view directory).
export type ProvenanceScope = 'managed' | 'all' | 'external'

interface ProvenanceScopeSelectProps {
  value: ProvenanceScope
  onChange: (value: ProvenanceScope) => void
}

/**
 * The ownership-scope selector used identically on the admin Inbounds
 * and Clients pages: managed (dashboard-created, the default view) /
 * all / external-only.
 */
export function ProvenanceScopeSelect({ value, onChange }: ProvenanceScopeSelectProps) {
  const { t } = useTranslation()
  return (
    <Select<ProvenanceScope>
      aria-label={t('admin.provenance.scopeLabel')}
      style={{ width: 150 }}
      value={value}
      onChange={onChange}
      options={[
        { label: t('admin.provenance.scopeManaged'), value: 'managed' },
        { label: t('admin.provenance.scopeAll'), value: 'all' },
        { label: t('admin.provenance.scopeExternal'), value: 'external' },
      ]}
    />
  )
}
