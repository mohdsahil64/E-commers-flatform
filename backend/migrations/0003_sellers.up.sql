-- =====================================================================
-- 0003 Sellers, onboarding, documents, bank accounts, pickup addresses
-- =====================================================================

CREATE TABLE sellers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    store_name            TEXT NOT NULL,
    store_slug            TEXT NOT NULL UNIQUE,
    store_description     TEXT NOT NULL DEFAULT '',
    store_logo_key        TEXT,
    -- Lifecycle status of the seller account.
    status                TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'under_review', 'approved',
                                              'rejected', 'suspended', 'blocked')),
    -- Business/owner/contact information.
    legal_business_name   TEXT NOT NULL DEFAULT '',
    business_type         TEXT NOT NULL DEFAULT '' ,  -- individual | proprietorship | pvt_ltd | llp ...
    owner_name            TEXT NOT NULL DEFAULT '',
    contact_email         CITEXT,
    contact_phone         TEXT,
    -- Tax / identity.
    gst_number            TEXT,
    pan_number            TEXT,
    -- Commission override (NULL => use marketplace default).
    commission_percent    NUMERIC(5,2),
    -- Ratings rollups (maintained by triggers/services).
    rating_avg            NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count          INTEGER NOT NULL DEFAULT 0,
    -- Review/approval bookkeeping.
    approved_at           TIMESTAMPTZ,
    approved_by           UUID REFERENCES users(id) ON DELETE SET NULL,
    rejection_reason      TEXT,
    suspended_reason      TEXT,
    agreement_accepted    BOOLEAN NOT NULL DEFAULT FALSE,
    agreement_accepted_at TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_sellers_status ON sellers (status);
CREATE INDEX ix_sellers_store_name_trgm ON sellers USING gin (store_name gin_trgm_ops);
CREATE TRIGGER trg_sellers_updated BEFORE UPDATE ON sellers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Structured onboarding record (one active per seller). Mirrors the multi-step
-- onboarding wizard and its submission/review state.
CREATE TABLE seller_onboarding (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id          UUID NOT NULL UNIQUE REFERENCES sellers(id) ON DELETE CASCADE,
    step               TEXT NOT NULL DEFAULT 'business'
                          CHECK (step IN ('business', 'contact', 'pickup', 'bank',
                                          'documents', 'agreement', 'submitted')),
    business_completed BOOLEAN NOT NULL DEFAULT FALSE,
    contact_completed  BOOLEAN NOT NULL DEFAULT FALSE,
    pickup_completed   BOOLEAN NOT NULL DEFAULT FALSE,
    bank_completed     BOOLEAN NOT NULL DEFAULT FALSE,
    documents_completed BOOLEAN NOT NULL DEFAULT FALSE,
    submitted_at       TIMESTAMPTZ,
    reviewed_at        TIMESTAMPTZ,
    reviewed_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    review_notes       TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TRIGGER trg_seller_onboarding_updated BEFORE UPDATE ON seller_onboarding
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- KYC / business documents uploaded to object storage (only keys stored here).
CREATE TABLE seller_documents (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id      UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    doc_type       TEXT NOT NULL,   -- gst | pan | cheque | address_proof | identity ...
    file_key       TEXT NOT NULL,
    file_name      TEXT NOT NULL DEFAULT '',
    mime_type      TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'verified', 'rejected')),
    rejection_note TEXT NOT NULL DEFAULT '',
    verified_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    verified_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_seller_documents_seller ON seller_documents (seller_id);

CREATE TABLE seller_bank_accounts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id          UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    account_holder     TEXT NOT NULL,
    account_number     TEXT NOT NULL,
    ifsc               TEXT NOT NULL,
    bank_name          TEXT NOT NULL DEFAULT '',
    branch             TEXT NOT NULL DEFAULT '',
    is_primary         BOOLEAN NOT NULL DEFAULT TRUE,
    verified           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_seller_bank_seller ON seller_bank_accounts (seller_id);
CREATE TRIGGER trg_seller_bank_updated BEFORE UPDATE ON seller_bank_accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE seller_pickup_addresses (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seller_id      UUID NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
    label          TEXT NOT NULL DEFAULT 'Primary',
    contact_name   TEXT NOT NULL,
    contact_phone  TEXT NOT NULL,
    line1          TEXT NOT NULL,
    line2          TEXT NOT NULL DEFAULT '',
    city           TEXT NOT NULL,
    state          TEXT NOT NULL,
    pincode        TEXT NOT NULL,
    country        TEXT NOT NULL DEFAULT 'India',
    is_default     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ix_seller_pickup_seller ON seller_pickup_addresses (seller_id);
CREATE TRIGGER trg_seller_pickup_updated BEFORE UPDATE ON seller_pickup_addresses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
