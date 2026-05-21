DROP INDEX IF EXISTS users_auto_renew_idx;
ALTER TABLE users DROP COLUMN IF EXISTS auto_renew;
