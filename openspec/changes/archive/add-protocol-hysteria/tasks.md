# Tasks — add-protocol-hysteria

Smaller change than #8 — Hysteria fits the unified
`/panel/api/clients/*` flow. Major work is URI rendering + UI.

## 0. Prereq verification

- [x] 0.1 Hysteria settings shape verified via T0 round-2
  API-create roundtrip (2026-05-20). Schema captured in
  `add-protocol-wireguard/notes/3xui-wg-api.md` §hysteria.
- [x] 0.2 `model.Client.Auth` field confirmed via source
  inspection of MHSanaei/3x-ui `web/service/client.go` +
  `database/model/model.go`.

## 1. Runtime client

- [x] 1.1 `runtime.Client.Auth` field added; `HysteriaStreamConfig`
  typed struct added for the streamSettings.hysteriaSettings block.
- [x] 1.2 No new endpoints — `AddInbound` / `AddClient` /
  `UpdateInbound` / `DelClient` all reused unchanged.

## 2. Provisioning

- [x] 2.1 `buildWireClient` "hysteria" / "hysteria2" branch:
  - Generates `auth` via `randomAuthString(16)` (crypto/rand
    backed, alphabet excludes ambiguous chars 0/O/1/l/I)
  - Builds `Client{Email, SubID, Auth, ...}` with empty ID +
    Password
  - Reuses the existing `XrayClient.AddClient(client, inboundIds=[id])`
- [x] 2.2 `ExpiryJob.disableOnNode` works for Hysteria unchanged —
  it calls `UpdateClient({Email, Enable: false})` which the
  fork accepts for any protocol on the unified
  `/clients/update/:email` endpoint.
- [~] 2.3 mockPanel-driven integration test DEFERRED — the
  service-level ProvisionClient flow is covered by the existing
  `service/client` test suite (still green); a Hysteria-specific
  end-to-end test would duplicate the existing AddClient routes
  verification.

## 3. Subscription

- [x] 3.1 `BuildLink` `hysteria` / `hysteria2` case emits
  `hysteria2://<auth>@host:port/?sni=<sni>&alpn=h3&insecure=0#<remark>`.
  Drops empty-auth + non-v2 inbounds silently.
- [x] 3.2 Clash: `type: hysteria2` proxy with password/sni/alpn
  /skip-cert-verify (when allowInsecure=true).
- [x] 3.3 sing-box: `type: hysteria2` outbound with tls block;
  empty SNI falls back to connect host (sing-box rejects
  empty server_name when tls.enabled=true).
- [x] 3.4 SIP008 SKIPS hysteria automatically (SS-only format);
  Base64 + JSON include hysteria2:// URIs from BuildLink.
- [x] 3.5 Tests in `internal/sub/hysteria_test.go`:
  URI happy path / empty-auth skip / v1 skip / Clash field
  shape / Clash allow-insecure / sing-box field shape / sing-box
  empty-SNI fallback (7 tests).

## 4. Frontend

- [x] 4.1 InboundEditorModal: 'hysteria' option in protocol
  dropdown; visibleTabs hides Stream + Sniffing for Hysteria
  (transport is fixed); 协议 tab gains SNI / Fingerprint /
  Allow Insecure inputs + a "TLS mandatory, ALPN=h3 locked" hint
  explaining the cert-path workflow; buildStream + buildSettings
  emit the fork's expected Hysteria shape (version: 2,
  streamSettings.hysteriaSettings).
- [x] 4.2 Inbounds list: 'hysteria' chip in protocol filter +
  sky-blue protoColor mapping; client-add modal gains an Auth
  input + Regen button for Hysteria, with validation requiring
  auth to be non-empty.
- [x] 4.3 Portal subscription unchanged — Hysteria URIs flow
  through existing Base64 / Clash / sing-box outputs that
  already grew Hysteria support in the backend renderers.
- [~] 4.4 Vitest smoke for the Hysteria editor — DEFERRED.
  Existing 62 frontend tests stay green; a dedicated mount-only
  test for Hysteria UI doesn't exercise the backend wire shape.

## 5. Capability gating

- [~] 5.1 `nodes.supported_protocols` — DEFERRED. The #8 design
  document REMOVED capability detection (target MHSanaei/3x-ui
  as a monolithic spec; non-WG forks were never going to have
  Hysteria either). docs/operator/3xui-fork-compat.md already
  documents the fork requirement once for the whole dashboard.

## 6. Spec promotion + ROADMAP

- [~] 6.1 Spec deltas folded into `openspec/specs/*` — DEFERRED
  to a post-ship cleanup change. Change-local deltas remain the
  source of truth until then (same approach as #8).
- [x] 6.2 ROADMAP: multi-protocol pillar (f) row flipped to ✅;
  change-queue #10 row marked shipped.
