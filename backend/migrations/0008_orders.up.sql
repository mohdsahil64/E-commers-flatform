-- =====================================================================
-- 0008 Orders, order_items, order_status_history
--
-- An order groups items that may span multiple sellers. Each order_item is
-- fulfilled by exactly one seller (via its seller_listing) and carries the
-- full financial breakdown required for settlement.
-- =====================================================================

CREATE TABLE orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number        TEXT NOT NULL UNIQUE,       -- human-friendly CRL-YYYY-XXXXXX
    customer_id         UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    -- Order-level lifecycle status (see order state machine).
    status              TEXT NOT NULL DEFAULT 'pending_payment'
                          CHECK (status IN ('pending_payment', 'confirmed', 'processing',
                                            'packed', 'ready_to_ship', 'shipped',
                                            'in_transit', 'out_for_delivery', 'delivered',
                                            'cancelled', 'return_requested', 'return_approved',
                                            'return_picked', 'return_received',
                                            'refund_processing', 'refunded')),
    payment_method      TEXT NOT NULL CHECK (payment_method IN ('online', 'cod')),
    payment_status      TEXT NOT NULL DEFAULT 'pending'
                          CHECK (payment_status IN ('pending', 'authorized', 'paid',
                                                    'failed', 'refunded', 'partially_refunded')),
    currency            TEXT NOT NULL DEFAULT 'INR',
    -- Money rollups (sum of item breakdowns + order-level charges).
    items_subtotal      NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount_total      NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax_total           NUMERIC(12,2) NOT NULL DEFAULT 0,
    shipping_total      NUMERIC(12,2) NOT NULL DEFAULT 0,
    grand_total         NUMERIC(12,2) NOT NULL DEFAULT 0,
    -- Snapshot of the shipping address at purchase time.
    ship_contact_name   TEXT NOT NULL,
    ship_contact_phone  TEXT NOT NULL,
    ship_line1          TEXT NOT NULL,
    ship_line2          TEXT NOT NULL DEFAULT '',
    ship_city           TEXT NOT NULL,
    ship_state          TEXT NOT NULL,
    ship_pincode        TEXT NOT NULL,
    ship_country        TEXT NOT NULL DEFAULT 'India',
    coupon_code         TEXT,
    placed_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_orders_customer ON orders (customer_id);
CREATE INDEX ix_orders_status ON orders (status);
CREATE INDEX ix_orders_payment_status ON orders (payment_status);
CREATE INDEX ix_orders_created ON orders (created_at DESC);
CREATE TRIGGER trg_orders_updated BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id            UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    seller_id           UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    seller_listing_id   UUID NOT NULL REFERENCES seller_listings(id) ON DELETE RESTRICT,
    -- Snapshots so historical orders are stable even if catalog changes.
    product_title       TEXT NOT NULL,
    variant_label       TEXT NOT NULL DEFAULT '',
    sku                 TEXT NOT NULL,
    -- Per-item lifecycle (an item can be fulfilled/returned independently).
    item_status         TEXT NOT NULL DEFAULT 'confirmed'
                          CHECK (item_status IN ('confirmed', 'processing', 'packed',
                                                 'ready_to_ship', 'shipped', 'in_transit',
                                                 'out_for_delivery', 'delivered',
                                                 'cancelled', 'return_requested',
                                                 'return_approved', 'return_picked',
                                                 'return_received', 'refunded')),
    quantity            INTEGER NOT NULL CHECK (quantity > 0),
    -- Financial breakdown (prompt section 7).
    unit_price          NUMERIC(12,2) NOT NULL DEFAULT 0,  -- selling price per unit
    gross_amount        NUMERIC(12,2) NOT NULL DEFAULT 0,  -- unit_price * qty
    discount_amount     NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax_amount          NUMERIC(12,2) NOT NULL DEFAULT 0,
    shipping_charge     NUMERIC(12,2) NOT NULL DEFAULT 0,
    commission_percent  NUMERIC(5,2) NOT NULL DEFAULT 0,
    commission_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,   -- marketplace earning
    payment_fee         NUMERIC(12,2) NOT NULL DEFAULT 0,
    refund_amount       NUMERIC(12,2) NOT NULL DEFAULT 0,
    seller_payable      NUMERIC(12,2) NOT NULL DEFAULT 0,   -- net to seller after deductions
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_order_items_order ON order_items (order_id);
CREATE INDEX ix_order_items_seller ON order_items (seller_id);
CREATE INDEX ix_order_items_listing ON order_items (seller_listing_id);
CREATE INDEX ix_order_items_status ON order_items (item_status);
CREATE TRIGGER trg_order_items_updated BEFORE UPDATE ON order_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only history of every order (and optionally item) status transition.
CREATE TABLE order_status_history (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    order_item_id UUID REFERENCES order_items(id) ON DELETE CASCADE,
    from_status   TEXT NOT NULL DEFAULT '',
    to_status     TEXT NOT NULL,
    changed_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role    TEXT NOT NULL DEFAULT '',   -- system | customer | seller | admin
    note          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_order_status_history_order ON order_status_history (order_id);
