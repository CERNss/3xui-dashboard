# payment-gateway-alipay

Alipay 当面付 (face-to-face / QR-based payment) gateway. Self-
contained in `internal/service/payment/alipay/` behind the generic
`payment.Gateway` interface — adding additional providers (Stripe,
WeChat Pay) does NOT require changes to this code.

## Purpose & boundaries

- **Owns**: outbound `alipay.trade.precreate` + `alipay.trade.query`
  HTTP calls; RSA2 signing of requests; verification of alipay-platform
  signatures on responses + async notifies; canonical-string algorithm
  per alipay's docs.
- **Does NOT own**: order state machine (lives in `billing-and-plans`),
  the notify HTTP endpoint (lives in `webhook-notifications`-adjacent
  `handler/public`), or any UI surface.

## Requirements

### Requirement: Alipay 当面付 Integration

The system SHALL integrate Alipay's 当面付 product as a payment
gateway, behind the generic `payment.Gateway` interface.

#### Scenario: Precreate generates a QR

- **WHEN** the billing service calls `gateway.CreatePayment(order)` for an alipay-method order
- **THEN** the gateway SHALL POST `alipay.trade.precreate` to the configured Alipay gateway URL
- **AND** SHALL sign the request envelope with RSA2 (SHA256withRSA) using the operator's private key
- **AND** SHALL pass the order ID as `out_trade_no`
- **AND** SHALL pass `subject = plan.name` and `total_amount` in yuan to two decimal places
- **AND** SHALL render `timestamp` + `time_expire` in Beijing time (CST/UTC+8) since alipay parses both as local time

#### Scenario: Precreate rejects unsigned response

- **WHEN** the alipay gateway returns a response missing the `sign` envelope field
- **THEN** the gateway SHALL return an error wrapping `ErrSignatureFailed`
- **AND** SHALL NOT advance the order

#### Scenario: Trade query failsafe

- **WHEN** the payment-poll job calls `gateway.Query(provider_order_id)`
- **THEN** the gateway SHALL POST `alipay.trade.query` to the Alipay gateway URL
- **AND** SHALL map `TRADE_SUCCESS` / `TRADE_FINISHED` → `payment.StatusPaid`
- **AND** SHALL map `WAIT_BUYER_PAY` → `payment.StatusPending`
- **AND** SHALL map `TRADE_CLOSED` → `payment.StatusFailed`

#### Scenario: Notify signature verification

- **WHEN** the notify endpoint passes the parsed form params + `sign` value to `gateway.VerifyNotify`
- **THEN** the gateway SHALL drop `sign` and `sign_type` keys
- **AND** SHALL sort remaining keys alphabetically, drop empty values, join `k=v` with `&`
- **AND** SHALL verify with RSA2 using the alipay-platform public key
- **AND** SHALL return nil on valid signature, an error wrapping `ErrSignatureFailed` otherwise

### Requirement: Configuration

The gateway SHALL be opt-in via environment variables. Absence of
required vars SHALL leave the provider unregistered.

#### Scenario: Required env vars

- **WHEN** any of `ALIPAY_APP_ID`, `ALIPAY_PRIVATE_KEY`, `ALIPAY_PUBLIC_KEY` is empty
- **THEN** the alipay gateway SHALL NOT register
- **AND** `GET /api/user/payment-methods` SHALL omit `"alipay"`

#### Scenario: Sandbox vs production

- **WHEN** `ALIPAY_GATEWAY` is unset
- **THEN** the gateway SHALL default to `https://openapi.alipay.com/gateway.do` (production)
- **WHEN** `ALIPAY_GATEWAY` is set to the sandbox URL
- **THEN** the gateway SHALL use that URL for all requests with no other behavior change

### Requirement: Pure-Stdlib Crypto

The gateway SHALL implement RSA2 sign + verify using only the Go
standard library (`crypto/rsa`, `crypto/sha256`, `encoding/pem`,
`encoding/base64`). The system SHALL NOT depend on any third-party
alipay SDK.

#### Scenario: Sign roundtrips with a generated keypair

- **WHEN** the gateway signs a canonical-form params map with a freshly-generated RSA-2048 private key
- **AND** the matching public key verifies the same params + signature
- **THEN** verification SHALL succeed

#### Scenario: Verify rejects tampered payload

- **WHEN** any byte of the canonical string is altered after signing
- **THEN** verification SHALL fail with a wrapped `rsa.VerificationError`

### Requirement: Beijing Timezone for Wire Timestamps

The gateway SHALL convert all timestamp + expiry values to Beijing
local time (Asia/Shanghai, UTC+8) before formatting them for the
wire. The wire format `2006-01-02 15:04:05` has no offset suffix —
alipay's servers interpret it as Beijing local.

#### Scenario: UTC server emits Beijing timestamp

- **GIVEN** the server clock reads `2026-05-20 06:00:00 UTC`
- **WHEN** the gateway constructs a precreate request
- **THEN** the `timestamp` form value SHALL be `2026-05-20 14:00:00` (the matching Beijing wall clock)
- **AND** SHALL NOT be the UTC wall clock value
