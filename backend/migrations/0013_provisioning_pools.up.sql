-- Provisioning pools let admins define the bounded set of inbounds a
-- plan may auto-provision onto. The portal no longer needs to ask a
-- buyer to choose a node/inbound; billing resolves one from the plan's
-- pool at purchase/confirmation time.
--
-- auto_create_* fields are deliberately inert in the first
-- implementation. They document the future boundary for creating new
-- inbounds only when a pool is exhausted.

BEGIN;

CREATE TABLE provisioning_pools (
    id                BIGSERIAL    PRIMARY KEY,
    name              TEXT         NOT NULL,
    description       TEXT         NOT NULL DEFAULT '',
    enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
    auto_create       BOOLEAN      NOT NULL DEFAULT FALSE,
    port_min          INTEGER,
    port_max          INTEGER,
    allowed_protocols JSONB        NOT NULL DEFAULT '[]'::jsonb,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT provisioning_pools_name_unique UNIQUE (name),
    CONSTRAINT provisioning_pools_port_range_chk CHECK (
        (port_min IS NULL AND port_max IS NULL)
        OR (port_min BETWEEN 1 AND 65535 AND port_max BETWEEN 1 AND 65535 AND port_min <= port_max)
    )
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

ALTER TABLE plans
    ADD COLUMN ip_limit INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN provisioning_pool_id BIGINT REFERENCES provisioning_pools(id) ON DELETE SET NULL;

ALTER TABLE plans
    ADD CONSTRAINT plans_ip_limit_chk CHECK (ip_limit >= 0);

CREATE INDEX plans_provisioning_pool_id
    ON plans (provisioning_pool_id)
    WHERE provisioning_pool_id IS NOT NULL;

COMMIT;
