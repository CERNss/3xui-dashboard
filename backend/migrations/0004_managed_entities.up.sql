-- ---------------------------------------------------------------------------
-- Provenance ledger: rows exist only for inbounds/clients this dashboard
-- created. client_ownerships stays the user↔client billing bridge; these
-- tables are the "created by us" signal (there is deliberately no adopt
-- flow — pre-existing upstream entities stay external forever).
-- ---------------------------------------------------------------------------

CREATE TABLE managed_inbounds (
  id           BIGSERIAL    PRIMARY KEY,
  node_id      BIGINT       NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  inbound_tag  TEXT         NOT NULL,
  created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT managed_inbounds_unique UNIQUE (node_id, inbound_tag)
);

CREATE TABLE managed_clients (
  id            BIGSERIAL    PRIMARY KEY,
  node_id       BIGINT       NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  inbound_tag   TEXT         NOT NULL,
  client_email  TEXT         NOT NULL,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT managed_clients_unique UNIQUE (node_id, inbound_tag, client_email)
);

-- Backfill: every existing ownership row was written by the dashboard's
-- own provisioning paths (the link UI never shipped), so they are all
-- dashboard-created clients. No equivalent backfill is possible for
-- inbounds — nothing recorded their origin before this table existed.
INSERT INTO managed_clients (node_id, inbound_tag, client_email, created_at)
SELECT node_id, inbound_tag, client_email, created_at FROM client_ownerships
ON CONFLICT DO NOTHING;
