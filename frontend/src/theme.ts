import type { ThemeConfig } from 'antd'
import { theme } from 'antd'

export const MD_BREAKPOINT = 768
export const LG_BREAKPOINT = 1024

const sharedTokens: ThemeConfig['token'] = {
  fontFamily:
    'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
  fontFamilyCode: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
  borderRadius: 7,
  borderRadiusSM: 5,
  borderRadiusLG: 11,
  wireframe: false
}

export const lightTheme: ThemeConfig = {
  cssVar: true,
  hashed: true,
  token: {
    ...sharedTokens,
    colorPrimary: '#0284c7',
    colorInfo: '#2563eb',
    colorLink: '#0284c7',
    colorSuccess: '#16a34a',
    colorWarning: '#d97706',
    colorError: '#dc2626',
    colorTextBase: '#0f172a',
    colorTextSecondary: '#52607a',
    colorTextTertiary: '#94a0b8',
    colorBgBase: '#eef1f7',
    colorBgLayout: 'transparent',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorFillAlter: '#f3f6fb',
    colorBorder: 'rgba(15, 23, 42, 0.16)',
    colorBorderSecondary: 'rgba(15, 23, 42, 0.09)',
    controlItemBgActive: 'rgba(2, 132, 199, 0.1)',
    controlOutline: 'rgba(2, 132, 199, 0.18)'
  },
  components: {
    Card: {
      borderRadiusLG: 16,
      colorBorderSecondary: 'rgba(15, 23, 42, 0.09)',
      boxShadowTertiary: '0 1px 2px rgba(15, 23, 42, 0.06), 0 6px 20px rgba(15, 23, 42, 0.06)'
    },
    Button: {
      primaryShadow: '0 0 0 3px rgba(2, 132, 199, 0.14)'
    },
    Input: {
      activeBorderColor: 'rgba(2, 132, 199, 0.4)',
      activeShadow: '0 0 0 3px rgba(2, 132, 199, 0.1)',
      colorBgContainer: '#f3f6fb',
      hoverBorderColor: 'rgba(2, 132, 199, 0.32)'
    },
    Select: {
      colorBgContainer: '#f3f6fb'
    },
    InputNumber: {
      colorBgContainer: '#f3f6fb'
    },
    Layout: {
      bodyBg: 'transparent',
      headerBg: '#ffffff',
      lightSiderBg: '#ffffff',
      siderBg: '#ffffff'
    },
    Menu: {
      itemSelectedBg: 'rgba(2, 132, 199, 0.1)',
      itemSelectedColor: '#0284c7'
    },
    Segmented: {
      itemSelectedBg: '#ffffff',
      trackBg: '#e7edf6'
    },
    Table: {
      borderColor: 'rgba(15, 23, 42, 0.09)',
      headerBg: '#f3f6fb',
      headerColor: '#94a0b8',
      rowHoverBg: '#f3f6fb',
      headerBorderRadius: 0
    },
    Tabs: {
      inkBarColor: '#0284c7',
      itemActiveColor: '#0369a1',
      itemHoverColor: '#0284c7',
      itemSelectedColor: '#0284c7'
    },
    Modal: {
      borderRadiusLG: 16
    }
  }
}

export const darkTheme: ThemeConfig = {
  cssVar: true,
  hashed: true,
  algorithm: theme.darkAlgorithm,
  token: {
    ...sharedTokens,
    colorPrimary: '#38bdf8',
    colorInfo: '#60a5fa',
    colorLink: '#38bdf8',
    colorSuccess: '#4ade80',
    colorWarning: '#fbbf24',
    colorError: '#f87171',
    colorTextBase: '#e8edf5',
    colorTextSecondary: '#828da6',
    colorTextTertiary: '#525d78',
    colorBgBase: '#080b12',
    colorBgLayout: 'transparent',
    colorBgContainer: '#121829',
    colorBgElevated: '#1a2236',
    colorFillAlter: '#1a2236',
    colorBorder: 'rgba(255, 255, 255, 0.13)',
    colorBorderSecondary: 'rgba(255, 255, 255, 0.07)',
    controlItemBgActive: 'rgba(56, 189, 248, 0.13)',
    controlOutline: 'rgba(56, 189, 248, 0.24)'
  },
  components: {
    Card: {
      borderRadiusLG: 16,
      colorBgContainer: '#121829',
      colorBorderSecondary: 'rgba(255, 255, 255, 0.07)',
      boxShadowTertiary: '0 1px 3px rgba(0, 0, 0, 0.4), 0 8px 24px rgba(0, 0, 0, 0.3)'
    },
    Button: {
      primaryShadow: '0 0 0 3px rgba(56, 189, 248, 0.2)',
      colorTextLightSolid: '#04121f'
    },
    Input: {
      activeBorderColor: 'rgba(56, 189, 248, 0.45)',
      activeShadow: '0 0 0 3px rgba(56, 189, 248, 0.13)',
      colorBgContainer: '#1a2236',
      hoverBorderColor: 'rgba(56, 189, 248, 0.35)'
    },
    Select: {
      colorBgContainer: '#1a2236'
    },
    InputNumber: {
      colorBgContainer: '#1a2236'
    },
    Layout: {
      bodyBg: 'transparent',
      headerBg: '#0e1320',
      lightSiderBg: '#0e1320',
      siderBg: '#0e1320'
    },
    Menu: {
      darkItemBg: '#0e1320',
      itemSelectedBg: 'rgba(56, 189, 248, 0.13)',
      itemSelectedColor: '#38bdf8'
    },
    Segmented: {
      itemSelectedBg: '#222c45',
      trackBg: '#121829'
    },
    Table: {
      borderColor: 'rgba(255, 255, 255, 0.07)',
      headerBg: '#1a2236',
      headerColor: '#525d78',
      rowHoverBg: '#1a2236',
      headerBorderRadius: 0
    },
    Tabs: {
      inkBarColor: '#38bdf8',
      itemActiveColor: '#7dd3fc',
      itemHoverColor: '#7dd3fc',
      itemSelectedColor: '#38bdf8'
    },
    Modal: {
      borderRadiusLG: 16,
      contentBg: '#121829',
      headerBg: '#121829'
    }
  }
}
