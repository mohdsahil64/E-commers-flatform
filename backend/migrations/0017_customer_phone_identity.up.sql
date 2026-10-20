-- =====================================================================
-- 0017 Customer phone-first identity
-- Customers sign in by mobile number; email is optional and added later
-- (OTP-verified). This makes users.email nullable and switches the unique
-- constraint to a partial index (unique only when email is present), and adds
-- a unique partial index on (account_type, phone) so a phone maps to at most
-- one account per portal. Seller/admin still use email.
-- =====================================================================

-- Allow customers to exist without an email.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Replace the strict unique index with a partial one that ignores NULL emails.
DROP INDEX IF EXISTS ux_users_type_email;
CREATE UNIQUE INDEX ux_users_type_email
    ON users (account_type, email)
    WHERE email IS NOT NULL;

-- One account per (portal, phone) when a phone is present.
CREATE UNIQUE INDEX ux_users_type_phone
    ON users (account_type, phone)
    WHERE phone IS NOT NULL AND phone <> '';
