BEGIN;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS plans_ip_limit_chk;

ALTER TABLE plans
    DROP COLUMN IF EXISTS provisioning_pool_id,
    DROP COLUMN IF EXISTS ip_limit;

DROP TABLE IF EXISTS provisioning_pool_targets;
DROP TABLE IF EXISTS provisioning_pools;

COMMIT;
