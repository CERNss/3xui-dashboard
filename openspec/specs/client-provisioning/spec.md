# client-provisioning

Xray client lifecycle on the node fleet: create, update, delete, list,
plus the internal provisioning hook shared by admin CRUD and plan
purchase.

## Purpose & boundaries

Adjacent modules: **`inbound-management`** owns the inbound containers
clients live inside; **`user-accounts`** owns the user→client ownership
table (`client_ownerships`); **`runtime-3xui-client`** owns the bearer-
authed HTTP wire to each node.

## Requirements

### Requirement: Create Client on Inbound

Administrators SHALL be able to create a client ("link") on a node's inbound.

#### Scenario: Create a client

- **WHEN** an admin submits a client (email, optional UUID/password, traffic
  limit in GB, expiry time, IP limit, enable flag, subscription id, comment)
  for a target node id and inbound
- **THEN** the system posts it to the node's `/panel/api/inbounds/addClient`
  and returns the created client including its generated identifier and the
  subscription id

#### Scenario: Generated identifiers

- **WHEN** a client is created without an explicit UUID/password
- **THEN** the system SHALL generate a protocol-appropriate identifier (UUID for
  VLESS/VMess, password for Trojan/Shadowsocks) and a subscription id if absent

#### Scenario: Duplicate email rejected

- **WHEN** an admin creates a client with an email that already exists on the inbound
- **THEN** the node's error is surfaced and no client is created

### Requirement: Update Client

Administrators SHALL be able to update an existing client, addressed by its UUID.

#### Scenario: Update client limits

- **WHEN** an admin updates a client's traffic limit, expiry, IP limit, comment,
  or enable flag
- **THEN** the system posts the change to `/panel/api/inbounds/updateClient/{uuid}`
  and the node applies it without affecting other clients on the inbound

#### Scenario: Enable/disable a client

- **WHEN** an admin toggles a client's enable flag
- **THEN** the client SHALL be activated or deactivated on the node, and a
  disabled client SHALL NOT be able to connect

### Requirement: Delete Client

Administrators SHALL be able to delete a client from an inbound.

#### Scenario: Delete a client

- **WHEN** an admin deletes a client by UUID
- **THEN** the system posts to `/panel/api/inbounds/delClient/{uuid}` and, if the
  client was mapped to a dashboard user, that ownership mapping SHALL be cleared

### Requirement: Client Listing

Administrators SHALL be able to list clients across an inbound or the whole fleet.

#### Scenario: List clients of an inbound

- **WHEN** an admin requests clients for a node + inbound
- **THEN** the system returns each client with email, identifier, traffic
  up/down, total limit, expiry, IP limit, enable flag, and owning dashboard user
  (if mapped)

#### Scenario: Search clients fleet-wide

- **WHEN** an admin searches clients by email across the fleet
- **THEN** the system returns matching clients annotated with their node and inbound

### Requirement: Self-Service Client Provisioning Hook

The system SHALL expose a single internal provisioning operation that creates or
extends a client and records its ownership, reused by both admin actions and
plan purchases.

#### Scenario: Provision creates and maps

- **WHEN** the provisioning operation runs for a dashboard user with a chosen
  node + inbound + plan parameters
- **THEN** it creates the client on the node, persists the client→user ownership
  mapping, and returns the resulting client and subscription id

#### Scenario: Provision extends an existing client

- **WHEN** the provisioning operation runs for a user who already owns a client
- **THEN** it SHALL extend that client's expiry and/or traffic limit instead of
  creating a duplicate
