## MODIFIED Requirements

### Requirement: Client Update Strategy With Fallback

The system SHALL attempt the most efficient client mutation
strategy first against the fork's `/panel/api/clients/*` endpoint
group, then fall back to a full re-push when the node rejects it
with an `EnvelopeError`. Network-level errors (404, transport
failures) SHALL surface verbatim — silent fallback is reserved
for application-level rejection.

#### Scenario: Strategy A (direct update) succeeds

- **WHEN** the runtime updates a single client via `POST /panel/api/clients/update/:email` (body: `model.Client`)
- **THEN** if the node responds `success: true`, no further action SHALL be taken

#### Scenario: Strategy B (re-push) on EnvelopeError

- **WHEN** Strategy A returns `EnvelopeError` (the node refused or didn't find the client)
- **THEN** the runtime SHALL fetch the full inbound, replace the client in the `settings.clients` array, and re-push via `UpdateInbound`
- **AND** the result of that re-push SHALL be returned to the caller

#### Scenario: Network 404 surfaces visibly

- **WHEN** the panel responds `404 Not Found` to `/panel/api/clients/add` (i.e. the fork lacks the `/clients/*` group)
- **THEN** the runtime SHALL return an error whose message contains the full path that 404'd
- **AND** the runtime SHALL NOT silently fall back to inbound re-push — the operator needs to see "fork version drift" surfaced, not have it papered over with a write that pretends success

## ADDED Requirements

### Requirement: Fork-Aligned Client Routes

The runtime client SHALL speak the MHSanaei/3x-ui fork's
`/panel/api/clients/*` endpoint group (registered in
`web/controller/client.go` `initRouter`) for every per-client
mutation. The legacy `/panel/api/inbounds/addClient`-style
routes SHALL NOT be used.

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
- **AND** the `inboundIds` array SHALL contain exactly the one resolved id for `inboundTag`
- **AND** the runtime SHALL NOT use the legacy `{id: int, settings: stringified-json}` envelope

#### Scenario: UpdateClient identifies by email

- **WHEN** the dashboard calls `Remote.UpdateClient(ctx, inboundTag, client)` with `client.Email != ""`
- **THEN** the runtime SHALL POST to `/panel/api/clients/update/:email` with the raw `model.Client` as the JSON body
- **AND** the inbound tag SHALL only be used by the re-push fallback (Strategy B); the primary path SHALL NOT resolve the inbound to construct the URL

### Requirement: Per-Inbound Client Traffic Reset Decomposes To Email Loop

The fork has no `/inbounds/:id/resetAllClientTraffics` endpoint
(the legacy path the dashboard previously called returns 404).
`Remote.ResetAllClientTraffics(inboundTag)` SHALL therefore
fetch the inbound, enumerate its clients, and call
`/panel/api/clients/resetTraffic/:email` once per client.

#### Scenario: Empty-email client is skipped

- **WHEN** an inbound contains a client whose `email` is the empty string (e.g. a placeholder)
- **THEN** the reset loop SHALL skip that entry without erroring — calling `resetTraffic/` (with a trailing slash) would 404 on the panel

### Requirement: Capability Detection Disallowed

The dashboard SHALL NOT call `/panel/api/inbounds/options` for
capability detection. The endpoint's handler invokes
`session.GetLoginUser(c)`, which returns nil for API-token
(Bearer) callers, so the endpoint responds 404 to every
dashboard-side call. Compatibility with a given node is declared
statically per `docs/operator/3xui-fork-compat.md`.

#### Scenario: Operator wants to probe protocol support

- **GIVEN** the dashboard wants to know whether a node supports WireGuard
- **THEN** the dashboard SHALL NOT GET `/panel/api/inbounds/options`
- **AND** SHALL instead attempt the actual `POST /panel/api/inbounds/add` with `protocol=wireguard` and surface any protocol-validation error to the admin verbatim
