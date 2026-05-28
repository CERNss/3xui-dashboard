import { adminClient } from '../client/admin'
import type { InboundTemplate } from './inboundTemplates'

export interface ProvisioningPoolTarget {
  id: number
  pool_id: number
  template_id?: number | null
  node_id: number
  node_name?: string
  inbound_tag: string
  protocol: string
  max_clients: number
  used_clients?: number
  priority: number
  enabled: boolean
  generated?: boolean
  template_name?: string
  created_at?: string
  updated_at?: string
}

export interface ProvisioningPool {
  id: number
  name: string
  description?: string
  enabled: boolean
  auto_create: boolean
  template_id?: number | null
  template?: InboundTemplate | null
  port_min?: number | null
  port_max?: number | null
  max_clients?: number
  allowed_protocols: string[]
  node_ids?: number[]
  targets?: ProvisioningPoolTarget[]
  created_at?: string
  updated_at?: string
}

export interface ProvisioningPoolInput {
  name: string
  description?: string
  enabled: boolean
  auto_create: boolean
  template_id?: number | null
  port_min?: number | null
  port_max?: number | null
  max_clients: number
  allowed_protocols: string[]
  node_ids: number[]
}

export interface ProvisioningPoolTargetInput {
  node_id: number
  inbound_tag: string
  protocol?: string
  max_clients: number
  priority: number
  enabled: boolean
}

export const provisioningPoolsApi = {
  list: () =>
    adminClient.get<{ pools: ProvisioningPool[] }>('/provisioning-pools').then((r) => r.data.pools),

  create: (input: ProvisioningPoolInput) =>
    adminClient.post<ProvisioningPool>('/provisioning-pools', input).then((r) => r.data),

  update: (id: number, input: Partial<ProvisioningPoolInput>) =>
    adminClient.put<ProvisioningPool>(`/provisioning-pools/${id}`, input).then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/provisioning-pools/${id}`),

  addTarget: (poolID: number, input: ProvisioningPoolTargetInput) =>
    adminClient.post<ProvisioningPoolTarget>(`/provisioning-pools/${poolID}/targets`, input).then((r) => r.data),

  updateTarget: (targetID: number, input: Partial<ProvisioningPoolTargetInput>) =>
    adminClient.put<void>(`/provisioning-pools/targets/${targetID}`, input).then((r) => r.data),

  removeTarget: (targetID: number) =>
    adminClient.delete<void>(`/provisioning-pools/targets/${targetID}`).then((r) => r.data),
}
