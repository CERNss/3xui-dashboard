BEGIN;

CREATE TABLE IF NOT EXISTS inbound_templates (
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

ALTER TABLE provisioning_pools
  ADD COLUMN IF NOT EXISTS template_id BIGINT REFERENCES inbound_templates(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS max_clients INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS node_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'provisioning_pools_max_clients_chk'
  ) THEN
    ALTER TABLE provisioning_pools
      ADD CONSTRAINT provisioning_pools_max_clients_chk CHECK (max_clients >= 0);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS provisioning_pools_template_id
  ON provisioning_pools (template_id)
  WHERE template_id IS NOT NULL;

ALTER TABLE provisioning_pool_targets
  ADD COLUMN IF NOT EXISTS template_id BIGINT REFERENCES inbound_templates(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS generated BOOLEAN NOT NULL DEFAULT FALSE;

COMMIT;
