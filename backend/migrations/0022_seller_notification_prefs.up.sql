-- =====================================================================
-- 0022 Seller notification preferences
-- Lets a seller control which lifecycle emails they receive. Defaults keep
-- the important ones on. Marketing is opt-in (off by default).
-- =====================================================================

ALTER TABLE sellers
    ADD COLUMN IF NOT EXISTS notify_order_emails      BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_return_emails     BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_settlement_emails BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_low_stock         BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS notify_marketing         BOOLEAN NOT NULL DEFAULT FALSE;
