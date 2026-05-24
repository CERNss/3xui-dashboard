-- Webhook templates — admin can override the default envelope shape
-- and HTTP method per webhook. Use cases:
--   - n8n / Zapier wants a flat form-encoded payload, not JSON
--   - Feishu / Slack want their own card JSON shape
--   - Status pages probe via GET with no body
--   - Custom auth header injection (e.g. internal services)
--
-- Defaults: empty body_template + method=POST produces the standard
-- JSON envelope with app-managed headers. Existing rows get those
-- defaults automatically.
ALTER TABLE webhooks
    ADD COLUMN method          TEXT  NOT NULL DEFAULT 'POST',
    ADD COLUMN headers         JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN body_template   TEXT  NOT NULL DEFAULT '',
    ADD COLUMN template_format TEXT  NOT NULL DEFAULT 'json';

-- method enum is validated at the app layer (GET / POST / PUT /
-- DELETE / PATCH); the SQL constraint is intentionally absent so
-- new methods don't require a migration.
