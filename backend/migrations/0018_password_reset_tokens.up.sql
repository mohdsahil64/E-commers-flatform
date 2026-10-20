-- =====================================================================
-- 0018 Password reset tokens
-- Supports the "forgot password" flow for email/password portals
-- (seller & admin). A request stores a single-use, hashed, short-lived
-- token. On reset the token is validated, consumed (used_at set), and the
-- user's password_hash is updated. Customers use mobile OTP and are not
-- covered here.
-- =====================================================================

CREATE TABLE password_reset_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL,                 -- SHA-256 hex of the raw token
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,                   -- set once the token is consumed
    created_ip  TEXT NOT NULL DEFAULT '',
    created_ua  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup by token hash (the value presented at reset time).
CREATE UNIQUE INDEX ux_password_reset_token_hash ON password_reset_tokens (token_hash);
-- Lookup / cleanup of a user's outstanding tokens.
CREATE INDEX ix_password_reset_user ON password_reset_tokens (user_id);
