-- =====================================================================
-- 0016 Shipment type (courier vs self-ship) + provider shipment id
-- Adds support for sellers choosing between a courier (delivery API) and
-- self-shipping (their own delivery), and stores the courier's internal
-- shipment id needed for label/pickup/track calls (distinct from the AWB).
-- =====================================================================

ALTER TABLE shipments
    ADD COLUMN IF NOT EXISTS ship_type TEXT NOT NULL DEFAULT 'courier'
        CHECK (ship_type IN ('courier', 'self')),
    ADD COLUMN IF NOT EXISTS provider_shipment_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS ix_shipments_provider_shipment
    ON shipments (provider_shipment_id);
