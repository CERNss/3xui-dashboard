-- Add `protocol` to client_ownerships so the ExpiryJob batch-knows
-- WG vs non-WG without doing a GetInbound() per row. Nullable so
-- the ExpiryJob can fall back to runtime lookup when older rows have
-- not been touched by ClientService.ProvisionClient.
ALTER TABLE client_ownerships
    ADD COLUMN protocol TEXT;

-- Partial index so the WG-aware ExpiryJob branch can short-circuit
-- the per-row inbound fetch when scanning expired rows.
CREATE INDEX client_ownerships_protocol_idx
    ON client_ownerships (protocol)
    WHERE protocol IS NOT NULL;
