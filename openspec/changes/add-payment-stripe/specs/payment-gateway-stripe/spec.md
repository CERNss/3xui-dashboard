## ADDED Requirements

### Requirement: Stripe Checkout Integration

The system SHALL integrate Stripe's Checkout Sessions API as a
payment gateway behind the existing `payment.Gateway` interface.
The gateway SHALL be self-contained in
`internal/service/payment/stripe/` so additional gateways added
later do not require changes to the Stripe code.

#### Scenario: Checkout Session creation

- **WHEN** the billing service calls `gateway.CreatePayment(order, planName)` for a stripe-method order
- **THEN** the gateway SHALL POST to `https://api.stripe.com/v1/checkout/sessions`
- **AND** SHALL include `Authorization: Bearer ${SecretKey}` header
- **AND** SHALL include `Stripe-Version: 2024-06-20` header
- **AND** SHALL include `Idempotency-Key: order-${order.ID}-v1` header
- **AND** SHALL build the body with `mode=payment`, `success_url`, `cancel_url`, `client_reference_id=${order.ID}`, and a single `line_items[]` entry with `price_data.unit_amount=${order.price_cents}`, `price_data.currency=${STRIPE_CURRENCY}`, `price_data.product_data.name=${planName}`, `quantity=1`
- **AND** SHALL return `(qr_url=session.url, provider_order_id=session.id, expires_at=session.expires_at)` on success

#### Scenario: Checkout Session query failsafe

- **WHEN** the payment-poll job calls `gateway.Query(session_id)`
- **THEN** the gateway SHALL GET `https://api.stripe.com/v1/checkout/sessions/${id}`
- **AND** SHALL map `session.payment_status`:
  - `paid` or `no_payment_required` → `payment.StatusPaid`
  - `unpaid` → `payment.StatusPending`

### Requirement: Webhook Signature Verification

The Stripe webhook handler SHALL verify the `Stripe-Signature`
HMAC against the raw request body before parsing the event JSON.
The handler SHALL read the body with `io.ReadAll` BEFORE any JSON
binding so the bytes passed to HMAC match what Stripe signed.

#### Scenario: Valid signature accepted

- **WHEN** a webhook POST arrives with `Stripe-Signature: t=${ts},v1=${sig}`
- **AND** `t` is within 5 minutes of now
- **AND** `sig` matches HMAC-SHA256(`${t}.${rawBody}`, `${WebhookSecret}`)
- **THEN** the handler SHALL parse the event and dispatch

#### Scenario: Tampered body rejected

- **WHEN** any byte of the body differs from what was signed
- **THEN** signature verification SHALL fail
- **AND** the handler SHALL respond `400 Bad Request`

#### Scenario: Replay attack rejected

- **WHEN** `t` is more than 5 minutes older than the current server time
- **THEN** the handler SHALL respond `400 Bad Request` without performing HMAC comparison

#### Scenario: Missing signature components

- **WHEN** the `Stripe-Signature` header is missing or lacks the `t` / `v1` keys
- **THEN** the handler SHALL respond `400 Bad Request`

### Requirement: Pure-Stdlib HMAC

The gateway SHALL implement HMAC-SHA256 verification using only
the Go standard library (`crypto/hmac`, `crypto/sha256`,
`crypto/subtle`). The system SHALL NOT depend on the `stripe-go`
SDK.

#### Scenario: Constant-time signature compare

- **WHEN** the signature is compared against the expected HMAC
- **THEN** the comparison SHALL use `crypto/subtle.ConstantTimeCompare` to avoid timing attacks on the secret

### Requirement: Configuration

The gateway SHALL be opt-in via environment variables. Absence of
required vars SHALL leave the provider unregistered.

#### Scenario: Required env vars

- **WHEN** either `STRIPE_SECRET_KEY` or `STRIPE_WEBHOOK_SECRET` is empty
- **THEN** the stripe gateway SHALL NOT register
- **AND** `GET /api/user/billing/payment-methods` SHALL omit `"stripe"`

#### Scenario: Default currency

- **WHEN** `STRIPE_CURRENCY` is unset
- **THEN** the gateway SHALL use `"usd"` for Checkout Session creation
