# Design — add-payment-stripe

## Why this is mostly plumbing

#5 (alipay) did the heavy lifting: the `payment.Gateway` interface,
the order state machine (payment_pending/paid/payment_failed/
payment_expired), the registry, the poll job, the frontend
payment-method picker. Stripe is a second Gateway implementation
that fits the same shape. Total new surface:

- New package `service/payment/stripe/` (~250 lines)
- New webhook route on the existing public handler (~80 lines)
- One env-var block in config (~15 lines)
- Frontend: change AlipayPayModal → a router that picks "QR modal
  for alipay" or "redirect to URL for stripe" (~30 lines)

No schema migration, no new billing methods, no new state in the
order machine.

## Differences from alipay (callouts)

| Dimension | Alipay 当面付 | Stripe Checkout |
|---|---|---|
| UX | QR code shown in modal, user scans with phone | Redirect user to hosted Checkout URL |
| Auth (outbound) | RSA2 sign of canonical params | Bearer token (Stripe-Version + secret key) |
| Auth (inbound webhook) | RSA2 verify of canonical params | HMAC-SHA256 over raw body + timestamp |
| Wire format (outbound) | application/x-www-form-urlencoded | application/x-www-form-urlencoded |
| Wire format (inbound) | application/x-www-form-urlencoded | application/json |
| Currency | CNY only | Configurable (USD default) |
| Session expiry | 15 min via TimeExpire param | 24 hours default; we override via env var |
| Idempotency on create | out_trade_no = order ID | Idempotency-Key header = order ID + version |

## Webhook signature verification (the only non-obvious bit)

Stripe signs the raw POST body with HMAC-SHA256 + the webhook
signing secret. The signature header `Stripe-Signature` looks like:

```
t=1492774577,v1=5257a869e7ecebed...,v0=...
```

Verification:
1. Parse `t` and `v1` from the header
2. Build the signed payload string: `${t}.${rawBody}`
3. Compute HMAC-SHA256(signed_payload, webhook_secret)
4. Constant-time compare against `v1`
5. Reject if `t` is more than 5 minutes old (replay protection)

`crypto/hmac` + `crypto/sha256` from stdlib. The trickiest part is
making sure Gin doesn't eat the body before we read it raw — we
read with `io.ReadAll(c.Request.Body)` BEFORE any JSON binding,
then re-create the body for downstream handlers if needed (the
webhook is the only thing on that route, so we don't need to).

## Idempotency strategy

Three layers, same shape as alipay:

1. **Local order idempotency_key** (already enforced via unique
   index from the original schema).
2. **Stripe `Idempotency-Key` header** on Checkout Session create —
   we use `order-${orderID}-v1` so retrying the same request
   returns the same session URL.
3. **Webhook guard** — `ConfirmPayment` uses status-guarded
   transition. A second `checkout.session.completed` (Stripe
   retries on 5xx for ~3 days) finds status=completed and no-ops.

## Why no stripe-go SDK

Same calculus as alipay. Pros of using stripe-go:
- Maintained by Stripe, follows API changes
- Typed responses
- Built-in retries with backoff

Cons:
- Big surface area (~80 packages); pulls in `time/rate`,
  `golang.org/x/text`, etc.
- Mixed responsibility: SDK + idiomatic types + HTTP client
- We use 2 endpoints + 1 webhook event verifier — a 250-line
  custom client is faster to read than stripe-go's call graph
- Update churn: stripe-go releases ~weekly; each release is a
  go.mod bump we'd have to review

We re-evaluate the moment we use a third Stripe API surface
(Subscriptions, Tax, etc.) — that's the inflection where stdlib
starts costing more than it saves.

## Currency handling

`price_cents` in our DB is the integer-cents amount the plan
costs. For alipay we render that as `元.分` (¥12.34). For Stripe
we pass it as `unit_amount` (integer smallest unit) + `currency`
(ISO 4217 lowercase: `usd`, `cny`, `gbp`).

The currency is set ONCE per deployment via `STRIPE_CURRENCY`. If
the operator wants to charge in multiple currencies, they need to
either:
- Run two deployments
- Wait for `add-multi-currency-pricing` (separate change, big
  scope — needs per-plan price-in-currency columns)

Stripe Checkout supports automatic localization of the payment
page UI based on user locale; we get that for free.

## Frontend change is small

`AlipayPayModal.vue` becomes `PaymentRedirect.vue` which:
- For `payment_method=alipay` → renders the QR modal as today
- For `payment_method=stripe` → renders a brief "正在跳转到
  Stripe..." message + immediately sets `location.href = qr_url`
  (Stripe's Checkout `url` lives in the same column)

OR we keep the QR modal alipay-specific and route Stripe through
a different code path. Cleaner: rename the modal generic and
branch internally on the order's payment_method.

## What we'll regret if we don't do it this way

- **Using stripe-go** — saves nothing for a 2-endpoint integration
  and adds a heavy dep + auto-update treadmill.
- **Hand-rolling SCA in PaymentIntents** — Checkout solves SCA for
  free; PaymentIntents is the wrong starting point.
- **Trusting the webhook without HMAC verify** — same as alipay:
  free plans for any attacker who finds the URL.
- **Re-using `payment_qr_url` for non-QR data** — slightly
  weird naming, but adding a new column means another migration
  for what's semantically "the redirect target the frontend uses
  to complete payment". Rename to `payment_redirect_url` could
  happen in a future cleanup change.
