-- Email verification codes. Short-lived 6-digit codes used to prove email
-- ownership during register and (future) password reset. Codes are scoped
-- by purpose so a register code can't be replayed to reset a password.
--
-- Rate limiting on send is enforced by checking sent_at on the most recent
-- row for (email, purpose) — 60s minimum between sends.
--
-- Codes are single-use: consumed_at flips on successful use, after which
-- the row is kept for audit but cannot be re-consumed.

CREATE TABLE email_verification_codes (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT        NOT NULL,
    purpose         TEXT        NOT NULL,                   -- 'register' | 'reset' | …
    code_hash       TEXT        NOT NULL,                   -- bcrypt or sha256 of the digit code
    expires_at      TIMESTAMPTZ NOT NULL,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at     TIMESTAMPTZ,
    attempts        INT         NOT NULL DEFAULT 0
);

-- Lookup index for SendCode rate-limit check + Consume validation.
CREATE INDEX email_verification_codes_active
    ON email_verification_codes (email, purpose, sent_at DESC);

-- Lookup for consume() — find the most recent unconsumed, unexpired code.
CREATE INDEX email_verification_codes_unconsumed
    ON email_verification_codes (email, purpose, expires_at)
    WHERE consumed_at IS NULL;
