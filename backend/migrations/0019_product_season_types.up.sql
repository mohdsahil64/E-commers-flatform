-- =====================================================================
-- 0019 Product season + generic "type" categories
-- Adds a season dimension to products (all / summer / winter) so the
-- storefront can filter seasonal ranges, and seeds gender-neutral product
-- "type" categories (shoes, innerwear, upperwear, bottomwear, underwear)
-- that a seller picks when listing. Customers can filter by these directly.
-- =====================================================================

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS season TEXT NOT NULL DEFAULT 'all'
        CHECK (season IN ('all', 'summer', 'winter'));

CREATE INDEX IF NOT EXISTS ix_products_season ON products (season);

-- Gender-neutral top-level product type categories. These are level-0 with a
-- NULL gender so any seller/product can use them and customers filter by slug.
INSERT INTO categories (name, slug, level, sort_order, gender_id)
VALUES
    ('Upperwear',  'upperwear',  0, 10, NULL),
    ('Bottomwear', 'bottomwear', 0, 11, NULL),
    ('Innerwear',  'innerwear',  0, 12, NULL),
    ('Underwear',  'underwear',  0, 13, NULL),
    ('Shoes',      'shoes',      0, 14, NULL)
ON CONFLICT (slug) DO NOTHING;
