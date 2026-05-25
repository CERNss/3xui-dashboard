BEGIN;

ALTER TABLE users
  ADD COLUMN oidc_subject TEXT;

UPDATE users u
SET oidc_subject = i.subject
FROM user_oidc_identities i
WHERE i.user_id = u.id
  AND i.provider_key = 'default';

CREATE UNIQUE INDEX users_oidc_subject_unique
  ON users (oidc_subject)
  WHERE oidc_subject IS NOT NULL;

DROP TABLE IF EXISTS user_oidc_identities;
DROP TABLE IF EXISTS oidc_providers;

ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_password_hash_not_blank_chk;

ALTER TABLE users
  ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users
  DROP COLUMN IF EXISTS display_name;

COMMIT;
