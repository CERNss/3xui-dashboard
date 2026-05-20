-- Add `protocol` to client_ownerships so the ExpiryJob batch-knows
-- WG vs non-WG without doing a GetInbound() per row. Nullable for
-- legacy rows; ClientService.ProvisionClient populates forward, and
-- the ExpiryJob falls back to runtime lookup when the column is
-- NULL so existing deployments keep working without a backfill.
ALTER TABLE client_ownerships
    ADD COLUMN protocol TEXT;

-- Partial index so the WG-aware ExpiryJob branch can short-circuit
-- the per-row inbound fetch when scanning expired rows.
CREATE INDEX client_ownerships_protocol_idx
    ON client_ownerships (protocol)
    WHERE protocol IS NOT NULL;
