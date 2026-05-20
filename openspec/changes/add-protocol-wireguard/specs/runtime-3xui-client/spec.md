## MODIFIED Requirements

### Requirement: Per-Node API Surface

The runtime client SHALL expose two interfaces per node — one for
the Xray-managed inbounds (existing) and one for the WireGuard
panel surface (new) — but share a single Bearer-token credential
and SSRF-guarded HTTP client.

#### Scenario: Node bundles both surfaces

- **WHEN** `runtime.Manager.Get(nodeID)` returns a `Node` handle
- **THEN** the handle SHALL implement both `XrayClient` and `WGClient`
- **AND** SHALL share the same `*http.Client` (one connection pool per node)
- **AND** SHALL share the same Bearer token

#### Scenario: WG capability detection on probe

- **WHEN** the probe job calls the WG list endpoint on a node
- **AND** the node returns HTTP 404 (WG panel not installed)
- **THEN** the probe SHALL mark `node.wg_supported = false`
- **AND** SHALL NOT mark the node offline (Xray-only nodes are still healthy)

#### Scenario: WG capability error vs node offline

- **WHEN** any non-WG endpoint (e.g. Xray list) fails with a timeout
- **THEN** the probe SHALL mark the node offline as before
- **WHEN** the WG list endpoint specifically returns 404
- **THEN** the node stays online with `wg_supported = false`

## ADDED Requirements

### Requirement: WireGuard Client Interface

The runtime package SHALL expose a `WGClient` interface for the
WG-specific endpoints, with operations matching the actual
3x-ui WG panel surface (paths captured in
`changes/add-protocol-wireguard/notes/3xui-wg-api.md` after task 0).

```go
type WGClient interface {
    ListWGInbounds(ctx context.Context) ([]WGInbound, error)
    AddWGPeer(ctx context.Context, inboundID int64, peer WGPeer) error
    RemoveWGPeer(ctx context.Context, inboundID int64, publicKey string) error
}
```

#### Scenario: Add peer with public key only

- **WHEN** the dashboard calls `WGClient.AddWGPeer(inbound, peer)`
- **THEN** the request body SHALL include the peer's PublicKey + AllowedIPs (allocated subnet/32)
- **AND** SHALL NOT include any private-key material
- **AND** the dashboard SHALL retain the private key locally, encrypted with `WG_MASTER_KEY`

### Requirement: WG-Specific Errors

The runtime client SHALL surface WG capability errors distinctly
from generic node errors so callers can branch (e.g. provisioning
SHOULD fail with a typed error if a non-WG-capable node receives
a WG provisioning request).

#### Scenario: ErrWGCapabilityAbsent

- **WHEN** `Manager.GetWG(nodeID)` is called on a node whose WG panel returns 404
- **THEN** it SHALL return `(nil, ErrWGCapabilityAbsent)`
- **AND** this error SHALL be distinct from `ErrNodeNotFound`
