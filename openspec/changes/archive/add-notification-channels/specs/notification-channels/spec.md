## ADDED Requirements

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
- **AND** SHALL set `color` to an RGB int derived from the Message Level (red for error, amber for warn, green for info)
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

#### Scenario: Feishu API error surfaced

- **WHEN** Feishu responds with `{code: 9499, msg: "Bad Request"}`
- **THEN** `Send` SHALL return an error wrapping the code + msg

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
