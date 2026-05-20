# Design — add-notification-channels

## Architecture

```
                   event.Bus
                       │
                       ▼ subscribe(*)
              ┌─────────────────────┐
              │   notify.Service    │
              │  (router + fanout)  │
              └─────────────────────┘
                       │ render → Message
        ┌──────────────┼──────────────┬─────────────┐
        ▼              ▼              ▼             ▼
   ┌─────────┐   ┌──────────┐   ┌──────────┐  ┌──────────┐
   │  Email  │   │ Telegram │   │ Discord  │  │  Feishu  │
   │ Channel │   │ Channel  │   │ Channel  │  │ Channel  │
   └─────────┘   └──────────┘   └──────────┘  └──────────┘
        │              │              │             │
        ▼              ▼              ▼             ▼
   SMTP server    api.telegram   discord.com/    open.feishu.cn
                     .org/bot     api/webhooks
```

## Channel interface

```go
package notify

type Channel interface {
    Name() string                              // "email", "telegram", "discord", "feishu"
    Enabled() bool                             // false → silently skip
    Send(ctx context.Context, msg Message) error
}

type Message struct {
    Level   Level    // info | warn | error
    Title   string   // short subject line — used as email subject, embed title, card header
    Body    string   // multi-line body (plain text; channels escape as needed)
    Fields  []Field  // optional structured key-value pairs (e.g. "Order ID: 42")
    URL     string   // optional "view in panel" link
}

type Field struct{ Key, Value string }

type Level int
const (
    LevelInfo Level = iota
    LevelWarn
    LevelError
)
```

## Routing rules

Channels are matched per event type via a simple env-configurable
rule table:

```
NOTIFY_ROUTES=node.offline:telegram,discord;node.recovered:telegram,discord;\
              order.payment_confirmed:telegram;order.completed:telegram;\
              client.expired:email;client.expiring_soon:email;client.over_limit:email
```

Parse: `<event_type>:<channel1>,<channel2>;<event_type>:<channel3>`.
Channels not configured (e.g. `telegram` without `TELEGRAM_BOT_TOKEN`)
are silently skipped — operators see a startup warning per missing
channel.

Default rules when `NOTIFY_ROUTES` is empty: client lifecycle to
email (current behavior), nothing else. So a fresh deployment
without channel config keeps the v1 email behavior.

## Message rendering per channel

Each channel adapts the same `Message` into its own native shape:

| Channel | Title | Body | Level color | URL render |
|---|---|---|---|---|
| Email | `[level] title` | text/plain body + fields as table | n/a | inline link |
| Telegram | `<b>title</b>` | HTML body w/ `<code>` for fields | emoji prefix (🟢🟡🔴) | inline `<a href>` |
| Discord | embed.title | embed.description; fields → embed.fields[] | embed.color (RGB) | embed.url |
| Feishu | card header | card md content; fields → divider + 2-col rows | header.template (green/yellow/red) | card action button |

## Why fan-out happens in the notify service, not the bus

Today the bus is a simple subscribe-based dispatcher; subscribers
run synchronously on the publisher's goroutine. Adding 4 channels ×
N events would mean N×4 subscriber registrations + per-channel
template logic scattered everywhere.

Centralizing in the notify service:
- One place to add a new channel (write `channels/X.go`, register
  in the constructor)
- One place to add a new event (subscribe in `service.Start`, render
  to `Message`)
- Router logic isn't channel-specific — just a map lookup
- Tests can stub the Channel interface; per-channel HTTP tests live
  in their own files against `httptest.NewServer`

## Why we don't reuse the webhook system

The `webhook` service exists for user-defined HTTP endpoints (CRM,
BI, customer-facing integrations) — it ships every event to every
webhook by design, with admin CRUD + delivery history + retries.

Channels are different:
- Routing is rule-based (only certain events to certain channels)
- Templating is channel-specific (Discord embeds vs Telegram HTML)
- Config is env-driven (boot-time), not admin CRUD
- Target is operator's own infra, not customer infra

Trying to unify would either:
- Push channel templating into the webhook system → bloat
- Push routing logic into webhook subscriptions → admin UI mess

Keep them separate; share the event source.

## Retry strategy

Each channel adapter handles its own transient retries:
- HTTP 5xx / network timeout: 2 retries with 1s, 5s backoff
- HTTP 4xx: no retry (config error, alert the operator via logs)
- HTTP 429 + Retry-After: honor the header (cap at 30s)

After the channel exhausts retries, the notify service logs at
warn with the channel name + event type so the operator can grep.
No persistent delivery queue in v1 — if every channel of a deployment
goes down for >1 hour, we accept the lost notification window.
`add-notification-persistence` would add a delivery_log table +
a retry job, but that's a separate change.

## Frontend: zero

No new admin UI. Channel configuration is env-driven; the operator
edits `.env` and restarts. A future admin-UI change can expose
routing rules + per-channel preview without touching the dispatch
core.

## What we'll regret if we don't do it this way

- **Hardcoded channel list inside notify.Service** — every new
  channel needs touching the service struct
- **Channel-specific routing inside the Bus** — bus becomes opinionated
  about IM platforms
- **Synchronous Send inside the bus subscriber** — a slow Discord
  endpoint blocks every other subscriber on the same event
- **Per-deployment templates committed to disk** — operator can't
  customize wording for their userbase, but adding template editor
  costs more than it earns at this scope. Punt to follow-up.
