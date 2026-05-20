# Tasks — add-protocol-wireguard

## 0. Prerequisite: verify 3x-ui WG API surface

Before any code, capture the actual endpoint shape from a real
node. The implementation pivots on this — different 3x-ui forks
expose WG differently.

- [ ] 0.1 Set up a test 3x-ui node with WG panel enabled
  (minimum version per the proposal — verify whether the
  panel ships in 3x-ui main or as a separate fork).
- [ ] 0.2 Capture exact paths + request/response envelopes for:
  - List WG inbounds
  - Add WG inbound (server-side config)
  - List WG peers under an inbound
  - Add WG peer (with public key)
  - Remove WG peer
- [ ] 0.3 Record findings in `openspec/changes/add-protocol-wireguard/notes/3xui-wg-api.md`.
- [ ] 0.4 Re-confirm or update design.md's "Runtime client split"
  section + the assumed endpoint paths in proposal.md.

## 1. Runtime client refactor

- [ ] 1.1 Extract `runtime.XrayClient` interface from the existing
  per-node Remote (no behavior change — pure rename + interface
  declaration so the future WGClient can sit alongside).
- [ ] 1.2 Add `runtime.WGClient` interface + `Manager.GetWG(nodeID)`
  helper that returns `(WGClient, error)`. Error includes
  `ErrWGCapabilityAbsent` distinct from `ErrNodeNotFound`.
- [ ] 1.3 Add `runtime.WGInbound` + `runtime.WGPeer` types.
- [ ] 1.4 Implement WGClient against the API shape captured in T0.
- [ ] 1.5 Tests: httptest fake matching the captured envelope shape.

## 2. Probe + WG capability detection

- [ ] 2.1 Extend `ProbeJob.probeOne` to call WG list endpoint after
  the Xray list. On 404 → record `node.wg_supported = false` in
  the node row + status object; on 200 → record true.
- [ ] 2.2 Migration: add `nodes.wg_supported BOOLEAN NOT NULL DEFAULT FALSE`.
- [ ] 2.3 Admin Nodes page surfaces the WG flag (icon chip).

## 3. Inbound model + storage

- [ ] 3.1 Migration: `node_inbound_snapshots.is_wireguard BOOLEAN`,
  `wg_peers` table per design.md.
- [ ] 3.2 New `internal/service/inbound/wg.go` with WG-typed CRUD.
- [ ] 3.3 Inbound admin handler: route `protocol=wireguard` to the
  WG service path.

## 4. Crypto: WG keypair gen + master encryption

- [ ] 4.1 `internal/service/wgcrypto/keypair.go`: generate curve25519
  pair via `golang.org/x/crypto/curve25519`. Public + private keys
  base64-encoded per WireGuard wire format.
- [ ] 4.2 `WG_MASTER_KEY` env var: 32 hex-encoded bytes for
  AES-256-GCM of stored private keys. Helper to encrypt/decrypt
  with per-row nonce stored prefix in BYTEA.
- [ ] 4.3 Tests: roundtrip; corrupt ciphertext rejected;
  wrong-master-key rejected.

## 5. Client provisioning: WG path

- [ ] 5.1 `clientsvc.ProvisionClient` branches on
  `inbound.IsWireguard`. WG path:
  - Reserve next-free IP from the inbound subnet via
    `pg_advisory_xact_lock` on `(inbound_id, ip_low_watermark)`
    so two concurrent provisions can't allocate the same IP.
  - Generate keypair, encrypt private, insert `wg_peers` row.
  - Call `WGClient.AddWGPeer` with public key + allocated IP/32.
  - Insert `client_ownerships` row referencing the inbound.
- [ ] 5.2 Port-conflict detection: reject WG inbound creation if its
  listen port collides with any existing Xray inbound on that node
  (the node's wg-quick + Xray would fight over the port otherwise).
- [ ] 5.3 Revocation (ExpiryJob.disableOnNode): branch on IsWG and
  call `RemoveWGPeer` instead of `UpdateClient(Enable=false)`.
- [ ] 5.4 Tests: provision → wg_peers row exists, public key on
  node; expire → peer removed from node, wg_peers row retained
  (for renewal); renew → peer re-added with the same public key.

## 6. Subscription assembly

- [ ] 6.1 `internal/sub/wireguard.go`: build `.conf` text from a
  `wg_peers` row + its inbound. Use Go's `text/template` for the
  Interface/Peer blocks.
- [ ] 6.2 New format keys: `wireguard` (single conf),
  `wireguard-zip` (multi-peer zip via `archive/zip`).
- [ ] 6.3 Extend Clash + sing-box converters to emit WG outbound
  stanzas when a peer exists for the user. URI base64 + SIP008
  formats SKIP wg peers (no representation).
- [ ] 6.4 Tests: single-peer conf has Interface + Peer; multi-peer
  zip contains one file per peer; Clash output has wireguard
  outbound type when peer present.

## 7. HTTP handlers

- [ ] 7.1 Admin: `POST /api/admin/inbounds` accepts
  `protocol=wireguard` and dispatches to the WG service. Same
  endpoint, branched by protocol field.
- [ ] 7.2 Admin: new `GET /api/admin/inbounds/wireguard/:id/peers`
  for ops debugging — lists allocated IPs + masked public keys.
- [ ] 7.3 Public sub: subscription handler already routes by
  `?format=` — wire the new format strings.

## 8. Frontend

- [ ] 8.1 Inbound list page: tab/section for WireGuard inbounds.
- [ ] 8.2 New `InboundEditorWGModal.vue`: subnet + listen port +
  server-generated keypair display (read-only, copy-to-clipboard).
- [ ] 8.3 Portal Subscription page: format picker gains
  "WireGuard"; button text + click handler swap to download
  `.conf` instead of QR/copy.
- [ ] 8.4 vitest: WG modal smoke + format-picker branch test.

## 9. Spec promotion + ROADMAP

- [ ] 9.1 Move `openspec/changes/add-protocol-wireguard/specs/*`
  → `openspec/specs/` (modifications folded into the canonical
  spec.md files; new `wireguard` capability NOT created — WG is
  a `multi-protocol` extension across runtime + inbound + sub).
- [ ] 9.2 Update `openspec/specs/README.md` pillar table: 多协议
  4/4 → 5/5; move "WireGuard runtime + sub" from gap list to
  module list.
- [ ] 9.3 Update `openspec/ROADMAP.md`: mark #8 ✅.

## 10. Documentation

- [ ] 10.1 `.env.example`: add `WG_MASTER_KEY` block with the
  generation command + rotation footnote.
- [ ] 10.2 `docs/operator/wireguard-setup.md`: brief note covering
  3x-ui WG panel installation prereq + WG_MASTER_KEY generation
  + first-WG-inbound walkthrough.
- [ ] 10.3 Mention port-conflict avoidance + revocation semantics
  in the operator doc.
