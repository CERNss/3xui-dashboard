# Tasks — add-protocol-wireguard (v2, post-T0)

## ✅ 0. T0 — endpoint capture (DONE 2026-05-20)

Findings in `notes/3xui-wg-api.md`. Key conclusions:
- Fork unifies WG under `/panel/api/inbounds/*` — no separate
  WGClient surface
- Peer add/remove = RMW on inbound.settings.peers[] via
  `POST /panel/api/inbounds/update/:id`
- Node generates keypairs by default; whether we can override is
  T1.5 to verify
- Side concern: existing dashboard XrayClient may not match the
  fork's actual route set — separate `audit-xrayclient-vs-fork`
  change tracks this

## 1. Schema + crypto (low risk, foundational)

- [x] 1.1 Migration `0007_wg_peers.up.sql` (numbered 0007, not 0009 —
  next-free at implementation time):
  ```sql
  CREATE TABLE wg_peers (
      id                    BIGSERIAL PRIMARY KEY,
      client_ownership_id   BIGINT NOT NULL UNIQUE REFERENCES client_ownerships(id) ON DELETE CASCADE,
      public_key            TEXT NOT NULL,
      private_key_encrypted BYTEA NOT NULL,
      allocated_ip          INET NOT NULL,
      inbound_id            BIGINT NOT NULL,
      created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  -- No supported_protocols column — capability detection removed.
  -- See design.md "Capability detection — REMOVED".
  ```
- [x] 1.2 `internal/service/wgcrypto/`:
  - [x] `keypair.go` — curve25519 keypair (RFC 7748 §5 clamp) +
    `DerivePublic`
  - [x] `cipher.go` — AES-256-GCM with key from `WG_MASTER_KEY`
    env; nonce stored as ciphertext prefix
  - [x] tests: roundtrip, tampered ciphertext rejected, nonce
    variance across seals, key format validation (8 tests, 11 cases)

## 2. Runtime client — extend `XrayClient` (no split)

- [x] 2.1 Added `Remote.UpdateInboundByID(ctx, id, *Inbound)` →
  `POST /panel/api/inbounds/update/:id`. Existing tag-keyed
  `UpdateInbound` now delegates to the id-keyed variant.
- [x] 2.2 ~~Probe `/inbounds/options` for capability detection~~ —
  REMOVED per spec (see design.md "Capability detection — REMOVED").
- [x] 2.3 Tests: `TestUpdateInboundByID_PostsFormToUpdatePath`
  verifies form-encoded body lands on /inbounds/update/:id with
  protocol + settings preserved; `TestInbound_IsWireguard`
  guards against accidental case-changes.

## 3. WG-aware Inbound type

- [x] 3.1 `runtime.WGSettings` Go struct matching captured JSON:
  ```go
  type WGSettings struct {
      MTU         int       `json:"mtu"`
      SecretKey   string    `json:"secretKey"`
      Peers       []WGPeer  `json:"peers"`
      NoKernelTun bool      `json:"noKernelTun"`
  }
  type WGPeer struct {
      PrivateKey string   `json:"privateKey"`
      PublicKey  string   `json:"publicKey"`
      AllowedIPs []string `json:"allowedIPs"`
      KeepAlive  int      `json:"keepAlive"`
  }
  ```
- [x] 3.2 Helper `(inb *Inbound) IsWireguard() bool` — exact
  lowercase match (case-folding would mask fork drift).

## 4. Provisioning — peer add/remove via RMW + advisory lock

- [x] 4.1 `WGProvisioner.ProvisionPeer(ctx, userID, nodeID, inboundTag, email, planID)`:
  - Opens tx via `peers.DB().Transaction`
  - `pg_advisory_xact_lock(inbound.id)` via `repository.AdvisoryLock`
  - GETs inbound under the lock (drift-tolerant — re-reads peers[])
  - Generates curve25519 keypair locally + AES-256-GCM seals private key
  - Allocates next-free IP from `10.0.0.0/24` excluding `.0` + `.1`
    and any addresses the panel already has
  - Appends peer to settings.peers[] and POSTs
    `/inbounds/update/:id` via `UpdateInboundByID`
  - Saves ownership row + `wg_peers` row in the same tx
- [x] 4.2 `WGProvisioner.RemovePeer(ctx, nodeID, inboundTag, email)`:
  - Same advisory-lock + RMW pattern; removes peer by
    public_key match; clears `wg_peers` mirror + ownership row
- [~] 4.3 Branch `ClientService.Provision` on `inbound.IsWireguard()`
  — DEFERRED. WGProvisioner is a sibling service rather than a
  branch inside `ProvisionClient` because WG ownerships have no
  `email/uuid/password` to sync via the unified flow. Plan-purchase
  callsite (billing) needs the branch — tracked as #8.1 follow-up.
- [~] 4.4 Branch `ExpiryJob.disableOnNode` — DEFERRED to #8.1
  follow-up alongside 4.3. The current expiry job calls
  `UpdateClient(Enable=false)` which 404s for WG ownerships;
  acceptable for v1 since WG is opt-in via WG_MASTER_KEY.
- [x] 4.5 Tests: allocator unit tests cover the IP picker
  (lowest free / skips taken / refuses gateway / errors on
  exhaustion / parses CIDR + bare-IP inputs).

## 5. Subscription rendering

- [x] 5.1 `internal/sub/wireguard.go::BuildWGConf(link)` — wg-quick
  ini text with [Interface] / [Peer] sections, DNS,
  PersistentKeepalive
- [x] 5.2 Sub handler: `?format=wireguard` + `/sub/wireguard/:subId`
  → `text/plain` .conf body
- [x] 5.3 `?format=wireguard-zip` + `/sub/wireguard-zip/:subId` →
  `application/zip` archive via `archive/zip`
- [x] 5.4 Clash converter (`clashWGNode`) — `type: wireguard`
  outbound per Mihomo schema
- [x] 5.5 sing-box converter (`singboxWGOutbound`) —
  `type: wireguard` outbound per sing-box schema
- [x] 5.6 Base64 + SIP008 + JSON SKIP WG peers automatically — they
  iterate `findClientByEmail` which doesn't apply to WG peers; the
  `X-Subscription-Hint: wireguard` header is DEFERRED as a v1.1
  polish (not strictly required for client compatibility)
- [x] 5.7 Tests cover `.conf` required lines + section ordering,
  zip archive contents (one .conf per WG link, non-WG skipped),
  safe filename rules, Clash + sing-box field shape

## 6. Handlers

- [x] 6.1 `admin.InboundHandler.create` already pipes the protocol
  string through — verified WG inbounds round-trip via the
  existing form-encoded `/inbounds/add` path. No handler change
  required.
- [~] 6.2 `admin.InboundHandler.listPeers(:id)` — DEFERRED to v1.1.
  Useful for ops debugging but not required for the user flow.
- [~] 6.3 UA auto-detect for "wireguard" client UAs — DEFERRED.
  The existing UA matcher (clash / sing-box / shadowsocks) doesn't
  recognise the WG official client; the explicit
  `?format=wireguard` query param is the v1 path.

## 7. Frontend

- [x] 7.1 Inbound editor modal: 'wireguard' option in protocol
  dropdown; visibleTabs hides Stream + Sniffing for WG;
  protocol-tab WG block explains the auto-managed-peers flow;
  buildSettings emits the empty WG shell (MTU defaults to 1420).
  Subnet input is DEFERRED — v1 uses hardcoded `10.0.0.0/24` per
  docs; surfacing it in the editor adds form complexity without
  changing behaviour.
- [x] 7.2 Inbounds list filter chip + protoColor mapping include
  "wireguard". Per-row peer-count display reuses the existing
  clientStats column — peer count is the same column for WG.
- [x] 7.3 Portal Subscription page: 2 new formats ("WireGuard",
  "WG (ZIP)"); ZIP variant flagged downloadOnly so the copy
  button becomes a download <a download>, QR hides, and the
  right card shows "下载即用" instead.
- [~] 7.4 vitest mount smoke for WG inbound editor — DEFERRED.
  Existing 62 frontend tests stay green; specific WG smoke would
  test rendering without exercising the backend RMW path.

## 8. ~~Capability gating~~ — REMOVED

Per design.md, the dashboard targets MHSanaei/3x-ui as a
monolithic spec. WG features are always available; if an operator
runs a non-MHSanaei fork that lacks WG, `/inbounds/add` with
`protocol="wireguard"` fails server-side and the dashboard
surfaces that as a regular operation error.

## 9. Spec promotion + ROADMAP

- [~] 9.1 Fold spec deltas into `openspec/specs/*` — DEFERRED to a
  post-ship cleanup change. The change-local spec deltas remain
  the source of truth until then.
- [x] 9.2 ROADMAP: 多协议节点 4/6 → 5/6; #8 🚧 → ✅ (partial)

## 10. Documentation

- [x] 10.1 `.env.example` block for `WG_MASTER_KEY` with
  `openssl rand -hex 32` instruction + rotation hazard footnote.
- [x] 10.2 `docs/operator/wireguard-setup.md` covering fork
  requirement, key generation, rotation hazard, the editor flow,
  the provisioning flow, panel-UI concurrency caveat, and a
  troubleshooting table.
