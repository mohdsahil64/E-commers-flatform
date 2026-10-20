-- =====================================================================
-- 0024 Seller Cash-on-Delivery (COD) setting
-- Lets a seller choose whether Cash on Delivery is offered on their
-- products. When OFF, COD is disabled at checkout for every item from
-- that seller. Defaults to ON so existing sellers keep COD available.
-- =====================================================================

ALTER TABLE sellers
    ADD COLUMN IF NOT EXISTS cod_enabled BOOLEAN NOT NULL DEFAULT TRUE;
