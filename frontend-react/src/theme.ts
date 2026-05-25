import type { ThemeConfig } from 'antd'
import { theme } from 'antd'

export const MD_BREAKPOINT = 768
export const LG_BREAKPOINT = 1024

const sharedTokens: ThemeConfig['token'] = {
  colorPrimary: '#4f46e5',
  colorSuccess: '#10b981',
  colorInfo: '#6366f1',
  colorLink: '#4f46e5',
  colorTextBase: '#0c0e12',
  colorBgBase: '#ffffff',
  fontFamily:
    'Geist, "DM Sans", Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  fontFamilyCode:
    '"Geist Mono", "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
  borderRadius: 8,
  wireframe: false
}

export const lightTheme: ThemeConfig = {
  cssVar: true,
  hashed: true,
  token: {
    ...sharedTokens,
    colorBgLayout: '#f7f8f9',
    colorBgContainer: '#ffffff',
    colorBorder: '#d6dadf'
  },
  components: {
    Button: {
      primaryShadow: '0 0 0 3px rgb(79 70 229 / 0.16)'
    }
  }
}

export const darkTheme: ThemeConfig = {
  cssVar: true,
  hashed: true,
  algorithm: theme.darkAlgorithm,
  token: {
    ...sharedTokens,
    colorTextBase: '#f7f8f9',
    colorBgBase: '#15181d',
    colorBgLayout: '#1e2e46',
    colorBgContainer: '#1b2b42',
    colorBorder: '#373f4a'
  },
  components: {
    Button: {
      primaryShadow: '0 0 0 3px rgb(79 70 229 / 0.24)'
    }
  }
}
