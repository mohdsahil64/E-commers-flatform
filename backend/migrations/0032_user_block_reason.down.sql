ALTER TABLE users
    DROP COLUMN IF EXISTS blocked_by,
    DROP COLUMN IF EXISTS blocked_at,
    DROP COLUMN IF EXISTS block_reason;
