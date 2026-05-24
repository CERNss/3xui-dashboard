import { defineStore } from 'pinia'
import { brandingApi, type Branding } from '@/api/branding'

interface State {
  iconUrl: string
  title: string
  subtitle: string
  description: string
  footer: string
  loaded: boolean
}

const defaults = {
  title: '3xui Central',
  subtitle: 'central panel',
  description: 'Multi-node 3x-ui · Fleet aggregation · Traffic accounting · Subscription export',
  footer: '© 2026 3xui Central · Self-hosted multi-node control panel',
}

function resolveIconUrl(raw: string): string {
  if (!raw) return ''
  if (/^https?:\/\//i.test(raw)) return raw
  if (raw.startsWith('/')) return raw
  return `/${raw}`
}

export const useBrandingStore = defineStore('branding', {
  state: (): State => ({
    iconUrl: '',
    title: defaults.title,
    subtitle: defaults.subtitle,
    description: defaults.description,
    footer: defaults.footer,
    loaded: false,
  }),
  actions: {
    async load(force = false) {
      if (this.loaded && !force) return
      const data = await brandingApi.get()
      this.apply(data)
    },
    apply(data: Partial<Branding>) {
      this.iconUrl = Object.prototype.hasOwnProperty.call(data, 'icon_url')
        ? resolveIconUrl(data.icon_url || '')
        : this.iconUrl
      this.title = data.title?.trim() || defaults.title
      this.subtitle = data.subtitle?.trim() || defaults.subtitle
      this.description = data.description?.trim() || defaults.description
      this.footer = data.footer?.trim() || defaults.footer
      this.loaded = true
    },
    setIcon(url: string) {
      this.iconUrl = resolveIconUrl(url)
      this.loaded = true
    },
    setInfo(info: Partial<Pick<State, 'title' | 'subtitle' | 'description' | 'footer'>>) {
      this.title = info.title?.trim() || defaults.title
      this.subtitle = info.subtitle?.trim() || defaults.subtitle
      this.description = info.description?.trim() || defaults.description
      this.footer = info.footer?.trim() || defaults.footer
      this.loaded = true
    },
    clearIcon() {
      this.iconUrl = ''
      this.loaded = true
    },
  },
})
