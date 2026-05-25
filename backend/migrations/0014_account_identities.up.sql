-- P5 account identity model:
-- - local login stays on users.email
-- - password_hash is always non-null
-- - display_name is profile metadata, not a login identifier
-- - OIDC bindings move out of users.oidc_subject into provider-scoped identities

BEGIN;

DROP INDEX IF EXISTS users_oidc_subject_unique;

ALTER TABLE users
  ADD COLUMN display_name TEXT NOT NULL DEFAULT '';

UPDATE users
SET password_hash = '!disabled-local-password'
WHERE password_hash IS NULL;

ALTER TABLE users
  ALTER COLUMN password_hash SET DEFAULT '!disabled-local-password',
  ALTER COLUMN password_hash SET NOT NULL;

ALTER TABLE users
  ADD CONSTRAINT users_password_hash_not_blank_chk CHECK (password_hash <> '');

CREATE TABLE oidc_providers (
  provider_key  TEXT         PRIMARY KEY,
  display_name  TEXT         NOT NULL,
  icon_url      TEXT         NOT NULL DEFAULT '',
  issuer        TEXT         NOT NULL,
  client_id     TEXT         NOT NULL,
  client_secret TEXT         NOT NULL DEFAULT '',
  redirect_url  TEXT         NOT NULL DEFAULT '',
  scopes        JSONB        NOT NULL DEFAULT '[]'::jsonb,
  auth_url      TEXT         NOT NULL DEFAULT '',
  token_url     TEXT         NOT NULL DEFAULT '',
  jwks_url      TEXT         NOT NULL DEFAULT '',
  user_info_url TEXT         NOT NULL DEFAULT '',
  enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE user_oidc_identities (
  id                      BIGSERIAL    PRIMARY KEY,
  user_id                 BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider_key            TEXT         NOT NULL REFERENCES oidc_providers(provider_key) ON DELETE CASCADE,
  subject                 TEXT         NOT NULL,
  provider_email          TEXT         NOT NULL,
  provider_email_verified BOOLEAN      NOT NULL DEFAULT FALSE,
  created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX user_oidc_identities_provider_subject_unique
  ON user_oidc_identities (provider_key, subject);
CREATE UNIQUE INDEX user_oidc_identities_user_provider_unique
  ON user_oidc_identities (user_id, provider_key);
CREATE INDEX user_oidc_identities_user_id_idx
  ON user_oidc_identities (user_id);

-- The service is pre-launch and has no compatibility requirements, but
-- preserving old dev rows makes local testing less surprising.
INSERT INTO oidc_providers (
  provider_key, display_name, issuer, client_id, client_secret, redirect_url,
  scopes, auth_url, token_url, jwks_url, user_info_url, enabled
)
SELECT
  'default', 'OIDC', 'legacy-provider', 'legacy-client', '', '',
  '[]'::jsonb, '', '', '', '', TRUE
WHERE EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_name = 'users' AND column_name = 'oidc_subject'
);

INSERT INTO user_oidc_identities (
  user_id, provider_key, subject, provider_email, provider_email_verified
)
SELECT
  id,
  'default',
  oidc_subject,
  COALESCE(email, ''),
  email_verified
FROM users
WHERE oidc_subject IS NOT NULL AND oidc_subject <> '';

ALTER TABLE users
  DROP COLUMN IF EXISTS oidc_subject;

COMMIT;
