-- Payment-gateway dimension on orders.
--
-- Pre-existing orders are balance-based and stay that way; DEFAULT
-- clauses backfill the new columns for all historical rows.
-- Provider-specific columns stay empty / NULL for balance orders.
--
-- payment_target_url is the gateway-agnostic "send the user here to
-- complete payment" URL:
--   - alipay: QR-source URL (portal renders as QR)
--   - stripe: Checkout redirect URL (portal navigates via location.href)
--   - future providers: whatever the gateway returns
--
-- provisioning_node_id + provisioning_inbound_tag capture WHERE to
-- create the client once payment confirms — needed because
-- PurchaseViaPayment can't run ProvisionClient inline (the user
-- hasn't paid yet); the notify/webhook handler runs it later.
--
-- The partial index on payment_provider_order_id only covers
-- non-empty values: gives a fast lookup from alipay's trade_no /
-- stripe's session id back to our order without burning index space
-- on every balance order.

ALTER TABLE orders
    ADD COLUMN payment_method            TEXT        NOT NULL DEFAULT 'balance',
    ADD COLUMN payment_provider_order_id TEXT        NOT NULL DEFAULT '',
    ADD COLUMN payment_target_url        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN payment_expires_at        TIMESTAMPTZ,
    ADD COLUMN provisioning_node_id      BIGINT,
    ADD COLUMN provisioning_inbound_tag  TEXT        NOT NULL DEFAULT '';

CREATE INDEX orders_payment_provider_order_id
    ON orders (payment_provider_order_id)
    WHERE payment_provider_order_id <> '';
