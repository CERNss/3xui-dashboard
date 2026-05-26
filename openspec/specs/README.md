# 3xui-dashboard — Specs

This directory is the canonical, current-state spec set. Each subdirectory
covers one module; `spec.md` inside is the truth-as-implemented.

`openspec/changes/<name>/` holds proposed or in-flight modifications. Once
a change ships, its delta is folded into the corresponding module spec
here (Original ADDED → present-tense REQUIRED in `specs/`).

## Project positioning

3xui-dashboard is a **Go + Vue 3 re-implementation of [sspanel-uim](https://github.com/anankke/sspanel-uim)**
on top of a **stock 3x-ui (Xray) node fleet**. sspanel-uim is the
reference for product surface (the 5 pillars below); 3x-ui is the
reference for node-side wire format. Our backend is one Go binary
that embeds the Vue SPA via `go:embed` — no PHP/Smarty, no MySQL,
no Redis.

### The five pillars (from sspanel-uim)

Modules below are tagged by which pillar(s) they back. A `★` marks
work still missing for full sspanel parity.

| Pillar | Modules | Gap (★ = not yet built) |
|---|---|---|
| **运维管理** (Operations) | `node-management`, `traffic-statistics`, `scheduler-jobs`, `settings`, `admin-views`, `admin-auth`, `auth-bootstrap` | — |
| **多协议支持** (Multi-protocol) | `inbound-management`, `client-provisioning`, `subscription`, `runtime-3xui-client` | ★ Hysteria2 / TUIC v5 on the node-runtime side (Xray-core gap); ★ WireGuard runtime + sub |
| **支付系统** (Payment) | `billing-and-plans`, `payment-gateway-alipay`, `payment-gateway-stripe` | ★ Cryptomus (crypto); ★ coupon system; ★ recurring billing (auto-renewal) |
| **通知系统** (Notification) | `notify-service`, `notification-channels`, `webhook-notifications`, `mailer`, `email-verification`, `event-bus` | ★ persistent delivery queue (today best-effort with single retry); ★ per-user channel routing (today telegram/discord/feishu deliver only to admin-configured targets) |
| **用户界面** (UI) | `design-system`, `theme-system`, `layouts-and-chrome`, `admin-views`, `unified-login`, `user-accounts`, `oidc-providers`, `i18n` | ★ mobile-responsive admin views |

## High-level architecture

3xui-dashboard is a **central controller** for a fleet of stock 3x-ui (Xray)
nodes. It does NOT run Xray itself. It coordinates remote nodes through
each node's `/panel/api/...` surface using a per-node Bearer token.

```
┌──────────────────────────────┐
│  Browser (admin / portal)    │   Vue 3 SPA, embedded via go:embed
└───────────────┬──────────────┘
                │ HTTP / JSON
┌───────────────▼──────────────────────────────────────────────┐
│  3xui-dashboard (single Go binary)                           │
│                                                              │
│  HTTP surface  (Gin)                                         │
│    /api/admin/*   admin-token-gated                          │
│    /api/user/*    user-token-gated (mostly)                  │
│    /api/public/*  no auth                                    │
│                                                              │
│  Service graph  (internal/app/app.go is the wiring root)     │
│    auth · user · node · inbound · client · traffic           │
│    billing · webhook · event · verification · mailer · sub   │
│                                                              │
│  Repositories  (GORM + pgx)  ─────► PostgreSQL               │
│  Scheduler  (robfig/cron)                                    │
│  3x-ui runtime client  (internal/runtime)                    │
│  SSRF guard  (internal/netsafe)                              │
└────────────────────┬───────────────────────────┬─────────────┘
                     │                           │
              ┌──────▼──────┐             ┌──────▼──────┐
              │ Node #1     │   …         │ Node #N     │
              │ 3x-ui panel │             │ 3x-ui panel │
              └─────────────┘             └─────────────┘
```

## Module index

Each entry maps to `openspec/specs/<key>/spec.md`.

### Identity & access
- `auth-bootstrap` — env-supplied admin credential, ADMIN_PASSWORD auto-generation.
- `admin-auth` — `/api/admin/auth/login`, admin JWT (audience `admin`).
- `user-accounts` — portal account model, register / login / change-pw / verified email change.
- `unified-login` — single `/login` route, role auto-fallback, login/register tabs.
- `email-verification` — 6-digit code, 10-min TTL, 60s send cooldown.
- `oidc-providers` — listing endpoint, login-page button row, and OIDC start/callback account completion.

### Fleet operations
- `node-management` — node CRUD, scheduled probe, drain.
- `inbound-management` — fleet inbound view, 8-transmission × 3-security editor.
- `client-provisioning` — Xray client CRUD, ownership table, snapshot.
- `traffic-statistics` — aggregation, reset (client / inbound / node).
- `subscription` — `/api/public/sub/:subId` URL + assembly.

### Plans & messaging
- `billing-and-plans` — plan CRUD, purchase (idempotency-key + row lock + refund), balance, gateway-pay state machine (`payment_pending` → `paid` → `completed`).
- `payment-gateway-alipay` — alipay 当面付 precreate + query + RSA2 notify verify; Beijing-TZ timestamps.
- `payment-gateway-stripe` — Stripe Checkout Sessions + HMAC webhook verify (raw-body) + replay protection.
- `notify-service` — event-bus subscriber that fans `client.*` / `node.*` / `order.*` to routed channels with per-channel dedup.
- `notification-channels` — Email / Telegram / Discord / Feishu adapters behind a common `Channel` interface.
- `webhook-notifications` — event subscription, signed delivery, persistent retry (customer-facing integrations; distinct from `notify-service` ops alerts).
- `mailer` — stdlib SMTP wrapper, STARTTLS + implicit-TLS branches, dev no-op fallback.
- `event-bus` — in-process publish/subscribe, typed payloads in `internal/service/event/payload`.

### Infrastructure
- `runtime-3xui-client` — bearer auth, `{success,msg,obj}` envelope, form-encoded inbound add.
- `scheduler-jobs` — cron registry (probe / webhook retry / etc.).
- `migrations` — golang-migrate iofs, schema version table.

### Frontend / UX
- `design-system` — Tailwind tokens: Geist font, accent/ink/surface palette, brand easing, fontSize scale, border radii.
- `theme-system` — light/dark/auto with localStorage override + system fallback.
- `layouts-and-chrome` — AuthLayout (login chrome), AdminLayout (sidebar), PortalLayout (topbar).
- `admin-views` — Status, Nodes, Inbounds (table-density Sub2API pattern), Settings.

## Conventions

- Specs use RFC 2119 keywords (MUST, SHOULD, MAY) inside Given/When/Then scenarios.
- File paths and identifiers use full repo-relative paths (e.g. `backend/internal/handler/user/auth.go::Register`).
- Schema specs cite the owning baseline or migration file when that detail matters.
- When a behavior depends on config, the spec MUST name the env var.
