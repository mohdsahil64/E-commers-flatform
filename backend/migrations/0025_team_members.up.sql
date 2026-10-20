-- =====================================================================
-- 0025 Team & Management members
-- Admin-editable company team roster shown on the customer storefront.
-- role/bio/sort_order are seeded system defaults; admins edit name + photo.
-- =====================================================================

CREATE TABLE IF NOT EXISTS team_members (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    role        TEXT NOT NULL,
    bio         TEXT NOT NULL DEFAULT '',
    photo_key   TEXT,
    sort_order  INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_team_members_active_sort
    ON team_members (is_active, sort_order);

INSERT INTO team_members (name, role, bio, photo_key, sort_order) VALUES
    ('Founder Name',    'Founder & Chairman',       'Founder and chairman guiding the company vision.',       NULL, 1),
    ('CEO Name',        'Chief Executive Officer',  'Leads overall strategy and company operations.',         NULL, 2),
    ('CTO Name',        'Chief Technology Officer', 'Heads technology, engineering and product delivery.',    NULL, 3),
    ('COO Name',        'Chief Operating Officer',  'Oversees day-to-day operations and execution.',          NULL, 4),
    ('Operations Head', 'Head of Operations',       'Manages fulfilment, logistics and operational quality.', NULL, 5);
