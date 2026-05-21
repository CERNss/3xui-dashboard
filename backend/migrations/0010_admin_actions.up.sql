-- admin_actions — audit trail of mutating admin requests. One row
-- per POST/PUT/DELETE/PATCH that lands on /api/admin/* after the
-- RequireAdmin middleware has resolved the JWT (i.e. successful
-- auth — failed-auth attempts are out of scope; the login rate
-- limiter handles that surface separately).
--
-- target_resource + target_id are best-effort URL-derived (e.g.
-- POST /api/admin/orders/42/refund → resource="orders", id="42");
-- left as TEXT so resources with non-numeric ids (slugs, uuids)
-- fit without a future schema migration.
--
-- We INTENTIONALLY do NOT capture request bodies — many admin
-- endpoints accept secrets (API tokens, SMTP passwords, OIDC
-- client secrets) where logging the body would defeat the
-- purpose. Path + query + result is enough to reconstruct "who
-- did what when" without a secret-leak risk.
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
