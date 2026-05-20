## MODIFIED Requirements

### Requirement: Purchase Endpoints

The system SHALL expose payment-method-specific purchase endpoints
that route to the chosen gateway. The endpoint URL pattern from #5
(`POST /api/user/billing/purchase/:provider`) already supports
arbitrary providers — Stripe SHALL register itself in the gateway
registry at boot so the same route accepts `provider=stripe`.

#### Scenario: Stripe purchase (added)

- **WHEN** a user posts to `POST /api/user/billing/purchase/stripe` with `{plan_id, idempotency_key, node_id, inbound_tag}`
- **THEN** the system SHALL create a payment_pending order
- **AND** SHALL call Stripe's `POST /v1/checkout/sessions` to create a hosted Checkout Session
- **AND** SHALL persist `payment_qr_url = checkout_session.url` and `payment_provider_order_id = checkout_session.id`
- **AND** SHALL return the order — the frontend redirects the user to `payment_qr_url`

#### Scenario: Stripe webhook on payment completion

- **WHEN** Stripe POSTs to `POST /api/public/payment/stripe/webhook` with `event.type = checkout.session.completed`
- **AND** the `Stripe-Signature` HMAC validates against the raw request body
- **THEN** the system SHALL transition the matching order from `payment_pending` → `paid` → `completed`
- **AND** SHALL provision the client on the chosen node + inbound
- **AND** SHALL publish `order.payment_confirmed` followed by `order.completed`

#### Scenario: Stripe webhook with invalid signature

- **WHEN** the `Stripe-Signature` HMAC does NOT validate
- **THEN** the system SHALL respond `400 Bad Request`
- **AND** SHALL NOT advance any order

#### Scenario: Stripe webhook replay protection

- **WHEN** the timestamp `t` in `Stripe-Signature` is more than 5 minutes older than now
- **THEN** the system SHALL reject the request with `400 Bad Request` (replay attack defense)

#### Scenario: Stripe payment-method discovery

- **WHEN** a user GETs `/api/user/billing/payment-methods`
- **AND** the Stripe gateway is configured (`STRIPE_SECRET_KEY` + `STRIPE_WEBHOOK_SECRET` set)
- **THEN** the response SHALL include `"stripe"` in the methods array alongside `"balance"` and any other configured providers

## ADDED Requirements

### Requirement: Payment URL Column Reuse

The `orders.payment_qr_url` column from #5 SHALL hold the
**redirect URL** when `payment_method=stripe`, not a QR-source
URL. The column name is preserved for backward compatibility;
the frontend distinguishes by `payment_method`.

#### Scenario: Frontend dispatches on payment_method

- **WHEN** the frontend receives an order with `payment_method=stripe` from a purchase response
- **THEN** the frontend SHALL redirect (`window.location.href`) to `payment_qr_url`
- **WHEN** `payment_method=alipay`
- **THEN** the frontend SHALL render `payment_qr_url` as a QR code (existing behavior)
