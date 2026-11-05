-- ---------------------------------------------------------------------
-- Seller -> Admin support.
--
-- A seller raises a help request to the CARLITOS admin team (payments,
-- settlements, account/KYC, technical, ...). This is separate from the
-- customer support tickets, where the seller is the responder.
--
-- Same shape as the customer system: one ticket, one shared message timeline
-- (seller + admin), and optional proof photos uploaded by the seller.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS seller_support_tickets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number TEXT NOT NULL UNIQUE,
    seller_id     UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    category      TEXT NOT NULL,
    subject       TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'in_progress', 'solved')),
    resolution    TEXT NOT NULL DEFAULT '',
    resolved_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_seller_support_tickets_seller ON seller_support_tickets (seller_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_seller_support_tickets_status ON seller_support_tickets (status, created_at DESC);

CREATE TABLE IF NOT EXISTS seller_support_messages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id   UUID NOT NULL REFERENCES seller_support_tickets(id) ON DELETE CASCADE,
    author_role TEXT NOT NULL,              -- seller | admin | system
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    body        TEXT NOT NULL DEFAULT '',
    image_key   TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_seller_support_messages_ticket ON seller_support_messages (ticket_id, created_at);

CREATE TABLE IF NOT EXISTS seller_support_photos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id  UUID NOT NULL REFERENCES seller_support_tickets(id) ON DELETE CASCADE,
    file_key   TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_seller_support_photos_ticket ON seller_support_photos (ticket_id);
