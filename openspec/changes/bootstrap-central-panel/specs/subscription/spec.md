## ADDED Requirements

### Requirement: Per-Client Link Generation

The system SHALL generate Xray connection links for a client from its inbound
configuration.

#### Scenario: Generate links for a client

- **WHEN** the system builds links for a client identified by node + inbound + email
- **THEN** it produces one link per applicable protocol/stream combination
  (VLESS, VMess, Trojan, Shadowsocks) encoding address, port, identifier,
  transport, and TLS/Reality parameters

#### Scenario: Remark formatting

- **WHEN** a link is generated
- **THEN** its remark SHALL be formatted from a configurable remark model
  (inbound remark, email, and node info) so links are human-distinguishable

### Requirement: Subscription Output Formats

The system SHALL serve subscription content in multiple client-compatible formats.

#### Scenario: Base64 subscription

- **WHEN** a subscription is requested in the default format
- **THEN** the system returns the newline-joined link list, base64-encoded, with
  subscription headers (e.g. `Subscription-Userinfo`, update interval, profile title)

#### Scenario: JSON subscription

- **WHEN** a subscription is requested in JSON format
- **THEN** the system returns an Xray-client JSON config assembled from the
  client's inbounds, including routing/fragment/mux options where configured

#### Scenario: Clash subscription

- **WHEN** a subscription is requested in Clash format
- **THEN** the system returns a Clash-compatible YAML config containing the
  client's proxies and a basic proxy group

### Requirement: Tokenized Public Subscription URL

The system SHALL serve subscriptions at a public, unguessable URL bound to a
subscription id, without requiring a dashboard login.

#### Scenario: Valid subscription id

- **WHEN** a client app fetches `/{subPath}/{subId}` for a known subscription id
- **THEN** the system returns that subscription's content in the requested format
  and HTTP 200

#### Scenario: Unknown subscription id

- **WHEN** the requested subscription id matches no client
- **THEN** the system returns HTTP 404 and SHALL NOT leak whether other ids exist

#### Scenario: Aggregated multi-client subscription

- **WHEN** a subscription id maps to several clients across nodes (one user,
  multiple nodes)
- **THEN** the subscription output SHALL include the links for all of that
  subscription id's clients

### Requirement: Subscription Info Headers

The system SHALL include usage and lifecycle metadata in subscription responses.

#### Scenario: Userinfo header

- **WHEN** a base64 subscription is served
- **THEN** the response SHALL include a `Subscription-Userinfo` header reporting
  upload, download, total, and expiry derived from the client's traffic

#### Scenario: Update interval advertised

- **WHEN** any subscription is served
- **THEN** the response SHALL advertise the configured refresh interval to client apps

### Requirement: User Portal Subscription View

End users SHALL be able to view and copy their subscription in the portal.

#### Scenario: User retrieves subscription link

- **WHEN** an authenticated `user` opens the Subscription page
- **THEN** the system returns the user's public subscription URL and the system
  renders it as copyable text and a QR code

#### Scenario: No subscription linked

- **WHEN** a `user` has no client mapped to their account
- **THEN** the portal SHALL show an empty state directing them to contact an
  administrator or purchase a plan
