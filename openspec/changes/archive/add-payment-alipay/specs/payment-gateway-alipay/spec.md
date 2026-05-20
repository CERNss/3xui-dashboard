## ADDED Requirements

### Requirement: Alipay 当面付 Integration

The system SHALL integrate Alipay's 当面付 (face-to-face payment)
product as a payment gateway, behind the generic `payment.Gateway`
interface. The gateway implementation SHALL be self-contained in
`internal/service/payment/alipay/` so adding additional providers
(Stripe, WeChat Pay) does not require changes to the alipay code.

#### Scenario: Precreate generates a QR

- **WHEN** the billing service calls `gateway.CreatePayment(order)` for an alipay-method order
- **THEN** the gateway SHALL POST `alipay.trade.precreate` to the configured Alipay gateway URL
- **AND** SHALL sign the request envelope with RSA2 (SHA256withRSA) using the operator's private key
- **AND** SHALL pass the order ID as `out_trade_no`
- **AND** SHALL pass `subject = plan.name` and `total_amount` in yuan to two decimal places
- **AND** SHALL return `(qr_code_url, expires_at, alipay_trade_no, nil)` on success

#### Scenario: Precreate rejects unsigned response

- **WHEN** the alipay gateway returns a response missing the `sign` envelope field
- **THEN** the gateway SHALL return an error wrapping `ErrSignatureFailed`
- **AND** SHALL NOT advance the order

#### Scenario: Trade query failsafe

- **WHEN** the payment-poll job calls `gateway.Query(provider_order_id)`
- **THEN** the gateway SHALL POST `alipay.trade.query` to the Alipay gateway URL
- **AND** SHALL return one of: `"paid"`, `"pending"`, `"failed"`, `"expired"`
- **AND** SHALL return `("pending", nil)` when alipay reports `WAIT_BUYER_PAY`
- **AND** SHALL return `("paid", nil)` when alipay reports `TRADE_SUCCESS` or `TRADE_FINISHED`

#### Scenario: Notify signature verification

- **WHEN** the notify endpoint passes the parsed form params + the `sign` value to `gateway.VerifyNotify`
- **THEN** the gateway SHALL drop the `sign` and `sign_type` keys
- **AND** SHALL build the canonical string by sorting remaining keys alphabetically, dropping empty values, joining `k=v` with `&`
- **AND** SHALL verify with RSA2 using the alipay platform public key
- **AND** SHALL return nil on valid signature, error otherwise

### Requirement: Configuration

The gateway SHALL be opt-in via environment variables. Absence of
required vars SHALL leave the provider unregistered (the billing
service hides it from `/payment-methods` and rejects purchases via
the route).

#### Scenario: Required env vars

- **WHEN** any of `ALIPAY_APP_ID`, `ALIPAY_PRIVATE_KEY`, `ALIPAY_PUBLIC_KEY` is empty
- **THEN** the alipay gateway SHALL NOT register
- **AND** `GET /api/user/billing/payment-methods` SHALL omit `"alipay"`

#### Scenario: Sandbox vs production

- **WHEN** `ALIPAY_GATEWAY` is unset
- **THEN** the gateway SHALL default to `https://openapi.alipay.com/gateway.do` (production)
- **WHEN** `ALIPAY_GATEWAY` is set to the sandbox URL
- **THEN** the gateway SHALL use that URL for all requests with no other behavior change

### Requirement: Pure-Stdlib Crypto

The gateway SHALL implement RSA2 sign and verify using only the Go
standard library (`crypto/rsa`, `crypto/sha256`, `encoding/pem`,
`encoding/base64`). The system SHALL NOT depend on any third-party
alipay SDK.

#### Scenario: Sign matches alipay reference

- **WHEN** the gateway signs the canonical string from the alipay docs example using the example private key
- **THEN** the resulting signature SHALL byte-match the published reference signature

#### Scenario: Verify rejects tampered payload

- **WHEN** any byte of a previously-verified canonical string is altered
- **AND** the original signature is re-verified against the altered payload
- **THEN** verification SHALL fail with a wrapped `rsa.VerificationError`
