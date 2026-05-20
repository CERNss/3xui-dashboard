DROP INDEX IF EXISTS orders_payment_provider_order_id;

ALTER TABLE orders
    DROP COLUMN IF EXISTS provisioning_inbound_tag,
    DROP COLUMN IF EXISTS provisioning_node_id,
    DROP COLUMN IF EXISTS payment_expires_at,
    DROP COLUMN IF EXISTS payment_target_url,
    DROP COLUMN IF EXISTS payment_provider_order_id,
    DROP COLUMN IF EXISTS payment_method;
