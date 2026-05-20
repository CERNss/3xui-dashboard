## MODIFIED Requirements

### Requirement: Client Provisioning Branch by Inbound Type

`ProvisionClient` SHALL branch on `inbound.IsWireguard`. The Xray
branch retains existing behavior (UUID / password generation, push
to node via `XrayClient.AddClient`). The WG branch follows a
distinct path documented below.

#### Scenario: Provision onto a WG inbound

- **WHEN** `ProvisionClient(user, wg_inbound)` is called
- **THEN** the system SHALL generate a fresh curve25519 keypair locally using `golang.org/x/crypto/curve25519`
- **AND** SHALL allocate the next-free IPv4 from the inbound's subnet via a transactional advisory lock (so concurrent provisions can't collide)
- **AND** SHALL insert a row into `wg_peers` with the public key, AES-256-GCM-encrypted private key, and allocated IP
- **AND** SHALL call `WGClient.AddWGPeer(inbound_id, {public_key, allowed_ips: ip/32})` — only the PUBLIC key reaches the node
- **AND** SHALL insert a `client_ownerships` row referencing the inbound

#### Scenario: IP allocation exhaustion

- **WHEN** the WG inbound's subnet has no free IPs left
- **THEN** `ProvisionClient` SHALL return an error wrapping a sentinel `ErrWGSubnetExhausted`
- **AND** SHALL NOT call the node's AddWGPeer endpoint

## ADDED Requirements

### Requirement: WG Peer Persistent Storage

The `wg_peers` table SHALL store peer keypairs + allocated IPs
1:1 with `client_ownerships` rows that target a WG inbound. The
private key SHALL be encrypted with AES-256-GCM using a key
derived from the `WG_MASTER_KEY` env var.

#### Scenario: Private key never plaintext at rest

- **WHEN** the DB is dumped (e.g. `pg_dump`)
- **THEN** `wg_peers.private_key_encrypted` SHALL be ciphertext bytes
- **AND** decryption SHALL require knowledge of `WG_MASTER_KEY`

#### Scenario: Master key rotation

- **WHEN** the operator rotates `WG_MASTER_KEY` without re-encrypting existing rows
- **THEN** subscription fetches for those rows SHALL fail with a wrapped decryption error
- **AND** the dashboard SHALL log a critical error identifying the affected ownership ids
- **AND** the `.env.example` SHALL document this as a "rotate carefully" hazard

### Requirement: WG Revocation Path

Revocation of a WG ownership (expiry, manual disable) SHALL remove
the peer from the node's WG config. The `wg_peers` row SHALL be
retained so renewal can re-add the SAME public key without forcing
the user to re-download the config.

#### Scenario: Expiry removes peer

- **WHEN** ExpiryJob processes a WG ownership past `expires_at`
- **THEN** the job SHALL call `WGClient.RemoveWGPeer(inbound_id, public_key)`
- **AND** SHALL flip `client_ownerships.enabled = false`
- **AND** SHALL retain the `wg_peers` row

#### Scenario: Renewal re-adds peer

- **WHEN** a user with a previously-expired WG ownership purchases a new plan onto the same inbound
- **THEN** `ProvisionClient` SHALL detect the existing `wg_peers` row
- **AND** SHALL call `WGClient.AddWGPeer` with the existing public key + allocated IP
- **AND** SHALL NOT generate a new keypair — the user's stored `.conf` file remains valid
