# netsafe-ssrf-guard

The SSRF-guarded `net.Dialer` and `http.Transport` used by every
outbound HTTP call originating from the dashboard. Lives in
`internal/netsafe`.

## Purpose & boundaries

There are two outbound surfaces a malicious or misconfigured input
could weaponize against internal infrastructure:

1. **Node runtime** — admin-supplied node URLs. Usually point at a
   public 3x-ui panel, but homelab deployments need to reach
   `10.0.0.0/8` etc., so an opt-in private-range allowance exists.
2. **Webhook delivery** — admin-supplied webhook URLs. Hard-blocked
   from private ranges (an admin should not be sending dashboard
   events to `169.254.169.254` / internal services unless we
   intentionally enable that).

Both surfaces share the same dialer; node-runtime calls attach a
context sentinel to opt into private ranges per-dial.

## Blocked address ranges

The guard refuses (by default) any dial whose resolved IP is in:

- Loopback (`127.0.0.0/8`, `::1/128`).
- Link-local (`169.254.0.0/16`, `fe80::/10`) — covers AWS / GCP
  metadata `169.254.169.254`.
- RFC 1918 private (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`).
- RFC 4193 unique-local (`fc00::/7`).
- RFC 6598 CGNAT (`100.64.0.0/10`).
- Multicast (`224.0.0.0/4`, `ff00::/8`).
- Unspecified (`0.0.0.0`, `::`).

## Requirements

### Requirement: Default Block On Private + Loopback Addresses

The system SHALL refuse any TCP dial whose resolved IP falls into the
blocked ranges above, unless the caller has explicitly opted in via
`WithAllowPrivate`.

#### Scenario: Webhook dial to RFC1918 refused

- **GIVEN** an admin has registered a webhook target `http://192.168.1.10/hook`
- **WHEN** the webhook delivery transport tries to dial that address
- **THEN** `netsafe.Dialer.DialContext` SHALL return an error before opening the connection
- **AND** the error message SHALL identify the resolved IP and the blocked range

#### Scenario: Loopback refused

- **WHEN** any non-opted-in dial resolves to `127.0.0.1` or `::1`
- **THEN** the dial SHALL be refused

#### Scenario: Metadata service refused

- **WHEN** any non-opted-in dial resolves to `169.254.169.254`
- **THEN** the dial SHALL be refused (link-local range)

### Requirement: Opt-In Private Allowance For Node Calls

The system SHALL permit private/loopback dials only when the dialing
goroutine has attached `WithAllowPrivate` to its context.

#### Scenario: Node runtime opts in per-dial

- **GIVEN** the node-runtime transport wraps each outbound call with `ctx = netsafe.WithAllowPrivate(ctx)`
- **WHEN** the dial reaches the guarded `DialContext`
- **THEN** the guard SHALL detect the sentinel and skip the address-range check
- **AND** the dial SHALL proceed normally

#### Scenario: Sentinel is per-dial, not process-global

- **WHEN** one goroutine attaches `WithAllowPrivate` to its context
- **THEN** dials from other goroutines (with their own context) SHALL NOT be affected — the sentinel does not leak across calls

### Requirement: DNS Resolution Before The Check

The system SHALL resolve the dial target's hostname before applying
the range check, so a malicious DNS that returns a private IP is
caught (defense against rebinding-at-resolve).

#### Scenario: Hostname resolves to private IP

- **WHEN** the dial target is `evil.example.com`, which DNS resolves to `10.1.2.3`
- **THEN** the resolved address SHALL be checked against the blocked ranges
- **AND** the dial SHALL be refused

> The current implementation does NOT defend against DNS-rebinding
> mid-connection (split-horizon attacks where the TCP target IP
> differs from the TLS verification IP). For our threat model — admin
> can always set the URL — this is acceptable.

### Requirement: HTTP Transport Wrappers

The system SHALL expose `netsafe.HTTPTransport()` (or equivalent) that
returns an `*http.Transport` whose `DialContext` is the guarded
dialer, with sane timeouts.

#### Scenario: Webhook service uses the wrapped transport

- **WHEN** `webhook-notifications` constructs its outbound HTTP client
- **THEN** the client's `Transport` SHALL be the netsafe-wrapped one
- **AND** the wrapped transport SHALL carry a per-dial timeout of 10s and an overall request timeout of 30s

#### Scenario: Runtime manager uses the wrapped transport

- **WHEN** `runtime.Manager` constructs a node `Remote`
- **THEN** the transport SHALL be the netsafe-wrapped one
- **AND** node-runtime callers SHALL wrap their context with `WithAllowPrivate` before issuing requests

## Out of scope

- IPv6 address-range coverage beyond the listed prefixes.
- TOCTOU defense against DNS rebinding mid-connection.
- Allowlist of specific private hosts (e.g. "block all private except
  10.0.0.5") — the per-dial sentinel is the only granularity.
