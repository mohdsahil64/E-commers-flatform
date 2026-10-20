-- =====================================================================
-- 0023 Coupon redemptions — per-user usage tracking for coupons.
-- The coupons table (0012) already holds limits (usage_limit_total,
-- usage_limit_per_user, used_count, min_order_amount, max_discount, window).
-- This records each successful redemption so per-user limits can be enforced
-- and an admin can view redemption history.
-- =====================================================================

CREATE TABLE coupon_redemptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coupon_id     UUID NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    coupon_code   TEXT NOT NULL,
    customer_id   UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    order_id      UUID REFERENCES orders(id) ON DELETE SET NULL,
    discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_coupon_redemptions_coupon   ON coupon_redemptions (coupon_id);
CREATE INDEX ix_coupon_redemptions_customer ON coupon_redemptions (customer_id);
CREATE INDEX ix_coupon_redemptions_order    ON coupon_redemptions (order_id);
