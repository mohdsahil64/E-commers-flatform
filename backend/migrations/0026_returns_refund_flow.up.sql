-- ---------------------------------------------------------------------
-- Returns / exchange / refund flow enrichment.
--
-- Adds: reject reason, customer refund destination (UPI / bank) captured at
-- request time, a return status-history table (who did what, when, and why),
-- customer-uploaded photo proof, a delivered timestamp on order items (for the
-- 7-day return window), and a UTR field on refunds (admin pays out-of-band and
-- records the bank UTR which is then visible to seller + customer).
-- ---------------------------------------------------------------------

-- When an item was delivered — drives the return/exchange eligibility window.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;

-- Refund destination + rejection reason on the return itself.
ALTER TABLE returns ADD COLUMN IF NOT EXISTS reason_code     TEXT NOT NULL DEFAULT '';
ALTER TABLE returns ADD COLUMN IF NOT EXISTS reject_reason   TEXT NOT NULL DEFAULT '';
ALTER TABLE returns ADD COLUMN IF NOT EXISTS refund_method   TEXT NOT NULL DEFAULT ''
    CHECK (refund_method IN ('', 'upi', 'bank'));
ALTER TABLE returns ADD COLUMN IF NOT EXISTS refund_upi      TEXT NOT NULL DEFAULT '';
ALTER TABLE returns ADD COLUMN IF NOT EXISTS refund_acc_name TEXT NOT NULL DEFAULT '';
ALTER TABLE returns ADD COLUMN IF NOT EXISTS refund_acc_no   TEXT NOT NULL DEFAULT '';
ALTER TABLE returns ADD COLUMN IF NOT EXISTS refund_ifsc     TEXT NOT NULL DEFAULT '';

-- Photo proof uploaded by the customer with a return request.
CREATE TABLE IF NOT EXISTS return_photos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id  UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    file_key   TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_return_photos_return ON return_photos (return_id);

-- Status history timeline: every transition with the actor, their role and the
-- (optional) reason, so all three portals can show exactly what happened when.
CREATE TABLE IF NOT EXISTS return_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id   UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,           -- requested | approved | rejected | received | refund_processing | refunded
    actor_role  TEXT NOT NULL DEFAULT '', -- customer | seller | admin | system
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    note        TEXT NOT NULL DEFAULT '', -- reason / UTR / free text
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_return_events_return ON return_events (return_id, created_at);

-- Bank UTR recorded by the admin when the refund is actually paid out.
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS utr        TEXT NOT NULL DEFAULT '';
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS return_id  UUID REFERENCES returns(id) ON DELETE SET NULL;
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS method     TEXT NOT NULL DEFAULT '';
