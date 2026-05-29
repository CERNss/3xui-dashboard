-- Baseline schema for the 3xui-dashboard central control panel.
--
-- The project has not been launched yet, so the pre-launch incremental
-- migrations are collapsed into this single bootstrap schema. Future
-- migrations can start at 0002 once there is a deployed database that
-- must be upgraded in place.
--
-- Postgres 16+ is assumed (no extensions required).

BEGIN;

-- ---------------------------------------------------------------------------
-- users — admin is environment-driven; this table is for portal users only.
-- Email is the local login identifier. OIDC provider identities live in
-- user_oidc_identities; password_hash is always present so every account
-- can satisfy the local credential invariant after account completion.
-- ---------------------------------------------------------------------------
CREATE TABLE users (
  id              BIGSERIAL    PRIMARY KEY,
  email           TEXT,
  password_hash   TEXT         NOT NULL DEFAULT '!disabled-local-password',
  display_name    TEXT         NOT NULL DEFAULT '',
  email_verified  BOOLEAN      NOT NULL DEFAULT FALSE,
  status          TEXT         NOT NULL DEFAULT 'active',    -- active | suspended
  balance_cents   BIGINT       NOT NULL DEFAULT 0,
  auto_renew      BOOLEAN      NOT NULL DEFAULT FALSE,
  sub_id          TEXT         NOT NULL,
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT users_password_hash_not_blank_chk CHECK (password_hash <> '')
);
CREATE UNIQUE INDEX users_email_unique         ON users (LOWER(email))      WHERE email IS NOT NULL;
CREATE UNIQUE INDEX users_sub_id_unique        ON users (sub_id);
CREATE INDEX users_auto_renew_idx ON users (id) WHERE auto_renew = TRUE;

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
  scheme        TEXT             NOT NULL DEFAULT 'https',
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
ALTER TABLE nodes ADD CONSTRAINT nodes_scheme_chk CHECK (scheme IN ('http', 'https'));
CREATE INDEX nodes_area_idx ON nodes (area);
CREATE INDEX nodes_area_province_idx ON nodes (area, province);

-- ---------------------------------------------------------------------------
-- oidc_providers + user_oidc_identities — provider-scoped OIDC account links.
-- Local login stays on users.email while each provider identity is keyed by
-- immutable (provider_key, subject).
-- ---------------------------------------------------------------------------
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
  protocol             TEXT,
  plan_id              BIGINT,
  order_id             BIGINT,
  expires_at           TIMESTAMPTZ,
  traffic_limit_bytes  BIGINT,
  enabled              BOOLEAN      NOT NULL DEFAULT TRUE,
  disabled_by_quota    BOOLEAN      NOT NULL DEFAULT FALSE,
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX client_ownerships_unique  ON client_ownerships (node_id, inbound_tag, client_email);
CREATE INDEX        client_ownerships_user_id ON client_ownerships (user_id);
CREATE INDEX        client_ownerships_order_id_idx ON client_ownerships (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX        client_ownerships_protocol_idx ON client_ownerships (protocol) WHERE protocol IS NOT NULL;
CREATE INDEX        client_ownerships_quota_group_idx
  ON client_ownerships (user_id, plan_id)
  WHERE plan_id IS NOT NULL;

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
-- inbound_templates — reusable inbound wire-shapes used by provisioning pools
-- to create real upstream inbounds on demand.
-- ---------------------------------------------------------------------------
CREATE TABLE inbound_templates (
  id              BIGSERIAL    PRIMARY KEY,
  name            TEXT         NOT NULL,
  description     TEXT         NOT NULL DEFAULT '',
  enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
  protocol        TEXT         NOT NULL,
  remark          TEXT         NOT NULL DEFAULT '',
  listen          TEXT         NOT NULL DEFAULT '',
  total           BIGINT       NOT NULL DEFAULT 0,
  expiry_time     BIGINT       NOT NULL DEFAULT 0,
  traffic_reset   TEXT         NOT NULL DEFAULT 'never',
  settings        TEXT         NOT NULL DEFAULT '{}',
  stream_settings TEXT         NOT NULL DEFAULT '{}',
  sniffing        TEXT         NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT inbound_templates_name_unique UNIQUE (name),
  CONSTRAINT inbound_templates_protocol_not_blank_chk CHECK (protocol <> '')
);

-- ---------------------------------------------------------------------------
-- provisioning_pools — curated lists of real inbounds that a plan can
-- assign new clients into when the user purchases. Inbounds themselves
-- are created manually by the operator (templates speed that up via
-- the dashboard's create-inbound form); this pool just decides which
-- existing inbound a purchase lands its client in.
-- ---------------------------------------------------------------------------
CREATE TABLE provisioning_pools (
  id                BIGSERIAL    PRIMARY KEY,
  name              TEXT         NOT NULL,
  description       TEXT         NOT NULL DEFAULT '',
  enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
  allowed_protocols JSONB        NOT NULL DEFAULT '[]'::jsonb,
  created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT provisioning_pools_name_unique UNIQUE (name)
);

CREATE TABLE provisioning_pool_targets (
  id             BIGSERIAL    PRIMARY KEY,
  pool_id        BIGINT       NOT NULL REFERENCES provisioning_pools(id) ON DELETE CASCADE,
  node_id        BIGINT       NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  inbound_tag    TEXT         NOT NULL,
  protocol       TEXT         NOT NULL DEFAULT '',
  max_clients    INTEGER      NOT NULL DEFAULT 0,
  priority       INTEGER      NOT NULL DEFAULT 100,
  enabled        BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT provisioning_pool_targets_max_clients_chk CHECK (max_clients >= 0),
  CONSTRAINT provisioning_pool_targets_priority_chk CHECK (priority >= 0)
);
CREATE UNIQUE INDEX provisioning_pool_targets_unique
  ON provisioning_pool_targets (pool_id, node_id, inbound_tag);
CREATE INDEX provisioning_pool_targets_pool
  ON provisioning_pool_targets (pool_id, enabled, priority, id);

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
  ip_limit             INTEGER      NOT NULL DEFAULT 0,
  provisioning_pool_id BIGINT       REFERENCES provisioning_pools(id) ON DELETE SET NULL,
  enabled              BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  CONSTRAINT plans_ip_limit_chk CHECK (ip_limit >= 0)
);
CREATE INDEX plans_provisioning_pool_id
  ON plans (provisioning_pool_id)
  WHERE provisioning_pool_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- orders — one row per purchase attempt. Balance and external gateway
-- purchases share this table; payment_* columns are empty/defaulted for
-- balance orders.
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
  payment_method            TEXT        NOT NULL DEFAULT 'balance',
  payment_provider_order_id TEXT        NOT NULL DEFAULT '',
  payment_target_url        TEXT        NOT NULL DEFAULT '',
  payment_expires_at        TIMESTAMPTZ,
  provisioning_node_id      BIGINT,
  provisioning_inbound_tag  TEXT        NOT NULL DEFAULT '',
  created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  completed_at         TIMESTAMPTZ
);
CREATE UNIQUE INDEX orders_idempotency_unique ON orders (idempotency_key);
CREATE INDEX        orders_user_created       ON orders (user_id, created_at DESC);
CREATE INDEX        orders_payment_provider_order_id
  ON orders (payment_provider_order_id)
  WHERE payment_provider_order_id <> '';

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
  method        TEXT         NOT NULL DEFAULT 'POST',
  headers       JSONB        NOT NULL DEFAULT '{}'::jsonb,
  body_template TEXT         NOT NULL DEFAULT '',
  template_format TEXT       NOT NULL DEFAULT 'json',
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
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at  TIMESTAMPTZ,
  error         TEXT         NOT NULL DEFAULT ''
);
CREATE INDEX webhook_deliveries_status_scheduled ON webhook_deliveries (status, scheduled_at);
CREATE INDEX webhook_deliveries_webhook_created  ON webhook_deliveries (webhook_id, scheduled_at DESC);
CREATE INDEX webhook_deliveries_due ON webhook_deliveries (status, next_attempt_at) WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- email_verification_codes — short-lived single-use codes for email ownership.
-- Purpose scopes prevent a register code from being replayed elsewhere.
-- ---------------------------------------------------------------------------
CREATE TABLE email_verification_codes (
  id              BIGSERIAL PRIMARY KEY,
  email           TEXT        NOT NULL,
  purpose         TEXT        NOT NULL,
  code_hash       TEXT        NOT NULL,
  expires_at      TIMESTAMPTZ NOT NULL,
  sent_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  consumed_at     TIMESTAMPTZ,
  attempts        INT         NOT NULL DEFAULT 0
);
CREATE INDEX email_verification_codes_active
  ON email_verification_codes (email, purpose, sent_at DESC);
CREATE INDEX email_verification_codes_unconsumed
  ON email_verification_codes (email, purpose, expires_at)
  WHERE consumed_at IS NULL;

-- ---------------------------------------------------------------------------
-- notification_log — persistent dedup for outbound user/ops notifications.
-- surface separates user-facing messages from ops notifications.
-- ---------------------------------------------------------------------------
CREATE TABLE notification_log (
  id             BIGSERIAL PRIMARY KEY,
  surface        TEXT        NOT NULL DEFAULT 'notification'
      CHECK (surface IN ('message', 'notification')),
  kind           TEXT        NOT NULL,
  ownership_id   BIGINT      NOT NULL REFERENCES client_ownerships(id) ON DELETE CASCADE,
  user_email     TEXT        NOT NULL DEFAULT '',
  sent_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX notification_log_dedup
  ON notification_log (surface, kind, ownership_id);

-- ---------------------------------------------------------------------------
-- wg_peers — dashboard-side WireGuard peer mirror. The private key is stored
-- AES-256-GCM-encrypted with WG_MASTER_KEY.
-- ---------------------------------------------------------------------------
CREATE TABLE wg_peers (
  id                    BIGSERIAL    PRIMARY KEY,
  client_ownership_id   BIGINT       NOT NULL UNIQUE REFERENCES client_ownerships(id) ON DELETE CASCADE,
  inbound_id            BIGINT       NOT NULL,
  public_key            TEXT         NOT NULL,
  private_key_encrypted BYTEA        NOT NULL,
  allocated_ip          INET         NOT NULL,
  created_at            TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX wg_peers_inbound_ip ON wg_peers (inbound_id, allocated_ip);

-- ---------------------------------------------------------------------------
-- admin_actions — audit trail of mutating admin requests. Request bodies are
-- intentionally excluded to avoid logging secrets.
-- ---------------------------------------------------------------------------
CREATE TABLE admin_actions (
  id              BIGSERIAL    PRIMARY KEY,
  admin_username  TEXT         NOT NULL,
  method          TEXT         NOT NULL,
  path            TEXT         NOT NULL,
  target_resource TEXT         NOT NULL DEFAULT '',
  target_id       TEXT         NOT NULL DEFAULT '',
  query_string    TEXT         NOT NULL DEFAULT '',
  ip              TEXT         NOT NULL DEFAULT '',
  user_agent      TEXT         NOT NULL DEFAULT '',
  status_code     INT          NOT NULL,
  error_msg       TEXT         NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX admin_actions_admin_username ON admin_actions (admin_username, created_at DESC);
CREATE INDEX admin_actions_created_at     ON admin_actions (created_at DESC);
CREATE INDEX admin_actions_target         ON admin_actions (target_resource, target_id, created_at DESC) WHERE target_resource <> '';

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
