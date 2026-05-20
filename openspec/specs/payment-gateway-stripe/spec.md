# payment-gateway-stripe

Stripe Checkout Sessions gateway. Self-contained in
`internal/service/payment/stripe/` behind the generic
`payment.Gateway` interface plus the optional
`payment.RawBodyVerifier` for HMAC webhook verification.

## Purpose & boundaries

- **Owns**: outbound Checkout Session create + retrieve HTTP calls;
  HMAC-SHA256 verification of inbound webhooks; pinned Stripe API
  version (`Stripe-Version: 2024-06-20`); idempotency-key plumbing.
- **Does NOT own**: order state machine (lives in `billing-and-plans`),
  webhook HTTP endpoint (lives in `handler/public`), any UI surface.
- **Differs from alipay**: redirect UX (no QR), HMAC auth on webhook
  (not RSA on per-param), multi-currency native.

## Requirements

### Requirement: Stripe Checkout Integration

The system SHALL integrate Stripe's Checkout Sessions API as a
payment gateway behind the generic `payment.Gateway` interface.

#### Scenario: Checkout Session creation

- **WHEN** the billing service calls `gateway.CreatePayment(order, planName)` for a stripe-method order
- **THEN** the gateway SHALL POST to `${StripeEndpoint}/v1/checkout/sessions` (defaults `https://api.stripe.com`)
- **AND** SHALL include `Authorization: Bearer ${SecretKey}` header
- **AND** SHALL include `Stripe-Version: 2024-06-20` header
- **AND** SHALL include `Idempotency-Key: order-${order.ID}-v1` header so a retried request returns the same session
- **AND** SHALL build the body with `mode=payment`, `client_reference_id=${order.ID}`, `success_url`, `cancel_url`, `expires_at`, and one `line_items[0]` entry with `price_data.unit_amount=${order.price_cents}`, `price_data.currency=${STRIPE_CURRENCY}`, `price_data.product_data.name=${planName}`, `quantity=1`
- **AND** SHALL return `(target_url=session.url, provider_order_id=session.id, expires_at=session.expires_at)` on success

#### Scenario: Checkout Session query failsafe

- **WHEN** the payment-poll job calls `gateway.Query(session_id)`
- **THEN** the gateway SHALL GET `${StripeEndpoint}/v1/checkout/sessions/${id}`
- **AND** SHALL map `session.payment_status`:
  - `paid` or `no_payment_required` → `payment.StatusPaid`
  - `unpaid` with `status=expired` → `payment.StatusExpired`
  - `unpaid` otherwise → `payment.StatusPending`

### Requirement: Webhook Signature Verification

The gateway SHALL implement `payment.RawBodyVerifier` so the
webhook handler can verify the `Stripe-Signature` HMAC against
the raw request body BEFORE any JSON parsing.

#### Scenario: Valid signature accepted

- **WHEN** a webhook POST arrives with `Stripe-Signature: t=${ts},v1=${sig}`
- **AND** `t` is within 5 minutes of now
- **AND** `sig` matches HMAC-SHA256(`${t}.${rawBody}`, `${WebhookSecret}`)
- **THEN** verification SHALL return nil

#### Scenario: Tampered body rejected

- **WHEN** any byte of the body differs from what was signed
- **THEN** verification SHALL fail with a wrapped `ErrSignatureFailed`

#### Scenario: Replay attack rejected

- **WHEN** `t` is more than 5 minutes older than the current server time
- **THEN** verification SHALL fail with `ErrReplay` without performing HMAC comparison

#### Scenario: Multiple v1 entries accepted

- **WHEN** the header carries multiple `v1=` values (Stripe's documented secret-rotation window)
- **THEN** verification SHALL accept any one matching value

### Requirement: Pure-Stdlib HMAC

The gateway SHALL implement HMAC-SHA256 verification using only the
standard library (`crypto/hmac`, `crypto/sha256`, `crypto/subtle`).
The system SHALL NOT depend on the `stripe-go` SDK.

#### Scenario: Constant-time signature compare

- **WHEN** the signature is compared against the expected HMAC
- **THEN** the comparison SHALL use `crypto/subtle.ConstantTimeCompare` to avoid timing attacks on the secret

### Requirement: Configuration

The gateway SHALL be opt-in via environment variables. Absence of
required vars SHALL leave the provider unregistered.

#### Scenario: Required env vars

- **WHEN** either `STRIPE_SECRET_KEY` or `STRIPE_WEBHOOK_SECRET` is empty
- **THEN** the stripe gateway SHALL NOT register
- **AND** `GET /api/user/payment-methods` SHALL omit `"stripe"`

#### Scenario: Default currency

- **WHEN** `STRIPE_CURRENCY` is unset
- **THEN** the gateway SHALL use `"usd"` for Checkout Session creation
- **AND** the app SHALL log the configured currency at INFO on boot so operators see it before a real charge

#### Scenario: Endpoint override

- **WHEN** `STRIPE_ENDPOINT` is set
- **THEN** the gateway SHALL POST/GET against that URL instead of `https://api.stripe.com`
- **AND** this override SHALL be used only for tests / sandbox routing — production deployments rely on the secret-key prefix (`sk_test_` vs `sk_live_`) to distinguish environments
