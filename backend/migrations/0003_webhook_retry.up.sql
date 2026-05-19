-- Adds the persistent retry queue column to webhook_deliveries.
--
-- Status semantics change:
--   pending — eligible for delivery (now or later); the cron job
--             picks rows where next_attempt_at <= now()
--   success — delivered successfully (terminal)
--   failed  — attempt count exhausted (terminal, no more retries)
--
-- Before this migration, every failure flipped status to "failed"
-- immediately, even when more retries were planned in memory. The
-- in-memory retry would then run against a status-failed row, and a
-- process crash dropped the queued retry entirely. With
-- next_attempt_at, retries are stored in the DB and survive restarts.

BEGIN;

ALTER TABLE webhook_deliveries
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Backfill: existing rows are treated as terminal — their
-- next_attempt_at default is "now" but the cron job filters by
-- status='pending', so rows already in failed/success won't be
-- picked up.

-- One index serves both the cron scan (status, next_attempt_at)
-- and the per-webhook history listing (webhook_id, scheduled_at).
CREATE INDEX IF NOT EXISTS webhook_deliveries_due
    ON webhook_deliveries (status, next_attempt_at)
    WHERE status = 'pending';

COMMIT;
