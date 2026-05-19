BEGIN;

DROP INDEX IF EXISTS webhook_deliveries_due;
ALTER TABLE webhook_deliveries DROP COLUMN IF EXISTS next_attempt_at;

COMMIT;
