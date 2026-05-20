# runtime-3xui-client

The bearer-authed HTTP client that talks to every remote 3x-ui panel,
plus the cache + envelope decode plumbing. Lives in `internal/runtime`.

## Purpose & boundaries

This is the only module that opens HTTP connections to nodes. Every
node-touching service goes through it. Adjacent:

- **`netsafe-ssrf-guard`** wraps the dialer — node calls may be
  permitted to private ranges via the `allowPrivateKey` context
  sentinel; webhook calls are not.
- **`node-management`** owns the database row + probe loop; this
  module is the transport layer beneath it.
- **`inbound-management`** / **`client-provisioning`** /
  **`traffic-statistics`** all call this module's higher-level methods.

## Wire format

3x-ui's `/panel/api/*` endpoints share a single envelope:

```json
{ "success": true, "msg": "ok", "obj": <type-specific> }
```

- `success: false` is the only signal for application-level failure;
  HTTP status alone is unreliable (a panel may 200 with `success:
  false`).
- `obj` is JSON-typed for list/get endpoints, an empty object or
  string for write endpoints.
- Auth: `Authorization: Bearer <api_token>` header. The token is
  configured per node by the operator in 3x-ui's
  Settings → API Tokens UI and stored in `nodes.api_token`.
- Body format: most endpoints accept JSON. `POST /inbounds/add`
  expects **`application/x-www-form-urlencoded`** with the inbound
  fields as form keys — empirically determined; the upstream code is
  the source of truth.

## Components

| File | Role |
|---|---|
| `runtime.go` | `Remote` interface + per-node implementation: `AddInbound`, `UpdateInbound`, `DelInbound`, `SetInboundEnable`, `AddClient`, `UpdateClient`, `DelClientByEmail`, traffic resets, `FetchTrafficSnapshot`, `Probe`. |
| `manager.go` | `Manager` caches one `Remote` per node id; `InvalidateNode` rebuilds it after a node row mutation; `ForEach` walks enabled nodes concurrently. |
| `cache.go` | Per-node tag↔remote-id cache for inbound operations (`inbounds/update/{id}` etc.). Refresh-on-miss via `/inbounds/list`. |
| `envelope.go` | `Envelope`, `EnvelopeError`, `DecodeObj`, `ErrEmptyObj`. |
| `sanitize.go` | `sanitizeStreamSettingsForRemote` strips `certificateFile`/`keyFile` when inline `certificate`/`key` arrays are non-empty. |
| `types.go` | Wire types (`Inbound`, `Client`, `ClientStat`, traffic snapshot shape). |

## Requirements

### Requirement: Bearer-Authed Envelope Transport

The system SHALL talk to every remote 3x-ui panel via HTTPS (or HTTP
when explicitly configured), authenticate with the node's API token in
the `Authorization: Bearer …` header, and parse the response envelope
`{success, msg, obj}`.

#### Scenario: Envelope success

- **WHEN** a node call returns HTTP 200 with `{"success": true, "msg": "ok", "obj": <payload>}`
- **THEN** the runtime SHALL decode `obj` into the caller's typed struct via `DecodeObj`
- **AND** return the decoded value with no error

#### Scenario: Envelope failure

- **WHEN** a node call returns HTTP 200 with `{"success": false, "msg": "<reason>"}`
- **THEN** the runtime SHALL return an `EnvelopeError` wrapping `msg`
- **AND** callers SHALL be able to type-assert to recover the original `msg`

#### Scenario: Empty obj on write endpoints

- **WHEN** the response envelope's `obj` field is absent, null, or an empty string
- **THEN** `DecodeObj` SHALL return `ErrEmptyObj` so callers can distinguish "no data" from "decode failed"

#### Scenario: Non-200 HTTP status

- **WHEN** the upstream returns a non-200 HTTP status (401, 502, etc.)
- **THEN** the runtime SHALL return an error that includes the status code and the response body excerpt
- **AND** SHALL NOT attempt to decode the envelope (the response may not be valid JSON)

#### Scenario: Bearer token in header

- **WHEN** any node call is dispatched
- **THEN** the request SHALL carry `Authorization: Bearer <node.api_token>`
- **AND** SHALL NOT carry that token in the URL or body

### Requirement: Form-Encoded Inbound Creation

The system SHALL POST to `/panel/api/inbounds/add` with
`application/x-www-form-urlencoded` body — JSON bodies are rejected
silently by 3x-ui for this endpoint.

#### Scenario: AddInbound serializes to form keys

- **WHEN** the runtime adds an inbound
- **THEN** the request body SHALL be form-encoded with keys matching the upstream Inbound model field names (e.g. `remark`, `port`, `protocol`, `settings`, `streamSettings`, `sniffing`, `total`, `expiryTime`)
- **AND** stringified-JSON fields (`settings`, `streamSettings`, `sniffing`) SHALL be the JSON string value, not the JSON object

#### Scenario: Other endpoints stay JSON

- **WHEN** any endpoint other than `/inbounds/add` is called
- **THEN** the request body SHALL be JSON (the upstream behavior)

### Requirement: Per-Node Remote Cached In Manager

The system SHALL keep one `Remote` instance per node id alive in the
Manager, recycle it when the node row changes, and remove it on node
delete.

#### Scenario: First call constructs Remote

- **WHEN** the Manager receives a node id it has not seen
- **THEN** it SHALL load the node row, build a `Remote` (URL, token, transport), and cache it

#### Scenario: Invalidation after node edit

- **WHEN** `node-management` updates a node's connection fields and calls `Manager.InvalidateNode(id)`
- **THEN** the next call for that id SHALL reload the row and construct a fresh `Remote`
- **AND** any in-flight call against the old `Remote` SHALL be allowed to complete (no forced cancellation)

#### Scenario: Disabled node refused

- **WHEN** a caller requests a `Remote` for a node whose `enabled = false`
- **THEN** the Manager SHALL return `ErrNodeDisabled`
- **AND** callers (e.g. the probe loop) SHALL treat this as "skip" rather than "error"

#### Scenario: Missing node row

- **WHEN** a caller requests a `Remote` for a node id that has no DB row
- **THEN** the Manager SHALL return `ErrNodeNotFound`

#### Scenario: ForEach over enabled nodes

- **WHEN** `Manager.ForEach(ctx, fn)` is invoked
- **THEN** the Manager SHALL load every enabled node and invoke `fn` concurrently
- **AND** per-node errors SHALL be joined into a single error returned to the caller (using `errors.Join`)
- **AND** one node's failure SHALL NOT abort the iteration over the others

### Requirement: Tag-to-Remote-Id Cache

The system SHALL maintain a per-node cache from inbound `tag` to the
node's numeric remote inbound id, refreshing on miss via
`/panel/api/inbounds/list`.

#### Scenario: Cache hit avoids the round-trip

- **WHEN** an operation needs the remote id for a known `(node_id, tag)` and the entry is cached
- **THEN** the operation SHALL use the cached id without an extra `/inbounds/list` call

#### Scenario: Cache miss triggers refresh

- **WHEN** the cache has no entry for `(node_id, tag)`
- **THEN** the runtime SHALL call `/panel/api/inbounds/list`, populate the entire mapping for that node, and look up the tag again
- **AND** if the tag is still absent the call SHALL return a "tag not found" error

#### Scenario: Tag eviction on delete

- **WHEN** `DelInbound(node_id, tag)` succeeds
- **THEN** the runtime SHALL evict the tag from the cache

#### Scenario: Tag re-insertion on add

- **WHEN** `AddInbound(node_id, …)` succeeds and the response includes the assigned tag + id
- **THEN** the runtime SHALL insert that mapping into the cache so the next operation skips the list call

### Requirement: Client Update Strategy With Fallback

The system SHALL attempt the most efficient client update strategy
first, then fall back to a full re-push when the node rejects it.

#### Scenario: Strategy A (direct update) succeeds

- **WHEN** the runtime updates a single client via `/panel/api/inbounds/updateClient/{uuid}`
- **THEN** if the node responds `success: true`, no further action SHALL be taken

#### Scenario: Strategy B (re-push) on EnvelopeError

- **WHEN** Strategy A returns `EnvelopeError` (the node refused or didn't find the client)
- **THEN** the runtime SHALL fetch the full inbound, replace the client in the `settings.clients` array, and re-push via `UpdateInbound`
- **AND** the result of that re-push SHALL be returned to the caller

### Requirement: Stream-Settings Sanitization

The system SHALL strip redundant `certificateFile`/`keyFile` entries
from stream settings before sending them to the node, retaining inline
`certificate`/`key` content where present.

#### Scenario: Both inline and file present

- **WHEN** an inbound's `streamSettings.tlsSettings.certificates[i]` contains BOTH inline `certificate`/`key` arrays (non-empty) AND `certificateFile`/`keyFile` paths
- **THEN** the runtime SHALL remove the file path fields before sending

#### Scenario: Only file path present

- **WHEN** an entry contains only `certificateFile`/`keyFile` (no inline content)
- **THEN** the file path entries SHALL be left untouched (the node will read the cert from disk)

#### Scenario: Malformed JSON passes through unchanged

- **WHEN** `streamSettings` cannot be parsed as JSON
- **THEN** the runtime SHALL pass the string through verbatim and let the upstream node reject it if it's truly bad

### Requirement: Probe Returns Structured Health

The system SHALL expose a `Probe(ctx, nodeID)` that returns latency,
Xray version, CPU %, memory %, and uptime in one call.

#### Scenario: Successful probe payload

- **WHEN** `Probe` calls `GET /panel/api/server/status` and gets a healthy envelope
- **THEN** the returned struct SHALL contain `latency_ms`, `xray_version`, `cpu_pct`, `mem_pct`, `uptime_secs`, `taken_at` (now), and a nil error

#### Scenario: Probe timeout

- **WHEN** the call exceeds the probe timeout (10s)
- **THEN** the returned struct SHALL have `status="offline"` and a non-nil error naming the timeout

### Requirement: Target Fork Declaration

The system SHALL target MHSanaei/3x-ui (any recent commit on
`main` or `bash` branch) as the canonical 3x-ui implementation,
since that fork includes the WireGuard + Hysteria protocol
modules absent from canonical 3x-ui. The dashboard SHALL NOT
attempt per-node capability detection — compatibility is
declared statically per `docs/operator/3xui-fork-compat.md`.

#### Scenario: Operator runs an incompatible fork

- **WHEN** an operator points the dashboard at a 3x-ui fork that lacks WG/Hysteria support
- **AND** the dashboard attempts the corresponding `POST /panel/api/inbounds/add`
- **THEN** the node SHALL respond with a protocol-validation error
- **AND** the dashboard SHALL surface that error to the admin verbatim

#### Scenario: /inbounds/options not Bearer-accessible

- **GIVEN** the controller for `/inbounds/options` calls `session.GetLoginUser(c)` which returns nil for API-token callers
- **THEN** the dashboard SHALL NOT use this endpoint
- **AND** the runtime client SHALL NOT have a method that calls `/inbounds/options`

### Requirement: Fork-Aligned Client Routes

The runtime client SHALL speak the MHSanaei/3x-ui fork's
`/panel/api/clients/*` endpoint group for every per-client
mutation. The legacy `/panel/api/inbounds/{addClient,...}` routes
are absent on the fork and SHALL NOT be used.

| Operation | Path |
|---|---|
| Add client | `POST /panel/api/clients/add` |
| Update client | `POST /panel/api/clients/update/:email` |
| Delete client | `POST /panel/api/clients/del/:email` |
| Per-client traffic read | `GET  /panel/api/clients/traffic/:email` |
| Reset one client | `POST /panel/api/clients/resetTraffic/:email` |
| Onlines list | `POST /panel/api/clients/onlines` |
| Last-online map | `POST /panel/api/clients/lastOnline` |

#### Scenario: AddClient body envelope

- **WHEN** the dashboard calls `Remote.AddClient(ctx, inboundTag, client)`
- **THEN** the runtime SHALL POST to `/panel/api/clients/add` with body `{client: model.Client, inboundIds: [int]}`
- **AND** the runtime SHALL NOT use the legacy `{id, settings: stringified-json}` envelope

#### Scenario: Network 404 surfaces visibly

- **WHEN** the panel responds 404 to a `/panel/api/clients/*` path
- **THEN** the runtime SHALL return an error whose message names the full path
- **AND** SHALL NOT silently fall back to inbound re-push (so fork-version drift surfaces, not hides)

### Requirement: WireGuard Inbound Settings Schema

The runtime SHALL expose typed Go structs for WG inbound settings:

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
    PSK        string   `json:"psk,omitempty"`
    AllowedIPs []string `json:"allowedIPs"`
    KeepAlive  int      `json:"keepAlive"`
}
```

Peer mutation uses `POST /panel/api/inbounds/update/:id` with a
read-modify-write cycle on `settings.peers[]` (the fork has no
per-peer endpoint). `Remote.UpdateInboundByID(ctx, id, *Inbound)`
exposes the id-keyed variant for the RMW path.

#### Scenario: Settings roundtrip

- **WHEN** a WG inbound is fetched via `GetInbound(tag)`
- **AND** its `settings` JSON string is unmarshalled into `WGSettings`
- **THEN** re-marshalling SHALL produce the same `settings` string the fork accepts on `UpdateInbound`

#### Scenario: IsWireguard match is case-strict

- **GIVEN** `Inbound.IsWireguard()` returns true only for lowercase `"wireguard"`
- **THEN** an inbound emitted with mixed case SHALL NOT match — case-folding here would hide fork-protocol drift

### Requirement: Hysteria StreamSettings Shape

The runtime SHALL recognize Hysteria 2 via `streamSettings.network ==
"hysteria"` and expose `runtime.HysteriaStreamConfig` for the
`hysteriaSettings` JSON block:

```go
type HysteriaStreamConfig struct {
    Version        int    `json:"version"`
    UDPIdleTimeout int    `json:"udpIdleTimeout"`
    Auth           string `json:"auth,omitempty"`
}
```

The per-client credential lives in `runtime.Client.Auth`, not
`Client.ID` (VLESS/VMess) or `Client.Password` (Trojan/SS).

#### Scenario: Provision a Hysteria client

- **WHEN** `ClientService.ProvisionClient` is called on a Hysteria inbound
- **THEN** `buildWireClient` SHALL populate `Client.Auth` via crypto/rand 16-char URL-safe alphabet (excluding ambiguous 0/O/1/l/I)
- **AND** SHALL leave `Client.ID` and `Client.Password` empty
- **AND** SHALL POST via the same `/panel/api/clients/add` envelope as VLESS/VMess
