DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'support.manage');
DELETE FROM permissions WHERE code = 'support.manage';

DROP TABLE IF EXISTS support_ticket_photos;
DROP TABLE IF EXISTS support_ticket_messages;
DROP TABLE IF EXISTS support_tickets;

-- Restore the placeholder support schema from 0015 so the DB stays consistent
-- with that migration after rolling 0027 back.
CREATE TABLE support_tickets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_number TEXT NOT NULL UNIQUE,
    raised_by     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject       TEXT NOT NULL,
    category      TEXT NOT NULL DEFAULT 'general',
    related_order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    priority      TEXT NOT NULL DEFAULT 'normal'
                    CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    status        TEXT NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    assigned_to   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_support_tickets_raised_by ON support_tickets (raised_by);
CREATE INDEX ix_support_tickets_status ON support_tickets (status);
CREATE TRIGGER trg_support_tickets_updated BEFORE UPDATE ON support_tickets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE support_messages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id    UUID NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    sender_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    body         TEXT NOT NULL,
    is_internal  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_support_messages_ticket ON support_messages (ticket_id);
