import { portalClient } from '../client/portal'

export interface ClientUsage {
  node_id: number
  inbound_tag: string
  client_email: string
  up: number
  down: number
  total: number
  limit?: number | null
  expires_at?: string | null
  last_sample_at?: string | null
}

export const portalTrafficApi = {
  own: () =>
    portalClient.get<{ clients: ClientUsage[] }>('/traffic').then((r) => r.data.clients),
}
