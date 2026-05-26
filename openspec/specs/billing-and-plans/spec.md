# billing-and-plans

Plan catalog, per-user balance, idempotent purchase flow with rollback on
provisioning failure.

## Purpose & boundaries

Adjacent modules: **`user-accounts`** owns the user row + balance log;
**`client-provisioning`** owns the actual node-side client creation that a
purchase triggers; **`event-bus`** routes the `order.*` events to webhooks.

## Requirements

### Requirement: Plan Definition

Administrators SHALL be able to define purchasable plans.

#### Scenario: Create a plan

- **WHEN** an admin creates a plan with name, price, traffic allowance (GB),
  duration (days), optional IP limit, and an enabled flag
- **THEN** the plan is persisted and becomes available for purchase

#### Scenario: Disable a plan

- **WHEN** an admin disables a plan
- **THEN** the plan SHALL remain valid for clients already provisioned from it
  but SHALL NOT be offered for new purchases

### Requirement: User Balance

The system SHALL maintain a monetary balance per dashboard user.

#### Scenario: Balance starts at zero

- **WHEN** a new dashboard user account is created
- **THEN** its balance SHALL be zero

#### Scenario: Admin adjusts balance

- **WHEN** an admin credits or debits a user's balance
- **THEN** the new balance is persisted and a balance-history entry recording the
  amount, reason, and actor SHALL be created

### Requirement: Plan Purchase

Users SHALL be able to purchase a plan, which provisions or extends a client.

#### Scenario: Successful purchase

- **WHEN** an authenticated `user` purchases an enabled plan and their balance
  covers the price
- **THEN** the system deducts the price, creates an order record, and invokes
  client provisioning to create or extend the user's client per the plan's
  traffic and duration

#### Scenario: Insufficient balance

- **WHEN** a user purchases a plan their balance cannot cover
- **THEN** the purchase is rejected, no order is created, and no client change occurs

#### Scenario: Provisioning failure rolls back

- **WHEN** the balance is deducted but client provisioning on the node fails
- **THEN** the system SHALL refund the deducted amount and mark the order failed,
  leaving the balance consistent

### Requirement: Order History

The system SHALL record every purchase as an order.

#### Scenario: User views own orders

- **WHEN** an authenticated `user` opens their order history
- **THEN** the system returns that user's orders with plan name, amount, status,
  and timestamp

#### Scenario: Admin views all orders

- **WHEN** an admin opens order management
- **THEN** the system returns a paginated, filterable list of all orders across users

### Requirement: Idempotent Purchase Submission

The system SHALL prevent a double-click or retried purchase from charging twice.

#### Scenario: Duplicate purchase request

- **WHEN** the same purchase request (same idempotency key) is submitted more than once
- **THEN** the system SHALL process it at most once and return the original order
  for subsequent submissions

### Requirement: Order State Machine

The system SHALL track each purchase through a typed lifecycle. The
balance-based path stays intact; payment-gateway paths add new
states distinguished by the `payment_method` column.

Existing states (balance path):
- `pending` — order created via balance flow, provisioning in progress
- `completed` — provisioning finished
- `failed` — provisioning failed; balance refunded
- `refunded` — manual refund operation

New states (apply to `payment_method != 'balance'` orders):
- `payment_pending` — order created, payment provider returned a
  QR / redirect URL, waiting for user payment confirmation
- `paid` — payment provider confirmed; provisioning in progress
- `payment_failed` — payment provider rejected the trade
- `payment_expired` — > 15 minutes elapsed without confirmation

#### Scenario: Payment-pending order from gateway precreate

- **WHEN** a user posts `purchase/:provider` with a valid plan and idempotency_key
- **THEN** the system SHALL insert an order with `status=payment_pending` and `payment_method=:provider`
- **AND** SHALL persist the gateway-returned target URL into `payment_target_url`
- **AND** SHALL persist `payment_expires_at = now + 15 minutes`
- **AND** SHALL persist the chosen `provisioning_node_id` + `provisioning_inbound_tag`
- **AND** SHALL NOT deduct from user balance

#### Scenario: Payment confirmed via notify/webhook endpoint

- **WHEN** the gateway POSTs to the notify/webhook endpoint with a valid signature and a "paid" status
- **THEN** the system SHALL look up the order by `payment_provider_order_id`
- **AND** SHALL transition `payment_pending` → `paid` → `completed` atomically via a guarded transition (concurrent writers race; only one wins)
- **AND** SHALL provision the client on the persisted `provisioning_node_id` + `provisioning_inbound_tag`
- **AND** SHALL publish `order.payment_confirmed` followed by `order.completed`

#### Scenario: Idempotent notify replay

- **WHEN** the gateway re-POSTs the same notify (alipay retries 8 times over 26h; Stripe retries for ~3 days)
- **AND** the order is already in `completed` status
- **THEN** the system SHALL respond success without re-provisioning
- **AND** SHALL NOT publish duplicate events

#### Scenario: Payment expired

- **WHEN** an order remains in `payment_pending` for more than 15 minutes
- **THEN** the payment-poll job SHALL transition it to `payment_expired`
- **AND** SHALL publish `order.payment_expired`

### Requirement: Purchase Endpoints

The system SHALL expose payment-method-specific purchase endpoints
so adding a new provider doesn't mutate the existing balance path.

#### Scenario: Balance purchase (existing)

- **WHEN** a user posts to `POST /api/user/purchase`
- **THEN** the system SHALL run the balance-deduct + provision flow

#### Scenario: Payment-gateway purchase

- **WHEN** a user posts to `POST /api/user/purchase/:provider` with `{plan_id, idempotency_key, node_id, inbound_tag}`
- **THEN** the system SHALL create a payment_pending order
- **AND** SHALL ask the registered gateway to create a payment session (`alipay.trade.precreate` / `stripe.checkout.sessions.create` / ...)
- **AND** SHALL return `{id, status, payment_method, payment_target_url, payment_provider_order_id, payment_expires_at}` so the portal can render the QR or redirect

#### Scenario: Payment methods discovery

- **WHEN** a user GETs `/api/user/payment-methods`
- **THEN** the system SHALL return the list of configured payment methods, always including `"balance"` and additionally each provider whose `Enabled()` returns true

### Requirement: Payment Gateway Plug-in Architecture

The system SHALL load payment gateways through a registry indexed
by provider name. Gateways implement the `payment.Gateway`
interface; gateways whose webhooks sign the raw body (Stripe HMAC)
additionally implement `payment.RawBodyVerifier`. Adding a third
provider (WeChat Pay, PayPal) SHALL NOT require modifying the
billing core.

#### Scenario: Gateway not configured

- **WHEN** a purchase request specifies a provider that isn't registered
- **THEN** the system SHALL respond `404 Not Found` with `payment provider not configured`

#### Scenario: Webhook delivery path agnostic

- **WHEN** a public payment notify/webhook endpoint receives a signed payload
- **THEN** the handler SHALL look the gateway up by provider name
- **AND** SHALL call either `gateway.VerifyNotify(params, sig)` (form-encoded) OR `gateway.(RawBodyVerifier).VerifyWebhookRaw(rawBody, sigHeader)` (HMAC) per the gateway's signing mode

### Requirement: Order Payment Columns

The `orders` table SHALL carry payment-provider state alongside the
original order fields. Columns SHALL NOT be NULLable for orders
created after the baseline schema; defaults ensure balance orders read
back as `payment_method='balance'`.

Columns:
- `payment_method` (text, NOT NULL, default `'balance'`)
- `payment_provider_order_id` (text, NOT NULL, default `''`)
- `payment_target_url` (text, NOT NULL, default `''`)
- `payment_expires_at` (timestamptz, nullable)
- `provisioning_node_id` (bigint, nullable)
- `provisioning_inbound_tag` (text, NOT NULL, default `''`)

#### Scenario: Balance orders default payment_method

- **WHEN** a balance order is created without gateway-specific fields
- **THEN** the row SHALL read back with `payment_method='balance'`
- **AND** the gateway-specific columns SHALL be empty / NULL

### Requirement: Async Payment Notify Endpoints

The system SHALL expose `POST /api/public/payment/<provider>/{notify,webhook}`
endpoints, one per payment provider. These routes SHALL be public
(no JWT) and SHALL be authenticated by the provider's signature
scheme alone.

#### Scenario: Alipay notify with valid signature

- **WHEN** alipay's servers POST `application/x-www-form-urlencoded` to `/api/public/payment/alipay/notify` with a valid RSA2 signature
- **THEN** the system SHALL respond `200 OK` with body `success` (plain text, exact)
- **AND** SHALL advance the order per the trade_status

#### Scenario: Stripe webhook with valid HMAC

- **WHEN** Stripe POSTs `application/json` to `/api/public/payment/stripe/webhook` with a valid `Stripe-Signature` HMAC and `event.type=checkout.session.completed`
- **THEN** the system SHALL respond `200 OK`
- **AND** SHALL advance the order via `ConfirmPayment`

#### Scenario: Invalid signature

- **WHEN** the signature fails verification for either gateway
- **THEN** the system SHALL respond `400 Bad Request`
- **AND** SHALL NOT advance any order
- **AND** SHALL log the rejection at warn level with the source IP

### Requirement: Payment-Poll Failsafe Job

The system SHALL run a payment-poll job on a fixed cadence (default
30 s) to recover orders whose webhook never arrived. The job SHALL
also expire abandoned `payment_pending` orders past the configured
window (default 15 min).

#### Scenario: Dropped webhook recovered

- **WHEN** an order's gateway session was paid but no webhook reached the dashboard
- **AND** the order is still `payment_pending`
- **THEN** on the next poll cycle the job SHALL call `gateway.Query(provider_order_id)`
- **AND** receiving `payment.StatusPaid` SHALL advance the order via the same `ConfirmPayment` path the webhook would have used

#### Scenario: Abandoned QR expires

- **WHEN** an order's `created_at` is older than the expiry window AND its status is still `payment_pending`
- **THEN** the job SHALL transition the order to `payment_expired`
- **AND** SHALL publish `order.payment_expired`
