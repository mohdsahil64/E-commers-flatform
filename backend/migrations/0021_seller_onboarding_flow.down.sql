-- Revert 0021 seller onboarding flow.

ALTER TABLE seller_onboarding DROP CONSTRAINT IF EXISTS seller_onboarding_step_check;
ALTER TABLE seller_onboarding
    ADD CONSTRAINT seller_onboarding_step_check
    CHECK (step IN ('business', 'contact', 'pickup', 'bank',
                    'documents', 'agreement', 'submitted'));

ALTER TABLE seller_onboarding DROP COLUMN IF EXISTS contract_completed;

ALTER TABLE sellers
    DROP COLUMN IF EXISTS resubmit_count,
    DROP COLUMN IF EXISTS onboarding_completed,
    DROP COLUMN IF EXISTS contract_signed,
    DROP COLUMN IF EXISTS contract_signed_at,
    DROP COLUMN IF EXISTS contract_signed_name,
    DROP COLUMN IF EXISTS contract_version,
    DROP COLUMN IF EXISTS contract_commission;
