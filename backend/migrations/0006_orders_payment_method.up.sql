-- Payment-method dimension on orders.
--
-- Pre-existing orders are balance-based and stay that way; the
-- DEFAULT clause backfills the new column for all historical rows.
-- Provider-specific columns (qr_url, provider order id, expires_at)
-- stay empty / NULL for balance orders — those rows never use them.
--
-- The partial index on payment_provider_order_id only covers
-- non-empty values: it gives us a fast lookup from alipay's
-- trade_no back to our order without burning index space on every
-- balance order.

ALTER TABLE orders
    ADD COLUMN payment_method            TEXT        NOT NULL DEFAULT 'balance',
    ADD COLUMN payment_provider_order_id TEXT        NOT NULL DEFAULT '',
    ADD COLUMN payment_qr_url            TEXT        NOT NULL DEFAULT '',
    ADD COLUMN payment_expires_at        TIMESTAMPTZ;

CREATE INDEX orders_payment_provider_order_id
    ON orders (payment_provider_order_id)
    WHERE payment_provider_order_id <> '';
