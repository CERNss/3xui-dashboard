ALTER TABLE orders
    DROP COLUMN IF EXISTS provisioning_inbound_tag,
    DROP COLUMN IF EXISTS provisioning_node_id;
