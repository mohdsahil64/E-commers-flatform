-- =====================================================================
-- 0009 Payments, payment_transactions, refunds
-- Provider-agnostic: the provider column records which gateway handled it.
-- =====================================================================

CREATE TABLE payments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL DEFAULT 'mock',   -- mock | razorpay | stripe
    method              TEXT NOT NULL CHECK (method IN ('online', 'cod')),
    amount              NUMERIC(12,2) NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'INR',
    status              TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'authorized', 'paid',
                                            'failed', 'refunded', 'partially_refunded')),
    provider_order_id   TEXT,   -- gateway order id (e.g. razorpay order_id)
    provider_payment_id TEXT,   -- gateway payment id
    provider_signature  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_payments_order ON payments (order_id);
CREATE INDEX ix_payments_status ON payments (status);
CREATE TRIGGER trg_payments_updated BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Immutable log of provider events / webhooks / transitions.
CREATE TABLE payment_transactions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id     UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    event_type     TEXT NOT NULL,   -- created | authorized | captured | failed | refund | webhook
    amount         NUMERIC(12,2) NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT '',
    provider_ref   TEXT NOT NULL DEFAULT '',
    raw_payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_payment_tx_payment ON payment_transactions (payment_id);

CREATE TABLE refunds (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    order_item_id  UUID REFERENCES order_items(id) ON DELETE SET NULL,
    payment_id     UUID REFERENCES payments(id) ON DELETE SET NULL,
    amount         NUMERIC(12,2) NOT NULL,
    reason         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'rejected')),
    provider_ref   TEXT NOT NULL DEFAULT '',
    initiated_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    processed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_refunds_order ON refunds (order_id);
CREATE INDEX ix_refunds_status ON refunds (status);
CREATE TRIGGER trg_refunds_updated BEFORE UPDATE ON refunds
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
