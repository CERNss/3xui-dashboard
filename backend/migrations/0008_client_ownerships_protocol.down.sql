DROP INDEX IF EXISTS client_ownerships_protocol_idx;
ALTER TABLE client_ownerships DROP COLUMN IF EXISTS protocol;
