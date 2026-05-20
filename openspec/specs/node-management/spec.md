# node-management

Node CRUD, periodic health probing, in-memory metric history, and the
SSRF-safe transport that fronts every node call.

## Purpose & boundaries

Adjacent modules: **`runtime-3xui-client`** owns the per-node bearer
HTTP client + envelope decode; this module owns the database row +
probe loop on top of it. **`scheduler-jobs`** drives the periodic
probe cron.

## Requirements

### Requirement: Node Registration

Administrators SHALL be able to register a remote 3x-ui node with the connection
details required to control it.

#### Scenario: Register a node

- **WHEN** an admin submits a node with name, scheme (`http`/`https`), address,
  port, base path, and API token
- **THEN** the system normalizes the values, persists the node, and the node
  becomes available for inbound/client operations

#### Scenario: Reject invalid node input

- **WHEN** an admin submits a node with an empty name, a port outside 1–65535,
  or an unresolvable address
- **THEN** the system returns a validation error and does not persist the node

#### Scenario: Base path normalization

- **WHEN** a node is saved with a base path missing leading/trailing slashes (or empty)
- **THEN** the system SHALL normalize it to a form bounded by `/` (empty becomes `/`)

### Requirement: Node Lifecycle Management

Administrators SHALL be able to edit, enable/disable, and delete registered nodes.

#### Scenario: Edit a node

- **WHEN** an admin updates a node's connection fields
- **THEN** the changes are persisted and any cached runtime/transport for that
  node SHALL be invalidated so the next operation uses fresh values

#### Scenario: Disable a node

- **WHEN** an admin disables a node
- **THEN** the node SHALL be excluded from probing and SHALL reject inbound/client
  operations with a clear "node disabled" error

#### Scenario: Delete a node

- **WHEN** an admin deletes a node
- **THEN** the node record and its in-memory metric history SHALL be removed

### Requirement: Node Health Probing

The system SHALL periodically probe each enabled node and record a heartbeat.

#### Scenario: Successful probe

- **WHEN** the probe loop calls a node's `/panel/api/server/status` with its
  Bearer token and receives `success: true`
- **THEN** the node status SHALL be set to `online`, and latency, Xray version,
  CPU %, memory %, and uptime SHALL be recorded with the heartbeat timestamp

#### Scenario: Failed probe

- **WHEN** a probe times out, returns a non-200 status, or returns `success: false`
- **THEN** the node status SHALL be set to `offline` and the failure reason
  SHALL be stored as the node's last error

#### Scenario: On-demand probe

- **WHEN** an admin triggers an immediate probe of a single node
- **THEN** the system performs the probe synchronously and returns the result
  (status, latency, version, cpu/mem, uptime, error) without waiting for the loop

### Requirement: Node Metric History

The system SHALL retain short-term CPU and memory history per node for charting.

#### Scenario: Metrics appended on heartbeat

- **WHEN** a probe marks a node `online`
- **THEN** the CPU % and memory % samples SHALL be appended to that node's
  in-memory time series keyed by node id

#### Scenario: Aggregated metric query

- **WHEN** an admin requests a node's CPU or memory history with a bucket size
  and max point count
- **THEN** the system returns time-bucketed aggregated samples suitable for a chart

### Requirement: SSRF-Safe Node Transport

All outbound HTTP calls to nodes SHALL go through a guarded transport.

#### Scenario: Private address blocked by default

- **WHEN** a node address resolves to a private/loopback range and the node does
  not have "allow private address" enabled
- **THEN** the connection SHALL be refused by the guarded dialer

#### Scenario: Private address allowed when opted in

- **WHEN** a node has "allow private address" enabled
- **THEN** connections to private ranges for that node SHALL be permitted
