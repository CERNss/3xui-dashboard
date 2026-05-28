-- Reverse of 0001_init.up.sql. Drops in reverse dependency order so
-- foreign-key references do not block. Wrapped in a transaction so a
-- partial failure leaves the database untouched.

BEGIN;

DROP TABLE IF EXISTS user_oidc_identities;
DROP TABLE IF EXISTS oidc_providers;
DROP TABLE IF EXISTS admin_actions;
DROP TABLE IF EXISTS wg_peers;
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS email_verification_codes;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS balance_logs;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS provisioning_pool_targets;
DROP TABLE IF EXISTS provisioning_pools;
DROP TABLE IF EXISTS inbound_templates;
DROP TABLE IF EXISTS traffic_samples;
DROP TABLE IF EXISTS client_ownerships;
DROP TABLE IF EXISTS nodes;
DROP TABLE IF EXISTS users;

COMMIT;
