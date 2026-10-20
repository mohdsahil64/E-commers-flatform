-- =====================================================================
-- 0004 Catalog taxonomy: genders, categories (nested), subcategories,
-- brands, attributes, attribute_values, category_attributes
-- =====================================================================

-- Gender is a first-class navigation dimension (Men / Women / Boys / Girls).
CREATE TABLE genders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        TEXT NOT NULL UNIQUE,      -- men | women | boys | girls
    name        TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

-- categories support nesting via parent_id. A category may be scoped to a
-- gender (nullable => applies across genders). Subcategories are simply
-- categories with a parent, but we also keep an explicit "level" for clarity.
CREATE TABLE categories (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id     UUID REFERENCES categories(id) ON DELETE CASCADE,
    gender_id     UUID REFERENCES genders(id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    slug          TEXT NOT NULL UNIQUE,
    level         INTEGER NOT NULL DEFAULT 0,   -- 0=top category, 1=subcategory, 2=nested
    description   TEXT NOT NULL DEFAULT '',
    image_key     TEXT,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_categories_parent ON categories (parent_id);
CREATE INDEX ix_categories_gender ON categories (gender_id);
CREATE INDEX ix_categories_name_trgm ON categories USING gin (name gin_trgm_ops);
CREATE TRIGGER trg_categories_updated BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE brands (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    slug        TEXT NOT NULL UNIQUE,
    logo_key    TEXT,
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_brands_name_trgm ON brands USING gin (name gin_trgm_ops);
CREATE TRIGGER trg_brands_updated BEFORE UPDATE ON brands
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- attributes are the definitions (e.g. "Size", "Color", "Fabric", "Fit").
-- input_type controls how the frontend renders the filter/selector.
CREATE TABLE attributes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         TEXT NOT NULL UNIQUE,     -- size | color | fabric | fit ...
    name         TEXT NOT NULL,
    input_type   TEXT NOT NULL DEFAULT 'select'
                    CHECK (input_type IN ('select', 'multiselect', 'text', 'number', 'boolean', 'color')),
    is_variant   BOOLEAN NOT NULL DEFAULT FALSE,   -- true => contributes to variant identity (size/color)
    is_filterable BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- attribute_values are the allowed values for an attribute (e.g. S, M, L for Size).
CREATE TABLE attribute_values (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id  UUID NOT NULL REFERENCES attributes(id) ON DELETE CASCADE,
    value         TEXT NOT NULL,           -- "M", "Red"
    display_value TEXT NOT NULL DEFAULT '',-- "Medium", "Crimson Red"
    meta          JSONB NOT NULL DEFAULT '{}'::jsonb,  -- e.g. {"hex":"#ff0000"} for colors
    sort_order    INTEGER NOT NULL DEFAULT 0,
    UNIQUE (attribute_id, value)
);
CREATE INDEX ix_attribute_values_attribute ON attribute_values (attribute_id);

-- Which attributes apply to which category (and whether required).
CREATE TABLE category_attributes (
    category_id   UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    attribute_id  UUID NOT NULL REFERENCES attributes(id) ON DELETE CASCADE,
    is_required   BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, attribute_id)
);
