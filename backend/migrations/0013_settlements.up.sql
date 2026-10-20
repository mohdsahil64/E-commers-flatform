-- =====================================================================
-- 0013 Commission config + seller settlements (immutable financial records)
-- =====================================================================

-- Commission configuration. A row may be global (category_id/seller_id NULL)
-- or scoped to a category and/or seller. Most specific match wins.
CREATE TABLE seller_commissions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id         UUID REFERENCES sellers(id) ON DELETE CASCADE,
    category_id       UUID REFERENCES categories(id) ON DELETE CASCADE,
    commission_percent NUMERIC(5,2) NOT NULL,
    effective_from    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_seller_commissions_seller ON seller_commissions (seller_id);
CREATE INDEX ix_seller_commissions_category ON seller_commissions (category_id);

-- A settlement is an immutable payout record to a seller for a set of items.
CREATE TABLE seller_settlements (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_number  TEXT NOT NULL UNIQUE,
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    period_start       DATE NOT NULL,
    period_end         DATE NOT NULL,
    gross_amount       NUMERIC(14,2) NOT NULL DEFAULT 0,
    commission_amount  NUMERIC(14,2) NOT NULL DEFAULT 0,
    shipping_deduction NUMERIC(14,2) NOT NULL DEFAULT 0,
    other_fees         NUMERIC(14,2) NOT NULL DEFAULT 0,
    refund_amount      NUMERIC(14,2) NOT NULL DEFAULT 0,
    net_payable        NUMERIC(14,2) NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'processing', 'settled', 'failed', 'reversed')),
    bank_account_id    UUID REFERENCES seller_bank_accounts(id) ON DELETE SET NULL,
    utr_reference      TEXT NOT NULL DEFAULT '',
    statement_key      TEXT,   -- downloadable statement in object storage
    processed_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    processed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
    -- Intentionally no updated_at/trigger: settlements are immutable; a status
    -- change to reversed creates a linked reversal, it does not mutate history.
);
CREATE INDEX ix_settlements_seller ON seller_settlements (seller_id);
CREATE INDEX ix_settlements_status ON seller_settlements (status);

-- Line items that make up a settlement (one per settled order item).
CREATE TABLE settlement_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_id  UUID NOT NULL REFERENCES seller_settlements(id) ON DELETE CASCADE,
    order_id       UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    order_item_id  UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    gross_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    commission     NUMERIC(12,2) NOT NULL DEFAULT 0,
    shipping       NUMERIC(12,2) NOT NULL DEFAULT 0,
    payment_fee    NUMERIC(12,2) NOT NULL DEFAULT 0,
    refund         NUMERIC(12,2) NOT NULL DEFAULT 0,
    net_payable    NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_item_id)   -- an item settles at most once
);
CREATE INDEX ix_settlement_items_settlement ON settlement_items (settlement_id);
