# ⓘ NOT PROMOTED 2026-05-21

> The inbound editor changes are admin UI concerns, not part of
> the canonical `inbound-management` capability surface (list /
> create / update / delete). Two-section UI (Xray / WG split),
> port-conflict detection, and the per-WG-inbound peer listing
> ops endpoint did not ship in v1 — this file is retained as the
> design record. Frontend protocol handling is documented in code
> (InboundEditorModal.vue + Inbounds.vue protocol filter chip).

## MODIFIED Requirements

### Requirement: Inbound Editor

The fleet inbound view SHALL recognize a fifth protocol —
`wireguard` — alongside the existing four. WG inbounds use a
distinct editor shape since the transport/security matrix does not
apply.

#### Scenario: WG inbound list section

- **WHEN** an admin opens the Inbounds page
- **THEN** the page SHALL render two sections: "Xray inbounds" (existing 4 protocols) and "WireGuard inbounds"
- **AND** the WG section SHALL display: listen port, subnet, server public key (masked), allocated peer count

#### Scenario: WG inbound editor fields

- **WHEN** an admin opens the WG inbound editor (create or edit)
- **THEN** the form SHALL expose: name, listen port (UDP), subnet (e.g. `10.66.0.0/24`), optional MTU, optional DNS list
- **AND** SHALL NOT expose: transport tabs, security tabs, the 8×3 matrix
- **AND** the server keypair SHALL be auto-generated server-side on first save, displayed read-only with a copy-to-clipboard button for the public key

## ADDED Requirements

### Requirement: Port-Conflict Detection

The system SHALL reject creation of a WG inbound whose UDP listen
port collides with any existing Xray inbound on the same node.
This avoids `wg-quick` and Xray fighting over the port at the
network layer.

#### Scenario: Conflicting port rejected

- **WHEN** an admin attempts to create a WG inbound on port `443` on a node where Xray already has an inbound on port `443`
- **THEN** the system SHALL respond `409 Conflict` with a message identifying the conflicting inbound

### Requirement: WG Peer Listing for Ops

The admin API SHALL expose a per-WG-inbound peer listing endpoint
showing allocated IPs + masked public keys (first 4 + last 4 chars,
ellipsis in between). Useful for ops debugging without exposing
full peer keys to admin UI logs.

#### Scenario: GET WG peers

- **WHEN** an admin GETs `/api/admin/inbounds/wireguard/:id/peers`
- **THEN** the response SHALL return one row per peer with `allocated_ip`, `public_key_masked`, `created_at`, `client_ownership_id`
- **AND** SHALL NOT return private keys or full unmasked public keys
