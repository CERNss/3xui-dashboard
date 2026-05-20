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

### Requirement: Protocol Capability Detection

The probe job SHALL populate `nodes.supported_protocols` with the
set of inbound protocols the node's panel build supports. The
dashboard SHALL hide protocol-specific UI on nodes that don't
support that protocol.

#### Scenario: Probe captures supported protocols

- **WHEN** `ProbeJob` runs against a node
- **AND** the node's `/panel/api/inbounds/options` endpoint returns an enumerable protocol list
- **THEN** the dashboard SHALL persist that list to `nodes.supported_protocols`
- **AND** SHALL refresh the cache on every successful probe (cheap; doesn't require change-detection)

#### Scenario: Fallback when /inbounds/options unavailable

- **WHEN** the node returns 404 on `/inbounds/options` OR the response shape doesn't enumerate protocols
- **THEN** the dashboard SHALL default `supported_protocols` to `['vless','vmess','trojan','shadowsocks']` (canonical 3x-ui baseline)
- **AND** SHALL log a warning identifying the node so the operator can manually flag it as WG-capable if needed

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
