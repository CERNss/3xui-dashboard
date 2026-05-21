-- Split notification_log by surface: 'message' (user-facing, SMTP only)
-- vs 'notification' (ops-facing, multi-channel + admin webhooks).
--
-- Same kind name can now dedup independently per surface — e.g. a
-- "low_balance" sent to the user (message) doesn't block a
-- "low_balance" ops alert (notification) from also firing.
ALTER TABLE notification_log
    ADD COLUMN surface TEXT NOT NULL DEFAULT 'notification'
        CHECK (surface IN ('message', 'notification'));

-- Drop the old (kind, ownership_id) dedup and replace with one that
-- includes surface. Existing rows all default to 'notification' so
-- the historical dedup boundary is preserved for the ops side.
DROP INDEX IF EXISTS notification_log_dedup;

CREATE UNIQUE INDEX notification_log_dedup
    ON notification_log (surface, kind, ownership_id);
