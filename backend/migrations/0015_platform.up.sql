-- =====================================================================
-- 0015 Platform: notifications, support, audit logs, ranking snapshots,
-- refresh tokens, and configurable settings (ranking weights etc.)
-- =====================================================================

-- Refresh tokens for JWT session rotation (hashed token stored).
CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    user_agent   TEXT NOT NULL DEFAULT '',
    ip           TEXT NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_refresh_tokens_user ON refresh_tokens (user_id);

-- Notifications (in-app record; email/sms delivery handled by workers).
CREATE TABLE notifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel      TEXT NOT NULL DEFAULT 'in_app'
                   CHECK (channel IN ('in_app', 'email', 'sms', 'whatsapp')),
    event_type   TEXT NOT NULL,   -- seller_approved | product_approved | new_order ...
    title        TEXT NOT NULL,
    body         TEXT NOT NULL DEFAULT '',
    data         JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_read      BOOLEAN NOT NULL DEFAULT FALSE,
    delivery_status TEXT NOT NULL DEFAULT 'pending'
                   CHECK (delivery_status IN ('pending', 'sent', 'failed', 'read')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at      TIMESTAMPTZ
);
CREATE INDEX ix_notifications_user ON notifications (user_id, is_read);

-- Support tickets + threaded messages.
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

-- Audit log of important administrative actions.
CREATE TABLE audit_logs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id       UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role     TEXT NOT NULL DEFAULT '',
    action         TEXT NOT NULL,   -- seller.approved | product.rejected | commission.changed ...
    entity_type    TEXT NOT NULL,
    entity_id      TEXT NOT NULL DEFAULT '',
    previous_value JSONB,
    new_value      JSONB,
    ip             TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_logs_actor ON audit_logs (actor_id);
CREATE INDEX ix_audit_logs_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX ix_audit_logs_created ON audit_logs (created_at DESC);

-- Precomputed ranking metadata so the storefront never runs the heavy weighted
-- calculation per request. Refreshed by a background job.
CREATE TABLE product_ranking_snapshots (
    product_id       UUID PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    score            NUMERIC(12,4) NOT NULL DEFAULT 0,
    units_sold       INTEGER NOT NULL DEFAULT 0,
    conversion_rate  NUMERIC(6,4) NOT NULL DEFAULT 0,
    return_rate      NUMERIC(6,4) NOT NULL DEFAULT 0,
    cancellation_rate NUMERIC(6,4) NOT NULL DEFAULT 0,
    recent_velocity  NUMERIC(12,4) NOT NULL DEFAULT 0,
    freshness_days   INTEGER NOT NULL DEFAULT 0,
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_ranking_score ON product_ranking_snapshots (score DESC);

-- Key/value application settings (ranking weights, feature flags, defaults).
CREATE TABLE app_settings (
    key         TEXT PRIMARY KEY,
    value       JSONB NOT NULL DEFAULT '{}'::jsonb,
    description TEXT NOT NULL DEFAULT '',
    updated_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_app_settings_updated BEFORE UPDATE ON app_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
