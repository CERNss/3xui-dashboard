-- email_outbox — persistent queue for outgoing email so a transient
-- SMTP failure doesn't lose verification codes or notifications.
-- Status lifecycle:
--   pending  → worker picks it up
--   sending  → row is locked by a worker (SELECT FOR UPDATE SKIP LOCKED)
--   sent     → terminal success
--   failed   → terminal failure after max_attempts
--
-- next_attempt_at lets the worker do exponential backoff between
-- retries without polling rows that aren't due yet.
CREATE TABLE email_outbox (
    id              BIGSERIAL    PRIMARY KEY,
    to_addr         TEXT         NOT NULL,
    subject         TEXT         NOT NULL,
    body            TEXT         NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'pending',
    attempts        INT          NOT NULL DEFAULT 0,
    last_error      TEXT         NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ
);

-- The worker query is "WHERE status = 'pending' AND next_attempt_at <= now()
-- ORDER BY next_attempt_at LIMIT N FOR UPDATE SKIP LOCKED". A partial
-- index on pending rows keeps it tight even after the table grows.
CREATE INDEX email_outbox_pending_ready
    ON email_outbox (next_attempt_at)
    WHERE status = 'pending';
