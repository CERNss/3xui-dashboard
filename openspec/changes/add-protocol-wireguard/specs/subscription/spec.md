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
WireGuard. When a user's subscription has NO entries that the
requested format can represent, the system SHALL return a
human-readable explanation rather than an empty body. A blank
response would surface in v2rayN / Shadowrocket as "subscription
empty / broken" with no path to recovery.

#### Scenario: Base64 format with WG-only subscription

- **WHEN** a user whose subscription contains ONLY WG peers requests `?format=base64` (the default)
- **THEN** the system SHALL return `200 OK` with a plain-text body explaining the situation:
  `Your subscription has only WireGuard peers. Use ?format=wireguard or ?format=wireguard-zip to download the .conf file(s), or ?format=clash for a mixed config.`
- **AND** the response SHALL include `X-Subscription-Hint: wireguard` so client tooling can branch
- **AND** SHALL include the `Subscription-Userinfo` header as usual
- **AND** SHALL NOT return an empty body

#### Scenario: Mixed subscription, base64 includes Xray only

- **WHEN** a user with both WG and Xray entries requests `?format=base64`
- **THEN** the system SHALL include only the Xray entries in the base64 bundle
- **AND** the WG entries SHALL be silently skipped (their representation lives in `wireguard` / `clash` / `singbox` formats)
- **AND** the response SHALL include `X-Subscription-Hint: mixed; wireguard-also-available` so a client that supports multi-format negotiation can offer the user the Clash download

#### Scenario: User-Agent auto-detect prefers wireguard for WG-only users

- **WHEN** the User-Agent auto-detect path would have selected `base64` (no `?format=` param, generic agent string)
- **AND** the user's subscription is WG-only
- **THEN** the auto-detect SHALL select `wireguard` instead of `base64`
- **AND** the response SHALL serve the `.conf` body directly, not the explanation text

#### Scenario: SIP008 format with WG-only subscription

- **WHEN** a user with no Shadowsocks (only WG) requests `?format=sip008`
- **THEN** the response SHALL be `200 OK` with a JSON body `{"version": 1, "servers": []}` (SIP008 is structured — empty array is the well-formed empty case, not a UX cliff like base64)
- **AND** the response SHALL include `X-Subscription-Hint: wireguard` if WG peers exist
