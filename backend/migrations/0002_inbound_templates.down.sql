BEGIN;

ALTER TABLE provisioning_pool_targets
  DROP COLUMN IF EXISTS generated,
  DROP COLUMN IF EXISTS template_id;

DROP INDEX IF EXISTS provisioning_pools_template_id;

ALTER TABLE provisioning_pools
  DROP CONSTRAINT IF EXISTS provisioning_pools_max_clients_chk,
  DROP COLUMN IF EXISTS node_ids,
  DROP COLUMN IF EXISTS max_clients,
  DROP COLUMN IF EXISTS template_id;

DROP TABLE IF EXISTS inbound_templates;

COMMIT;
