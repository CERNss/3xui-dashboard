DROP INDEX IF EXISTS orders_payment_provider_order_id;

ALTER TABLE orders
    DROP COLUMN IF EXISTS payment_expires_at,
    DROP COLUMN IF EXISTS payment_qr_url,
    DROP COLUMN IF EXISTS payment_provider_order_id,
    DROP COLUMN IF EXISTS payment_method;
