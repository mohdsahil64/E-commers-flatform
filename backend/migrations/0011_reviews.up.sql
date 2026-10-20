-- =====================================================================
-- 0011 Reviews & review_images
-- A customer can review a product only once per eligible purchased order item.
-- =====================================================================

CREATE TABLE reviews (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    customer_id    UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    seller_id      UUID REFERENCES sellers(id) ON DELETE SET NULL,
    order_item_id  UUID REFERENCES order_items(id) ON DELETE SET NULL,
    rating         SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title          TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL DEFAULT '',
    is_verified_purchase BOOLEAN NOT NULL DEFAULT FALSE,
    status         TEXT NOT NULL DEFAULT 'published'
                     CHECK (status IN ('pending', 'published', 'rejected', 'hidden')),
    helpful_count  INTEGER NOT NULL DEFAULT 0,
    moderated_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    moderated_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Prevent duplicate reviews for the same eligible order item.
    UNIQUE (order_item_id)
);
CREATE INDEX ix_reviews_product ON reviews (product_id);
CREATE INDEX ix_reviews_customer ON reviews (customer_id);
CREATE INDEX ix_reviews_status ON reviews (status);
CREATE TRIGGER trg_reviews_updated BEFORE UPDATE ON reviews
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE review_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id   UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    file_key    TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_review_images_review ON review_images (review_id);
