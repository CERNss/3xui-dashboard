import { defineStore } from 'pinia'
import { brandingApi } from '@/api/branding'

interface State {
  iconUrl: string
  loaded: boolean
}

export const useBrandingStore = defineStore('branding', {
  state: (): State => ({
    iconUrl: '',
    loaded: false,
  }),
  actions: {
    async load(force = false) {
      if (this.loaded && !force) return
      const data = await brandingApi.get()
      this.iconUrl = data.icon_url || ''
      this.loaded = true
    },
    setIcon(url: string) {
      this.iconUrl = url
      this.loaded = true
    },
  },
})
