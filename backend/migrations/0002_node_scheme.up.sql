-- Adds the panel HTTP scheme column to nodes. The dashboard talks
-- to most stock 3x-ui installations over https (port 2053 + auto-TLS
-- by default), so 'https' is the default for the column. Homelab
-- installs with plain HTTP can override per node.

BEGIN;

ALTER TABLE nodes ADD COLUMN scheme TEXT NOT NULL DEFAULT 'https';
ALTER TABLE nodes ADD CONSTRAINT nodes_scheme_chk CHECK (scheme IN ('http', 'https'));

COMMIT;
