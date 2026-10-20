-- =====================================================================
-- 0005 Product model (normalized marketplace catalog)
--
-- Relationship:
--   Seller ─┐
--           ├─> seller_listings ──> products ──> product_variants
--   Product ┘                          │              │
--           category / subcategory / gender / brand    └─> product_variant_attributes
--
-- A single Product is shared; each Seller connects to it (or a specific
-- variant) via a seller_listing that carries seller-specific SKU / price /
-- stock / status. Inventory lives in its own table keyed by seller_listing.
-- =====================================================================

-- products: the canonical catalogue item (brand + gender + category), owned by
-- the seller who created it but modelled to allow multiple sellers to list it.
CREATE TABLE products (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by_seller UUID REFERENCES sellers(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    slug             TEXT NOT NULL UNIQUE,
    description      TEXT NOT NULL DEFAULT '',
    brand_id         UUID REFERENCES brands(id) ON DELETE SET NULL,
    gender_id        UUID REFERENCES genders(id) ON DELETE SET NULL,
    category_id      UUID REFERENCES categories(id) ON DELETE SET NULL,
    subcategory_id   UUID REFERENCES categories(id) ON DELETE SET NULL,
    -- Marketplace approval workflow status for the product record.
    status           TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft', 'submitted', 'under_review',
                                          'approved', 'rejected', 'suspended', 'archived')),
    rejection_reason TEXT NOT NULL DEFAULT '',
    approved_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at      TIMESTAMPTZ,
    -- Denormalized rating rollups for fast storefront reads.
    rating_avg       NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count     INTEGER NOT NULL DEFAULT 0,
    -- Tax metadata (HSN + gst percent) captured at product level.
    hsn_code         TEXT NOT NULL DEFAULT '',
    tax_percent      NUMERIC(5,2) NOT NULL DEFAULT 0,
    -- Free-form product attributes not tied to variants (fabric, care, etc.).
    attributes       JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_products_status ON products (status);
CREATE INDEX ix_products_category ON products (category_id);
CREATE INDEX ix_products_subcategory ON products (subcategory_id);
CREATE INDEX ix_products_gender ON products (gender_id);
CREATE INDEX ix_products_brand ON products (brand_id);
CREATE INDEX ix_products_creator ON products (created_by_seller);
CREATE INDEX ix_products_title_trgm ON products USING gin (title gin_trgm_ops);
-- Full-text search vector over title + description (weighted).
ALTER TABLE products ADD COLUMN search_tsv tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(description, '')), 'B')
    ) STORED;
CREATE INDEX ix_products_search_tsv ON products USING gin (search_tsv);
CREATE TRIGGER trg_products_updated BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- product_images: stored in object storage; DB holds keys + metadata only.
CREATE TABLE product_images (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id   UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id   UUID,   -- optional link to a specific variant (FK added below)
    file_key     TEXT NOT NULL,
    alt_text     TEXT NOT NULL DEFAULT '',
    mime_type    TEXT NOT NULL DEFAULT '',
    width        INTEGER NOT NULL DEFAULT 0,
    height       INTEGER NOT NULL DEFAULT 0,
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    is_primary   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_product_images_product ON product_images (product_id);

-- product_variants: a concrete purchasable variant (size + color combination).
CREATE TABLE product_variants (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id    UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_code  TEXT NOT NULL,          -- internal variant identifier
    barcode       TEXT,
    -- Canonical MRP/selling price at the product level (seller listings can
    -- override with their own price).
    mrp           NUMERIC(12,2) NOT NULL DEFAULT 0,
    selling_price NUMERIC(12,2) NOT NULL DEFAULT 0,
    weight_grams  INTEGER NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (product_id, variant_code)
);
CREATE INDEX ix_product_variants_product ON product_variants (product_id);
CREATE TRIGGER trg_product_variants_updated BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Now that variants exist, link product_images.variant_id.
ALTER TABLE product_images
    ADD CONSTRAINT fk_product_images_variant
    FOREIGN KEY (variant_id) REFERENCES product_variants(id) ON DELETE SET NULL;

-- product_variant_attributes: the attribute values that define a variant
-- (e.g. Size=M, Color=Red). Together they make the variant unique.
CREATE TABLE product_variant_attributes (
    variant_id          UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    attribute_id        UUID NOT NULL REFERENCES attributes(id) ON DELETE RESTRICT,
    attribute_value_id  UUID NOT NULL REFERENCES attribute_values(id) ON DELETE RESTRICT,
    PRIMARY KEY (variant_id, attribute_id)
);
CREATE INDEX ix_pva_value ON product_variant_attributes (attribute_value_id);

-- seller_listings: connects a seller to a product/variant with seller-specific
-- commercial data. This is the row that actually appears in orders.
CREATE TABLE seller_listings (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id        UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    product_id       UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id       UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    sku              TEXT NOT NULL,          -- seller-specific SKU
    mrp              NUMERIC(12,2) NOT NULL DEFAULT 0,
    selling_price    NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
    currency         TEXT NOT NULL DEFAULT 'INR',
    status           TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'inactive', 'out_of_stock', 'blocked')),
    -- Seller-specific extra attributes if required.
    seller_attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (seller_id, variant_id),
    UNIQUE (seller_id, sku)
);
CREATE INDEX ix_seller_listings_seller ON seller_listings (seller_id);
CREATE INDEX ix_seller_listings_product ON seller_listings (product_id);
CREATE INDEX ix_seller_listings_variant ON seller_listings (variant_id);
CREATE INDEX ix_seller_listings_status ON seller_listings (status);
CREATE TRIGGER trg_seller_listings_updated BEFORE UPDATE ON seller_listings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
