## Why

We operate a fleet of 3x-ui (Xray) proxy nodes. The stock 3x-ui panel only
manages one server per panel, exposes far more surface than we need, and has no
clean separation between operator and end-user. We want a single central panel
that controls every node, plus a self-service portal for end users — built by
extracting the genuinely useful 3x-ui logic into our own front/back end rather
than wrapping the legacy UI.

## What Changes

- Introduce `3xui-dashboard`, a **pure central controller**: it runs no Xray
  itself and instead drives a fleet of vanilla 3x-ui nodes through each node's
  `/panel/api/...` endpoints using a per-node Bearer API token.
- **Extract & re-implement** from upstream 3x-ui (`cern-3x-ui`):
  - the remote-node runtime/transport (`web/runtime/remote.go`) — the Bearer-auth
    HTTP client, envelope decoding, tag→remote-id resolution, traffic snapshot.
  - node lifecycle + probing (`web/service/node.go`) — register, normalize,
    probe `/panel/api/server/status`, heartbeat status, CPU/mem metric history.
  - subscription link generation (`sub/subService.go`, `sub/links.go`,
    `sub/subJsonService.go`, `sub/subClashService.go`) — VLESS/VMess/Trojan/SS
    link building, base64 + JSON + Clash subscription formats.
  - inbound/client data model (`database/model/model.go`).
- **Drop** from upstream: local Xray process management, Telegram bot, geo-asset
  management, the legacy multi-page HTML UI, single-server assumptions.
- **Add net-new**: a fully separated admin console and user portal, a
  PostgreSQL-backed account & billing layer, and a client→user ownership mapping
  that 3x-ui lacks.
- **Auth model**: admin and user authentication are completely separate — the
  single administrator is a static credential supplied via environment
  (`.env`), never stored in the database and with no registration flow; end
  users authenticate via standard OIDC **or** email/password and live in the
  PostgreSQL `users` table.
- Frontend is a Vue 3 SPA embedded into the Go binary via `go:embed`.

This **supersedes** the earlier proxy-only scaffold (which merely forwarded raw
3x-ui API calls) — that approach is replaced by owning the logic.

## Capabilities

### New Capabilities

- `admin-auth`: a single-administrator console login backed by a static
  credential from environment config, with its own JWT audience, separate
  login endpoint, and no database record or registration flow.
- `user-accounts`: end-user authentication via standard OIDC and via
  email/password, user account lifecycle, JWT sessions for the user portal,
  email-address binding, a public-registration on/off switch, an email-domain
  allowlist, an optional SMTP integration for verification email,
  admin-side user-account administration, and the client→user ownership link.
- `node-management`: register/edit/delete remote 3x-ui nodes, per-node Bearer
  token, enable/disable, periodic probe + heartbeat, online/offline status,
  CPU/mem/latency/uptime metrics and short-term metric history.
- `inbound-management`: list/create/update/delete Xray inbounds on a selected
  node, fleet-wide inbound listing, protocol/port/stream-settings handling.
- `client-provisioning`: create/update/delete clients on a node's inbound
  (the "remote link creation" flow), with traffic limit, expiry, IP limit, and
  enable/disable — addressable by client UUID/email.
- `traffic-statistics`: pull traffic snapshots from every node, aggregate
  per-node / per-inbound / per-client usage, online-client detection, reset
  traffic, and time-bucketed history for charts.
- `subscription`: generate per-client subscription output — raw link list,
  base64 subscription, JSON subscription, and Clash config — served at a
  tokenized public subscription URL, plus QR code for the user portal.
- `billing-and-plans`: admin-defined traffic/duration plans, user balance,
  plan purchase that provisions or extends a client, and balance/order history.
- `webhook-notifications`: admin-configured outbound webhooks that deliver
  signed JSON payloads to external endpoints on subscribed events (node
  online/offline, traffic thresholds, client expiry, registrations, orders),
  with retry and a delivery log — the plumbing for later message/alert setups.

### Modified Capabilities

<!-- None — greenfield project, no existing specs. -->

## Impact

- New project tree: `3xui-dashboard/{backend,frontend,deploy,openspec}`.
- Backend: Go 1.26 + Gin + GORM + **PostgreSQL** (replaces the scaffold's SQLite)
  + JWT. New migrations for users, nodes, clients-ownership, traffic samples,
  plans, orders, webhooks, and webhook deliveries. No `admins` table — the admin
  credential and OIDC client settings are environment-supplied.
- Configuration: admin credential (`ADMIN_USERNAME`, `ADMIN_PASSWORD`), OIDC
  provider settings (issuer/discovery URL, client id/secret, scopes, redirect
  URLs), an optional SMTP section (host/port/credentials/from/TLS/enabled flag,
  default disabled), a public-registration flag, and an email-domain allowlist
  are read from `.env` / environment.
- Frontend: Vue 3 + TS + Vite + Tailwind + Pinia + vue-router SPA, embedded via
  `go:embed`.
- External contract: depends on vanilla 3x-ui nodes exposing `/panel/api/*`
  with API-token auth. Node 3x-ui version compatibility must be documented.
- The previously generated proxy scaffold's handlers/services are rewritten;
  the SQLite dependency is removed.
