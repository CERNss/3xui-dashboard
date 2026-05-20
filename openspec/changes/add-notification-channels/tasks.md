# Tasks — add-notification-channels

## 1. Channel interface + Message types

- [ ] 1.1 Create `internal/service/notify/channel.go` with the
  `Channel` interface, `Message`, `Level`, `Field` types.
- [ ] 1.2 Add `Level.String()` for log fields + the colored severity
  helpers `(emoji, hexColor, fsTemplate)` channels reference for
  rendering.

## 2. Router (`internal/service/notify/router.go`)

- [ ] 2.1 `Router` struct holds `map[eventType] → []channelName`.
- [ ] 2.2 `ParseRoutes(raw string) (Router, error)` parses the
  env-var format: `event_type:chan1,chan2;event_type2:chan3`.
  Empty input returns the default rule set.
- [ ] 2.3 `Router.Channels(eventType) []string` returns matching
  channel names (preserves declaration order, no dedup needed
  since admins shouldn't list a channel twice).
- [ ] 2.4 Tests: parse roundtrip, empty input → defaults, malformed
  input returns error with the bad token.

## 3. Email channel adapter (`channels/email.go`)

- [ ] 3.1 Wrap the existing `mailer.Mailer` in a `Channel` adapter.
  Email `Send` formats `Message` into a multipart with the title
  as subject and body as plain text + fields as a key:value list
  appended.
- [ ] 3.2 `Enabled()` returns `mailer.Mailer.Configured()` (or
  equivalent — surface SMTP `Enabled()` from config).

## 4. Telegram channel adapter (`channels/telegram.go`)

- [ ] 4.1 `Client` struct: `botToken, chatID string; http
  *http.Client`.
- [ ] 4.2 `Send` POSTs to `https://api.telegram.org/bot${TOKEN}/sendMessage`
  with `{chat_id, text, parse_mode: "HTML", disable_web_page_preview: true}`.
- [ ] 4.3 Render: `<b>${title}</b>\n\n${body}\n\n${fields-as-code-block}`,
  prefix the level emoji per the design doc.
- [ ] 4.4 Errors: parse Telegram's `{ok: false, description: ...}`
  envelope and surface `description` as the error string.
- [ ] 4.5 Tests: httptest server roundtrips a valid `sendMessage`
  call; Telegram error envelope surfaces.

## 5. Discord channel adapter (`channels/discord.go`)

- [ ] 5.1 `Client` struct: `webhookURL string; http *http.Client`.
- [ ] 5.2 `Send` POSTs `{embeds: [{title, description, color, fields, url}]}`.
- [ ] 5.3 Color: Level → RGB int (red, yellow, green).
- [ ] 5.4 Tests: httptest roundtrip checks the embed shape; 204
  response treated as success (Discord returns 204).

## 6. Feishu channel adapter (`channels/feishu.go`)

- [ ] 6.1 `Client` struct: `webhookURL string; http *http.Client`.
- [ ] 6.2 `Send` POSTs a `msg_type=interactive` card with title,
  body, divider, and field rows.
- [ ] 6.3 Header template: Level → `green`/`yellow`/`red`.
- [ ] 6.4 Tests: httptest roundtrip checks the card JSON shape;
  Feishu's `{code, msg}` error envelope surfaces.

## 7. Generalize notify.Service

- [ ] 7.1 Replace the hardcoded mailer dispatch with a fanout to
  the routed channels. Existing client lifecycle handlers keep
  the same kind-suffixed dedup keys (`expiring_soon_telegram`,
  `expiring_soon_discord`, …) so two-layer dedup keeps working
  per-channel.
- [ ] 7.2 Add subscribers for `order.payment_confirmed`,
  `order.completed`, `order.failed`, `order.payment_failed`,
  `order.payment_expired`, `node.offline`, `node.recovered` —
  each renders an event-specific `Message`.
- [ ] 7.3 Each subscriber:
  - looks up routed channels via Router
  - for each channel: builds the Message, calls channel.Send
  - logs warn on failure (does NOT block other channels)
- [ ] 7.4 Tests: stub Channel records calls; Router with `event.foo:telegram`
  routes a fake `event.foo` to the stub and not to other channels.

## 8. NodeRecovered event

- [ ] 8.1 Add `event.NodeRecovered` constant.
- [ ] 8.2 Wire the probe job to publish `NodeRecovered` (NOT
  `NodeOnline`) on the FIRST online tick after one or more
  offline ticks. Subsequent `online` ticks keep using the existing
  `NodeOnline` (still useful for the webhook stream).
- [ ] 8.3 Tests: synthetic offline → online transition publishes
  NodeRecovered exactly once; staying online publishes nothing
  new on that channel.

## 9. Config

- [ ] 9.1 Add `Notify` struct to `internal/config/config.go`:
  - `Routes string` (raw rule string)
  - `Telegram TelegramConfig` with `BotToken, ChatID string`
  - `Discord DiscordConfig` with `WebhookURL string`
  - `Feishu FeishuConfig` with `WebhookURL string`
- [ ] 9.2 Env vars: `NOTIFY_ROUTES`, `TELEGRAM_BOT_TOKEN`,
  `TELEGRAM_CHAT_ID`, `DISCORD_WEBHOOK_URL`, `FEISHU_WEBHOOK_URL`.
- [ ] 9.3 `Enabled()` per sub-config returns true iff its required
  fields are non-empty.

## 10. App wiring

- [ ] 10.1 In `internal/app/app.go`: build the channels (email
  always; telegram/discord/feishu only when their config is set),
  build the Router from `cfg.Notify.Routes`, pass both into
  `notify.New(...)`.
- [ ] 10.2 Log a startup warning per channel that's mentioned in
  routes but unconfigured.

## 11. Spec deltas + ROADMAP

- [ ] 11.1 Update `openspec/changes/add-notification-channels/specs/notify-service/spec.md`
  with the MODIFIED requirement: dispatch via Channel registry.
- [ ] 11.2 Write `openspec/changes/add-notification-channels/specs/notification-channels/spec.md`
  with ADDED requirements for Telegram, Discord, Feishu adapters.
- [ ] 11.3 Update `ROADMAP.md`: 通知 50% → 80%, mark #7 ✅.

## 12. Documentation

- [ ] 12.1 `.env.example`: NOTIFY_ROUTES + the three channel blocks.
- [ ] 12.2 Brief operator note in proposal about getting bot token
  / webhook URLs from each platform.
