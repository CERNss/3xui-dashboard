# add-notification-channels

## Why

通知系统 sits at **50%** per the ROADMAP: the event bus is in place,
the notify service routes `client.expired` / `expiring_soon` /
`over_limit` to email, and the webhook service handles arbitrary
user-defined HTTP endpoints. What's missing is the **operator's
own** alerting surface:

- Ops watching node fleet health needs `node.offline` /
  `node.probe_failed` pings to a Telegram group or Discord channel
- Billing alerts (`order.payment_failed`, big revenue spikes) want
  to land in the team's IM, not a mailbox
- The notify service today is hardcoded to email; adding a second
  channel today means forking the dispatch logic

#7 generalizes the notify service into a **Channel** abstraction +
ships three concrete adapters: **Telegram**, **Discord**, **Feishu**
(also Lark). Email keeps working — it becomes one channel among
four, sharing the same `Channel` interface so a fifth (Slack, Mattermost,
WeChat Work) is just one new file.

References:
- Telegram Bot API: <https://core.telegram.org/bots/api#sendmessage>
- Discord webhooks: <https://discord.com/developers/docs/resources/webhook>
- Feishu (Lark) webhooks: <https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot>

## What Changes

### Modified capability: `notify-service`

The existing per-event handler that calls mailer directly becomes a
fanout to a list of `Channel`s. Each channel decides whether it
wants the event (via configurable routing rules — `node.*` to
ops channel, `client.*` to email, etc.) and renders the event
into its own native message shape.

- **`internal/service/notify/channel.go`** — `Channel` interface:
  `Name() string; Send(ctx, msg) error`.
- **`internal/service/notify/router.go`** — config-driven rules:
  one rule per (event-type-pattern → channel-name list). Loaded
  from settings at boot, hot-reload not in scope.
- **`internal/service/notify/message.go`** — `Message` struct
  carries title + body + level + metadata + raw event data; each
  channel formats it into its own wire shape.
- **`internal/service/notify/service.go`** — existing single-email
  dispatch refactored: instead of `mailer.Send(...)`, walk the
  channels matching the event, call `chan.Send(ctx, msg)` on each.

### New capability: `notification-channels`

Three new adapters in `internal/service/notify/channels/`:

- **`telegram.go`** — Bot API `sendMessage` to a configured `chat_id`
  with HTML parse_mode (Telegram supports a tiny safe subset:
  `<b>`, `<i>`, `<code>`, `<a>`). Bot token via env.
- **`discord.go`** — Webhook POST with `{content, embeds[]}`.
  Webhook URL via env. Uses Discord rich embeds for color-coded
  severity (red = error, amber = warn, green = ok).
- **`feishu.go`** — Custom-bot webhook POST. Supports `msg_type=text`
  (simple) and `msg_type=interactive` (cards). We use cards for ops
  alerts — they render with title, body, and a button linking back
  to the panel.

Email remains in `internal/mailer/` but gets a thin Channel adapter
wrapper so the dispatch loop treats it uniformly.

### Modified capability: `event-bus`

Adds 2 missing constants the new channels listen to:

- `NodeRecovered` — emitted when a previously-offline node returns.
  Today the probe job re-publishes `NodeOnline` for every healthy
  tick; we add `NodeRecovered` as the first online-after-offline
  transition so subscribers don't get spammed.
- Already-published `OrderPaymentFailed` / `OrderPaymentExpired`
  from #5 get their first subscribers.

### Migration: none

Channel configuration lives in env vars (same pattern as alipay /
stripe / smtp). Per-channel preferences could later move into the
`settings` table for runtime tuning, but that's a follow-up change.

## Out of scope

- **Per-user routing.** Today email is per-user (lookup user → send
  to user.email). Telegram/Discord/Feishu in v1 send to **admin-
  configured** chat/webhook targets — the alerts are for the
  operator, not the customer. Per-user telegram_chat_id is a separate
  change (`add-user-notification-prefs`).
- **Interactive bot commands.** Telegram bot won't respond to
  `/balance` or `/status` — only outbound notifications. Inbound
  bot API is a separate change.
- **Templates per-deployment.** Each channel adapter ships fixed
  message templates. Per-event-type templates editable from admin
  UI are deferred.
- **Slack / Mattermost / WeChat Work.** The channel interface is
  designed to accept them; we ship 3 to prove the abstraction.

## Assumptions called out

- Operators register a bot or webhook with each platform:
  - Telegram: chat with `@BotFather`, get token; add bot to target
    chat; get chat ID via `getUpdates` once
  - Discord: server settings → integrations → webhooks → copy URL
  - Feishu: 群设置 → 群机器人 → 添加机器人 → 自定义机器人 → copy URL
- Outbound HTTP from the dashboard reaches the IM providers. We
  rely on `internal/netsafe` for SSRF guardrails on user-supplied
  URLs; the channel webhook URLs come from env vars so SSRF isn't
  a concern there.
- Channel deliveries are best-effort with one retry. If both fail,
  the event is logged at warn level and the operator can grep the
  log. Persistent delivery queue is a follow-up.
