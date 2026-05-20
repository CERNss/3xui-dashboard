## MODIFIED Requirements

### Requirement: Order State Machine

The system SHALL track each purchase through a typed lifecycle. The
original balance-based path stays intact; payment-gateway paths add
new states distinguished by the `payment_method` column.

Existing states (unchanged):
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

#### Scenario: Payment-pending order from alipay precreate

- **WHEN** a user posts `purchase/alipay` with a valid plan and idempotency_key
- **THEN** the system SHALL insert an order with `status=payment_pending` and `payment_method=alipay`
- **AND** SHALL persist the alipay-returned `qr_code` URL into `payment_qr_url`
- **AND** SHALL persist `payment_expires_at = now + 15 minutes`
- **AND** SHALL NOT deduct from user balance

#### Scenario: Payment confirmed via notify endpoint

- **WHEN** alipay POSTs to the notify endpoint with a valid RSA2 signature and `trade_status` in `{TRADE_SUCCESS, TRADE_FINISHED}`
- **THEN** the system SHALL look up the order by `payment_provider_order_id`
- **AND** SHALL transition `payment_pending` → `paid` → `completed` atomically
- **AND** SHALL provision the client on the chosen node + inbound
- **AND** SHALL persist `client_ownership_id` on the order
- **AND** SHALL publish `order.payment_confirmed` followed by `order.completed`

#### Scenario: Idempotent notify replay

- **WHEN** alipay re-POSTs the same notify (their docs allow up to 8 retries over 26 hours)
- **AND** the order is already in `completed` status
- **THEN** the system SHALL respond `success` without re-provisioning
- **AND** SHALL NOT publish duplicate events

#### Scenario: Payment expired

- **WHEN** an order remains in `payment_pending` for more than 15 minutes
- **THEN** the payment-poll job SHALL transition it to `payment_expired`
- **AND** SHALL publish `order.payment_expired`

### Requirement: Purchase Endpoints

The system SHALL expose payment-method-specific purchase endpoints
so adding a new provider doesn't mutate the existing balance path.

#### Scenario: Balance purchase (existing, unchanged)

- **WHEN** a user posts to `POST /api/user/billing/purchase`
- **THEN** the system SHALL run the balance-deduct + provision flow as before

#### Scenario: Alipay purchase

- **WHEN** a user posts to `POST /api/user/billing/purchase/alipay` with `{plan_id, idempotency_key, node_id, inbound_tag}`
- **THEN** the system SHALL create a payment_pending order and return `{order_id, qr_url, expires_at}`

#### Scenario: Payment methods discovery

- **WHEN** a user GETs `/api/user/billing/payment-methods`
- **THEN** the system SHALL return the list of configured payment methods, always including `"balance"` and additionally each provider whose `Enabled()` returns true

## ADDED Requirements

### Requirement: Order Payment Columns

The `orders` table SHALL carry payment-provider state alongside the
original order fields. Columns SHALL NOT be NULLable for orders
created post-migration; the migration SHALL set defaults so historical
balance orders read back as `payment_method='balance'`.

#### Scenario: Migration backfills payment_method

- **WHEN** migration `0006_orders_payment_method` runs against a DB with pre-existing balance orders
- **THEN** every existing row SHALL read back with `payment_method='balance'` (via column DEFAULT)
- **AND** the new `payment_provider_order_id`, `payment_qr_url`, `payment_expires_at` columns SHALL be empty / NULL for those rows

### Requirement: Async Payment Notify Endpoint

The system SHALL expose `POST /api/public/payment/<provider>/notify`
endpoints, one per payment provider. These routes SHALL be public
(no JWT) and SHALL be authenticated by the provider's signature
scheme alone.

#### Scenario: Alipay notify with valid signature

- **WHEN** alipay's servers POST `application/x-www-form-urlencoded` to `/api/public/payment/alipay/notify` with a valid RSA2 signature
- **THEN** the system SHALL respond `200 OK` with body `success` (plain text, exact)
- **AND** SHALL advance the order per the trade_status

#### Scenario: Alipay notify with invalid signature

- **WHEN** the signature fails verification
- **THEN** the system SHALL respond `400 Bad Request`
- **AND** SHALL NOT advance any order
- **AND** SHALL log the rejection at warn level with the source IP
