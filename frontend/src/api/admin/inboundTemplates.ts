import { adminClient } from '../client/admin'

export interface InboundTemplate {
  id: number
  name: string
  description: string
  enabled: boolean
  protocol: string
  remark: string
  listen: string
  total: number
  expiryTime: number
  trafficReset: string
  settings: string
  streamSettings: string
  sniffing: string
  created_at?: string
  updated_at?: string
}

export interface InboundTemplateInput {
  name: string
  description: string
  enabled: boolean
  protocol: string
  remark: string
  listen: string
  total: number
  expiryTime: number
  trafficReset: string
  settings: string
  streamSettings: string
  sniffing: string
}

export const inboundTemplatesApi = {
  list: () =>
    adminClient
      .get<{ templates: InboundTemplate[] }>('/inbound-templates')
      .then((r) => r.data.templates),

  create: (input: InboundTemplateInput) =>
    adminClient.post<InboundTemplate>('/inbound-templates', input).then((r) => r.data),

  update: (id: number, input: Partial<InboundTemplateInput>) =>
    adminClient.put<InboundTemplate>(`/inbound-templates/${id}`, input).then((r) => r.data),

  remove: (id: number) => adminClient.delete<void>(`/inbound-templates/${id}`).then((r) => r.data),
}
