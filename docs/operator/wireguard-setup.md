# WireGuard setup

WireGuard support in this dashboard requires:

1. Every WG-capable node runs **MHSanaei/3x-ui** fork (the canonical
   3x-ui upstream lacks the WG module — see
   [3xui-fork-compat.md](./3xui-fork-compat.md)).
2. The dashboard has `WG_MASTER_KEY` configured in env.

If either is missing the dashboard logs the gap at boot and refuses
to create WG inbounds. Non-WG flows are unaffected.

## Generate WG_MASTER_KEY

```sh
openssl rand -hex 32
```

Paste the 64-char hex output into `.env` (gitignored) or your
secrets manager:

```
WG_MASTER_KEY=<64-hex-char value>
```

The dashboard loads this once at boot and uses it to AES-256-GCM
encrypt every WG peer's private key before writing to the
`wg_peers.private_key_encrypted` bytea column. The plaintext
private key never touches disk.

## Rotation hazard

**v1 has no key rotation flow.** Rotating `WG_MASTER_KEY` makes
every existing `wg_peers` row unreadable — the GCM authentication
tag won't verify under a different key, and the subscription
renderer drops those peers from output.

If you must rotate:

1. Disable every WG inbound in the admin UI so no new peers land
2. Delete the affected `wg_peers` rows (they're regenerated)
3. Set the new key + restart
4. Re-provision each user (the dashboard generates new keypairs)

Plan a maintenance window — every WG client will need to re-import
their `.conf`.

## Creating a WG inbound

1. Admin → Inbounds → 新建入站
2. Protocol = `wireguard`
3. Port = your WG listener (default 51820 if free)
4. Save

The node side generates the server keypair on first POST; the
dashboard stores the public key (read from `settings.secretKey`
roundtrip) for use when rendering peer `.conf` bodies.

There is no need to fill in Stream / Sniffing tabs — those
tabs hide when protocol is `wireguard` because WG carries its
own UDP transport.

## Provisioning a peer

When a portal user buys a plan whose inbound is WG, the
provisioning flow:

1. Acquires `pg_advisory_xact_lock(inbound_id)` to serialize
   dashboard-side RMW cycles on this inbound
2. Generates a curve25519 keypair locally
3. Allocates the next free IP from `10.0.0.0/24` (skipping
   `.0` network and `.1` gateway), drift-tolerant against
   addresses the panel knows about but the dashboard doesn't
4. POSTs the updated `inbound.settings.peers[]` back via
   `/panel/api/inbounds/update/:id`
5. Inserts the `wg_peers` row with the AES-256-GCM-sealed
   private key

The peer's `.conf` is then served at
`https://<dashboard>/sub/wireguard/<sub_id>` or as a ZIP at
`/sub/wireguard-zip/<sub_id>`.

## Concurrency: dashboard ↔ panel UI

The `pg_advisory_xact_lock` only serializes mutations made
**through the dashboard**. If an operator manually edits an
inbound's peers[] via the 3x-ui panel UI at the same time the
dashboard is mid-RMW, the panel UI's last-write-wins and the
dashboard's POST can clobber a manual edit (or vice versa).

**Operational rule of thumb:** once a node is registered with
the dashboard, manage WG peers ONLY from the dashboard. The
panel UI's peer editor exists but using it from both sides is
unsafe.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `.conf` body is empty | No `wg_peers` row for this ownership — re-provision |
| Subscription returns 404 on `/sub/wireguard/:id` | sub_id wrong or WG_MASTER_KEY not set on dashboard |
| Peer can't connect, dashboard config looks right | Node-side WG kernel module / iptables forwarding rules missing — check `wg show` on the node |
| Allocator returns "subnet exhausted" | More than 252 peers on one inbound — split into multiple inbounds, the `/24` is hardcoded in v1 |
| AES-GCM open errors in logs | WG_MASTER_KEY was rotated mid-flight; see "Rotation hazard" above |

## What v1 does NOT support

- IPv6 peer allocation (subnet is hardcoded to `10.0.0.0/24`)
- Custom AllowedIPs per peer (every peer gets `0.0.0.0/0, ::/0`)
- Pre-shared keys (`psk`) — the schema accepts them but the
  provisioner never sets them
- Multiple WG inbounds with overlapping subnets (each inbound's
  allocator runs against its own row set, but if you reuse
  `10.0.0.0/24` across two inbounds the IPs can collide at the
  Linux routing layer — use distinct subnets manually for now)
- Key rotation (see "Rotation hazard" above)

These all land in v2 when there's actual demand. File an issue
with the use case if you need one.
