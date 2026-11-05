-- =====================================================================
-- 0031 Admin staff (employees) & secure first-login setup
--
-- Admin accounts can invite employees. An invited employee gets a one-time
-- temporary password by email. On first login the temporary password is
-- NOT enough to get a session: the employee must verify an emailed one-time
-- code and choose a new password. Only then does the account become active.
--
--   admin_staff            lifecycle of an invited employee
--   admin_setup_challenges short-lived, single-use first-login challenges
--                          (hashed token + hashed email code, attempt-limited)
--
-- The user's role(s) live in the existing user_roles table; roles and their
-- permissions live in roles / role_permissions (custom roles are non-system).
-- =====================================================================

CREATE TABLE admin_staff (
    user_id              UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- invited: temp password issued, not yet accepted. active: setup completed.
    status               TEXT NOT NULL DEFAULT 'invited'
                           CHECK (status IN ('invited', 'active')),
    invited_by           UUID REFERENCES users(id) ON DELETE SET NULL,
    invited_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    invite_expires_at    TIMESTAMPTZ,
    accepted_at          TIMESTAMPTZ,
    -- While TRUE the account cannot obtain a session or call the API.
    must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
    invite_count         INTEGER NOT NULL DEFAULT 1,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_admin_staff_status ON admin_staff (status);
CREATE TRIGGER trg_admin_staff_updated BEFORE UPDATE ON admin_staff
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE admin_setup_challenges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SHA-256 of the opaque setup token handed to the browser.
    token_hash      TEXT NOT NULL UNIQUE,
    -- SHA-256(token_hash || code): the emailed 6-digit code is never stored.
    code_hash       TEXT NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    resend_count    INTEGER NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ NOT NULL,
    code_expires_at TIMESTAMPTZ NOT NULL,
    last_sent_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at         TIMESTAMPTZ,
    created_ip      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_admin_setup_user ON admin_setup_challenges (user_id);
