# ⓘ PROMOTED 2026-05-21

> Shipped scenarios folded into
> `openspec/specs/client-provisioning/spec.md` (Protocol Branching
> §"Hysteria provision uses auth field").

## ADDED Requirements

### Requirement: Hysteria Client Provisioning

`ClientService.Provision` SHALL support `inbound.protocol ==
"hysteria"`. The flow reuses the unified `POST /panel/api/clients/add`
endpoint — Hysteria differs from VLESS/VMess only in which Client
struct field carries the per-client credential.

#### Scenario: Provision Hysteria client

- **WHEN** `ProvisionClient(user, hysteria_inbound)` is called
- **THEN** the system SHALL generate a random `auth` string (16 URL-safe characters via `crypto/rand`)
- **AND** SHALL build a `model.Client` with `Auth` populated, `Id` and `Password` empty, plus the standard `Email`, `SubID`, `ExpiryTime`, `LimitIP`, `TotalGB`, `Enable`, `TgID`, `Reset` fields
- **AND** SHALL POST `/panel/api/clients/add` with `{client: {...}, inboundIds: [hysteria_inbound.id]}`
- **AND** SHALL insert a `client_ownerships` row identical in shape to non-WG protocols

#### Scenario: TLS configuration is operator's responsibility

- **WHEN** an admin creates a Hysteria inbound
- **THEN** the system SHALL store `tlsSettings.certificates[].certificateFile` and `.keyFile` as node-local filesystem paths
- **AND** the dashboard SHALL NOT offer a cert upload widget — operator manages certs externally (acme.sh, certbot on the node)
- **AND** the inbound editor SHALL display "node-local file path required" next to the cert path inputs
