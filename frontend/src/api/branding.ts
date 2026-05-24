import axios from 'axios'

export interface Branding {
  icon_url: string
  title: string
  subtitle: string
  description: string
  footer: string
}

export const brandingApi = {
  get: () =>
    axios.get<Branding>('/api/public/branding').then((r) => r.data),
}
