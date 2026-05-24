# WireGuard setup

WireGuard support in this dashboard requires:

1. Every WG-capable node runs **MHSanaei/3x-ui** fork (the canonical
   3x-ui upstream lacks the WG module — see
   [3xui-node-contract.md](./3xui-node-contract.md)).
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

## End-to-end walkthrough — from zero to a working WG peer

A cookbook for the most common setup: one fresh VPS hosting a
3x-ui fork node, one dashboard, one end user downloading their
`.conf`.

### 1. Prepare the node (5 min, on the VPS)

```sh
# Install MHSanaei/3x-ui fork — pick the latest release.
bash <(curl -Ls https://raw.githubusercontent.com/MHSanaei/3x-ui/main/install.sh)

# On first run the panel prints initial username + password +
# port + base path. Note them.
x-ui settings  # menu lets you change creds; or web UI later

# Enable an API token: panel → Settings → API → Generate Token.
# Copy the token; the dashboard authenticates via this Bearer.
```

The node's kernel needs WireGuard support. Modern Debian / Ubuntu
LTS already have `wireguard-tools`; if `wg show` errors, run
`apt install wireguard` first.

### 2. Configure the dashboard (2 min)

```sh
# In your dashboard checkout's .env:
WG_MASTER_KEY=$(openssl rand -hex 32)

# Restart dashboard so the new key takes effect.
systemctl restart 3xui-dashboard  # or your equivalent
```

Boot log should say `wireguard provisioning enabled`. If you see
`WG_MASTER_KEY not set` instead, double-check the .env path the
dashboard reads from.

### 3. Register the node in the dashboard (1 min, in admin UI)

Admin → 节点 → 新建节点:

- Name: anything memorable (`tokyo-wg1`)
- Host: the VPS's public address
- Port: the panel port (default 54321)
- Base path: whatever the panel installer chose
- API Token: paste from step 1

Save. Within ~30s the probe cron flips status to `online` (green
dot). If it stays `unknown`/`offline`, check that the Bearer
token works:

```sh
curl -sH "Authorization: Bearer <TOKEN>" \
     "https://<host>:<port>/<base>/panel/api/server/status"
# Expect: {"success":true,"obj":{...}}
```

### 4. Create the WG inbound (1 min, in admin UI)

Admin → 入站 → 新建入站:

- Node: the one you just added
- Protocol: `wireguard`
- Port: 51820 (or any free UDP port)
- 备注: a label like `wg-tokyo-home`
- Save

The Stream / Sniffing tabs are hidden for WG — that's expected.
The node generates its server keypair on first POST. The
dashboard records the inbound; refresh and confirm the row shows
up with the emerald `wireguard` chip.

### 5. Provision a user (1 min, in admin UI)

Two paths:

**(a) Via a plan + admin-purchased order**: user logs in once
so a `sub_id` exists, then admin → 用户 → 充值 to give them
balance, then user purchases the plan that targets this inbound.

**(b) Direct provisioning via API** (no plan/order):

```sh
curl -sH "Authorization: Bearer <ADMIN_JWT>" \
     -H "Content-Type: application/json" \
     -X POST \
     "https://<dashboard>/api/admin/clients/provision" \
     -d '{
       "user_id": 42,
       "node_id": 1,
       "inbound_tag": "wg-tokyo-home",
       "duration_days": 30,
       "traffic_limit_bytes": 0
     }'
```

The dashboard acquires `pg_advisory_xact_lock(inbound_id)`,
generates a curve25519 keypair, allocates the next free IP
(skips `.0` + `.1` of `10.0.0.0/24`), POSTs the new peer to the
node, and writes the `wg_peers` row with the AES-sealed private
key.

### 6. User downloads their `.conf` (the end-user side)

The user opens the portal → 订阅 page, picks the **WireGuard**
format chip, hits 下载文件. The URL behind the button is

```
https://<dashboard>/sub/wireguard/<user-sub-id>
```

— shareable directly. For users with multiple WG peers across
different nodes, the **WG (ZIP)** format chip bundles all of
them into one archive (`Content-Disposition: attachment`).

### 7. Connect (on the user's device)

- **macOS / iOS / Android**: WireGuard official app → import
  from file (or paste). Toggle the tunnel on.
- **Linux**: `wg-quick up ./wg-tokyo-home.conf`.
- **Windows**: WireGuard app → add tunnel from file.

If the tunnel handshakes (`wg show` on the node lists the peer
as `latest handshake`) but traffic doesn't route, the node's
NAT forwarding is missing:

```sh
# On the node, one-shot:
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
sysctl -p
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
# Persist iptables with iptables-persistent or your distro's
# equivalent. Run `iptables -L -t nat` to verify.
```

### Common first-day pitfalls

| Symptom | Likely cause |
|---|---|
| Admin UI shows the WG inbound but `wg show` on the node has no listener | The panel didn't start its WG service. Restart Xray from the panel's web UI — first-time WG inbound creation often needs that one nudge |
| Peer handshakes but no DNS | `.conf` ships `DNS = 1.1.1.1, 8.8.8.8` — works for most cases. China / sanctioned regions: edit to a CDN-routed DNS the node can reach |
| New WG peer 404 in the wg_peers table after a successful provision | A previous DB migration is missing — `\d wg_peers` should show the table. Run `dashboard migrate up` |

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
