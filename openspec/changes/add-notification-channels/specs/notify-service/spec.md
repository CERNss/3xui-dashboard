## MODIFIED Requirements

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
- **THEN** the notify service SHALL fall back to the legacy rule set: client lifecycle events (expired / expiring_soon / over_limit) routed to email; no other channels routed

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
