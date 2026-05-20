## MODIFIED Requirements

### Requirement: Subscription Output Formats

The subscription handler SHALL serve two additional format keys for
WireGuard peers, alongside the existing five (base64 / JSON / Clash
/ sing-box / SIP008). Existing format behavior is unchanged.

#### Scenario: Single WG conf format

- **WHEN** a client GETs `/api/public/sub/:subId?format=wireguard`
- **AND** the subscribing user has exactly one active WG peer
- **THEN** the system SHALL return a single WG `.conf` file (plain text)
- **AND** Content-Type SHALL be `application/x-wireguard-conf`
- **AND** Content-Disposition SHALL be `attachment; filename="<node>-<inbound>.conf"`

#### Scenario: Multi-peer WG zip format

- **WHEN** the user has more than one active WG peer
- **AND** `?format=wireguard-zip` is requested
- **THEN** the system SHALL return a ZIP archive with one `.conf` per peer
- **AND** Content-Type SHALL be `application/zip`
- **AND** Content-Disposition SHALL include a `.zip` filename

#### Scenario: WG conf body shape

- **WHEN** the system emits a WG `.conf`
- **THEN** the `[Interface]` block SHALL contain `PrivateKey`, `Address` (allocated IP / subnet bits), `DNS` (defaults `1.1.1.1, 8.8.8.8`)
- **AND** the `[Peer]` block SHALL contain `PublicKey` (server's), `Endpoint` (node host + UDP port), `AllowedIPs = 0.0.0.0/0, ::/0`, `PersistentKeepalive = 25`

### Requirement: Clash + Sing-box WG Integration

The existing Clash + sing-box converters SHALL emit a WireGuard
outbound stanza per active WG peer when the user has a mixed
subscription (some Xray inbounds + some WG peers).

#### Scenario: Clash output includes WG proxy

- **WHEN** a user with at least one WG peer requests `?format=clash`
- **THEN** the Mihomo YAML SHALL contain a `proxies:` entry of type `wireguard` per peer, with the same Interface + Peer fields as the standalone `.conf` would have
- **AND** the proxy SHALL appear in the auto-select + select groups alongside the Xray-based proxies

#### Scenario: sing-box output includes WG outbound

- **WHEN** a user with at least one WG peer requests `?format=singbox`
- **THEN** the sing-box JSON SHALL contain an `outbound` entry of type `wireguard` per peer

## ADDED Requirements

### Requirement: WG-Skip on Non-WG-Capable Formats

The URI-bundle base64 format AND the SIP008 format SHALL omit WG
peers without erroring — these formats have no representation for
WireGuard.

#### Scenario: Base64 format with WG-only subscription

- **WHEN** a user whose subscription contains ONLY WG peers requests `?format=base64` (the default)
- **THEN** the system SHALL return an empty link bundle (zero-length base64) with a `200 OK` status
- **AND** SHALL include the `Subscription-Userinfo` header as usual
- **AND** SHALL NOT 500 / 404

#### Scenario: Mixed subscription, base64 includes Xray only

- **WHEN** a user with both WG and Xray entries requests `?format=base64`
- **THEN** the system SHALL include only the Xray entries in the link bundle
- **AND** the WG entries SHALL be silently skipped (their representation lives in `wireguard` / `clash` / `singbox` formats)
