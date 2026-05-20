# event-bus

In-process synchronous publish/subscribe used by domain services to
announce things that happen (`node.online`, `user.registered`,
`order.completed`, etc.). One `Bus` per process. Lives in
`internal/service/event`.

## Purpose & boundaries

The bus is intentionally synchronous: every subscriber runs inline on
the publisher's goroutine, in registration order. Subscribers that
need to do real work (e.g. HTTP delivery) MUST enqueue the work
themselves — see `webhook-notifications` which persists deliveries to
the DB and runs `scheduler-jobs::webhook.retry` to fan them out.

## Event taxonomy

Well-known event types are constants in `event/bus.go` so renames
stay tractable:

| Constant | Type string |
|---|---|
| `NodeOnline` | `node.online` |
| `NodeOffline` | `node.offline` |
| `NodeProbeFailed` | `node.probe_failed` |
| `UserRegistered` | `user.registered` |
| `OrderCreated` | `order.created` |
| `OrderCompleted` | `order.completed` |
| `OrderFailed` | `order.failed` |
| `ClientExpired` | `client.expired` |
| `ClientOverLimit` | `client.over_limit` |

Per-type payload structs are owned by their producing service (e.g.
`usersvc.RegisteredPayload`). The bus stores them as `any` —
subscribers type-assert.

## Requirements

### Requirement: Single Process-Wide Bus

The system SHALL instantiate exactly one `event.Bus` at startup,
shared across every domain service.

#### Scenario: app.Build constructs the bus once

- **WHEN** `app.Build` runs
- **THEN** it SHALL call `event.NewBus(logger)` exactly once
- **AND** pass that instance to every service constructor that needs to publish (user, node, billing, etc.)

### Requirement: Synchronous Subscriber Dispatch

The system SHALL invoke every subscriber for a matching event type
synchronously, in the order they were registered.

#### Scenario: Multiple subscribers run in order

- **GIVEN** subscribers A and B both registered for `node.online`, A first
- **WHEN** a service publishes a `node.online` event
- **THEN** A SHALL be invoked first, B second, on the publisher's goroutine
- **AND** Publish SHALL block until both subscribers have returned

#### Scenario: Subscriber panic isolated

- **WHEN** a subscriber panics during dispatch
- **THEN** the bus SHALL recover the panic, log it at ERROR with the event type, and continue dispatching to the remaining subscribers
- **AND** Publish SHALL return normally

#### Scenario: Type filter is exact

- **WHEN** an event of type `order.created` is published
- **THEN** only subscribers that registered for the exact string `order.created` SHALL be invoked
- **AND** the bus SHALL NOT support glob/wildcard subscription at the bus level (the webhook service handles wildcard at the *subscription* layer, not the bus layer)

### Requirement: Event Carries Type, Time, Data

Every event SHALL be a struct of `{Type string, Time time.Time, Data any}`.

#### Scenario: PublishType helper

- **WHEN** a service calls `bus.PublishType("user.registered", payload)`
- **THEN** the bus SHALL construct an `Event{Type: …, Time: time.Now(), Data: payload}` and dispatch it

#### Scenario: Subscriber receives full event

- **WHEN** a subscriber's callback runs
- **THEN** the callback signature SHALL be `func(event.Event)` (not `func(any)`)
- **AND** the subscriber SHALL be free to read `Type`, `Time`, and `Data` from the event

## Out of scope

- Cross-process eventing (Kafka, NATS, etc.).
- Persistent event log / replay. Webhook delivery has its own DB-backed
  retry queue (`webhook-notifications`); the event-bus itself is lossy
  by design.
- Subscriber priorities or async fan-out — subscribers that need async
  behavior must implement it themselves.
