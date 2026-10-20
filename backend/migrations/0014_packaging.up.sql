-- =====================================================================
-- 0014 CARLITOS packaging management
-- packaging_items (SKUs) -> packaging_inventory (central + per-seller issued)
-- packaging_requests (seller asks) -> packaging_issues (admin issues)
-- packaging_usage (seller records consumption). Full ledger via packaging_ledger.
-- =====================================================================

CREATE TABLE packaging_items (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku          TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL CHECK (type IN ('polybag', 'courier_bag', 'box',
                                               'tape', 'sticker', 'shipping_label',
                                               'packing_slip')),
    unit         TEXT NOT NULL DEFAULT 'piece',
    description  TEXT NOT NULL DEFAULT '',
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_packaging_items_updated BEFORE UPDATE ON packaging_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Central warehouse stock for each packaging item (admin-managed).
CREATE TABLE packaging_inventory (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    packaging_item_id  UUID NOT NULL UNIQUE REFERENCES packaging_items(id) ON DELETE CASCADE,
    available_quantity INTEGER NOT NULL DEFAULT 0 CHECK (available_quantity >= 0),
    issued_quantity    INTEGER NOT NULL DEFAULT 0 CHECK (issued_quantity >= 0),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_packaging_inventory_updated BEFORE UPDATE ON packaging_inventory
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seller requests packaging.
CREATE TABLE packaging_requests (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_number     TEXT NOT NULL UNIQUE,
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    packaging_item_id  UUID NOT NULL REFERENCES packaging_items(id) ON DELETE RESTRICT,
    requested_quantity INTEGER NOT NULL CHECK (requested_quantity > 0),
    approved_quantity  INTEGER,
    status             TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'approved', 'rejected', 'fulfilled', 'cancelled')),
    note               TEXT NOT NULL DEFAULT '',
    reviewed_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at        TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_packaging_requests_seller ON packaging_requests (seller_id);
CREATE INDEX ix_packaging_requests_status ON packaging_requests (status);
CREATE TRIGGER trg_packaging_requests_updated BEFORE UPDATE ON packaging_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Admin issues packaging to a seller (fulfils a request or ad-hoc).
CREATE TABLE packaging_issues (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id         UUID REFERENCES packaging_requests(id) ON DELETE SET NULL,
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    packaging_item_id  UUID NOT NULL REFERENCES packaging_items(id) ON DELETE RESTRICT,
    issued_quantity    INTEGER NOT NULL CHECK (issued_quantity > 0),
    issued_by          UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_packaging_issues_seller ON packaging_issues (seller_id);

-- Per-seller balance of issued packaging (issued - used = remaining).
CREATE TABLE packaging_seller_balances (
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    packaging_item_id  UUID NOT NULL REFERENCES packaging_items(id) ON DELETE CASCADE,
    issued_quantity    INTEGER NOT NULL DEFAULT 0 CHECK (issued_quantity >= 0),
    used_quantity      INTEGER NOT NULL DEFAULT 0 CHECK (used_quantity >= 0),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (seller_id, packaging_item_id)
);
CREATE TRIGGER trg_packaging_balances_updated BEFORE UPDATE ON packaging_seller_balances
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seller records consumption of packaging (e.g. against a shipment).
CREATE TABLE packaging_usage (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    packaging_item_id  UUID NOT NULL REFERENCES packaging_items(id) ON DELETE RESTRICT,
    used_quantity      INTEGER NOT NULL CHECK (used_quantity > 0),
    reference_type     TEXT NOT NULL DEFAULT '',   -- shipment | order | manual
    reference_id       UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_packaging_usage_seller ON packaging_usage (seller_id);

-- Complete immutable ledger of all packaging movements.
CREATE TABLE packaging_ledger (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    packaging_item_id  UUID NOT NULL REFERENCES packaging_items(id) ON DELETE CASCADE,
    seller_id          UUID REFERENCES sellers(id) ON DELETE SET NULL,
    entry_type         TEXT NOT NULL
                         CHECK (entry_type IN ('stock_in', 'issue', 'use', 'adjustment', 'return')),
    quantity_delta     INTEGER NOT NULL,
    note               TEXT NOT NULL DEFAULT '',
    reference_type     TEXT NOT NULL DEFAULT '',
    reference_id       UUID,
    created_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_packaging_ledger_item ON packaging_ledger (packaging_item_id);
CREATE INDEX ix_packaging_ledger_seller ON packaging_ledger (seller_id);
