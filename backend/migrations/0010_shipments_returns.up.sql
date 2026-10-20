-- =====================================================================
-- 0010 Shipments, shipment_tracking_events, returns, return_items, exchanges
-- Courier-agnostic: provider column records which courier handled it.
-- =====================================================================

CREATE TABLE shipments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    seller_id         UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    provider          TEXT NOT NULL DEFAULT 'mock',   -- mock | shiprocket | delhivery | bluedart
    awb_number        TEXT,
    courier_name      TEXT NOT NULL DEFAULT '',
    label_key         TEXT,       -- object storage key for the shipping label
    manifest_key      TEXT,
    status            TEXT NOT NULL DEFAULT 'created'
                        CHECK (status IN ('created', 'label_generated', 'pickup_scheduled',
                                          'picked_up', 'in_transit', 'out_for_delivery',
                                          'delivered', 'failed', 'rto', 'cancelled')),
    -- Direction lets us reuse this table for forward and return shipments.
    direction         TEXT NOT NULL DEFAULT 'forward'
                        CHECK (direction IN ('forward', 'return')),
    pickup_scheduled_at TIMESTAMPTZ,
    shipped_at        TIMESTAMPTZ,
    delivered_at      TIMESTAMPTZ,
    tracking_url      TEXT NOT NULL DEFAULT '',
    weight_grams      INTEGER NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_shipments_order ON shipments (order_id);
CREATE INDEX ix_shipments_seller ON shipments (seller_id);
CREATE INDEX ix_shipments_awb ON shipments (awb_number);
CREATE INDEX ix_shipments_status ON shipments (status);
CREATE TRIGGER trg_shipments_updated BEFORE UPDATE ON shipments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Which order items are in a given shipment.
CREATE TABLE shipment_items (
    shipment_id    UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    order_item_id  UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (shipment_id, order_item_id)
);

CREATE TABLE shipment_tracking_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id   UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    status        TEXT NOT NULL,
    location      TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    event_time    TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_payload   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_tracking_events_shipment ON shipment_tracking_events (shipment_id);

-- Returns (and the exchange variant) are raised against delivered items.
CREATE TABLE returns (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_number  TEXT NOT NULL UNIQUE,
    order_id       UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    customer_id    UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    seller_id      UUID NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    type           TEXT NOT NULL DEFAULT 'return' CHECK (type IN ('return', 'exchange')),
    reason         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'requested'
                     CHECK (status IN ('requested', 'approved', 'rejected', 'picked',
                                       'received', 'refund_processing', 'refunded', 'completed')),
    resolution     TEXT NOT NULL DEFAULT '',  -- refund | exchange | store_credit
    return_shipment_id UUID REFERENCES shipments(id) ON DELETE SET NULL,
    approved_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_returns_order ON returns (order_id);
CREATE INDEX ix_returns_seller ON returns (seller_id);
CREATE INDEX ix_returns_status ON returns (status);
CREATE TRIGGER trg_returns_updated BEFORE UPDATE ON returns
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE return_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id      UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    order_item_id  UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    quantity       INTEGER NOT NULL CHECK (quantity > 0),
    reason         TEXT NOT NULL DEFAULT '',
    condition_note TEXT NOT NULL DEFAULT '',
    refund_amount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    restocked      BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX ix_return_items_return ON return_items (return_id);

-- Exchange target: the new listing a customer wants in place of the returned one.
CREATE TABLE exchanges (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id             UUID NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    original_order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    new_seller_listing_id UUID NOT NULL REFERENCES seller_listings(id) ON DELETE RESTRICT,
    quantity              INTEGER NOT NULL CHECK (quantity > 0),
    price_difference      NUMERIC(12,2) NOT NULL DEFAULT 0,
    status                TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'confirmed', 'shipped', 'completed', 'cancelled')),
    new_order_id          UUID REFERENCES orders(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_exchanges_return ON exchanges (return_id);
