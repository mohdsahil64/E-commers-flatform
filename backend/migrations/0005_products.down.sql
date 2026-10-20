DROP TABLE IF EXISTS seller_listings;
DROP TABLE IF EXISTS product_variant_attributes;
ALTER TABLE product_images DROP CONSTRAINT IF EXISTS fk_product_images_variant;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS products;
