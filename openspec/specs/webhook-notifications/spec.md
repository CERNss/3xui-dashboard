# webhook-notifications

Outbound webhook fan-out: endpoint config, event catalog, signed
delivery, persistent retry queue, delivery log, manual test + replay.

## Purpose & boundaries

Adjacent modules: **`event-bus`** is the in-process pub/sub that
producers (probe loop, billing, provisioning) publish to;
**`scheduler-jobs`** drives the persistent-retry cron.

**Distinct from `notify-service`**: this module ships
**user-defined** webhook endpoints (CRM, BI, customer-facing
integrations) — admin CRUD, signed delivery, persistent retry,
delivery log, replay. The newer `notify-service` covers
**operator-defined** alerts to fixed IM channels
(email/telegram/discord/feishu) configured via env vars at boot
with per-channel templating. The two share the event bus as
source but diverge on:

| | webhook-notifications | notify-service |
|---|---|---|
| Audience | end-user / 3rd party integrations | operator's own ops channel |
| Config surface | admin CRUD UI + DB | env vars at boot |
| Retry | persistent queue + cron | in-process best-effort + 1 retry |
| Templating | none (ships raw event JSON) | per-channel native (HTML / embed / card) |
| Dedup | per-delivery row | `notification_log` keyed by event-kind + ownership |

A single domain event can fan out to both: webhook subscribers
receive the raw JSON; notify-service emits a human-rendered
message to its routed channels.

## Requirements

### Requirement: Webhook Endpoint Configuration

The administrator SHALL be able to register, edit, enable/disable, and delete
outbound webhook endpoints.

#### Scenario: Register a webhook

- **WHEN** the admin creates a webhook with a target URL, an optional signing
  secret, a set of subscribed event types, and an enabled flag
- **THEN** the webhook is persisted and becomes eligible to receive matching events

#### Scenario: Reject invalid webhook URL

- **WHEN** the admin submits a webhook with a malformed URL or a non-`http(s)` scheme
- **THEN** the system returns a validation error and does not persist the webhook

#### Scenario: Disable a webhook

- **WHEN** the admin disables a webhook
- **THEN** the webhook SHALL stop receiving events while its configuration and
  delivery history are retained

#### Scenario: SSRF-safe delivery

- **WHEN** a webhook target URL resolves to a private/loopback address
- **THEN** delivery SHALL be refused by the guarded transport unless the webhook
  is explicitly marked to allow private addresses

### Requirement: Event Catalog and Subscription

The system SHALL expose a fixed catalog of event types, and each webhook SHALL
receive only the event types it subscribes to.

#### Scenario: Event catalog available

- **WHEN** the admin opens webhook configuration
- **THEN** the system returns the available event types, at minimum:
  `node.online`, `node.offline`, `node.probe_failed`,
  `client.traffic_threshold`, `client.traffic_exhausted`,
  `client.expiring_soon`, `client.expired`,
  `user.registered`, `order.created`, `order.completed`, `order.failed`

#### Scenario: Only subscribed events delivered

- **WHEN** an event fires and a webhook is not subscribed to that event type
- **THEN** that webhook SHALL NOT receive a delivery for the event

#### Scenario: Wildcard subscription

- **WHEN** a webhook subscribes to all events
- **THEN** it SHALL receive every event type in the catalog, including event
  types added in future catalog versions

### Requirement: Webhook Payload Format

Webhook deliveries SHALL carry a consistent, versioned JSON payload.

#### Scenario: Payload envelope

- **WHEN** an event is delivered
- **THEN** the request body SHALL be a JSON object containing a payload schema
  version, a unique event id, the event type, an ISO-8601 timestamp, and an
  event-specific `data` object

#### Scenario: Event data is self-describing

- **WHEN** a `node.offline` or `client.traffic_exhausted` (or other) event is delivered
- **THEN** the `data` object SHALL include the identifiers a consumer needs
  (e.g. node id/name, client email, inbound tag, usage figures) without
  requiring a follow-up API call

### Requirement: Delivery Signing

The system SHALL sign webhook deliveries so receivers can verify authenticity.

#### Scenario: Signed request

- **WHEN** a webhook has a signing secret configured and an event is delivered
- **THEN** the request SHALL include a signature header computed as an HMAC of
  the raw request body using the secret, plus a timestamp header

#### Scenario: Unsigned when no secret

- **WHEN** a webhook has no signing secret
- **THEN** the delivery is sent without a signature header and this is recorded
  as the webhook's configured behavior

### Requirement: Delivery, Retry, and Timeout

The system SHALL deliver events asynchronously with bounded retries and SHALL
not let a slow or failing endpoint block event producers.

#### Scenario: Asynchronous delivery

- **WHEN** an event fires
- **THEN** the producing operation (probe loop, provisioning, purchase, etc.)
  SHALL enqueue the event and continue without waiting for HTTP delivery

#### Scenario: Successful delivery

- **WHEN** the webhook endpoint responds with a 2xx status within the timeout
- **THEN** the delivery is marked succeeded

#### Scenario: Retry on failure

- **WHEN** a delivery times out or returns a non-2xx status
- **THEN** the system SHALL retry with exponential backoff up to a bounded
  maximum attempt count, after which the delivery is marked failed
- **AND** retry state is persisted (`webhook_deliveries.next_attempt_at`, set
  by migration `0003_webhook_retry.up.sql`) so retries survive process restarts

#### Scenario: Failing webhook does not stall others

- **WHEN** one webhook endpoint is persistently slow or unreachable
- **THEN** deliveries to other webhooks and other events SHALL proceed unaffected

### Requirement: Delivery Log and Manual Test

The administrator SHALL be able to inspect recent deliveries and send a test event.

#### Scenario: View delivery log

- **WHEN** the admin opens a webhook's delivery history
- **THEN** the system returns recent deliveries with event type, timestamp,
  attempt count, final status, response status code, and any error

#### Scenario: Send a test event

- **WHEN** the admin triggers a test delivery for a webhook
- **THEN** the system sends a synthetic `webhook.test` event to that endpoint
  and reports the immediate result (status code, latency, error)

#### Scenario: Replay a failed delivery

- **WHEN** the admin replays a previously failed delivery
- **THEN** the system re-sends the original payload and records the result as a
  new delivery attempt
