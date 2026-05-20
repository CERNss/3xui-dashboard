# Design — add-payment-alipay

## Architecture in one diagram

```
                     ┌─────────────────────────────────────────┐
                     │            Portal (Vue)                 │
                     │  Plans.vue → POST purchase/alipay       │
                     │   ← { qr_url, order_id, expires_at }   │
                     │  Renders QR via `qrcode` (same pkg as   │
                     │  subscription page)                     │
                     └─────────────────────────────────────────┘
                                       │ POST
                                       ▼
        ┌──────────────────────────────────────────────────────────┐
        │                   Billing Service                        │
        │  Purchase(plan, method=alipay):                          │
        │   1. Insert order (status=payment_pending)              │
        │   2. Call AlipayGateway.Precreate(order)                │
        │   3. Persist qr_url + expires_at                        │
        │   4. Return { order, qr_url }                           │
        │                                                          │
        │  ConfirmPayment(order_id, alipay_trade_no):              │
        │   1. Idempotent: bail if order already completed        │
        │   2. Advance order: payment_pending → paid              │
        │   3. Bus.Publish(order.payment_confirmed)               │
        │   4. ClientProvisioning.Create() (same flow as balance)│
        │   5. Mark completed + persist client_ownership_id       │
        └──────────────────────────────────────────────────────────┘
              │ HTTP            ▲ verify+route         ▲ cron poll
              ▼                 │                      │
        ┌─────────────┐  ┌───────────────────┐  ┌──────────────────┐
        │ Alipay      │  │ POST /payment/    │  │ payment-poll job │
        │ Gateway     │  │   alipay/notify   │  │ @every 30s:      │
        │ (sandbox or │  │ - RSA2 verify    │  │ trade.query for  │
        │  prod URL)  │  │ - Idempotency by │  │ pending<15min    │
        └─────────────┘  │   alipay_trade_no│  │ (failsafe vs     │
              ▲          └───────────────────┘  │  dropped notify) │
              │                                  └──────────────────┘
              └─── trade.precreate ──── trade.query ────────────────
```

## Why a standalone gateway package instead of folding into billing

`add-payment-stripe` is next on the queue. If we cram alipay logic
inside `service/billing`, Stripe ends up there too, and the
service becomes a junk drawer of provider-specific quirks. By
making `service/payment/alipay/` self-contained — gateway client
behind a small interface — we get:

- Billing depends on `payment.Gateway` (interface), not alipay.
- Each provider's RSA / HMAC / OAuth quirks stay local.
- Tests for billing mock the gateway with a stub; tests for
  alipay test alipay's crypto + parser in isolation.
- Adding Stripe = new package + register provider in app.go,
  zero touches in billing.

## Order state machine

Existing states: `pending`, `completed`, `failed`, `refunded`.

New states:

- `payment_pending` — order created, QR served, waiting for alipay
- `paid` — alipay confirmed, awaiting provisioning. Transient.
- `payment_failed` — alipay rejected the trade.
- `payment_expired` — > 15 min elapsed, no confirmation.

Transitions:

```
                                   provisioning ok
   payment_pending ───alipay───►   paid ──────────►   completed
        │              ok                │
        │                                │ provisioning fail
        │                                ▼
        │                              refunded (manual op)
        │
        │ alipay reject
        ├──────────►   payment_failed (terminal)
        │
        │ 15 min, no result
        └──────────►   payment_expired (terminal)
```

The pre-existing `pending` → `completed` / `failed` path is
balance-based and untouched. The payment_*  states never appear on
balance-pay orders.

## Idempotency

Three layers, ordered cheapest-first:

1. **HTTP idempotency_key** at purchase time (already enforced by
   `orders.idempotency_key` unique index from the original schema).
2. **Alipay `out_trade_no`** — we use the order ID as out_trade_no.
   If alipay sees a duplicate Precreate with the same out_trade_no
   for an unpaid order, they return the same QR. Free retry.
3. **Notify-side guard** — `ConfirmPayment` checks order status
   before advancing. A second notify (alipay retries up to 8 times
   over 26 hours per their docs) finds `status=completed` and
   responds `success` without re-provisioning.

## Crypto: pure stdlib, no third-party SDK

Alipay 当面付 needs:

- RSA2 (SHA256withRSA) sign for outbound calls.
- RSA2 verify for inbound notify.
- Canonical string assembly: all non-empty params sorted by key,
  `k=v` joined by `&`, then signed.

`crypto/rsa` + `crypto/sha256` + `encoding/base64` + `encoding/pem`
cover all of this. No alipay SDK dependency.

Why this matters: every alipay Go SDK on github has a different
opinion on JSON encoding, defaults, and time formatting. The
canonical-string algorithm in the alipay docs is simple enough that
implementing it ourselves avoids the SDK churn / vulnerability
surface, AND keeps the code aligned with the reference docs anyone
debugging would reach for.

## Sandbox vs production

One env var (`ALIPAY_GATEWAY`) determines the endpoint. The
sandbox + production gateways share the same request/response
shape; only the URL and the merchant keypair change. Tests run
against neither — they exercise our request builder + response
parser against fixture payloads captured from the alipay docs.

## Frontend changes (minimal)

- Existing `Plans.vue` gains a "支付方式" picker above the buy
  button: 余额支付 / 支付宝。Default = whichever is configured.
- Picking 支付宝 swaps the confirm flow: instead of deducting
  balance and immediately routing to `/portal/orders`, we open a
  modal containing the alipay QR + an order-status polling
  indicator.
- Polling: portal hits `GET /api/user/billing/orders/:id` every
  3s. On `status=completed` the modal flips to a "支付成功"
  state and redirects to `/portal/orders` after 1s.
- Modal also exposes a `打开支付宝 APP` deep link for mobile
  (the same `qr_code` URL alipay returns is a valid alipay scheme
  URL).

## Backward compatibility

- Existing balance-based purchase endpoint stays at the same URL,
  same wire shape — we add a NEW endpoint for alipay rather than
  mutating the old one.
- Existing orders rows have NULL `payment_method` after the
  migration; queries that filter on it MUST treat NULL as
  "balance" (the migration sets `DEFAULT 'balance'` for new rows
  but doesn't backfill old ones — historical rows are accurate
  without the column anyway).

## What we'll regret if we don't do it this way

- Folding gateways into billing → stripe addition costs 3× more
  than necessary.
- Single endpoint that dispatches by `payment_method` param → too
  much branching, harder to test.
- Polling-only (no notify) → 30s lag on the user-facing "支付
  成功" flip — terrible UX. Notify makes it ~instant; poll just
  cleans up the long tail.
- Trusting alipay's notify without RSA verify → trivial fraud:
  attacker POSTs a fake notify and gets a free plan. RSA verify
  is non-negotiable.
