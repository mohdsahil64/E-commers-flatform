-- =====================================================================
-- 0002 Identity & RBAC
-- users, roles, permissions, role_permissions, user_roles, customers,
-- customer_addresses
-- =====================================================================

-- Reusable trigger to maintain updated_at columns.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------
-- users: the single identity table. A user belongs to exactly one
-- "account_type" portal boundary (customer | seller | admin) but shares
-- the same authentication mechanism. Portal-scoped login is enforced by
-- checking account_type at login time.
-- ---------------------------------------------------------------------
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_type    TEXT NOT NULL CHECK (account_type IN ('customer', 'seller', 'admin')),
    email           CITEXT NOT NULL,
    phone           TEXT,
    password_hash   TEXT NOT NULL,
    full_name       TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'inactive', 'suspended', 'blocked')),
    email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    phone_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Email is unique per portal (same email may exist as customer and seller).
CREATE UNIQUE INDEX ux_users_type_email ON users (account_type, email);
CREATE INDEX ix_users_phone ON users (phone);
CREATE TRIGGER trg_users_updated BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------
-- roles & permissions (RBAC)
-- ---------------------------------------------------------------------
CREATE TABLE roles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         TEXT NOT NULL UNIQUE,   -- e.g. SUPER_ADMIN, SELLER, CUSTOMER
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    is_system    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_roles_updated BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE permissions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         TEXT NOT NULL UNIQUE,   -- e.g. seller.approve, product.reject
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id        UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id  UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

-- ---------------------------------------------------------------------
-- customers: profile data for account_type = 'customer'.
-- ---------------------------------------------------------------------
CREATE TABLE customers (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    display_name             TEXT NOT NULL DEFAULT '',
    gender                   TEXT CHECK (gender IN ('male', 'female', 'other', 'unspecified')) DEFAULT 'unspecified',
    date_of_birth            DATE,
    default_address_id       UUID,   -- FK added after customer_addresses exists
    notify_email             BOOLEAN NOT NULL DEFAULT TRUE,
    notify_sms               BOOLEAN NOT NULL DEFAULT TRUE,
    notify_push              BOOLEAN NOT NULL DEFAULT TRUE,
    marketing_opt_in         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_customers_updated BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE customer_addresses (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id    UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    label          TEXT NOT NULL DEFAULT 'Home',   -- Home / Work / Other
    contact_name   TEXT NOT NULL,
    contact_phone  TEXT NOT NULL,
    line1          TEXT NOT NULL,
    line2          TEXT NOT NULL DEFAULT '',
    landmark       TEXT NOT NULL DEFAULT '',
    city           TEXT NOT NULL,
    state          TEXT NOT NULL,
    pincode        TEXT NOT NULL,
    country        TEXT NOT NULL DEFAULT 'India',
    address_type   TEXT NOT NULL DEFAULT 'home' CHECK (address_type IN ('home', 'work', 'other')),
    is_default     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_customer_addresses_customer ON customer_addresses (customer_id);
CREATE TRIGGER trg_customer_addresses_updated BEFORE UPDATE ON customer_addresses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Wire the default address FK now that the table exists.
ALTER TABLE customers
    ADD CONSTRAINT fk_customers_default_address
    FOREIGN KEY (default_address_id) REFERENCES customer_addresses(id) ON DELETE SET NULL;
