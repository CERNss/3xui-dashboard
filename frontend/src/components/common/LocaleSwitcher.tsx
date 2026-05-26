import { DownOutlined, GlobalOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { useAppStore } from '@/stores/app'

export type LocaleValue = 'en-US' | 'zh-CN'

export interface LocaleOption {
  labelKey: 'language.english' | 'language.chinese'
  shortLabel: string
  value: LocaleValue
}

export interface LocaleSwitcherProps {
  className?: string
  options?: LocaleOption[]
  variant?: 'switch' | 'chip'
}

interface LocaleStoreState {
  locale: LocaleValue
  setLocale: (locale: LocaleValue) => void
}

const defaultOptions: LocaleOption[] = [
  { labelKey: 'language.english', shortLabel: 'EN', value: 'en-US' },
  { labelKey: 'language.chinese', shortLabel: '中', value: 'zh-CN' }
]

export function LocaleSwitcher({ className, options = defaultOptions, variant = 'switch' }: LocaleSwitcherProps) {
  const { i18n, t } = useTranslation()
  const locale = useAppStore((state: LocaleStoreState) => state.locale)
  const setLocale = useAppStore((state: LocaleStoreState) => state.setLocale)
  const current = options.find((option) => option.value === locale) ?? options[0]
  const currentLabel = t(current.labelKey)
  const next = options.find((option) => option.value !== locale) ?? current
  const changeLocale = (nextLocale: LocaleValue) => {
    if (nextLocale === locale) return
    setLocale(nextLocale)
    void i18n.changeLanguage(nextLocale === 'zh-CN' ? 'zh' : 'en')
  }
  const classes = ['locale-switcher-button', variant === 'chip' ? 'locale-switcher-button--chip' : '', className]
    .filter(Boolean)
    .join(' ')

  return (
    <button
      aria-checked={locale === 'zh-CN'}
      aria-label={`${t('language.label')}: ${currentLabel}`}
      className={classes}
      data-locale={locale}
      onClick={() => changeLocale(next.value)}
      role="switch"
      type="button"
    >
      {variant === 'chip' ? (
        <>
          <span aria-hidden="true" className="locale-switcher-flag" />
          <span className="locale-switcher-current">{current.shortLabel}</span>
          <DownOutlined aria-hidden="true" className="locale-switcher-chevron" />
        </>
      ) : (
        <>
          <GlobalOutlined className="locale-switcher-icon" />
          <span aria-hidden="true" className="locale-switcher-track">
            <span className="locale-switcher-thumb" />
            {options.map((option) => (
              <span className="locale-switcher-option" key={option.value}>
                {option.shortLabel}
              </span>
            ))}
          </span>
        </>
      )}
    </button>
  )
}
