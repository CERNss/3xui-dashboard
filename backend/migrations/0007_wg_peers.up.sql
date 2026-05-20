-- wg_peers — per-peer state for WireGuard clients. One row per
-- (client_ownership, inbound) pair. The private key is stored
-- AES-256-GCM-encrypted with WG_MASTER_KEY; the dashboard never
-- writes the plaintext to disk.
--
-- The fork's WG inbound stores all peer state in its own
-- settings.peers[] JSON; this table is the dashboard-side mirror
-- so we can (a) re-render subscriptions without round-tripping to
-- the node for the private key, and (b) reconcile drift if the
-- panel and dashboard disagree.
CREATE TABLE wg_peers (
    id                    BIGSERIAL    PRIMARY KEY,
    client_ownership_id   BIGINT       NOT NULL UNIQUE REFERENCES client_ownerships(id) ON DELETE CASCADE,
    inbound_id            BIGINT       NOT NULL,
    public_key            TEXT         NOT NULL,
    private_key_encrypted BYTEA        NOT NULL,
    allocated_ip          INET         NOT NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Unique allocated_ip per inbound so two peers never collide on
-- the same address within the same WG tunnel.
CREATE UNIQUE INDEX wg_peers_inbound_ip ON wg_peers (inbound_id, allocated_ip);
