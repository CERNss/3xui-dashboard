# 3xui-dashboard Specs

This directory is the canonical current-state design requirement set.
Each module owns one `spec.md`; those specs describe the behavior the
product is expected to provide now.

`openspec/changes/<name>/` is only for proposed or in-flight work. When a
change ships, keep the durable requirements in `openspec/specs/` and remove
the change narrative from the active set.

## Project positioning

3xui-dashboard is a Go + React/Ant Design central controller for a stock
3x-ui (Xray) node fleet. The backend is one Go binary that serves JSON APIs,
coordinates remote 3x-ui panels, persists state in PostgreSQL, and embeds the
React SPA via `go:embed`.

The project borrows product shape from sspanel-style panels, but the source of
truth for this repository is the specs below plus the current Go/React code.
The supported deployment stack is Go, React, PostgreSQL, and embedded static
assets.

## Product pillars

| Pillar | Modules |
|---|---|
| Operations | `node-management`, `traffic-statistics`, `scheduler-jobs`, `settings`, `admin-views`, `admin-auth`, `auth-bootstrap` |
| Multi-protocol runtime | `inbound-management`, `client-provisioning`, `subscription`, `runtime-3xui-client` |
| Billing | `billing-and-plans`, `payment-gateway-alipay`, `payment-gateway-stripe` |
| Notifications | `notify-service`, `notification-channels`, `webhook-notifications`, `mailer`, `email-verification`, `event-bus` |
| User experience | `frontend-platform-react`, `design-system`, `theme-system`, `layouts-and-chrome`, `admin-views`, `unified-login`, `user-accounts`, `oidc-providers`, `i18n` |

## High-level architecture

3xui-dashboard is a central controller. It does not run Xray locally. It talks
to each remote node through that node's `/panel/api/...` surface using the
node's configured API token.

```
┌──────────────────────────────┐
│  Browser (admin / portal)    │   React SPA, embedded via go:embed
└───────────────┬──────────────┘
                │ HTTP / JSON
┌───────────────▼──────────────────────────────────────────────┐
│  3xui-dashboard (single Go binary)                           │
│                                                              │
│  HTTP surface (Gin)                                          │
│    /api/admin/*   admin-token-gated                          │
│    /api/user/*    user-token-gated where required            │
│    /api/public/*  public integration and branding endpoints  │
│    /sub/*         public subscription output                 │
│                                                              │
│  Service graph (internal/app/app.go is the wiring root)      │
│    auth · user · node · inbound · client · traffic           │
│    billing · webhook · event · verification · mailer · sub   │
│                                                              │
│  Repositories (GORM + pgx) ─────► PostgreSQL                 │
│  Scheduler (robfig/cron)                                     │
│  3x-ui runtime client (internal/runtime)                     │
│  SSRF guard (internal/netsafe)                               │
└────────────────────┬───────────────────────────┬─────────────┘
                     │                           │
              ┌──────▼──────┐             ┌──────▼──────┐
              │ Node #1     │   ...       │ Node #N     │
              │ 3x-ui panel │             │ 3x-ui panel │
              └─────────────┘             └─────────────┘
```

## Module index

Each entry maps to `openspec/specs/<key>/spec.md`.

### Identity & Access

- `auth-bootstrap` - env-supplied administrator credential and `ADMIN_PASSWORD` generation.
- `admin-auth` - `/api/admin/auth/login` and admin-audience JWTs.
- `user-accounts` - portal account model, registration API, login API, email changes, OIDC linkage, moderation.
- `unified-login` - single `/login` SPA entry for admin password login and portal OIDC start.
- `email-verification` - 6-digit code and scoped verification-token flows.
- `oidc-providers` - provider discovery, OIDC start/callback, and account completion endpoints.

### Fleet Operations

- `node-management` - remote node CRUD, health probes, drain semantics.
- `inbound-management` - fleet inbound view and editor wire format.
- `client-provisioning` - Xray client CRUD, ownership table, provisioning snapshots.
- `traffic-statistics` - node/client/inbound traffic collection, aggregation, reset, retention.
- `subscription` - `/sub/:subId` and format-specific subscription assembly.

### Plans & Messaging

- `billing-and-plans` - plans, orders, balance, provisioning pools, payment state machine.
- `payment-gateway-alipay` - Alipay precreate/query/notify verification.
- `payment-gateway-stripe` - Stripe Checkout Sessions and webhook verification.
- `notify-service` - internal ops notifications from event-bus events.
- `notification-channels` - Email, Telegram, Discord, and Feishu channel adapters.
- `webhook-notifications` - customer-facing signed webhooks with persistent retry.
- `mailer` - SMTP transport with no-op development fallback.
- `event-bus` - in-process typed event fanout.

### Infrastructure

- `runtime-3xui-client` - stock 3x-ui panel API client.
- `scheduler-jobs` - cron registry and built-in recurring jobs.
- `migrations` - embedded SQL baseline and startup migration runner.
- `netsafe-ssrf-guard` - outbound URL/IP validation for node and webhook targets.
- `settings` - server-driven mutable settings, branding, OIDC config, and subscription templates.

### Frontend / UX

- `frontend-platform-react` - React SPA stack, routing, state/query boundaries, build output.
- `design-system` - AntD theme tokens, CSS conventions, typography, density, shared primitives.
- `theme-system` - light/dark/system mode state, persistence, and toggle placement.
- `layouts-and-chrome` - `AuthLayout`, `AdminLayout`, `PortalLayout`.
- `admin-views` - admin routes under `/admin/*`, including overview, ops, nodes, inbounds, users, billing, webhooks, and settings.
- `i18n` - `i18next`/`react-i18next` locale loading and locale switch behavior.

## Conventions

- Specs use RFC 2119 keywords (`MUST`, `SHALL`, `SHOULD`, `MAY`) and Given/When/Then scenarios.
- File paths and identifiers use repo-relative paths, for example `backend/internal/handler/user/auth.go::Register`.
- Current specs avoid implementation history. Historical proposals may stay in `openspec/changes/archive/`, but active specs should read as product/design requirements.
- When behavior depends on config, the spec MUST name the setting key or environment variable.
