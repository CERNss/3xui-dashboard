DROP INDEX IF EXISTS notification_log_dedup;

ALTER TABLE notification_log
    DROP COLUMN surface;

CREATE UNIQUE INDEX notification_log_dedup
    ON notification_log (kind, ownership_id);
