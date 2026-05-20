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

- [ ] 1.1 Migration `0009_wg_peers.up.sql`:
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
  ALTER TABLE nodes
      ADD COLUMN supported_protocols TEXT[] NOT NULL DEFAULT ARRAY['vless','vmess','trojan','shadowsocks'];
  ```
- [ ] 1.2 `internal/service/wgcrypto/`:
  - `keypair.go` — generate curve25519 pair via
    `golang.org/x/crypto/curve25519`; derive public from private
    via `X25519(priv, BasePoint)`
  - `cipher.go` — AES-256-GCM with key from `WG_MASTER_KEY` env;
    nonce stored as ciphertext prefix
  - tests: roundtrip, tampered ciphertext rejected, derive matches
    a known WG keypair fixture

## 2. Runtime client — extend `XrayClient` (no split)

- [ ] 2.1 Add `XrayClient.UpdateInbound(ctx, id int64, inbound runtime.Inbound) error`
  → `POST /panel/api/inbounds/update/:id`. Existing `AddInbound`
  + `RemoveInbound` patterns; same `{success, msg, obj}` envelope.
- [ ] 2.2 Confirm probe call → `GET /panel/api/inbounds/options`
  returns enumerable protocol list. If yes, parse + persist into
  `nodes.supported_protocols`. If no (T0 didn't verify shape),
  fall back to a probe-create+probe-delete sniff.
- [ ] 2.3 Tests: httptest fake serves the captured WG inbound shape;
  client round-trips through marshal/unmarshal.

## 3. WG-aware Inbound type

- [ ] 3.1 `runtime.WGSettings` Go struct matching captured JSON:
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
- [ ] 3.2 Helper `(inb Inbound) IsWireguard() bool` — checks
  `protocol == "wireguard"`. Used everywhere conditional logic
  branches.

## 4. Provisioning — peer add/remove via RMW + advisory lock

- [ ] 4.1 `ClientService.AddWGPeer(ctx, ownership, inbound)`:
  - Open tx
  - `SELECT pg_advisory_xact_lock(inbound_id)` to serialize
  - GET inbound (fresh state, others may have added peers since
    we last looked)
  - Generate keypair locally via wgcrypto
  - Allocate next-free IP from `<inbound.subnet>` (parse from
    settings.peers[].allowedIPs first byte / hardcoded 10.0.0.0/24
    in v1 if no explicit subnet field)
  - Append peer to settings.peers[]
  - POST `/inbounds/update/:id` with new settings
  - Insert `wg_peers` row with AES-encrypted private key
  - Commit
- [ ] 4.2 `ClientService.RemoveWGPeer(ctx, ownership)`:
  - Same RMW pattern; remove peer by public_key match
- [ ] 4.3 Branch `ClientService.Provision` on `inbound.IsWireguard()`:
  - WG → AddWGPeer path above
  - non-WG → existing `/clients/add` path
- [ ] 4.4 Branch `ExpiryJob.disableOnNode` similarly:
  - WG → RemoveWGPeer
  - non-WG → existing `UpdateClient(Enable=false)`
- [ ] 4.5 Tests: integration test with mockPanel that captures the
  full RMW sequence (GET, mutate, POST); verify final inbound has
  the new peer; verify ownership row + wg_peers row both written

## 5. Subscription rendering

- [ ] 5.1 `internal/sub/wireguard.go`: `Build(peer, inbound, node) string`
  → produces `.conf` ini text per design.md format
- [ ] 5.2 Sub handler: `?format=wireguard` → text/plain `.conf` body
- [ ] 5.3 `?format=wireguard-zip` → ZIP with one `.conf` per peer
  (via `archive/zip`)
- [ ] 5.4 Extend Clash converter — WG outbound per peer
- [ ] 5.5 Extend sing-box converter — WG outbound per peer
- [ ] 5.6 Base64 + SIP008 SKIP WG peers (per existing sub spec
  delta — emit `X-Subscription-Hint: wireguard` header if user
  has WG entries)
- [ ] 5.7 Tests: `wg_peers` fixtures → `.conf` body parses cleanly
  via `gopkg.in/ini.v1`; Clash YAML output valid via `yaml.Unmarshal`

## 6. Handlers

- [ ] 6.1 `admin.InboundHandler.create` route already exists; ensure
  it pipes `protocol=wireguard` through unchanged (the unified
  XrayClient.AddInbound handles it)
- [ ] 6.2 `admin.InboundHandler.listPeers(:id)` — new endpoint
  exposing wg_peers rows (masked private keys) for ops debugging
- [ ] 6.3 Sub handler: ensure UA auto-detect prefers `wireguard`
  format for WG-only subscriptions (per subscription spec delta)

## 7. Frontend

- [ ] 7.1 Inbound editor modal: conditional fields on `protocol ==
  'wireguard'` — drop transport/security tabs, show subnet input
  (defaulting `10.0.0.0/24`), MTU input
- [ ] 7.2 Inbound list page: "WireGuard" filter tab; column shows
  peer count instead of client count
- [ ] 7.3 Portal Subscription page: format picker gets "WireGuard"
  option; button text → "下载配置文件" instead of "复制 URL"
- [ ] 7.4 vitest mount smoke for WG inbound editor + WG sub format

## 8. Capability gating

- [ ] 8.1 Backend: list of available providers + protocols at
  `GET /api/admin/capabilities` (admin frontend reads this on
  page load)
- [ ] 8.2 Frontend: hide WG features when no probed node has
  `wireguard` in supported_protocols

## 9. Spec promotion + ROADMAP

- [ ] 9.1 Fold `changes/add-protocol-wireguard/specs/*` into
  `openspec/specs/{runtime-3xui-client,inbound-management,
  client-provisioning,subscription}/spec.md`
- [ ] 9.2 ROADMAP: 多协议 4/4 → 5/5; #8 ❌ → ✅

## 10. Documentation

- [ ] 10.1 `.env.example`: WG_MASTER_KEY block + generation
  command (`openssl rand -hex 32`) + rotation hazard footnote
- [ ] 10.2 `docs/operator/wireguard-setup.md`: fork compatibility
  note (this dashboard's WG support requires the extended 3x-ui
  fork running on the operator's nodes), WG_MASTER_KEY setup,
  first-WG-inbound walkthrough
