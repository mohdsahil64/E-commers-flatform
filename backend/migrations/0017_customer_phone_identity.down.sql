DROP INDEX IF EXISTS ux_users_type_phone;
DROP INDEX IF EXISTS ux_users_type_email;
-- Restore the original strict unique index. (Requires all emails non-null.)
CREATE UNIQUE INDEX ux_users_type_email ON users (account_type, email);
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
