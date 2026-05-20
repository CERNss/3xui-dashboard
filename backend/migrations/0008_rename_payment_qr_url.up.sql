-- Rename payment_qr_url → payment_target_url to reflect what it
-- actually holds:
--
--   - alipay: QR-source URL (qr.alipay.com/...)
--   - stripe: redirect-target URL (checkout.stripe.com/...)
--   - future providers (wechatpay, paypal): whatever the gateway
--     returns as the "send the user here to complete payment" URL
--
-- The column was added in 0006 as `payment_qr_url` because alipay
-- was the only gateway; #6 added Stripe and reused the column with
-- a different semantic meaning. Renaming it now is one migration;
-- waiting longer means more frontend + backend code to update.

ALTER TABLE orders
    RENAME COLUMN payment_qr_url TO payment_target_url;
