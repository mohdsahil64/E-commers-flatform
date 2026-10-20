-- =====================================================================
-- 0006 Inventory + inventory_transactions (per seller_listing)
--
-- available_quantity  : sellable, unreserved stock
-- reserved_quantity   : held for in-flight orders (reserved at checkout)
-- damaged_quantity    : written off / unsellable
-- returned_quantity    : units back from returns awaiting inspection/restock
-- Overselling is prevented by atomic conditional updates + row locking in the
-- service layer (see inventory repository).
-- =====================================================================

CREATE TABLE inventory (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_listing_id  UUID NOT NULL UNIQUE REFERENCES seller_listings(id) ON DELETE CASCADE,
    available_quantity INTEGER NOT NULL DEFAULT 0 CHECK (available_quantity >= 0),
    reserved_quantity  INTEGER NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    damaged_quantity   INTEGER NOT NULL DEFAULT 0 CHECK (damaged_quantity >= 0),
    returned_quantity  INTEGER NOT NULL DEFAULT 0 CHECK (returned_quantity >= 0),
    low_stock_threshold INTEGER NOT NULL DEFAULT 5,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_inventory_low_stock
    ON inventory (seller_listing_id)
    WHERE available_quantity <= low_stock_threshold;
CREATE TRIGGER trg_inventory_updated BEFORE UPDATE ON inventory
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Immutable ledger of every stock movement for auditability.
CREATE TABLE inventory_transactions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_listing_id  UUID NOT NULL REFERENCES seller_listings(id) ON DELETE CASCADE,
    change_type        TEXT NOT NULL
                          CHECK (change_type IN ('restock', 'adjustment', 'reserve',
                                                 'release', 'sale', 'return_in',
                                                 'damage', 'correction')),
    quantity_delta     INTEGER NOT NULL,      -- signed change
    balance_after      INTEGER NOT NULL,      -- available_quantity after change
    reference_type     TEXT NOT NULL DEFAULT '',   -- order | return | manual ...
    reference_id       UUID,
    note               TEXT NOT NULL DEFAULT '',
    created_by         UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_inventory_tx_listing ON inventory_transactions (seller_listing_id);
CREATE INDEX ix_inventory_tx_reference ON inventory_transactions (reference_type, reference_id);
