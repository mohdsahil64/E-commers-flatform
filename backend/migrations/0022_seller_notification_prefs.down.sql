-- Revert 0022 seller notification preferences.

ALTER TABLE sellers
    DROP COLUMN IF EXISTS notify_order_emails,
    DROP COLUMN IF EXISTS notify_return_emails,
    DROP COLUMN IF EXISTS notify_settlement_emails,
    DROP COLUMN IF EXISTS notify_low_stock,
    DROP COLUMN IF EXISTS notify_marketing;
