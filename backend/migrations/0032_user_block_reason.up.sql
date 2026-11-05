-- Why / when / by whom an account was blocked by an admin. The reason is shown
-- to the user when they try to log in to a blocked account.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS block_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS blocked_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS blocked_by   UUID REFERENCES users(id) ON DELETE SET NULL;
