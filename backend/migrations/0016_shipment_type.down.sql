DROP INDEX IF EXISTS ix_shipments_provider_shipment;
ALTER TABLE shipments
    DROP COLUMN IF EXISTS provider_shipment_id,
    DROP COLUMN IF EXISTS ship_type;
