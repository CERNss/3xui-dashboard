import { adminClient } from '../client/admin'

export type WebhookMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
export type WebhookFormat = 'json' | 'form' | 'text' | 'raw'

export interface Webhook {
  id: number
  name: string
  url: string
  events: string[]
  enabled: boolean
  allow_private: boolean
  method: WebhookMethod
  headers: Record<string, string>
  body_template: string
  template_format: WebhookFormat
  created_at: string
  updated_at: string
}

export interface WebhookInput {
  name: string
  url: string
  events: string[]
  enabled: boolean
  allow_private: boolean
  method: WebhookMethod
  headers: Record<string, string>
  body_template: string
  template_format: WebhookFormat
  secret?: string
}

export interface WebhookDelivery {
  id: number
  webhook_id: number
  event_type: string
  status: string
  http_status: number
  response_body?: string
  attempt: number
  scheduled_at: string
  next_attempt_at: string
  delivered_at?: string | null
  error?: string
}

export const adminWebhooksApi = {
  list: () =>
    adminClient.get<{ webhooks: Webhook[] }>('/webhooks').then((r) => r.data.webhooks),

  get: (id: number) => adminClient.get<Webhook>(`/webhooks/${id}`).then((r) => r.data),

  create: (input: WebhookInput) =>
    adminClient.post<Webhook>('/webhooks', input).then((r) => r.data),

  update: (id: number, patch: Partial<WebhookInput>) =>
    adminClient.put<Webhook>(`/webhooks/${id}`, patch).then((r) => r.data),

  remove: (id: number) => adminClient.delete(`/webhooks/${id}`).then(() => {}),

  test: (id: number) =>
    adminClient.post<WebhookDelivery>(`/webhooks/${id}/test`).then((r) => r.data),

  deliveries: (id: number) =>
    adminClient
      .get<{ deliveries: WebhookDelivery[] }>(`/webhooks/${id}/deliveries`)
      .then((r) => r.data.deliveries),

  // Replay queues a fresh delivery for the same (event_type, payload)
  // as the original. The historical row stays as audit trail; the new
  // delivery has its own id and retry cycle.
  replay: (deliveryID: number) =>
    adminClient
      .post<WebhookDelivery>(`/webhooks/deliveries/${deliveryID}/replay`)
      .then((r) => r.data),
}
