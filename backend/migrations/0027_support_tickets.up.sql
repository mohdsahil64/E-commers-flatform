-- ---------------------------------------------------------------------
-- Help & Support ticketing.
--
-- A customer raises a support request from the Help Centre. Based on the
-- chosen category the ticket is ROUTED to exactly one responder:
--
--   * product-related issues  ->  the SELLER of the selected order item
--   * login / payment / account / other  ->  ADMIN
--
-- Seller handles (replies + resolves) only its own product tickets. Admin
-- handles every non-product ticket, and can READ (but not act on) product
-- tickets so it can audit the customer<->seller conversation.
--
-- Every reply (customer / seller / admin) is a row in support_ticket_messages,
-- giving all portals one shared, ordered conversation timeline. Photo proof the
-- customer attaches at open time lives in support_ticket_photos.
-- ---------------------------------------------------------------------

-- Replace the unused placeholder support schema from 0015 (raised_by /
-- related_order_id / priority / assigned_to), which no code ever used, with
-- the richer routed-ticketing schema below. CASCADE also removes the new
-- message/photo tables if a prior run of this migration partially applied, so
-- the rebuild below is fully idempotent.
DROP TABLE IF EXISTS support_ticket_photos CASCADE;
DROP TABLE IF EXISTS support_ticket_messages CASCADE;
DROP TABLE IF EXISTS support_messages CASCADE;
DROP TABLE IF EXISTS support_tickets CASCADE;

CREATE TABLE support_tickets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number   TEXT NOT NULL UNIQUE,
    customer_id     UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,

    -- Routing. assignee = 'seller' | 'admin'. seller_id is set only for
    -- product tickets (the owner of the chosen order item).
    category        TEXT NOT NULL,              -- product | payment | login | account | refund | other
    assignee        TEXT NOT NULL,              -- seller | admin
    seller_id       UUID REFERENCES sellers(id) ON DELETE SET NULL,

    -- Product context (only for product tickets).
    order_id        UUID REFERENCES orders(id) ON DELETE SET NULL,
    order_item_id   UUID REFERENCES order_items(id) ON DELETE SET NULL,

    subject         TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'open'    -- open | in_progress | solved
        CHECK (status IN ('open', 'in_progress', 'solved')),

    -- Resolution note recorded by the responder when marking solved; this is
    -- the "why it happened / what we did" text surfaced in the resolution email.
    resolution      TEXT NOT NULL DEFAULT '',
    resolved_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at     TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_support_tickets_customer ON support_tickets (customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_support_tickets_seller   ON support_tickets (seller_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_support_tickets_assignee ON support_tickets (assignee, status, created_at DESC);

-- Shared conversation timeline. author_role = customer | seller | admin.
CREATE TABLE support_ticket_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    author_role TEXT NOT NULL,              -- customer | seller | admin | system
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    body        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_support_messages_ticket ON support_ticket_messages (ticket_id, created_at);

-- Photo proof uploaded by the customer when opening the ticket.
CREATE TABLE support_ticket_photos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    file_key    TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_support_photos_ticket ON support_ticket_photos (ticket_id);

-- Grant the new support.manage permission to the roles that should see the
-- admin support desk (SUPER_ADMIN is implicitly all-powerful in middleware).
INSERT INTO permissions (code, description)
VALUES ('support.manage', 'Manage customer support tickets')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE p.code = 'support.manage'
  AND r.code IN ('SUPER_ADMIN', 'ADMIN', 'SUPPORT_ADMIN')
ON CONFLICT DO NOTHING;
