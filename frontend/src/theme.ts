import type { ThemeConfig } from 'antd'
import { theme } from 'antd'

export const MD_BREAKPOINT = 768
export const LG_BREAKPOINT = 1024

const sharedTokens: ThemeConfig['token'] = {
  colorSuccess: '#10b981',
  colorTextBase: '#0c0e12',
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
    colorPrimary: '#2563eb',
    colorInfo: '#0f6ecf',
    colorLink: '#1d4ed8',
    colorTextBase: '#1f2937',
    colorBgBase: '#f8fafc',
    colorBgLayout: '#f4f7fb',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorFillAlter: '#f1f5f9',
    colorBorder: '#d8e1ec',
    colorBorderSecondary: '#e5ebf2',
    controlItemBgActive: '#e8f1ff',
    controlOutline: 'rgba(37, 99, 235, 0.18)'
  },
  components: {
    Card: {
      colorBorderSecondary: '#e5ebf2'
    },
    Button: {
      primaryShadow: '0 0 0 3px rgb(37 99 235 / 0.16)'
    },
    Input: {
      activeBorderColor: '#2563eb',
      activeShadow: '0 0 0 3px rgb(37 99 235 / 0.12)',
      colorBgContainer: '#ffffff',
      hoverBorderColor: '#93b4f5'
    },
    Layout: {
      bodyBg: '#f4f7fb',
      headerBg: '#ffffff',
      lightSiderBg: '#ffffff',
      siderBg: '#ffffff'
    },
    Menu: {
      itemSelectedBg: '#e8f1ff',
      itemSelectedColor: '#1d4ed8'
    },
    Segmented: {
      itemSelectedBg: '#ffffff',
      trackBg: '#eef3f8'
    },
    Table: {
      borderColor: '#e2e8f0',
      headerBg: '#f2f6fa',
      headerColor: '#334155',
      rowHoverBg: '#f8fbff'
    },
    Tabs: {
      inkBarColor: '#2563eb',
      itemActiveColor: '#1d4ed8',
      itemHoverColor: '#2563eb',
      itemSelectedColor: '#1d4ed8'
    }
  }
}

export const darkTheme: ThemeConfig = {
  cssVar: true,
  hashed: true,
  algorithm: theme.darkAlgorithm,
  token: {
    ...sharedTokens,
    colorPrimary: '#60a5fa',
    colorInfo: '#38bdf8',
    colorLink: '#93c5fd',
    colorTextBase: '#f3f8fb',
    colorBgBase: '#070b19',
    colorBgLayout: '#080d1d',
    colorBgContainer: '#172234',
    colorBgElevated: '#1b2a3d',
    colorFillAlter: '#101a2c',
    colorBorder: '#314158',
    colorBorderSecondary: '#24344a',
    controlItemBgActive: 'rgba(96, 165, 250, 0.16)',
    controlOutline: 'rgba(96, 165, 250, 0.24)'
  },
  components: {
    Card: {
      colorBgContainer: '#172234',
      colorBorderSecondary: '#314158'
    },
    Button: {
      primaryShadow: '0 0 0 3px rgb(96 165 250 / 0.22)'
    },
    Input: {
      activeBorderColor: '#60a5fa',
      activeShadow: '0 0 0 3px rgb(96 165 250 / 0.16)',
      colorBgContainer: '#172234',
      hoverBorderColor: '#93c5fd'
    },
    Layout: {
      bodyBg: '#080d1d',
      headerBg: '#111b2c',
      lightSiderBg: '#111b2c',
      siderBg: '#111b2c'
    },
    Menu: {
      darkItemBg: '#111b2c',
      itemSelectedBg: 'rgba(96, 165, 250, 0.16)',
      itemSelectedColor: '#93c5fd'
    },
    Segmented: {
      itemSelectedBg: '#172234',
      trackBg: '#111b2c'
    },
    Tabs: {
      inkBarColor: '#60a5fa',
      itemActiveColor: '#93c5fd',
      itemHoverColor: '#bfdbfe',
      itemSelectedColor: '#93c5fd'
    }
  }
}
