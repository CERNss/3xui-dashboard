import type { Webhook, WebhookInput } from '@/api/admin/webhooks'

export function headersToText(headers: Record<string, string>) {
  return Object.entries(headers)
    .map(([key, value]) => `${key}: ${value}`)
    .join('\n')
}

export function textToHeaders(text: string) {
  const headers: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const index = line.indexOf(':')
    if (index <= 0) continue
    const key = line.slice(0, index).trim()
    const value = line.slice(index + 1).trim()
    if (key) headers[key] = value
  }
  return headers
}

export function eventsToText(events: string[]) {
  return events.join(', ')
}

export function textToEvents(text: string) {
  return text
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
}

export function blankWebhookInput(): WebhookInput {
  return {
    name: '',
    url: '',
    events: ['*'],
    enabled: true,
    allow_private: false,
    method: 'POST',
    headers: {},
    body_template: '',
    template_format: 'json',
  }
}

export function webhookToInput(webhook: Webhook): WebhookInput {
  return {
    name: webhook.name,
    url: webhook.url,
    events: webhook.events,
    enabled: webhook.enabled,
    allow_private: webhook.allow_private,
    method: webhook.method,
    headers: { ...webhook.headers },
    body_template: webhook.body_template,
    template_format: webhook.template_format,
  }
}
