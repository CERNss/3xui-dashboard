-- Dedicated columns for the (node, inbound) target a payment-gateway
-- order will provision onto once payment confirms.
--
-- Why a migration: #5 (alipay) stashed this in orders.error_message
-- as `target:<nodeID>:<tag>` to avoid a schema change, but that
-- overloads a column with actual error-description semantics — a
-- failed order can't carry BOTH the target and the error. This
-- migration adds proper columns + backfills any existing
-- payment_pending rows by parsing the legacy encoded form.

ALTER TABLE orders
    ADD COLUMN provisioning_node_id     BIGINT,
    ADD COLUMN provisioning_inbound_tag TEXT NOT NULL DEFAULT '';

-- Backfill existing payment_pending rows that still carry the
-- legacy `target:N:tag` encoding in error_message. Format is
-- exactly `target:<int>:<rest>` per encodeProvisioningTarget. Rows
-- not matching the pattern are left untouched.
UPDATE orders
SET
    provisioning_node_id     = NULLIF(split_part(substring(error_message FROM 8), ':', 1), '')::BIGINT,
    provisioning_inbound_tag = substring(error_message FROM (8 + position(':' IN substring(error_message FROM 8)))),
    error_message            = ''
WHERE status = 'payment_pending'
  AND error_message LIKE 'target:%:%';
