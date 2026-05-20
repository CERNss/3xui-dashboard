ALTER TABLE webhooks
    DROP COLUMN IF EXISTS method,
    DROP COLUMN IF EXISTS headers,
    DROP COLUMN IF EXISTS body_template,
    DROP COLUMN IF EXISTS template_format;
