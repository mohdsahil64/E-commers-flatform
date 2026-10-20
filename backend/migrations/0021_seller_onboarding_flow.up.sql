-- =====================================================================
-- 0021 Seller onboarding flow
-- Extends the seller lifecycle to support the full onboarding journey:
--   verified registration -> owner/bank details -> pickup location ->
--   commission contract signing -> submit -> admin review.
-- Adds resubmit tracking (max 3 attempts then permanent block) and the
-- commission-contract signing record.
-- =====================================================================

-- ── sellers: resubmit tracking + onboarding completion + contract ────────
ALTER TABLE sellers
    ADD COLUMN IF NOT EXISTS resubmit_count       INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS onboarding_completed  BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS contract_signed       BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS contract_signed_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS contract_signed_name  TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS contract_version      TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS contract_commission   NUMERIC(5,2);

-- The commission rate a seller agrees to when signing the contract. Default
-- marketplace commission is applied when the seller signs (see backend).
COMMENT ON COLUMN sellers.contract_commission IS 'Commission % the seller agreed to in the signed contract';

-- ── seller_onboarding: add pickup + contract step flags ──────────────────
-- Extend the step CHECK to include the contract step and keep 'submitted'.
ALTER TABLE seller_onboarding DROP CONSTRAINT IF EXISTS seller_onboarding_step_check;
ALTER TABLE seller_onboarding
    ADD CONSTRAINT seller_onboarding_step_check
    CHECK (step IN ('business', 'contact', 'owner', 'bank', 'pickup',
                    'documents', 'contract', 'agreement', 'submitted'));

ALTER TABLE seller_onboarding
    ADD COLUMN IF NOT EXISTS contract_completed BOOLEAN NOT NULL DEFAULT FALSE;
