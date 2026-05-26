# traffic-statistics

Per-node snapshot collection, aggregated reporting at node/inbound/
client levels, time-bucketed history for charts, traffic-reset
operations, and the user-facing portal usage view.

## Purpose & boundaries

Adjacent modules: **`scheduler-jobs`** drives the collection cron;
**`event-bus`** routes the threshold/exhausted events to webhooks;
**`subscription`** consumes the user-side usage figures for the
`Subscription-Userinfo` header.

## Requirements

### Requirement: Traffic Snapshot Collection

The system SHALL periodically collect a traffic snapshot from every enabled node.

#### Scenario: Collect a node snapshot

- **WHEN** the collection loop runs for an enabled node
- **THEN** the system fetches the node's inbound list, online client emails, and
  last-online timestamps, and stores a snapshot keyed by node id and time

#### Scenario: Node failure during collection

- **WHEN** a node fails to return a snapshot
- **THEN** the failure is logged, the node's other metrics are left intact, and
  collection continues for the remaining nodes

### Requirement: Aggregated Usage Reporting

The system SHALL report traffic usage aggregated at node, inbound, and client levels.

#### Scenario: Per-node totals

- **WHEN** an admin requests node-level traffic
- **THEN** the system returns upload, download, and total bytes per node for the
  selected time range
- **AND** when both inbound rollup samples and per-client samples exist for the
  same inbound, the system SHALL count the inbound rollup once and SHALL NOT add
  the client samples again

#### Scenario: Per-client usage

- **WHEN** an admin or owning user requests a client's usage
- **THEN** the system returns the client's upload, download, total bytes, the
  configured traffic limit, and the remaining allowance

#### Scenario: Online client detection

- **WHEN** usage is reported
- **THEN** each client SHALL be flagged online or offline based on the node's
  most recent online-emails snapshot, with its last-online timestamp included

### Requirement: Traffic History for Charts

The system SHALL retain time-bucketed traffic history to drive charts.

#### Scenario: Bucketed history query

- **WHEN** a caller requests traffic history for a node, inbound, or client with
  a bucket size and range
- **THEN** the system returns an ordered series of buckets, each with upload and
  download deltas for that interval

#### Scenario: Counter reset handled

- **WHEN** a node's traffic counter decreases between two snapshots (e.g. after a
  reset or restart)
- **THEN** the system SHALL treat the delta as the new value rather than a
  negative number, avoiding corrupt history

### Requirement: Traffic Reset

Administrators SHALL be able to reset traffic counters.

#### Scenario: Reset a single client

- **WHEN** an admin resets a client's traffic
- **THEN** the system posts to the node's reset-client-traffic endpoint and the
  client's up/down counters return to zero

#### Scenario: Reset an inbound or node

- **WHEN** an admin resets all client traffic on an inbound, or all traffic on a node
- **THEN** the system posts to the corresponding node reset endpoint and confirms success

### Requirement: User-Facing Traffic View

End users SHALL be able to view their own traffic usage.

#### Scenario: User views own usage

- **WHEN** an authenticated `user` opens their dashboard
- **THEN** the system returns, for each client mapped to that user, the used vs.
  total traffic, percentage consumed, and days remaining until expiry

#### Scenario: User cannot see others' traffic

- **WHEN** a `user` requests traffic data
- **THEN** the response SHALL contain only clients owned by that user
