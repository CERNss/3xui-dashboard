# Tasks — add-payment-stripe

## 1. Config

- [ ] 1.1 Add `Stripe` struct to `internal/config/config.go`:
  - `SecretKey` (`sk_test_...` / `sk_live_...`)
  - `WebhookSecret` (`whsec_...`)
  - `Currency` (ISO 4217 lowercase, defaults `usd`)
  - `SuccessURL` (where Stripe redirects after success)
  - `CancelURL` (where Stripe redirects on cancel)
  - `SessionExpiryMinutes` (defaults 30)
- [ ] 1.2 `Enabled()` returns true iff SecretKey + WebhookSecret set.
- [ ] 1.3 Env vars: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`,
  `STRIPE_CURRENCY`, `STRIPE_SUCCESS_URL`, `STRIPE_CANCEL_URL`,
  `STRIPE_SESSION_EXPIRY_MINUTES`.

## 2. Stripe HTTP client (`internal/service/payment/stripe/client.go`)

- [ ] 2.1 `Client` struct: `secretKey, baseURL, currency string;
  http *http.Client; now func() time.Time`.
- [ ] 2.2 `CreateCheckoutSession(ctx, req)` → POST
  `/v1/checkout/sessions` with Idempotency-Key header. Returns
  `{ID, URL, ExpiresAt}`.
- [ ] 2.3 `RetrieveCheckoutSession(ctx, id)` → GET
  `/v1/checkout/sessions/{id}`. Returns the session's
  `payment_status`.
- [ ] 2.4 Auth: `Authorization: Bearer ${SecretKey}` +
  `Stripe-Version: 2024-06-20` (pin to a known-good version so
  Stripe doesn't break us by updating defaults).

## 3. Webhook verification (`internal/service/payment/stripe/webhook.go`)

- [ ] 3.1 `VerifyWebhook(rawBody []byte, sigHeader, secret string) error`:
  - Parse `t=...,v1=...` from header
  - Build signed payload: `${t}.${rawBody}`
  - HMAC-SHA256 with secret
  - Constant-time compare against `v1`
  - Reject if `t` is more than 5 minutes old (replay protection)
- [ ] 3.2 Tests: signature roundtrip, tampered body, tampered
  signature, replay (old `t`), missing `t` / `v1`.

## 4. Gateway adapter (`internal/service/payment/stripe/gateway.go`)

- [ ] 4.1 `Gateway` struct adapts `Client` + webhook verifier to
  `payment.Gateway`.
- [ ] 4.2 `New(cfg config.Stripe) payment.Gateway` returns nil if
  not configured.
- [ ] 4.3 `Provider() string` returns `"stripe"`.
- [ ] 4.4 `CreatePayment` calls `CreateCheckoutSession`, returns
  `payment.CreateResult{QRURL: session.URL, ProviderOrderID:
  session.ID, ExpiresAt: session.ExpiresAt}`. (QRURL is misnamed
  for Stripe — it holds the redirect URL — but we re-use the
  column rather than adding a new one. Documented in design.md.)
- [ ] 4.5 `Query` maps Stripe's `payment_status`:
  - `paid` → `payment.StatusPaid`
  - `unpaid` → `payment.StatusPending`
  - `no_payment_required` → `payment.StatusPaid`
- [ ] 4.6 `VerifyNotify` is unused for Stripe (sig is on raw body
  not params); returns nil so the interface check passes, and the
  webhook handler bypasses this method.

## 5. Public webhook handler

- [ ] 5.1 Add `POST /api/public/payment/stripe/webhook` to
  `internal/handler/public/payment.go`:
  - Read raw body with `io.ReadAll` BEFORE any JSON parsing
  - Verify HMAC via the Stripe gateway's `VerifyWebhook`
  - Parse the event JSON
  - Dispatch on `event.type`:
    - `checkout.session.completed` → ConfirmPayment(session.id)
    - `checkout.session.async_payment_failed` → FailPayment
    - `checkout.session.expired` → ExpirePayment by lookup
- [ ] 5.2 Respond 200 with `{}` on accept; 400 on bad signature.

## 6. Wiring

- [ ] 6.1 `app.go`:
  ```go
  paymentRegistry.Register(stripe.New(cfg.Stripe))
  ```
  Plus pass the cfg through to the webhook handler.
- [ ] 6.2 Webhook handler needs a reference to the Stripe gateway
  to call its `VerifyWebhook` — extend the handler constructor
  to take the registry directly.

## 7. Frontend

- [ ] 7.1 `frontend/src/api/portal/billing.ts`: extend
  `PaymentMethod` type with `'stripe'`. Existing
  `purchaseViaPayment` already takes a provider param.
- [ ] 7.2 `Plans.vue`: when `paymentMethods.includes('stripe')`,
  the picker shows a "Stripe" option. On select + buy, the order
  response has `payment_qr_url = stripe checkout URL` — branch:
  - alipay: open `AlipayPayModal`
  - stripe: `window.location.href = order.payment_qr_url`
- [ ] 7.3 Stripe success URL (configured server-side via
  STRIPE_SUCCESS_URL) should point to `/portal/orders?stripe=ok`
  so the user lands on their order list with a flash. Cancel URL
  → `/portal/plans?stripe=cancel`.

## 8. Spec deltas + ROADMAP

- [ ] 8.1 Update `openspec/changes/add-payment-stripe/specs/billing-and-plans/spec.md`
  with the additional purchase endpoint scenario for stripe.
- [ ] 8.2 Write `openspec/changes/add-payment-stripe/specs/payment-gateway-stripe/spec.md`
  (NEW capability — promoted to `openspec/specs/payment-gateway-stripe/`
  on merge).
- [ ] 8.3 Update `ROADMAP.md`: 支付系统 45% → 60%, mark #6 ✅.

## 9. Documentation

- [ ] 9.1 `.env.example`: add the STRIPE_* block.
- [ ] 9.2 Brief note in proposal/design that this design intentionally
  excludes Subscriptions / recurring billing — that's #future.
