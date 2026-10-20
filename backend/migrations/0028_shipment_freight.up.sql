-- Actual courier freight billed to the platform's courier account for a
-- shipment (captured at AWB assignment). It is split onto the seller's
-- order_items.shipping_charge so the seller settlement deducts the real
-- delivery cost from their payout (admin pays the courier up front, recovers it
-- from the seller's remittance after delivery).
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS freight_charge NUMERIC(12,2) NOT NULL DEFAULT 0;
