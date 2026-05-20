# ⓘ PROMOTED 2026-05-21

> Shipped requirements folded into
> `openspec/specs/client-provisioning/spec.md` (Protocol Branching
> / Pre-flight Provision Check / WG Peer Revocation via RMW).
> Aspirational scenarios (key rotation, renewal re-add of same
> peer) remain here as the design record but are not v1 behavior.

## MODIFIED Requirements

### Requirement: Client Provisioning Branch by Inbound Type

`ProvisionClient` SHALL branch on `inbound.IsWireguard()`. The
non-WG branch uses the unified `POST /panel/api/clients/add`
endpoint (with `inboundIds`). The WG branch uses read-modify-write
on the inbound's settings.peers[] because the fork has no
`/clients/*` peer-management path for WG.

#### Scenario: Provision non-WG client

- **WHEN** `ProvisionClient(user, inbound)` is called and `inbound.IsWireguard() == false`
- **THEN** the system SHALL call `runtime.XrayClient.AddClient` which maps to `POST /panel/api/clients/add` with body `{client: {...}, inboundIds: [inbound.id]}`
- **AND** SHALL insert a `client_ownerships` row referencing the inbound

#### Scenario: Provision WG client

- **WHEN** `ProvisionClient(user, inbound)` is called and `inbound.IsWireguard() == true`
- **THEN** the system SHALL open a DB transaction
- **AND** SHALL acquire `pg_advisory_xact_lock(inbound.id)` to serialize concurrent WG mutations on the same inbound
- **AND** SHALL `GetInbound(inbound.id)` to obtain the current peers[] (avoiding the lost-update race)
- **AND** SHALL generate a fresh curve25519 keypair locally via `golang.org/x/crypto/curve25519`
- **AND** SHALL allocate the next-free IPv4 from the inbound's subnet
- **AND** SHALL append the new peer `{privateKey, publicKey, allowedIPs: [ip/32], keepAlive: 0}` to `settings.peers[]`
- **AND** SHALL `UpdateInbound(inbound.id, ...)` to push the new settings back to the node
- **AND** SHALL insert a `wg_peers` row with the AES-encrypted private key, public key, and allocated IP
- **AND** SHALL commit the transaction
- **AND** failure of any step SHALL roll back the entire mutation (no orphan ownership / wg_peers rows)

#### Scenario: IP allocation exhaustion

- **WHEN** the WG inbound's subnet has no free IPs left
- **THEN** the system SHALL return an error wrapping `ErrWGSubnetExhausted`
- **AND** SHALL NOT call `UpdateInbound` (no half-state on the node)

## ADDED Requirements

### Requirement: WG Peer Persistent Storage

The `wg_peers` table SHALL store peer keypairs + allocated IPs
1:1 with `client_ownerships` rows that target a WG inbound. The
private key SHALL be encrypted with AES-256-GCM using a key
derived from the `WG_MASTER_KEY` env var. The table denormalizes
`inbound_id` so the RMW lookup path doesn't need a join.

#### Scenario: Private key never plaintext at rest

- **WHEN** the DB is dumped (e.g. `pg_dump`)
- **THEN** `wg_peers.private_key_encrypted` SHALL be ciphertext bytes
- **AND** decryption SHALL require knowledge of `WG_MASTER_KEY`

#### Scenario: Master key rotation hazard documented

- **WHEN** the operator rotates `WG_MASTER_KEY` without re-encrypting existing rows
- **THEN** subscription fetches for those rows SHALL fail with a wrapped decryption error
- **AND** the dashboard SHALL log a critical error identifying the affected ownership ids
- **AND** the `.env.example` SHALL document this as a "rotate carefully" hazard with a step-by-step migration recipe

### Requirement: WG Revocation Path

Revocation of a WG ownership (expiry, manual disable) SHALL
remove the peer from the node's WG settings via the SAME RMW
pattern as add. The `wg_peers` row SHALL be retained so renewal
can re-add the same public key without forcing the user to
re-download the config.

#### Scenario: Expiry removes peer via RMW

- **WHEN** ExpiryJob processes a WG ownership past `expires_at`
- **THEN** the job SHALL acquire `pg_advisory_xact_lock(wg_peers.inbound_id)`
- **AND** SHALL `GetInbound` → remove the peer whose `publicKey` matches `wg_peers.public_key` → `UpdateInbound`
- **AND** SHALL flip `client_ownerships.enabled = false`
- **AND** SHALL retain the `wg_peers` row for potential renewal

#### Scenario: Renewal re-adds the same peer

- **WHEN** a user with a previously-expired WG ownership purchases a new plan onto the same inbound
- **THEN** `ProvisionClient` SHALL detect the existing `wg_peers` row by (user_id, inbound_id)
- **AND** SHALL re-add the peer to settings.peers[] using the existing keypair (not generate a new one)
- **AND** the user's stored `.conf` file SHALL remain valid — no re-download needed

### Requirement: Concurrency Safety

WG peer mutations on the same inbound SHALL be serialized via
`pg_advisory_xact_lock`. Concurrent peer adds on different
inbounds SHALL proceed in parallel.

#### Scenario: Concurrent peer adds on same inbound

- **GIVEN** inbound `id=42` has a single peer `[A]`
- **WHEN** two `ProvisionClient` calls land simultaneously for the same inbound
- **THEN** the second one SHALL wait inside `pg_advisory_xact_lock` for the first to commit
- **AND** the final state SHALL have all three peers `[A, B, C]` (no lost update)
