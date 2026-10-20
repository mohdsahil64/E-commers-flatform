-- =====================================================================
-- 0012 Merchandising: coupons, promotions, collections, banners
-- =====================================================================

-- ---------------------------------------------------------------------
-- Coupons & rules
-- ---------------------------------------------------------------------
CREATE TABLE coupons (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code               TEXT NOT NULL UNIQUE,
    description        TEXT NOT NULL DEFAULT '',
    discount_type      TEXT NOT NULL CHECK (discount_type IN ('percent', 'fixed')),
    discount_value     NUMERIC(12,2) NOT NULL,
    max_discount       NUMERIC(12,2),
    min_order_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    usage_limit_total  INTEGER,          -- NULL => unlimited
    usage_limit_per_user INTEGER,
    used_count         INTEGER NOT NULL DEFAULT 0,
    starts_at          TIMESTAMPTZ,
    ends_at            TIMESTAMPTZ,
    is_active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_coupons_updated BEFORE UPDATE ON coupons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Optional targeting rules (restrict a coupon to categories/brands/sellers).
CREATE TABLE coupon_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coupon_id   UUID NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    rule_type   TEXT NOT NULL,   -- category | brand | seller | gender | first_order
    rule_value  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_coupon_rules_coupon ON coupon_rules (coupon_id);

-- Promotions are broader campaigns (banners/collections can reference them).
CREATE TABLE promotions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    starts_at    TIMESTAMPTZ,
    ends_at      TIMESTAMPTZ,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_promotions_updated BEFORE UPDATE ON promotions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------
-- Collections (New Arrivals, Best Sellers, Trending, Sale, ...)
-- A collection can be manual (collection_products) and/or rule-based
-- (collection_rules). type marks the intended behaviour.
-- ---------------------------------------------------------------------
CREATE TABLE collections (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    slug         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT 'manual'
                   CHECK (type IN ('manual', 'automatic', 'hybrid')),
    kind         TEXT NOT NULL DEFAULT 'featured'
                   CHECK (kind IN ('new_arrivals', 'best_sellers', 'trending', 'sale',
                                   'featured', 'recommended', 'festival', 'seasonal')),
    image_key    TEXT,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_collections_updated BEFORE UPDATE ON collections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE collection_products (
    collection_id  UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    product_id     UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sort_order     INTEGER NOT NULL DEFAULT 0,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (collection_id, product_id)
);

-- Rules for automatic/hybrid collections (category/seller/attribute/price/ranking).
CREATE TABLE collection_rules (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_id  UUID NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    rule_type      TEXT NOT NULL,   -- category | subcategory | gender | brand | seller | attribute | price_min | price_max | ranking
    rule_operator  TEXT NOT NULL DEFAULT 'eq',   -- eq | lte | gte | in
    rule_value     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_collection_rules_collection ON collection_rules (collection_id);

-- ---------------------------------------------------------------------
-- Banners + targeting rules (destination is data-driven, never hardcoded).
-- ---------------------------------------------------------------------
CREATE TABLE banners (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title          TEXT NOT NULL,
    subtitle       TEXT NOT NULL DEFAULT '',
    image_key      TEXT NOT NULL DEFAULT '',
    mobile_image_key TEXT,
    placement      TEXT NOT NULL DEFAULT 'home_hero'
                     CHECK (placement IN ('home_hero', 'home_strip', 'category_top',
                                          'gender_top', 'promo')),
    -- Destination kind resolved by the storefront into a listing query.
    destination_type TEXT NOT NULL DEFAULT 'category'
                       CHECK (destination_type IN ('category', 'subcategory', 'gender',
                                                   'seller', 'product', 'collection',
                                                   'search', 'filter')),
    sort_order     INTEGER NOT NULL DEFAULT 0,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    starts_at      TIMESTAMPTZ,
    ends_at        TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_banners_placement ON banners (placement);
CREATE TRIGGER trg_banners_updated BEFORE UPDATE ON banners
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Targeting rows: a banner can carry multiple filter targets that together
-- form the destination listing query (e.g. gender=women, category=dresses,
-- color=pastel, availability=in_stock).
CREATE TABLE banner_targets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id    UUID NOT NULL REFERENCES banners(id) ON DELETE CASCADE,
    target_key   TEXT NOT NULL,   -- gender | category | subcategory | seller | collection | search | color | size | brand | price_min | price_max | availability
    target_value TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_banner_targets_banner ON banner_targets (banner_id);
