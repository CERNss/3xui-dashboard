# add-payment-stripe

## Why

Alipay covers the Chinese-mainland userbase but locks out everyone
else: no overseas card, no PayPal-style flow, no SEPA, no Apple Pay.
Per the ROADMAP, 支付系统 is at 45% post-alipay; Stripe is the
single biggest lever for getting to ~60% because it unlocks USD /
EUR / GBP + cards + Apple Pay / Google Pay through one integration.

Reference: <https://docs.stripe.com/checkout/quickstart>. We're using
**Checkout Sessions** (hosted, redirect-based) — the lowest-friction
Stripe surface. The PaymentIntents flow gives finer control but
requires our own card form + Strong Customer Authentication handling,
which is several weeks more work for the same conversion rate.

## What Changes

### New capability: `payment-gateway-stripe`

Drops in alongside `payment-gateway-alipay` behind the same
`payment.Gateway` interface. The alipay change already paid this
abstraction cost; Stripe just registers itself.

- **`internal/service/payment/stripe/`** — request builder + HMAC
  webhook verification + response parser. **Pure stdlib, no
  stripe-go dependency** — same rationale as alipay (small surface,
  no SDK lock-in, audit-friendly).
- **`POST /v1/checkout/sessions`** — creates the hosted Checkout
  page. Returns a `url` we redirect the user to. We pass
  `client_reference_id = orderID` so the webhook payload links
  back to our order.
- **`GET /v1/checkout/sessions/{id}`** — failsafe status poll.
- **Webhook endpoint** — `POST /api/public/payment/stripe/webhook`
  validates the `Stripe-Signature` HMAC over the raw body + a
  timestamp, then dispatches on `event.type`:
  - `checkout.session.completed` → ConfirmPayment
  - `checkout.session.async_payment_failed` → FailPayment
  - `checkout.session.expired` → FailPayment (with reason)

### Modified capability: `billing-and-plans`

No schema changes — the migration from #5 already added every
column we need (`payment_method`, `payment_provider_order_id`,
`payment_qr_url`, `payment_expires_at`). For Stripe,
`payment_qr_url` holds the Checkout `url` (we reuse the column;
the frontend treats it as a redirect target instead of a QR
source when `payment_method=stripe`).

- **Endpoint already exists**: `POST /api/user/billing/purchase/:provider`
  dispatches by route param — passing `stripe` works as soon as
  the gateway is registered.
- **Payment-poll job** already handles all providers in the
  registry; Stripe rows get polled the same way alipay rows do.

## Out of scope

- **Subscription billing** — Stripe's Subscriptions product needs
  webhook handling for `invoice.paid` / `invoice.payment_failed`
  on a recurring schedule. We defer to `add-billing-auto-renewal`
  which would build that on top of any gateway, not just Stripe.
- **Multi-currency picker** — Stripe is single-currency per
  Checkout Session. The operator picks one currency via
  `STRIPE_CURRENCY` env var; users see plans in that currency.
  Multi-currency is a separate change.
- **Strong Customer Authentication (SCA) UX customization** —
  Checkout handles this automatically; PaymentIntents would
  require us to.

## Assumptions called out

- Operator registers at <https://dashboard.stripe.com>, gets:
  - Publishable key (`pk_live_...` / `pk_test_...`) — not used
    server-side, frontend doesn't render any Stripe UI either
    since we redirect to Checkout
  - Secret key (`sk_live_...` / `sk_test_...`) — server uses this
    to call the Stripe API
  - Webhook signing secret (`whsec_...`) — server verifies inbound
    webhooks with this
- Default currency = USD. Plan `price_cents` is the amount in the
  smallest currency unit (cents for USD, pence for GBP). The
  operator is responsible for making sure plan prices make sense
  in the configured currency (no automatic conversion).
- Stripe API base URL is `https://api.stripe.com` (no sandbox
  toggle — Stripe distinguishes test vs live by the secret-key
  prefix, not by URL).
- Webhook is **public** (Stripe's IPs vary). Auth is by HMAC
  signature on the raw body — same pattern as alipay's RSA notify.
