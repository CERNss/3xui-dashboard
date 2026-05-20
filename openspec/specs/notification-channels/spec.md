# notification-channels

Per-platform adapters that the `notify-service` fans events out to.
Each adapter implements `notify.Channel` and renders the same
gateway-agnostic `Message` into its native wire shape. Ships three
in-tree implementations (Telegram, Discord, Feishu) plus an Email
adapter that wraps the existing `mailer`.

## Purpose & boundaries

- **Owns**: per-platform wire formats, severity-derived styling
  (emoji / RGB / card template), per-channel retry policy.
- **Does NOT own**: event routing (lives in `notify-service`'s
  Router), dedup gate (lives in the `notification_log` table
  managed by `notify-service`).

## Requirements

### Requirement: Channel Interface

All notification destinations SHALL implement a common `Channel`
interface so the notify service treats them uniformly:

```go
type Channel interface {
    Name() string
    Enabled() bool
    Send(ctx context.Context, msg Message) error
}
```

#### Scenario: Disabled channel skipped

- **WHEN** the notify service iterates the routed channels
- **AND** a channel's `Enabled()` returns false (e.g. its required env vars are unset)
- **THEN** the notify service SHALL skip that channel without error

### Requirement: Telegram Bot Channel

The system SHALL deliver notifications to a Telegram chat via the
Bot API. The channel SHALL be opt-in via `TELEGRAM_BOT_TOKEN` +
`TELEGRAM_CHAT_ID` env vars.

#### Scenario: Successful Telegram delivery

- **WHEN** `channel.Send(msg)` is called on a configured Telegram channel
- **THEN** the channel SHALL POST to `https://api.telegram.org/bot${TOKEN}/sendMessage`
- **AND** SHALL pass `{chat_id: ${CHAT_ID}, text: ..., parse_mode: "HTML", disable_web_page_preview: true}`
- **AND** SHALL prefix the title with a level emoji (🟢 info / 🟡 warn / 🔴 error)
- **AND** SHALL HTML-escape user-supplied content so `<script>` payloads can't be injected through field values

#### Scenario: Telegram API error surfaced

- **WHEN** Telegram responds with `{ok: false, description: "Bad Request: chat not found"}`
- **THEN** `Send` SHALL return an error wrapping the description

### Requirement: Discord Webhook Channel

The system SHALL deliver notifications to a Discord channel via a
Discord webhook URL. The channel SHALL be opt-in via
`DISCORD_WEBHOOK_URL` env var.

#### Scenario: Successful Discord delivery

- **WHEN** `channel.Send(msg)` is called on a configured Discord channel
- **THEN** the channel SHALL POST to the webhook URL with `{embeds: [{title, description, color, fields, url}]}`
- **AND** SHALL set `color` to an RGB int derived from the Message Level (red 0xE74C3C for error, amber 0xF1C40F for warn, green 0x2ECC71 for info)
- **AND** SHALL treat HTTP 204 as success (Discord's documented success status)

### Requirement: Feishu (Lark) Webhook Channel

The system SHALL deliver notifications to a Feishu/Lark group via
a custom-bot webhook URL. The channel SHALL be opt-in via
`FEISHU_WEBHOOK_URL` env var.

#### Scenario: Successful Feishu delivery

- **WHEN** `channel.Send(msg)` is called on a configured Feishu channel
- **THEN** the channel SHALL POST a `{msg_type: "interactive", card: {...}}` payload
- **AND** the card's `header.template` SHALL be `"green"` / `"yellow"` / `"red"` per Level
- **AND** fields SHALL render as a two-column field grid below the body
- **AND** a non-empty `URL` SHALL produce a "查看详情" primary button at the bottom

#### Scenario: Feishu API error surfaced

- **WHEN** Feishu responds with `{code: 9499, msg: "Bad Request"}`
- **THEN** `Send` SHALL return an error wrapping the code + msg

### Requirement: Email Channel

The `email` channel SHALL wrap the existing `mailer.Mailer` so the
notify service can treat email as one channel among many. Email
delivery SHALL prefer `Message.Recipient` (set by per-user
lifecycle dispatch); when empty, SHALL fall back to
`NOTIFY_OPS_RECIPIENT`.

#### Scenario: Per-user recipient honored

- **WHEN** a client-lifecycle event dispatch sets `Message.Recipient = user.email`
- **THEN** the email channel SHALL deliver to that address

#### Scenario: Ops fallback when recipient empty

- **WHEN** an ops event (no per-user recipient) routes to email
- **AND** `NOTIFY_OPS_RECIPIENT` is configured
- **THEN** the email channel SHALL deliver to that fallback address

#### Scenario: Missing fallback drops the message

- **WHEN** an ops event routes to email
- **AND** both `Message.Recipient` and `NOTIFY_OPS_RECIPIENT` are empty
- **THEN** the email channel SHALL no-op (return nil)
- **AND** the app SHALL log a warning at boot if email is routed for ops events but `NOTIFY_OPS_RECIPIENT` is empty

### Requirement: Per-Channel Retry

Each channel SHALL attempt one retry on transient delivery failure.

#### Scenario: HTTP 5xx triggers retry

- **WHEN** the channel POST returns HTTP 5xx or a network error
- **THEN** the channel SHALL retry once after a 1-second delay
- **AND** SHALL treat a second failure as terminal

#### Scenario: HTTP 4xx does not retry

- **WHEN** the channel POST returns HTTP 4xx (configuration error: bad chat ID, expired token, etc.)
- **THEN** the channel SHALL NOT retry
- **AND** SHALL return the error directly so the operator sees it in logs

#### Scenario: Retry-After honored on 429

- **WHEN** the channel POST returns HTTP 429 with a `Retry-After` header
- **THEN** the channel SHALL wait the indicated duration (capped at 30 seconds) before retrying
