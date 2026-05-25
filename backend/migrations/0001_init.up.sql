-- Initial schema for the 3xui-dashboard central control panel.
--
-- This is the bootstrap migration: every table required by the
-- bootstrap-central-panel change is created here. Subsequent
-- migrations should be additive (new tables / columns) and use
-- a fresh NNNN_<topic> file pair.
--
-- Postgres 16+ is assumed (no extensions required).

BEGIN;

-- ---------------------------------------------------------------------------
-- users — admin is environment-driven; this table is for portal users only.
-- email + oidc_subject are independently nullable: a user can be
-- email/password, OIDC-only, or both (after linking). Uniqueness is enforced
-- with partial indexes so multiple NULLs are allowed.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
  id              BIGSERIAL    PRIMARY KEY,
  email           TEXT,
  password_hash   TEXT,
  oidc_subject    TEXT,
  email_verified  BOOLEAN      NOT NULL DEFAULT FALSE,
  status          TEXT         NOT NULL DEFAULT 'active',    -- active | suspended
  balance_cents   BIGINT       NOT NULL DEFAULT 0,
  sub_id          TEXT         NOT NULL,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_unique         ON users (LOWER(email))      WHERE email IS NOT NULL;
CREATE UNIQUE INDEX users_oidc_subject_unique  ON users (oidc_subject)      WHERE oidc_subject IS NOT NULL;
CREATE UNIQUE INDEX users_sub_id_unique        ON users (sub_id);

-- ---------------------------------------------------------------------------
-- nodes — every remote 3x-ui panel the dashboard talks to.
-- api_token is the Bearer token issued by the upstream panel admin; storing
-- it plaintext for now (see follow-up: encrypt-at-rest with a KEK derived
-- from a separate env secret).
-- ---------------------------------------------------------------------------
CREATE TABLE nodes (
  id            BIGSERIAL        PRIMARY KEY,
  name          TEXT             NOT NULL,
  area          TEXT             NOT NULL DEFAULT 'unknown',
  province      TEXT             NOT NULL DEFAULT 'unknown',
  host          TEXT             NOT NULL,
  port          INTEGER          NOT NULL,
  base_path     TEXT             NOT NULL DEFAULT '',
  api_token     TEXT             NOT NULL,
  enabled       BOOLEAN          NOT NULL DEFAULT TRUE,
  last_seen_at  TIMESTAMPTZ,
  cpu_pct       DOUBLE PRECISION NOT NULL DEFAULT 0,
  mem_pct       DOUBLE PRECISION NOT NULL DEFAULT 0,
  xray_version  TEXT             NOT NULL DEFAULT '',
  uptime_s      BIGINT           NOT NULL DEFAULT 0,
  status        TEXT             NOT NULL DEFAULT 'unknown', -- online | offline | unknown
  created_at    TIMESTAMPTZ      NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ      NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX nodes_name_unique ON nodes (LOWER(name));
ALTER TABLE nodes ADD CONSTRAINT nodes_area_chk CHECK (
  area IN ('jp', 'sg', 'hk', 'tw', 'us', 'gb', 'de', 'fr', 'nl', 'ca', 'au', 'kr', 'in', 'th', 'vn', 'unknown')
);
ALTER TABLE nodes ADD CONSTRAINT nodes_province_not_blank_chk CHECK (province <> '');
CREATE INDEX nodes_area_idx ON nodes (area);
CREATE INDEX nodes_area_province_idx ON nodes (area, province);

-- ---------------------------------------------------------------------------
-- client_ownerships — bridges a portal user to a 3x-ui client on a node's
-- inbound. (node_id, inbound_tag, client_email) is unique: at most one
-- ownership per panel-side client.
-- ---------------------------------------------------------------------------
CREATE TABLE client_ownerships (
  id                   BIGSERIAL    PRIMARY KEY,
  user_id              BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  node_id              BIGINT       NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  inbound_tag          TEXT         NOT NULL,
  client_email         TEXT         NOT NULL,
  plan_id              BIGINT,
  expires_at           TIMESTAMPTZ,
  traffic_limit_bytes  BIGINT,
  enabled              BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX client_ownerships_unique  ON client_ownerships (node_id, inbound_tag, client_email);
CREATE INDEX        client_ownerships_user_id ON client_ownerships (user_id);

-- ---------------------------------------------------------------------------
-- traffic_samples — cumulative byte counters captured by the periodic
-- collection job. Both inbound-level (client_email IS NULL) and
-- client-level (client_email IS NOT NULL) samples live in this table.
-- Deltas are computed at query time from successive rows.
-- ---------------------------------------------------------------------------
CREATE TABLE traffic_samples (
  id              BIGSERIAL    PRIMARY KEY,
  node_id         BIGINT       NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  inbound_tag     TEXT,
  client_email    TEXT,
  up_cum_bytes    BIGINT       NOT NULL,
  down_cum_bytes  BIGINT       NOT NULL,
  taken_at        TIMESTAMPTZ  NOT NULL
);
CREATE INDEX traffic_samples_node_time     ON traffic_samples (node_id, taken_at);
CREATE INDEX traffic_samples_client_time   ON traffic_samples (client_email, taken_at) WHERE client_email IS NOT NULL;
CREATE INDEX traffic_samples_inbound_time  ON traffic_samples (node_id, inbound_tag, taken_at) WHERE inbound_tag IS NOT NULL;

-- ---------------------------------------------------------------------------
-- plans — purchasable plan templates.
-- traffic_limit_bytes = 0 means unlimited. duration_days = 0 means non-
-- expiring (combine with traffic_limit_bytes>0 for traffic-only plans).
-- ---------------------------------------------------------------------------
CREATE TABLE plans (
  id                   BIGSERIAL    PRIMARY KEY,
  name                 TEXT         NOT NULL,
  description          TEXT         NOT NULL DEFAULT '',
  duration_days        INTEGER      NOT NULL,
  traffic_limit_bytes  BIGINT       NOT NULL,
  price_cents          BIGINT       NOT NULL,
  enabled              BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- orders — one row per purchase attempt. idempotency_key is enforced unique
-- so a retried request returns the original order.
-- ---------------------------------------------------------------------------
CREATE TABLE orders (
  id                   BIGSERIAL    PRIMARY KEY,
  user_id              BIGINT       NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  plan_id              BIGINT       NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
  idempotency_key      TEXT         NOT NULL,
  price_cents          BIGINT       NOT NULL,
  status               TEXT         NOT NULL DEFAULT 'pending', -- pending | completed | failed | refunded
  client_ownership_id  BIGINT       REFERENCES client_ownerships(id) ON DELETE SET NULL,
  error_message        TEXT         NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  completed_at         TIMESTAMPTZ
);
CREATE UNIQUE INDEX orders_idempotency_unique ON orders (idempotency_key);
CREATE INDEX        orders_user_created       ON orders (user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- balance_logs — audit trail for every change to users.balance_cents.
-- ---------------------------------------------------------------------------
CREATE TABLE balance_logs (
  id                    BIGSERIAL    PRIMARY KEY,
  user_id               BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  delta_cents           BIGINT       NOT NULL,
  balance_after_cents   BIGINT       NOT NULL,
  reason                TEXT         NOT NULL, -- admin_adjust | order_charge | order_refund | bonus
  order_id              BIGINT       REFERENCES orders(id) ON DELETE SET NULL,
  note                  TEXT         NOT NULL DEFAULT '',
  created_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX balance_logs_user_created ON balance_logs (user_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- webhooks — outbound event-subscription targets configured by admins.
-- events is a JSONB array of event-name patterns (e.g. ["node.*",
-- "order.completed"]).
-- ---------------------------------------------------------------------------
CREATE TABLE webhooks (
  id            BIGSERIAL    PRIMARY KEY,
  name          TEXT         NOT NULL,
  url           TEXT         NOT NULL,
  secret        TEXT         NOT NULL,
  events        JSONB        NOT NULL DEFAULT '[]'::jsonb,
  enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
  allow_private BOOLEAN      NOT NULL DEFAULT FALSE,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- webhook_deliveries — one row per delivery attempt. payload is the full
-- versioned envelope as sent. response_body is truncated by the app layer
-- before insert.
-- ---------------------------------------------------------------------------
CREATE TABLE webhook_deliveries (
  id            BIGSERIAL    PRIMARY KEY,
  webhook_id    BIGINT       NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  event_type    TEXT         NOT NULL,
  payload       JSONB        NOT NULL,
  status        TEXT         NOT NULL DEFAULT 'pending', -- pending | success | failed
  http_status   INTEGER      NOT NULL DEFAULT 0,
  response_body TEXT         NOT NULL DEFAULT '',
  attempt       INTEGER      NOT NULL DEFAULT 0,
  scheduled_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
  delivered_at  TIMESTAMPTZ,
  error         TEXT         NOT NULL DEFAULT ''
);
CREATE INDEX webhook_deliveries_status_scheduled ON webhook_deliveries (status, scheduled_at);
CREATE INDEX webhook_deliveries_webhook_created  ON webhook_deliveries (webhook_id, scheduled_at DESC);

-- ---------------------------------------------------------------------------
-- settings — runtime-mutable key/value store for admin-controlled toggles
-- (public_registration_enabled, email_domain_allowlist, subscription remark
-- template, traffic thresholds, …).
-- ---------------------------------------------------------------------------
CREATE TABLE settings (
  key         TEXT         PRIMARY KEY,
  value       TEXT         NOT NULL,
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMIT;
