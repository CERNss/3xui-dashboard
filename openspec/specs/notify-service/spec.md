# notify-service

Subscribes to domain events on the bus and fans each out to a
routed list of `Channel`s. Lives in `internal/service/notify`.

## Purpose & boundaries

- **Owns**: bus subscriptions for `client.*` / `node.*` / `order.*`
  events; the Router that maps event types to channel name lists;
  the dedup gate via `notification_log` (per-channel keys); typed
  payload switches on `event/payload.*` types.
- **Does NOT own**: per-platform wire formats (lives in
  `notification-channels` adapters), the bus itself (lives in
  `event-bus`), the dedup table schema (lives in `migrations`).

## Requirements

### Requirement: Event-Driven Notification Dispatch

The notify service SHALL subscribe to event-bus events and
dispatch each to a configurable set of notification channels. The
service SHALL NOT be hardcoded to any single delivery mechanism.

#### Scenario: Multi-channel fan-out

- **WHEN** the event bus publishes `node.offline`
- **AND** the Router config maps `node.offline → telegram,discord`
- **THEN** the notify service SHALL build one Message
- **AND** SHALL call `Send` on the telegram channel AND on the discord channel
- **AND** failure of one channel SHALL NOT prevent the other from being attempted

#### Scenario: Channel-specific per-event dedup keys

- **WHEN** the notify service decides to send via two channels for the same `(event_kind, ownership_id)` pair
- **THEN** each channel send SHALL use a separate `notification_log` dedup key (e.g. `expiring_soon_telegram` vs `expiring_soon_email`)
- **AND** a redelivery to one channel SHALL NOT block the other

#### Scenario: Default routing when NOTIFY_ROUTES is empty

- **WHEN** the operator has not configured `NOTIFY_ROUTES`
- **THEN** the notify service SHALL fall back to the legacy rule set: client lifecycle events (`expired` / `expiring_soon` / `over_limit`) routed to email; no other channels routed

### Requirement: Router Configuration

The system SHALL accept a routing rule string in the form
`event_type:channel1,channel2;event_type2:channel3`. Parser
errors SHALL surface at boot, not at first event.

#### Scenario: Valid routing string

- **WHEN** `NOTIFY_ROUTES=node.offline:telegram;client.expired:email`
- **THEN** the router SHALL map `node.offline` to `[telegram]` and `client.expired` to `[email]`

#### Scenario: Malformed routing string

- **WHEN** the routing string contains a token without `:` (e.g. `node.offline-telegram`)
- **THEN** the parser SHALL return an error identifying the bad token
- **AND** the application SHALL refuse to start

#### Scenario: Unconfigured channel referenced

- **WHEN** the routing string references `telegram` but `TELEGRAM_BOT_TOKEN` is empty
- **THEN** the application SHALL log a warning at boot identifying the missing config
- **AND** SHALL continue without that channel
- **AND** events routed only to the missing channel SHALL be dropped (no error to publishers)

### Requirement: Typed Payload Switches

The service SHALL consume bus events via typed switches on the
payload types in `internal/service/event/payload`. Reflection-based
field extraction SHALL NOT be used — a field rename in the
publishing package MUST produce a compile error in the subscriber.

#### Scenario: Unknown payload type warned

- **WHEN** an event arrives with a payload that doesn't match any expected type for that event
- **THEN** the service SHALL log a warning with the actual type
- **AND** SHALL NOT panic or attempt to send

### Requirement: Ops Event Coverage

The service SHALL subscribe to ops events (no per-user recipient)
in addition to the per-user client lifecycle events. Each ops
event SHALL render a Message with severity-appropriate Level.

| Event type | Level | Subscriber |
|---|---|---|
| `node.offline` | error | onNodeOffline |
| `node.recovered` | info | onNodeRecovered |
| `order.payment_confirmed` | info | opsOrderEvent |
| `order.payment_failed` | warn | opsOrderEvent |
| `order.payment_expired` | warn | opsOrderEvent |
| `order.failed` | error | opsOrderEvent |

#### Scenario: Ops event dispatched to admin channels

- **WHEN** the bus publishes `node.offline` with `payload.NodeStatusChanged{NodeID: 42, Name: "tokyo-1"}`
- **AND** `NOTIFY_ROUTES` routes `node.offline` to `telegram`
- **THEN** the telegram channel SHALL receive a Message with `Level=Error`, title containing `tokyo-1`, and structured fields for Node ID + Time
- **AND** the recipient SHALL be empty (channel falls back to its configured target)

#### Scenario: Order event renders typed fields

- **WHEN** the bus publishes `order.payment_confirmed` with `payload.Order{OrderID: 42, UserID: 7, PriceCents: 500}`
- **THEN** the routed channels SHALL receive a Message with fields:
  - Order ID = `42`
  - User ID = `7`
  - Amount = `5.00` (formatted yuan, two decimals)
