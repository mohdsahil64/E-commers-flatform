-- =====================================================================
-- 0020 Sequential human-friendly order numbers (CRL-00001, CRL-00002 …)
-- A dedicated sequence gives gap-free-ish, monotonically increasing order
-- numbers that are easy for sellers/customers to read and reference.
-- =====================================================================

CREATE SEQUENCE IF NOT EXISTS order_number_seq START 1 INCREMENT 1;
