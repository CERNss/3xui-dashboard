# add-payment-alipay

## Why

`billing-and-plans` today supports one purchase path: **deduct from
user balance**. A user without prior balance cannot buy anything,
and there is **zero** way to put money into the balance — the admin
must manually run `adjustBalance` for every user. That's untenable
for any non-toy deployment.

Per `ROADMAP.md`, 支付系统 sits at **20%** ("骨架齐 + 0 个真实
网关"), and 支付宝当面付 is the listed next focus. Alipay is the
default on-ramp for a Chinese-speaking userbase, has a stable
HTTP+RSA2 surface (no SDK lock-in), and 当面付 (alipay.trade.precreate)
gives us a QR code we can render directly in the portal without
redirecting the user off-site.

Reference docs:
- 当面付 API: <https://opendocs.alipay.com/open/194/106078>
- 异步通知 + 验签: <https://opendocs.alipay.com/open/204/105301>
- 主动查询: <https://opendocs.alipay.com/open/api_1/alipay.trade.query>

## What Changes

### New capability: `payment-gateway-alipay`

A standalone payment gateway module — fully self-contained so #6
(`add-payment-stripe`) can slot in next to it without touching the
billing core.

- **`internal/service/payment/alipay/`** — request builder, RSA2
  sign/verify, response parser. Pure logic; no HTTP client coupling
  so unit tests stay deterministic.
- **`alipay.trade.precreate`** — server-side QR creation. Returns
  a `qr_code` URL we encode for the portal QR widget.
- **`alipay.trade.query`** — failsafe status poll for when the
  async notify is dropped (NAT, transient outage). Job-driven; runs
  every 30s for orders in `pending` status younger than 15 min.
- **Async notify endpoint** — `POST /api/public/payment/alipay/notify`
  validates the signature, looks the order up, advances state, and
  responds `success` exactly per Alipay's contract.
- **Sandbox/production toggle** — `ALIPAY_GATEWAY` env var. Empty
  defaults to the production URL; operators set
  `https://openapi-sandbox.dl.alipaydev.com/gateway.do` for staging.

### Modified capability: `billing-and-plans`

The order lifecycle gains a `payment_pending` state and a payment
provider dimension. Purchasing with `payment_method=alipay` no
longer deducts from balance — instead, the order is held at
`payment_pending` until alipay confirms, then provisions normally.
Failed payments stay at `payment_failed` for audit.

- **New columns**: `orders.payment_method` (string),
  `orders.payment_provider_order_id` (string, alipay's `trade_no`),
  `orders.payment_qr_url` (string, the alipay-returned `qr_code`).
- **New endpoint**: `POST /api/user/billing/purchase/alipay`
  returns `{ order_id, qr_url, expires_at }` so the portal renders
  the QR. The existing balance-based `purchase` endpoint stays
  untouched.
- **New event**: `order.payment_confirmed` — fired when alipay
  notify validates, before provisioning runs. Subscribers (webhook,
  notify) can act on payment-only signals.

### New cron job: `payment-poll`

`@every 30s` — for each order in `payment_pending` with
`created_at > now - 15min`, calls `alipay.trade.query`. If alipay
reports `TRADE_SUCCESS` / `TRADE_FINISHED`, advances the order the
same way the notify endpoint would. Pure idempotent failsafe; if
notify already fired, the query just confirms the same state.

After 15 minutes a pending order is marked `payment_expired` —
the QR has expired at that point per Alipay's 2h default cap
anyway, but we expire eagerly so the orders table doesn't accumulate
stuck rows.

## Out of scope

- **Refunds via alipay.trade.refund** — manual admin operation
  for now. Adding a `refund` button is a separate, smaller change.
- **Recurring / auto-renewal** — deferred to
  `add-billing-auto-renewal` (next change). Auto-renewal needs a
  saved-credentials model that alipay 当面付 doesn't support
  natively; we'd lean on alipay 周期扣款 or a stored-token
  abstraction either way.
- **Multi-merchant support** — single appid for the deployment.

## Assumptions called out

- Operator can register at <https://open.alipay.com>, get an
  `app_id`, generate an RSA2 keypair, and upload the public key to
  the Alipay open platform. Spec covers the env-var format but not
  the registration walkthrough.
- We render QR client-side from the `qr_code` URL alipay returns —
  same `qrcode` package the subscription page already uses, so no
  new frontend dep.
- The notify endpoint is **public** (alipay's servers POST to it
  from variable IPs). Auth is by signature only; we do not require
  user JWT or admin credentials on this route.
