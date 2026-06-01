-- Subscription rule profiles + reusable rulesets for the /sub converter.
--
-- A profile is the target-agnostic routing policy (proxy-groups, rules,
-- transforms, per-target base overrides) applied when rendering a
-- user's aggregated nodes into Clash / Surge / Quantumult X / Loon /
-- sing-box. Rulesets are reusable rule lists (remote URL or inline)
-- referenced by a profile's rules.
--
-- The policy sub-documents (proxy_groups / rules / transforms /
-- base_overrides) are stored as JSONB: they're always read and edited
-- as a whole, never queried by inner field, so a column-per-field
-- schema would buy nothing.

CREATE TABLE subscription_rulesets (
  id              BIGSERIAL    PRIMARY KEY,
  key             TEXT         NOT NULL,
  name            TEXT         NOT NULL DEFAULT '',
  source_type     TEXT         NOT NULL DEFAULT 'remote',   -- 'remote' | 'inline'
  url             TEXT         NOT NULL DEFAULT '',
  content         TEXT         NOT NULL DEFAULT '',          -- inline rule list (one matcher per line)
  behavior        TEXT         NOT NULL DEFAULT 'classical', -- 'domain' | 'ipcidr' | 'classical'
  format          TEXT         NOT NULL DEFAULT 'yaml',      -- source/provider format hint: 'yaml' | 'text'
  ttl_seconds     INTEGER      NOT NULL DEFAULT 86400,
  enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
  last_fetched_at TIMESTAMPTZ,
  last_status     TEXT         NOT NULL DEFAULT '',          -- last refresh result / error
  cached_content  TEXT         NOT NULL DEFAULT '',          -- server-side cache (for preview / expand / non-fetching targets)
  cached_etag     TEXT         NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT subscription_rulesets_key_unique UNIQUE (key)
);

CREATE TABLE subscription_profiles (
  id              BIGSERIAL    PRIMARY KEY,
  key             TEXT         NOT NULL,
  name            TEXT         NOT NULL DEFAULT '',
  description     TEXT         NOT NULL DEFAULT '',
  is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
  enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
  proxy_groups    JSONB        NOT NULL DEFAULT '[]'::jsonb,
  rules           JSONB        NOT NULL DEFAULT '[]'::jsonb,
  transforms      JSONB        NOT NULL DEFAULT '{}'::jsonb,
  base_overrides  JSONB        NOT NULL DEFAULT '{}'::jsonb,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT subscription_profiles_key_unique UNIQUE (key)
);

-- At most one profile may be the global default.
CREATE UNIQUE INDEX subscription_profiles_one_default
  ON subscription_profiles (is_default) WHERE is_default;
