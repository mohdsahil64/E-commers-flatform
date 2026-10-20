-- Revert 0024 seller COD setting.

ALTER TABLE sellers
    DROP COLUMN IF EXISTS cod_enabled;
