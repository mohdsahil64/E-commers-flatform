DELETE FROM categories
 WHERE slug IN ('upperwear', 'bottomwear', 'innerwear', 'underwear', 'shoes');

DROP INDEX IF EXISTS ix_products_season;
ALTER TABLE products DROP COLUMN IF EXISTS season;
