-- Persistent dedup for outbound user notifications.
--
-- Each row records that we have already sent one (kind, ownership_id)
-- notification — the unique index on (kind, ownership_id) is the
-- dedup boundary. The first send wins; subsequent attempts hit a
-- conflict and the notify service silently skips.
--
-- Why DB-backed instead of process memory:
--   - Restarts mid-expiry-window would re-spam the user otherwise.
--   - One canonical source of truth across replicas if/when we scale.
--   - Easier to audit ("did Alice actually get her expiry warning?")
--
-- The table is append-only — no UPDATE / DELETE in the service layer.
-- Cleanup of old rows (after the ownership row itself is gone) is
-- handled by the ON DELETE CASCADE on ownership_id.

CREATE TABLE notification_log (
    id             BIGSERIAL PRIMARY KEY,
    kind           TEXT        NOT NULL,
    ownership_id   BIGINT      NOT NULL REFERENCES client_ownerships(id) ON DELETE CASCADE,
    user_email     TEXT        NOT NULL DEFAULT '',
    sent_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX notification_log_dedup
    ON notification_log (kind, ownership_id);
