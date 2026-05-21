-- auto_renew — admin per-user toggle for the auto-renewal cron.
-- When true, the AutoRenewJob attempts to use the user's balance
-- to repurchase the same plan when their ownership row is within
-- 24h of expiry. Defaults to false: only an explicit admin opt-in
-- gets a user enrolled.
ALTER TABLE users
    ADD COLUMN auto_renew BOOLEAN NOT NULL DEFAULT FALSE;

-- Partial index over the candidates the cron actually looks at.
-- Keeps the scan tight even when the users table grows.
CREATE INDEX users_auto_renew_idx ON users (id) WHERE auto_renew = TRUE;
