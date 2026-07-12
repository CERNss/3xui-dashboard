import { GlobalOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons'
import { Typography } from 'antd'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet } from 'react-router-dom'
import { LocaleSwitcher } from '@/components/common'
import { useBranding } from '@/hooks/queries/branding'
import { useThemeStore } from '@/stores/theme'

export interface AuthLayoutProps {
  children?: ReactNode
  cardTitle?: ReactNode
  cardSubtitle?: ReactNode
}

/* Decorative "flowing water" backdrop: blurred drifting blobs, three
 * scrolling wave lines, rising particles, a faint grid and a vignette
 * that pools the light behind the login card. Pure CSS animation —
 * every element is aria-hidden. */
function AuthBackdrop() {
  return (
    <>
      <div aria-hidden="true" className="auth-bg-grid" />
      <div aria-hidden="true" className="auth-bg-flow">
        <span className="auth-blob auth-blob-1" />
        <span className="auth-blob auth-blob-2" />
        <span className="auth-blob auth-blob-3" />
        <span className="auth-blob auth-blob-4" />
        <span className="auth-blob auth-blob-5" />
        <div className="auth-waves">
          <svg preserveAspectRatio="none" viewBox="0 0 2880 600" xmlns="http://www.w3.org/2000/svg">
            <path
              className="auth-wave auth-wave-1"
              d="M0,300 C240,240 480,360 720,300 C960,240 1200,360 1440,300 C1680,240 1920,360 2160,300 C2400,240 2640,360 2880,300"
              fill="none"
            />
            <path
              className="auth-wave auth-wave-2"
              d="M0,340 C240,400 480,280 720,340 C960,400 1200,280 1440,340 C1680,400 1920,280 2160,340 C2400,400 2640,280 2880,340"
              fill="none"
            />
            <path
              className="auth-wave auth-wave-3"
              d="M0,380 C240,320 480,440 720,380 C960,320 1200,440 1440,380 C1680,320 1920,440 2160,380 C2400,320 2640,440 2880,380"
              fill="none"
            />
          </svg>
        </div>
      </div>
      <div aria-hidden="true" className="auth-particles">
        <i />
        <i />
        <i />
        <i />
        <i />
        <i />
        <i />
        <i />
      </div>
      <div aria-hidden="true" className="auth-vignette" />
    </>
  )
}

export function AuthLayout({ cardSubtitle, cardTitle, children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { data: branding } = useBranding()
  const themeMode = useThemeStore((state) => state.resolvedTheme)
  const toggleTheme = useThemeStore((state) => state.toggle)
  const title = branding?.title ?? t('app.title')
  const subtitle = branding?.description ?? branding?.subtitle ?? t('brand.slogan')
  const footer = branding?.footer ?? t('brand.footer')
  const nextThemeLabel = themeMode === 'dark' ? t('theme.light') : t('theme.dark')
  const toggleThemeLabel = themeMode === 'dark' ? t('theme.toggleLight') : t('theme.toggleDark')

  return (
    <main className="auth-layout" data-testid="auth-layout">
      <AuthBackdrop />
      <div className="auth-locale">
        <LocaleSwitcher />
      </div>
      <div className="auth-theme">
        <button aria-label={toggleThemeLabel} className="auth-theme-button" onClick={toggleTheme} type="button">
          <span aria-hidden="true" className="auth-theme-icon">
            {themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
          </span>
          <span>{nextThemeLabel}</span>
        </button>
      </div>
      <div className="auth-layout-inner">
        <div className="auth-brand">
          <span aria-hidden="true" className="auth-brand-mark">
            <span className="auth-brand-ring" />
            {branding?.icon_url ? (
              <img alt="" className="auth-brand-icon" src={branding.icon_url} />
            ) : (
              <GlobalOutlined />
            )}
          </span>
          <Typography.Title className="auth-brand-title" level={2}>
            {title}
          </Typography.Title>
          {subtitle ? <Typography.Text className="auth-brand-subtitle">{subtitle}</Typography.Text> : null}
        </div>
        {cardTitle || cardSubtitle ? (
          <div className="auth-login-card">
            <div className="auth-login-heading">
              {cardTitle ? <Typography.Title level={3}>{cardTitle}</Typography.Title> : null}
              {cardSubtitle ? <Typography.Text type="secondary">{cardSubtitle}</Typography.Text> : null}
            </div>
            {children ?? <Outlet />}
          </div>
        ) : (
          children ?? <Outlet />
        )}
        {footer ? (
          <Typography.Text className="auth-footer" type="secondary">
            {footer}
          </Typography.Text>
        ) : null}
      </div>
    </main>
  )
}
