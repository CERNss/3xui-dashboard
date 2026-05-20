# Tasks — add-payment-alipay

All checkboxes start `[ ]`. Each task is a single PR-able unit;
intermediate `go build ./...` should stay green.

## 1. Config + migration

- [ ] 1.1 Add `Alipay` struct to `internal/config/config.go` with fields
  `AppID, PrivateKey, AlipayPublicKey, Gateway, NotifyURL,
  ReturnURL string`. `splitCSV` not needed — all scalars. `Enabled()`
  returns true iff `AppID + PrivateKey + AlipayPublicKey` all set.
- [ ] 1.2 Wire env reads: `ALIPAY_APP_ID, ALIPAY_PRIVATE_KEY,
  ALIPAY_PUBLIC_KEY, ALIPAY_GATEWAY, ALIPAY_NOTIFY_URL,
  ALIPAY_RETURN_URL`. Default `ALIPAY_GATEWAY` to
  `https://openapi.alipay.com/gateway.do`. Keys are PEM-encoded
  (operators paste the whole `-----BEGIN ... END-----` block).
- [ ] 1.3 Migration `0006_orders_payment_method.up.sql`:
  ```sql
  ALTER TABLE orders
    ADD COLUMN payment_method TEXT NOT NULL DEFAULT 'balance',
    ADD COLUMN payment_provider_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN payment_qr_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN payment_expires_at TIMESTAMPTZ;
  CREATE INDEX orders_payment_provider_order_id
    ON orders (payment_provider_order_id)
    WHERE payment_provider_order_id <> '';
  ```
  And `.down.sql` mirroring.
- [ ] 1.4 Model: add the 4 fields to `model/Order`. `PaymentExpiresAt *time.Time`.

## 2. Alipay crypto (`internal/service/payment/alipay/crypto.go`)

- [ ] 2.1 `SignRSA2(params map[string]string, pemPrivateKey string) (string, error)`:
  - Sort keys, drop empty values, join `k=v` with `&`
  - SHA256withRSA sign, base64-encode result
- [ ] 2.2 `VerifyRSA2(params map[string]string, signature, pemPublicKey string) error`:
  - Same canonical string algorithm
  - Drop `sign` and `sign_type` keys (per alipay spec)
  - Return wrapped error on bad signature
- [ ] 2.3 Tests: known-good keypair + canonical string → known signature
  (fixture captured from alipay docs example). Verify with the public
  key, mutate one char, expect rejection.

## 3. Alipay HTTP client (`internal/service/payment/alipay/client.go`)

- [ ] 3.1 `Client` struct: `appID, privateKey, publicKey, gateway,
  notifyURL string; httpClient *http.Client`. Constructor takes
  these from config; default httpClient = `&http.Client{Timeout: 10s}`
  so tests can swap a stub.
- [ ] 3.2 `precreate(ctx, req PrecreateRequest) (*PrecreateResponse, error)`:
  - Build the alipay envelope: app_id, method=`alipay.trade.precreate`,
    charset=utf-8, sign_type=RSA2, timestamp (server now in
    Asia/Shanghai), version=1.0, biz_content (JSON of req)
  - Sign, POST application/x-www-form-urlencoded
  - Parse response; verify alipay's sign on the outer envelope
  - Map alipay error codes to typed errors (`ErrSignatureFailed`,
    `ErrInvalidParameter`, …) so callers don't `strings.Contains` on
    Chinese error messages
- [ ] 3.3 `tradeQuery(ctx, outTradeNo string) (*QueryResponse, error)`:
  - Same envelope shape, method=`alipay.trade.query`
  - Returns `{TradeStatus, TradeNo, BuyerLogonID, TotalAmount}`
- [ ] 3.4 Tests use httptest.NewServer; fixtures for: precreate success,
  precreate signature failure, query found+paid, query found+pending,
  query not-found. No real network calls.

## 4. Gateway interface + registry (`internal/service/payment/gateway.go`)

- [ ] 4.1 Interface:
  ```go
  type Gateway interface {
      Provider() string  // "alipay", "stripe", …
      CreatePayment(ctx, order *model.Order) (qrURL string, expiresAt time.Time, providerOrderID string, err error)
      Query(ctx, providerOrderID string) (status string, err error)  // "paid"|"pending"|"failed"
      VerifyNotify(params map[string]string, signature string) error
  }
  ```
- [ ] 4.2 Registry: `map[string]Gateway` keyed by provider name.
  Built from config — only enabled providers (alipay.Enabled() true)
  get registered.
- [ ] 4.3 `service/payment/alipay.New(cfg config.Alipay) Gateway` returns
  the alipay impl wrapping `Client`.

## 5. Billing service: alipay purchase flow

- [ ] 5.1 `BillingService.PurchaseViaPayment(ctx, userID, planID,
  idempotencyKey, nodeID, inboundTag, providerName) (*OrderWithPayment, error)`:
  - Look up plan, validate enabled
  - Insert order with `status=payment_pending, payment_method=provider`
  - Resolve gateway from registry (404 if not configured)
  - Call `gateway.CreatePayment(ctx, order)`
  - Persist qr_url + payment_expires_at + payment_provider_order_id
  - Return order + qr_url
- [ ] 5.2 `BillingService.ConfirmPayment(ctx, providerOrderID string)`:
  - Find order by `payment_provider_order_id`
  - Status guard: if already `completed`/`refunded`/`payment_failed`, no-op
  - Transition `payment_pending` → `paid`
  - Bus.Publish(`order.payment_confirmed`)
  - Provision client (same code path as balance-pay)
  - Mark `completed` + persist client_ownership_id
  - All wrapped in one DB transaction
- [ ] 5.3 `BillingService.FailPayment(ctx, providerOrderID, reason string)`:
  - Transition `payment_pending` → `payment_failed`
  - Bus.Publish(`order.payment_failed`)
- [ ] 5.4 Add `OrderStatusPaymentPending, OrderStatusPaymentFailed,
  OrderStatusPaymentExpired, OrderStatusPaid` constants to model.
- [ ] 5.5 Tests: precreate ok → confirm idempotent (2nd call no-op);
  failed precreate leaves order in `payment_failed`; query returns
  failed → order moves to payment_failed.

## 6. HTTP handlers

- [ ] 6.1 `user.BillingHandler.purchaseAlipay`:
  - Route: `POST /api/user/billing/purchase/alipay`
  - Body: `{plan_id, idempotency_key, node_id, inbound_tag}` (same as
    balance purchase minus the implicit balance check)
  - Response: `{order_id, qr_url, expires_at}`
- [ ] 6.2 `public.PaymentNotifyHandler.alipayNotify`:
  - Route: `POST /api/public/payment/alipay/notify`
  - Parse `application/x-www-form-urlencoded` body
  - Verify signature via `gateway.VerifyNotify`
  - On `trade_status=TRADE_SUCCESS` or `TRADE_FINISHED` →
    `BillingService.ConfirmPayment`
  - Respond `success` (plaintext, exactly that string — alipay's
    contract; anything else triggers retry)
- [ ] 6.3 Tests: notify with valid signature advances order; notify
  with mutated signature returns 400 and does NOT advance; notify
  for non-existent order returns 200 `success` (alipay must not
  retry a permanent miss).

## 7. Payment-poll cron job

- [ ] 7.1 `internal/job/payment_poll.go` — `PaymentPollJob`. Runs every 30s.
  - Find orders with `status=payment_pending AND created_at > now - 15min`
  - For each, call `gateway.Query(providerOrderID)`
  - If `paid` → `BillingService.ConfirmPayment`
  - If `failed` → `BillingService.FailPayment`
- [ ] 7.2 Job also handles expiry: orders with
  `status=payment_pending AND created_at <= now - 15min` get marked
  `payment_expired`.
- [ ] 7.3 Wire in `app.go`: `scheduler.Add("payment-poll", "@every 30s", paymentPollJob.RunOnce)`.
- [ ] 7.4 Tests: pending+paid in alipay → confirms; pending+pending → no-op;
  pending older than 15min → expires.

## 8. Frontend

- [ ] 8.1 `frontend/src/api/portal/billing.ts`: add
  ```ts
  purchaseAlipay({plan_id, idempotency_key, node_id, inbound_tag}):
      Promise<{order_id, qr_url, expires_at}>
  ```
- [ ] 8.2 `Plans.vue`: payment method picker above buy button. If only
  one method is configured backend-side, hide the picker. Backend
  exposes available methods via `GET /api/user/billing/payment-methods`
  (returns `["balance"]` or `["balance","alipay"]`).
- [ ] 8.3 New component `frontend/src/components/portal/AlipayPayModal.vue`:
  - Render QR via existing `qrcode` package
  - Poll `GET /api/user/billing/orders/:id` every 3s
  - On `status=completed` → flip to success state + redirect
  - On `status=payment_failed|payment_expired` → flip to error state
  - On QR expires_at passed → "二维码已过期" + retry button (creates
    a new order)
- [ ] 8.4 Mobile deep-link button: `<a :href="qr_url">打开支付宝 APP</a>`
  — the `qr_code` URL alipay returns IS the alipay scheme link.

## 9. Spec deltas + ROADMAP

- [ ] 9.1 Update `openspec/changes/add-payment-alipay/specs/billing-and-plans/spec.md`
  with MODIFIED requirements for the new states + endpoints.
- [ ] 9.2 Write `openspec/specs/payment-gateway-alipay/spec.md`
  (NEW capability — promoted from the change's spec delta on merge).
- [ ] 9.3 Update `ROADMAP.md`: 支付系统 20% → 45%, mark #5 ✅.

## 10. Documentation

- [ ] 10.1 `docs/operator/alipay-setup.md`: walkthrough for getting
  app_id + uploading the RSA key + finding the alipay public key in
  the open platform UI. Screenshots optional.
- [ ] 10.2 `.env.example` block for ALIPAY_* vars.
