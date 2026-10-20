// Package seed populates the database with development data so the application
// can be demonstrated immediately after setup. It is idempotent: re-running it
// will not create duplicates (inserts use ON CONFLICT / existence checks).
package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/auth"
)

// Default development credentials (printed at the end of the seed).
const (
	adminEmail    = "admin@carlitos.local"
	adminPassword = "Pass@123"

	sellerEmail    = "seller@carlitos.local"
	sellerPassword = "Pass@123"

	customerEmail    = "customer@carlitos.local"
	customerPassword = "Pass@123"
)

// Run executes the full development seed.
func Run(ctx context.Context, a *app.App) error {
	pool := a.DB.Pool
	cost := a.Config.JWT.BcryptCost
	a.Logger.Info("seed: starting")

	if err := seedRolesAndPermissions(ctx, pool); err != nil {
		return fmt.Errorf("roles/permissions: %w", err)
	}
	adminID, err := seedAdmin(ctx, pool, cost)
	if err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if err := seedTaxonomy(ctx, pool); err != nil {
		return fmt.Errorf("taxonomy: %w", err)
	}
	sellerID, err := seedSeller(ctx, pool, cost)
	if err != nil {
		return fmt.Errorf("seller: %w", err)
	}
	customerID, err := seedCustomer(ctx, pool, cost)
	if err != nil {
		return fmt.Errorf("customer: %w", err)
	}
	if err := seedProducts(ctx, pool, sellerID, adminID); err != nil {
		return fmt.Errorf("products: %w", err)
	}
	if err := seedMerchandising(ctx, pool); err != nil {
		return fmt.Errorf("merchandising: %w", err)
	}
	if err := seedPackaging(ctx, pool); err != nil {
		return fmt.Errorf("packaging: %w", err)
	}
	if err := seedSampleOrder(ctx, pool, customerID); err != nil {
		return fmt.Errorf("sample order: %w", err)
	}

	a.Logger.Info("seed: done")
	printCredentials(a)
	return nil
}

// seedRolesAndPermissions inserts all permissions and roles and links them via
// the RBAC matrix from the auth package (single source of truth).
func seedRolesAndPermissions(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, perm := range auth.AllPermissions() {
			if _, err := tx.Exec(ctx,
				`INSERT INTO permissions (code) VALUES ($1) ON CONFLICT (code) DO NOTHING`, perm); err != nil {
				return err
			}
		}
		// Roles (system roles).
		roleNames := map[string]string{
			auth.RoleSuperAdmin: "Super Admin", auth.RoleAdmin: "Admin",
			auth.RoleCatalogAdmin: "Catalog Admin", auth.RoleOrderAdmin: "Order Admin",
			auth.RoleSellerAdmin: "Seller Admin", auth.RoleFinanceAdmin: "Finance Admin",
			auth.RoleSupportAdmin: "Support Admin", auth.RoleSeller: "Seller",
			auth.RoleSellerStaff: "Seller Staff", auth.RoleCustomer: "Customer",
		}
		for code, name := range roleNames {
			if _, err := tx.Exec(ctx,
				`INSERT INTO roles (code, name, is_system) VALUES ($1,$2,TRUE)
				 ON CONFLICT (code) DO NOTHING`, code, name); err != nil {
				return err
			}
		}
		// Link role -> permissions from the matrix.
		for role, perms := range auth.RolePermissionMatrix() {
			for _, perm := range perms {
				if _, err := tx.Exec(ctx, `
					INSERT INTO role_permissions (role_id, permission_id)
					SELECT r.id, p.id FROM roles r, permissions p
					WHERE r.code = $1 AND p.code = $2
					ON CONFLICT DO NOTHING`, role, perm); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// seedAdmin creates the SUPER_ADMIN user (admin provisioning is not public).
func seedAdmin(ctx context.Context, pool *pgxpool.Pool, cost int) (string, error) {
	hash, err := auth.HashPassword(adminPassword, cost)
	if err != nil {
		return "", err
	}
	var userID string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO users (account_type, email, password_hash, full_name, status, email_verified)
			VALUES ('admin',$1,$2,'CARLITOS Admin','active',TRUE)
			ON CONFLICT (account_type, email) WHERE email IS NOT NULL
			DO UPDATE SET full_name = EXCLUDED.full_name, password_hash = EXCLUDED.password_hash, status = 'active'
			RETURNING id`, adminEmail, hash).Scan(&userID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE code = $2 ON CONFLICT DO NOTHING`,
			userID, auth.RoleSuperAdmin)
		return err
	})
	return userID, err
}

// seedTaxonomy creates genders, categories, subcategories, brands and attributes.
func seedTaxonomy(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Genders.
		genders := []struct{ code, name string; sort int }{
			{"men", "Men", 1}, {"women", "Women", 2}, {"boys", "Boys", 3}, {"girls", "Girls", 4},
		}
		for _, g := range genders {
			if _, err := tx.Exec(ctx,
				`INSERT INTO genders (code, name, sort_order) VALUES ($1,$2,$3)
				 ON CONFLICT (code) DO NOTHING`, g.code, g.name, g.sort); err != nil {
				return err
			}
		}

		// Top categories (per gender) + a couple of subcategories.
		type cat struct {
			gender, name, slug string
			subs               []struct{ name, slug string }
		}
		cats := []cat{
			{"men", "Topwear", "men-topwear", []struct{ name, slug string }{
				{"T-Shirts", "men-tshirts"}, {"Shirts", "men-shirts"}}},
			{"men", "Bottomwear", "men-bottomwear", []struct{ name, slug string }{
				{"Jeans", "men-jeans"}, {"Trousers", "men-trousers"}}},
			{"women", "Ethnic Wear", "women-ethnic", []struct{ name, slug string }{
				{"Kurtas", "women-kurtas"}, {"Sarees", "women-sarees"}}},
			{"women", "Western Wear", "women-western", []struct{ name, slug string }{
				{"Dresses", "women-dresses"}, {"Tops", "women-tops"}}},
			{"boys", "Clothing", "boys-clothing", []struct{ name, slug string }{
				{"T-Shirts", "boys-tshirts"}}},
			{"girls", "Clothing", "girls-clothing", []struct{ name, slug string }{
				{"Dresses", "girls-dresses"}}},
		}
		for _, c := range cats {
			var parentID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO categories (gender_id, name, slug, level)
				VALUES ((SELECT id FROM genders WHERE code=$1),$2,$3,0)
				ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
				RETURNING id`, c.gender, c.name, c.slug).Scan(&parentID); err != nil {
				return err
			}
			for _, s := range c.subs {
				if _, err := tx.Exec(ctx, `
					INSERT INTO categories (parent_id, gender_id, name, slug, level)
					VALUES ($1,(SELECT id FROM genders WHERE code=$2),$3,$4,1)
					ON CONFLICT (slug) DO NOTHING`, parentID, c.gender, s.name, s.slug); err != nil {
					return err
				}
			}
		}

		// Brands.
		brands := []struct{ name, slug string }{
			{"Carlitos Basics", "carlitos-basics"}, {"Urban Thread", "urban-thread"},
			{"Nova Denim", "nova-denim"}, {"Ethnika", "ethnika"},
		}
		for _, b := range brands {
			if _, err := tx.Exec(ctx,
				`INSERT INTO brands (name, slug) VALUES ($1,$2) ON CONFLICT (slug) DO NOTHING`,
				b.name, b.slug); err != nil {
				return err
			}
		}

		// Attributes: Size + Color (variant-defining, filterable).
		if _, err := tx.Exec(ctx, `
			INSERT INTO attributes (code, name, input_type, is_variant, is_filterable, sort_order)
			VALUES ('size','Size','select',TRUE,TRUE,1)
			ON CONFLICT (code) DO NOTHING`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO attributes (code, name, input_type, is_variant, is_filterable, sort_order)
			VALUES ('color','Color','color',TRUE,TRUE,2)
			ON CONFLICT (code) DO NOTHING`); err != nil {
			return err
		}
		sizes := []string{"XS", "S", "M", "L", "XL", "XXL"}
		for i, s := range sizes {
			if _, err := tx.Exec(ctx, `
				INSERT INTO attribute_values (attribute_id, value, display_value, sort_order)
				VALUES ((SELECT id FROM attributes WHERE code='size'),$1,$1,$2)
				ON CONFLICT (attribute_id, value) DO NOTHING`, s, i); err != nil {
				return err
			}
		}
		colors := []struct{ v, hex string }{
			{"Black", "#111111"}, {"White", "#ffffff"}, {"Navy", "#1f2a44"},
			{"Beige", "#e9e0d2"}, {"Olive", "#5b613f"},
		}
		for i, c := range colors {
			if _, err := tx.Exec(ctx, `
				INSERT INTO attribute_values (attribute_id, value, display_value, meta, sort_order)
				VALUES ((SELECT id FROM attributes WHERE code='color'),$1,$1,$2::jsonb,$3)
				ON CONFLICT (attribute_id, value) DO NOTHING`,
				c.v, fmt.Sprintf(`{"hex":"%s"}`, c.hex), i); err != nil {
				return err
			}
		}
		return nil
	})
}

// seedSeller creates an approved seller with a pickup address and completed
// onboarding, so products can be listed immediately.
func seedSeller(ctx context.Context, pool *pgxpool.Pool, cost int) (string, error) {
	hash, err := auth.HashPassword(sellerPassword, cost)
	if err != nil {
		return "", err
	}
	var sellerID string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (account_type, email, phone, password_hash, full_name, status, email_verified)
			VALUES ('seller',$1,'9800000001',$2,'Riya Sharma','active',TRUE)
			ON CONFLICT (account_type, email) WHERE email IS NOT NULL
			DO UPDATE SET full_name = EXCLUDED.full_name, password_hash = EXCLUDED.password_hash, status = 'active'
			RETURNING id`, sellerEmail, hash).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE code=$2 ON CONFLICT DO NOTHING`,
			userID, auth.RoleSeller); err != nil {
			return err
		}
		// Seller store (approved).
		err := tx.QueryRow(ctx, `
			INSERT INTO sellers (user_id, store_name, store_slug, store_description, status,
				legal_business_name, business_type, owner_name, contact_email, contact_phone,
				gst_number, pan_number, commission_percent, agreement_accepted, agreement_accepted_at, approved_at)
			VALUES ($1,'Thread & Co.','thread-and-co','Premium everyday fashion essentials.','approved',
				'Thread and Co Pvt Ltd','pvt_ltd','Riya Sharma',$2,'9800000001',
				'27AAAAA0000A1Z5','AAAAA0000A',10.0,TRUE,now(),now())
			ON CONFLICT (user_id) DO UPDATE SET status='approved'
			RETURNING id`, userID, sellerEmail).Scan(&sellerID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_onboarding (seller_id, step, business_completed, contact_completed,
				pickup_completed, bank_completed, documents_completed, submitted_at, reviewed_at)
			VALUES ($1,'submitted',TRUE,TRUE,TRUE,TRUE,TRUE,now(),now())
			ON CONFLICT (seller_id) DO NOTHING`, sellerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO seller_pickup_addresses (seller_id, contact_name, contact_phone, line1,
				city, state, pincode, is_default)
			SELECT $1,'Riya Sharma','9800000001','12 Fashion Street','Mumbai','Maharashtra','400001',TRUE
			WHERE NOT EXISTS (SELECT 1 FROM seller_pickup_addresses WHERE seller_id=$1)`, sellerID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO seller_bank_accounts (seller_id, account_holder, account_number, ifsc, bank_name, verified)
			SELECT $1,'Thread and Co Pvt Ltd','000123456789','HDFC0000001','HDFC Bank',TRUE
			WHERE NOT EXISTS (SELECT 1 FROM seller_bank_accounts WHERE seller_id=$1)`, sellerID)
		return err
	})
	return sellerID, err
}

// seedCustomer creates a customer with a default address, cart and wishlist.
func seedCustomer(ctx context.Context, pool *pgxpool.Pool, cost int) (string, error) {
	hash, err := auth.HashPassword(customerPassword, cost)
	if err != nil {
		return "", err
	}
	var customerID string
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (account_type, email, phone, password_hash, full_name, status, email_verified)
			VALUES ('customer',$1,'9900000001',$2,'Aarav Mehta','active',TRUE)
			ON CONFLICT (account_type, phone) WHERE phone IS NOT NULL AND phone <> ''
			DO UPDATE SET full_name = EXCLUDED.full_name, password_hash = EXCLUDED.password_hash, status = 'active'
			RETURNING id`, customerEmail, hash).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE code=$2 ON CONFLICT DO NOTHING`,
			userID, auth.RoleCustomer); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO customers (user_id, display_name, gender)
			VALUES ($1,'Aarav Mehta','male')
			ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
			RETURNING id`, userID).Scan(&customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO carts (customer_id) VALUES ($1) ON CONFLICT (customer_id) DO NOTHING`, customerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO wishlists (customer_id) VALUES ($1) ON CONFLICT (customer_id) DO NOTHING`, customerID); err != nil {
			return err
		}
		// Default address.
		var addrExists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customer_addresses WHERE customer_id=$1)`, customerID).Scan(&addrExists)
		if !addrExists {
			var addrID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO customer_addresses (customer_id, label, contact_name, contact_phone,
					line1, city, state, pincode, is_default)
				VALUES ($1,'Home','Aarav Mehta','9900000001','88 Residency Road','Bengaluru','Karnataka','560025',TRUE)
				RETURNING id`, customerID).Scan(&addrID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE customers SET default_address_id=$2 WHERE id=$1`, customerID, addrID); err != nil {
				return err
			}
		}
		return nil
	})
	return customerID, err
}

// seedProducts creates approved products with per-size variants, seller
// listings and inventory. It is idempotent AND self-healing: for products that
// already exist (e.g. seeded before size support), it ensures each size variant
// exists so old products show sizes too.
func seedProducts(ctx context.Context, pool *pgxpool.Pool, sellerID, adminID string) error {
	products := []struct {
		title, slug, brandSlug, gender, catSlug, subSlug string
		mrp, price                                       float64
		sizes                                            []struct {
			size  string
			stock int
		}
	}{
		{"Classic Cotton T-Shirt", "classic-cotton-tshirt", "carlitos-basics", "men", "men-topwear", "men-tshirts", 999, 649,
			[]struct{ size string; stock int }{{"S", 12}, {"M", 20}, {"L", 15}, {"XL", 8}}},
		{"Slim Fit Denim Jeans", "slim-fit-denim-jeans", "nova-denim", "men", "men-bottomwear", "men-jeans", 2999, 1799,
			[]struct{ size string; stock int }{{"30", 6}, {"32", 10}, {"34", 7}, {"36", 4}}},
		{"Floral Summer Dress", "floral-summer-dress", "urban-thread", "women", "women-western", "women-dresses", 2499, 1499,
			[]struct{ size string; stock int }{{"S", 10}, {"M", 12}, {"L", 8}}},
		{"Handloom Cotton Kurta", "handloom-cotton-kurta", "ethnika", "women", "women-ethnic", "women-kurtas", 1999, 1299,
			[]struct{ size string; stock int }{{"M", 9}, {"L", 7}, {"XL", 4}}},
		{"Kids Graphic Tee", "kids-graphic-tee", "carlitos-basics", "boys", "boys-clothing", "boys-tshirts", 799, 499,
			[]struct{ size string; stock int }{{"XS", 20}, {"S", 18}, {"M", 12}}},
	}

	for _, p := range products {
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			// Find or create the product.
			var productID string
			err := tx.QueryRow(ctx, `SELECT id FROM products WHERE slug=$1`, p.slug).Scan(&productID)
			if err == pgx.ErrNoRows {
				if err := tx.QueryRow(ctx, `
					INSERT INTO products (created_by_seller, title, slug, description, brand_id, gender_id,
						category_id, subcategory_id, status, tax_percent, approved_by, approved_at, published_at)
					VALUES ($1,$2,$3,$4,
						(SELECT id FROM brands WHERE slug=$5),
						(SELECT id FROM genders WHERE code=$6),
						(SELECT id FROM categories WHERE slug=$7),
						(SELECT id FROM categories WHERE slug=$8),
						'approved',5.0,$9,now(),now())
					RETURNING id`,
					sellerID, p.title, p.slug, p.title+" — premium quality, everyday comfort.",
					p.brandSlug, p.gender, p.catSlug, p.subSlug, adminID).Scan(&productID); err != nil {
					return err
				}
				// Primary image placeholder key (only for new products).
				if _, err := tx.Exec(ctx, `
					INSERT INTO product_images (product_id, file_key, alt_text, sort_order, is_primary)
					VALUES ($1,$2,$3,0,TRUE)`,
					productID, "seed/products/"+p.slug+".jpg", p.title); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}

			// Clean up the legacy single "M-DEFAULT" variant (and its listing/
			// inventory) if present, so it doesn't show as a stray size. Safe
			// only when it has no orders referencing its listing.
			if _, err := tx.Exec(ctx, `
				DELETE FROM seller_listings sl
				USING product_variants pv
				WHERE pv.id = sl.variant_id
				  AND pv.product_id = $1
				  AND pv.variant_code = 'M-DEFAULT'
				  AND NOT EXISTS (SELECT 1 FROM order_items oi WHERE oi.seller_listing_id = sl.id)`,
				productID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				DELETE FROM product_variants pv
				WHERE pv.product_id = $1 AND pv.variant_code = 'M-DEFAULT'
				  AND NOT EXISTS (SELECT 1 FROM seller_listings sl WHERE sl.variant_id = pv.id)`,
				productID); err != nil {
				return err
			}

			discount := round1((p.mrp - p.price) / p.mrp * 100)
			// Ensure a variant + listing + inventory per size (create if missing).
			for _, s := range p.sizes {
				var variantID string
				err := tx.QueryRow(ctx,
					`SELECT id FROM product_variants WHERE product_id=$1 AND variant_code=$2`,
					productID, s.size).Scan(&variantID)
				if err == pgx.ErrNoRows {
					if err := tx.QueryRow(ctx, `
						INSERT INTO product_variants (product_id, variant_code, mrp, selling_price, weight_grams)
						VALUES ($1,$2,$3,$4,300) RETURNING id`,
						productID, s.size, p.mrp, p.price).Scan(&variantID); err != nil {
						return err
					}
					// Link the size attribute value (auto-create the value if needed).
					if _, err := tx.Exec(ctx, `
						INSERT INTO attribute_values (attribute_id, value, display_value)
						SELECT (SELECT id FROM attributes WHERE code='size'), $1, $1
						ON CONFLICT (attribute_id, value) DO NOTHING`, s.size); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `
						INSERT INTO product_variant_attributes (variant_id, attribute_id, attribute_value_id)
						SELECT $1, a.id, av.id FROM attributes a
						JOIN attribute_values av ON av.attribute_id = a.id
						WHERE a.code='size' AND av.value=$2
						ON CONFLICT (variant_id, attribute_id) DO NOTHING`, variantID, s.size); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}

				// Ensure a seller listing for this variant.
				var listingID string
				err = tx.QueryRow(ctx,
					`SELECT id FROM seller_listings WHERE seller_id=$1 AND variant_id=$2`,
					sellerID, variantID).Scan(&listingID)
				if err == pgx.ErrNoRows {
					if err := tx.QueryRow(ctx, `
						INSERT INTO seller_listings (seller_id, product_id, variant_id, sku, mrp, selling_price, discount_percent, status)
						VALUES ($1,$2,$3,$4,$5,$6,$7,'active') RETURNING id`,
						sellerID, productID, variantID, "SKU-"+p.slug+"-"+s.size, p.mrp, p.price, discount).Scan(&listingID); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}

				// Ensure an inventory row for this listing (create if missing).
				var invExists bool
				if err := tx.QueryRow(ctx,
					`SELECT EXISTS(SELECT 1 FROM inventory WHERE seller_listing_id=$1)`, listingID).Scan(&invExists); err != nil {
					return err
				}
				if !invExists {
					if _, err := tx.Exec(ctx,
						`INSERT INTO inventory (seller_listing_id, available_quantity) VALUES ($1,$2)`,
						listingID, s.stock); err != nil {
						return err
					}
					if _, err := tx.Exec(ctx, `
						INSERT INTO inventory_transactions (seller_listing_id, change_type, quantity_delta, balance_after, reference_type)
						VALUES ($1,'restock',$2,$2,'seed')`, listingID, s.stock); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// seedMerchandising creates collections (with some products) and hero banners
// with data-driven targets.
func seedMerchandising(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Collections.
		collections := []struct{ name, slug, kind, typ string }{
			{"New Arrivals", "new-arrivals", "new_arrivals", "manual"},
			{"Best Sellers", "best-sellers", "best_sellers", "manual"},
			{"Summer Edit", "summer-edit", "seasonal", "manual"},
		}
		for _, c := range collections {
			var colID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO collections (name, slug, kind, type, is_active)
				VALUES ($1,$2,$3,$4,TRUE)
				ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
				RETURNING id`, c.name, c.slug, c.kind, c.typ).Scan(&colID); err != nil {
				return err
			}
			// Attach up to 4 approved products.
			if _, err := tx.Exec(ctx, `
				INSERT INTO collection_products (collection_id, product_id, sort_order)
				SELECT $1, p.id, row_number() OVER (ORDER BY p.created_at)
				FROM products p WHERE p.status='approved'
				ON CONFLICT DO NOTHING`, colID); err != nil {
				return err
			}
		}

		// Hero banner: "Summer Collection" -> Women / Dresses / in stock.
		var exists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM banners WHERE title='Summer Collection')`).Scan(&exists)
		if !exists {
			var bannerID string
			if err := tx.QueryRow(ctx, `
				INSERT INTO banners (title, subtitle, image_key, placement, destination_type, sort_order, is_active)
				VALUES ('Summer Collection','Breezy styles for warm days','seed/banners/summer.jpg','home_hero','filter',1,TRUE)
				RETURNING id`).Scan(&bannerID); err != nil {
				return err
			}
			targets := map[string]string{"gender": "women", "category": "women-dresses", "availability": "in_stock"}
			for k, v := range targets {
				if _, err := tx.Exec(ctx,
					`INSERT INTO banner_targets (banner_id, target_key, target_value) VALUES ($1,$2,$3)`,
					bannerID, k, v); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// seedPackaging creates central packaging items with stock.
func seedPackaging(ctx context.Context, pool *pgxpool.Pool) error {
	items := []struct {
		sku, name, typ string
		stock          int
	}{
		{"PKG-POLY-S", "Polybag (Small)", "polybag", 1000},
		{"PKG-BOX-M", "Corrugated Box (Medium)", "box", 500},
		{"PKG-TAPE", "Packing Tape Roll", "tape", 200},
		{"PKG-LABEL", "Shipping Label Sheet", "shipping_label", 800},
	}
	for _, it := range items {
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			var id string
			var existed bool
			_ = tx.QueryRow(ctx, `SELECT id, TRUE FROM packaging_items WHERE sku=$1`, it.sku).Scan(&id, &existed)
			if existed {
				return nil
			}
			if err := tx.QueryRow(ctx, `
				INSERT INTO packaging_items (sku, name, type) VALUES ($1,$2,$3) RETURNING id`,
				it.sku, it.name, it.typ).Scan(&id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO packaging_inventory (packaging_item_id, available_quantity) VALUES ($1,$2)`,
				id, it.stock); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO packaging_ledger (packaging_item_id, entry_type, quantity_delta, reference_type)
				VALUES ($1,'stock_in',$2,'seed')`, id, it.stock)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// seedSampleOrder creates one delivered sample order for the customer so the
// dashboards, order history and reviews have data. It reserves + commits stock
// through the normal columns.
func seedSampleOrder(ctx context.Context, pool *pgxpool.Pool, customerID string) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var exists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE customer_id=$1)`, customerID).Scan(&exists)
		if exists {
			return nil
		}
		// Pick one active listing.
		var listingID, sellerID, title, sku string
		var price, taxPct, commission float64
		err := tx.QueryRow(ctx, `
			SELECT sl.id, sl.seller_id, p.title, sl.sku, sl.selling_price, p.tax_percent,
			       COALESCE(s.commission_percent,10)
			FROM seller_listings sl
			JOIN products p ON p.id = sl.product_id
			JOIN sellers s ON s.id = sl.seller_id
			WHERE sl.status='active' LIMIT 1`).
			Scan(&listingID, &sellerID, &title, &sku, &price, &taxPct, &commission)
		if err != nil {
			return err // no listings yet
		}

		// Address snapshot.
		var name, phone, line1, city, state, pincode string
		if err := tx.QueryRow(ctx, `
			SELECT contact_name, contact_phone, line1, city, state, pincode
			FROM customer_addresses WHERE customer_id=$1 AND is_default=TRUE LIMIT 1`, customerID).
			Scan(&name, &phone, &line1, &city, &state, &pincode); err != nil {
			return err
		}

		qty := 1
		gross := price * float64(qty)
		tax := round2(gross * taxPct / 100)
		commAmt := round2(gross * commission / 100)
		grand := round2(gross + tax)
		payable := round2(gross - commAmt)

		var orderID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO orders (order_number, customer_id, status, payment_method, payment_status,
				items_subtotal, tax_total, grand_total, ship_contact_name, ship_contact_phone,
				ship_line1, ship_city, ship_state, ship_pincode, placed_at)
			VALUES ($1,$2,'delivered','cod','pending',$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
			RETURNING id`,
			"CRL-SEED-000001", customerID, gross, tax, grand,
			name, phone, line1, city, state, pincode).Scan(&orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, seller_id, seller_listing_id, product_title, sku,
				item_status, quantity, unit_price, gross_amount, tax_amount, commission_percent,
				commission_amount, seller_payable)
			VALUES ($1,$2,$3,$4,$5,'delivered',$6,$7,$8,$9,$10,$11,$12)`,
			orderID, sellerID, listingID, title, sku, qty, price, gross, tax, commission, commAmt, payable); err != nil {
			return err
		}
		// Reflect the sale in inventory (available -1).
		if _, err := tx.Exec(ctx, `
			UPDATE inventory SET available_quantity = GREATEST(available_quantity - $2, 0)
			WHERE seller_listing_id=$1`, listingID, qty); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,'','delivered','system','Seed sample order')`, orderID)
		return err
	})
}

func printCredentials(a *app.App) {
	a.Logger.Info("seed credentials",
		"admin", adminEmail+" / "+adminPassword,
		"seller", sellerEmail+" / "+sellerPassword,
		"customer", customerEmail+" / "+customerPassword)
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }
