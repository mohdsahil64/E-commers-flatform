-- =====================================================================
-- 0007 Carts & Wishlists
-- =====================================================================

CREATE TABLE carts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id  UUID NOT NULL UNIQUE REFERENCES customers(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_carts_updated BEFORE UPDATE ON carts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE cart_items (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id           UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    seller_listing_id UUID NOT NULL REFERENCES seller_listings(id) ON DELETE CASCADE,
    quantity          INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    -- Price snapshot for display; authoritative price is recomputed at checkout.
    unit_price_snapshot NUMERIC(12,2) NOT NULL DEFAULT 0,
    added_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cart_id, seller_listing_id)
);
CREATE INDEX ix_cart_items_cart ON cart_items (cart_id);
CREATE TRIGGER trg_cart_items_updated BEFORE UPDATE ON cart_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE wishlists (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id  UUID NOT NULL UNIQUE REFERENCES customers(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE wishlist_items (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wishlist_id  UUID NOT NULL REFERENCES wishlists(id) ON DELETE CASCADE,
    product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    -- Optional preferred variant/listing to jump straight to.
    seller_listing_id UUID REFERENCES seller_listings(id) ON DELETE SET NULL,
    added_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (wishlist_id, product_id)
);
CREATE INDEX ix_wishlist_items_wishlist ON wishlist_items (wishlist_id);
