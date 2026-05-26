import { Segmented } from 'antd'
import type { SegmentedProps } from 'antd'
import { useAppStore } from '@/stores/app'

export type LocaleValue = 'en-US' | 'zh-CN'

export interface LocaleSwitcherProps {
  options?: SegmentedProps<LocaleValue>['options']
}

interface LocaleStoreState {
  locale: LocaleValue
  setLocale: (locale: LocaleValue) => void
}

const defaultOptions: SegmentedProps<LocaleValue>['options'] = [
  { label: 'EN', value: 'en-US' },
  { label: '中文', value: 'zh-CN' }
]

export function LocaleSwitcher({ options = defaultOptions }: LocaleSwitcherProps) {
  const locale = useAppStore((state: LocaleStoreState) => state.locale)
  const setLocale = useAppStore((state: LocaleStoreState) => state.setLocale)

  return <Segmented<LocaleValue> size="small" options={options} value={locale} onChange={setLocale} />
}
