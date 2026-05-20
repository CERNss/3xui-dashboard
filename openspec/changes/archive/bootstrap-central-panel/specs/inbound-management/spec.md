## ADDED Requirements

### Requirement: List Inbounds Per Node

Administrators SHALL be able to list all Xray inbounds on a selected node.

#### Scenario: List a node's inbounds

- **WHEN** an admin requests inbounds for a node id
- **THEN** the system calls the node's `/panel/api/inbounds/list`, decodes the
  envelope, and returns each inbound with id, tag, remark, protocol, port,
  enable flag, up/down/total traffic, expiry, and client stats

#### Scenario: Node unreachable

- **WHEN** the target node is offline or returns an error
- **THEN** the system returns a clear error identifying the node and SHALL NOT
  fail the entire request for other nodes in a fleet-wide query

### Requirement: Fleet-Wide Inbound View

Administrators SHALL be able to view inbounds aggregated across all enabled nodes.

#### Scenario: Aggregate inbound listing

- **WHEN** an admin requests the fleet inbound view
- **THEN** the system queries each enabled node concurrently and returns inbounds
  annotated with their owning node id and node name

#### Scenario: Partial failure is surfaced

- **WHEN** some nodes fail during a fleet-wide query
- **THEN** the response includes results from healthy nodes plus a per-node
  error list for the failed ones

### Requirement: Create Inbound

Administrators SHALL be able to create an inbound on a node.

#### Scenario: Create inbound on a node

- **WHEN** an admin submits an inbound (protocol, port, listen, settings,
  streamSettings, sniffing, remark, tag, total, expiry) for a node id
- **THEN** the system posts it to the node's `/panel/api/inbounds/add` and
  returns the created inbound including its remote id and tag

#### Scenario: Tag-to-remote-id cache primed

- **WHEN** an inbound is created successfully
- **THEN** the system SHALL cache the mapping from the inbound tag to its remote
  id for that node to avoid extra lookups on later operations

### Requirement: Update Inbound

Administrators SHALL be able to update an existing inbound on a node.

#### Scenario: Update by tag resolution

- **WHEN** an admin updates an inbound and the system knows the inbound tag
- **THEN** the system resolves the tag to the node's remote inbound id (using
  the cache, refreshing from `/panel/api/inbounds/list` on a miss) and posts to
  `/panel/api/inbounds/update/{id}`

#### Scenario: Update falls back to create

- **WHEN** the inbound tag cannot be resolved on the node (it no longer exists)
- **THEN** the system SHALL create the inbound instead so state converges

#### Scenario: Stream settings sanitized

- **WHEN** an inbound's TLS stream settings contain both inline certificate
  content and file paths
- **THEN** the redundant file paths SHALL be stripped before sending to the node,
  while entries that contain only file paths are left untouched

### Requirement: Delete Inbound

Administrators SHALL be able to delete an inbound from a node.

#### Scenario: Delete an inbound

- **WHEN** an admin deletes an inbound
- **THEN** the system resolves the tag to its remote id and posts to
  `/panel/api/inbounds/del/{id}`, then evicts the tag from the cache

#### Scenario: Already-deleted inbound

- **WHEN** the inbound tag cannot be resolved (already gone)
- **THEN** the delete SHALL be treated as a success (idempotent) with a warning logged
