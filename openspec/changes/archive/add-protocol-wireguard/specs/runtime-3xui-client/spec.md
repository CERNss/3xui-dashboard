# ⓘ PROMOTED 2026-05-21

> The shipped requirements from this delta have been folded into
> `openspec/specs/runtime-3xui-client/spec.md` (Target Fork
> Declaration / Fork-Aligned Client Routes / WireGuard Inbound
> Settings Schema / Hysteria StreamSettings Shape).
>
> This change-local file is retained for historical context and
> aspirational scenarios (e.g. re-add renewal flow) that did not
> ship in v1.

## MODIFIED Requirements

### Requirement: Per-Node API Surface

The runtime client SHALL speak ONE API surface per node — the
unified `/panel/api/inbounds/*` set used for VLESS/VMess/Trojan/
Shadowsocks/WireGuard/Hysteria alike, distinguished by the
`protocol` field on each inbound. There SHALL NOT be a separate
per-protocol client interface.

#### Scenario: Unified inbound creation

- **WHEN** `XrayClient.AddInbound(ctx, inbound)` is called with `inbound.Protocol = "wireguard"`
- **THEN** the client SHALL POST to `/panel/api/inbounds/add` with the same envelope used for VLESS / VMess inbounds
- **AND** SHALL serialize the WG-specific settings into the `settings` field as a JSON string per `notes/3xui-wg-api.md`

#### Scenario: Inbound update for peer mutation

- **WHEN** the dashboard needs to add or remove a WG peer
- **THEN** the runtime client SHALL expose `UpdateInbound(ctx, id, inbound)` mapping to `POST /panel/api/inbounds/update/:id`
- **AND** callers SHALL perform a read-modify-write cycle: `GetInbound(id)` → mutate `settings.peers[]` → `UpdateInbound(id, inbound)`

## ADDED Requirements

### Requirement: Target Fork Declaration

The system SHALL target MHSanaei/3x-ui (any recent commit on
`main` or `bash` branch — verified 2026-05-20 to be content-
identical at the controller + model layer) as the canonical
3x-ui implementation. Since this fork has WG + Hysteria + the 4
Xray protocols built in, the dashboard SHALL NOT attempt
per-node capability detection.

#### Scenario: Operator runs an incompatible fork

- **WHEN** an operator points the dashboard at a 3x-ui fork that lacks WG support
- **AND** the dashboard attempts `POST /panel/api/inbounds/add` with `protocol=wireguard`
- **THEN** the node SHALL respond with a protocol-validation error
- **AND** the dashboard SHALL surface that error to the admin verbatim
- **AND** docs SHALL note "MHSanaei/3x-ui required for WG features"

#### Scenario: /inbounds/options not Bearer-accessible

- **GIVEN** the controller for `/inbounds/options` calls `session.GetLoginUser(c)` which returns nil for API-token callers
- **THEN** the dashboard SHALL NOT use this endpoint for capability detection
- **AND** the runtime client SHALL NOT have a method that calls `/inbounds/options`

### Requirement: WG Inbound Settings Schema

The runtime package SHALL expose typed Go structs for WG settings,
serializable to the JSON-string `settings` field of an Inbound:

```go
type WGSettings struct {
    MTU         int      `json:"mtu"`
    SecretKey   string   `json:"secretKey"`
    Peers       []WGPeer `json:"peers"`
    NoKernelTun bool     `json:"noKernelTun"`
}
type WGPeer struct {
    PrivateKey string   `json:"privateKey"`
    PublicKey  string   `json:"publicKey"`
    AllowedIPs []string `json:"allowedIPs"`
    KeepAlive  int      `json:"keepAlive"`
}
```

#### Scenario: Settings roundtrip

- **WHEN** a WG inbound is fetched via `GetInbound(id)`
- **AND** its `settings` field is JSON-unmarshalled into `WGSettings`
- **THEN** the resulting struct SHALL faithfully reproduce all peer keypairs and allowed IPs
- **AND** re-marshalling SHALL produce the SAME `settings` string the fork accepts on `UpdateInbound`
