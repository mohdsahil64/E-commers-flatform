package catalog

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/inventory"
)

// AuditHook lets the admin approval flow record audit entries without the
// catalog package depending on the audit package. actorID is the admin user.
type AuditHook func(ctx context.Context, actorID, action, productID string)

// Service composes catalog repositories and enforces business rules for
// product creation, submission and approval.
type Service struct {
	tax   *TaxonomyRepository
	prod  *ProductRepository
	store *StorefrontRepository
	inv   *inventory.Repository
	audit AuditHook
}

// NewService builds a catalog Service.
func NewService(tax *TaxonomyRepository, prod *ProductRepository, store *StorefrontRepository, inv *inventory.Repository) *Service {
	return &Service{tax: tax, prod: prod, store: store, inv: inv, audit: func(context.Context, string, string, string) {}}
}

// WithAuditHook attaches an audit hook (fluent; returns the service).
func (s *Service) WithAuditHook(h AuditHook) *Service {
	if h != nil {
		s.audit = h
	}
	return s
}

// ---- taxonomy passthroughs ----

func (s *Service) Genders(ctx context.Context) ([]Gender, error) { return s.tax.ListGenders(ctx) }
func (s *Service) Brands(ctx context.Context) ([]Brand, error)   { return s.tax.ListBrands(ctx) }
func (s *Service) Attributes(ctx context.Context) ([]Attribute, error) {
	return s.tax.ListAttributes(ctx)
}

// CategoryTree returns categories as a nested tree.
func (s *Service) CategoryTree(ctx context.Context) ([]Category, error) {
	flat, err := s.tax.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return BuildTree(flat), nil
}

// ---- storefront ----

// Search runs the storefront listing query.
func (s *Service) Search(ctx context.Context, f ListingFilter) ([]ListingCard, int64, error) {
	return s.store.Search(ctx, f)
}

// ProductBySlug returns an approved product for the storefront.
func (s *Service) ProductBySlug(ctx context.Context, slug string) (*Product, error) {
	p, err := s.prod.GetProductBySlug(ctx, slug)
	if err == ErrNotFound {
		return nil, httpx.NotFound("product_not_found", "Product not found")
	}
	return p, err
}

// ---- seller catalog ----

// CreateProductRequest is the seller-facing product creation payload including
// a single default variant + listing (more variants can be added later).
type CreateProductRequest struct {
	SellerID      string
	Title         string
	Description   string
	BrandID       *string
	GenderID      *string
	CategoryID    *string
	SubcategoryID *string
	HSNCode       string
	TaxPercent    float64
	SKU           string
	MRP           float64
	SellingPrice  float64
	Discount      float64
	InitialStock  int
	VariantCode   string
	Barcode       *string
	Season        string
	CategorySlug  string // optional: resolved to CategoryID if provided
	BrandName     string // optional: free-text brand, resolved/created to BrandID
	Images        []ImageInput
	// Sizes, when provided, creates one variant + listing + inventory per size
	// (e.g. S/M/L for clothing, 6/7/8 for shoes). Falls back to a single
	// default variant with InitialStock when empty.
	Sizes []SizeStockInput
}

// SizeStockInput is a single size and its starting stock.
type SizeStockInput struct {
	Size     string
	Quantity int
}

// CreateProduct creates a product with one variant + seller listing + inventory
// row per size (or a single default variant), atomically, in 'draft' status.
func (s *Service) CreateProduct(ctx context.Context, req CreateProductRequest) (string, error) {
	if strings.TrimSpace(req.Title) == "" {
		return "", httpx.BadRequest("validation_failed", "Title is required")
	}
	if req.SellingPrice <= 0 {
		return "", httpx.BadRequest("validation_failed", "Selling price must be greater than zero")
	}
	// Resolve a category slug (e.g. "shoes") to its id, creating it if needed.
	if req.CategoryID == nil && req.CategorySlug != "" {
		cid, err := s.prod.ResolveCategoryBySlug(ctx, req.CategorySlug)
		if err != nil {
			return "", err
		}
		if cid != "" {
			req.CategoryID = &cid
		}
	}
	// Resolve a free-text brand name to its id, creating the brand if needed.
	if req.BrandID == nil && req.BrandName != "" {
		bid, err := s.prod.ResolveBrandByName(ctx, req.BrandName)
		if err != nil {
			return "", err
		}
		if bid != "" {
			req.BrandID = &bid
		}
	}
	slug := Slugify(req.Title)
	variantCode := req.VariantCode
	if variantCode == "" {
		variantCode = "DEFAULT"
	}

	var productID string
	err := pgx.BeginFunc(ctx, s.prod.Pool(), func(tx pgx.Tx) error {
		pid, err := s.prod.CreateProduct(ctx, tx, CreateProductInput{
			SellerID: req.SellerID, Title: req.Title, Slug: slug + "-" + shortID(),
			Description: req.Description, BrandID: req.BrandID, GenderID: req.GenderID,
			CategoryID: req.CategoryID, SubcategoryID: req.SubcategoryID,
			HSNCode: req.HSNCode, TaxPercent: req.TaxPercent, Season: req.Season,
		})
		if err != nil {
			return err
		}
		productID = pid

		// Attach uploaded images (first one becomes primary).
		if len(req.Images) > 0 {
			if err := s.prod.ReplaceImages(ctx, tx, pid, req.Images); err != nil {
				return err
			}
		}

		// makeVariant creates one variant + listing + inventory row for a given
		// size label (empty label = a single default variant).
		makeVariant := func(sizeAttrID, sizeLabel string, qty int) error {
			code := sizeLabel
			if code == "" {
				code = variantCode
			}
			vid, err := s.prod.CreateVariant(ctx, tx, pid, code, req.MRP, req.SellingPrice, req.Barcode, 0)
			if err != nil {
				return err
			}
			// Link the size attribute value to the variant.
			if sizeLabel != "" && sizeAttrID != "" {
				valID, err := s.prod.ResolveSizeValueID(ctx, tx, sizeAttrID, sizeLabel)
				if err != nil {
					return err
				}
				if valID != "" {
					if err := s.prod.SetVariantAttribute(ctx, tx, vid, sizeAttrID, valID); err != nil {
						return err
					}
				}
			}
			sku := strings.ToUpper(code) + "-" + shortID()
			listingID, err := s.prod.CreateSellerListing(ctx, tx, SellerListing{
				SellerID: req.SellerID, ProductID: pid, VariantID: vid, SKU: sku,
				MRP: req.MRP, SellingPrice: req.SellingPrice, DiscountPercent: req.Discount,
			})
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO inventory (seller_listing_id, available_quantity) VALUES ($1,$2)`,
				listingID, qty); err != nil {
				return err
			}
			if qty > 0 {
				if _, err := tx.Exec(ctx, `
					INSERT INTO inventory_transactions
						(seller_listing_id, change_type, quantity_delta, balance_after, reference_type)
					VALUES ($1,'restock',$2,$2,'initial')`, listingID, qty); err != nil {
					return err
				}
			}
			return nil
		}

		if len(req.Sizes) > 0 {
			sizeAttrID, err := s.prod.SizeAttributeID(ctx, tx)
			if err != nil {
				return err
			}
			for _, sz := range req.Sizes {
				label := strings.TrimSpace(sz.Size)
				if label == "" {
					continue
				}
				if err := makeVariant(sizeAttrID, label, sz.Quantity); err != nil {
					return err
				}
			}
			return nil
		}
		// No sizes: single default variant with the scalar InitialStock.
		return makeVariant("", "", req.InitialStock)
	})
	if err != nil {
		return "", err
	}
	return productID, nil
}

// UpdateProductRequest is the seller-facing product edit payload.
type UpdateProductRequest struct {
	ProductID    string
	SellerID     string
	Title        string
	Description  string
	BrandID      *string
	GenderID     *string
	CategoryID   *string
	CategorySlug string // optional: resolved to CategoryID if provided
	BrandName    string // optional: free-text brand, resolved/created to BrandID
	Season       string
	MRP          float64
	SellingPrice float64
	Discount     float64
	// Stock is the absolute available quantity to set (nil = leave unchanged).
	Stock  *int
	Images []ImageInput
}

// UpdateProduct edits an existing seller-owned product: core fields, price on
// the primary listing, images, and (optionally) the absolute stock level.
// Everything runs in one transaction. Approved products are moved back to
// 'submitted' so admin re-reviews material changes.
func (s *Service) UpdateProduct(ctx context.Context, req UpdateProductRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return httpx.BadRequest("validation_failed", "Title is required")
	}
	if req.SellingPrice <= 0 {
		return httpx.BadRequest("validation_failed", "Selling price must be greater than zero")
	}
	if req.Stock != nil && *req.Stock < 0 {
		return httpx.BadRequest("validation_failed", "Stock cannot be negative")
	}
	if req.CategoryID == nil && req.CategorySlug != "" {
		cid, err := s.prod.ResolveCategoryBySlug(ctx, req.CategorySlug)
		if err != nil {
			return err
		}
		if cid != "" {
			req.CategoryID = &cid
		}
	}
	if req.BrandID == nil && req.BrandName != "" {
		bid, err := s.prod.ResolveBrandByName(ctx, req.BrandName)
		if err != nil {
			return err
		}
		if bid != "" {
			req.BrandID = &bid
		}
	}

	err := pgx.BeginFunc(ctx, s.prod.Pool(), func(tx pgx.Tx) error {
		if err := s.prod.UpdateSellerProduct(ctx, tx, req.ProductID, req.SellerID, UpdateProductInput{
			Title: req.Title, Description: req.Description, BrandID: req.BrandID,
			GenderID: req.GenderID, CategoryID: req.CategoryID, Season: req.Season,
		}); err != nil {
			return err
		}
		if err := s.prod.UpdateListingPrice(ctx, tx, req.ProductID, req.SellerID, req.MRP, req.SellingPrice, req.Discount); err != nil {
			return err
		}
		// Images: only replace when a set is provided, so an edit that doesn't
		// touch images keeps the existing ones.
		if req.Images != nil {
			if err := s.prod.ReplaceImages(ctx, tx, req.ProductID, req.Images); err != nil {
				return err
			}
		}
		// Stock: set the absolute available quantity + ledger entry.
		if req.Stock != nil {
			listingID, err := s.prod.PrimaryListingID(ctx, req.ProductID, req.SellerID)
			if err != nil {
				return err
			}
			var current int
			if err := tx.QueryRow(ctx,
				`SELECT available_quantity FROM inventory WHERE seller_listing_id = $1 FOR UPDATE`,
				listingID).Scan(&current); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE inventory SET available_quantity = $2 WHERE seller_listing_id = $1`,
				listingID, *req.Stock); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_transactions
					(seller_listing_id, change_type, quantity_delta, balance_after, reference_type)
				VALUES ($1,'adjustment',$2,$3,'manual')`,
				listingID, *req.Stock-current, *req.Stock); err != nil {
				return err
			}
		}
		return nil
	})
	if err == ErrNotFound {
		return httpx.NotFound("product_not_found", "Product not found")
	}
	return err
}

// SetStock sets the absolute available quantity for one of a seller's product
// listings and records a ledger entry. If listingID is empty it targets the
// product's primary (first) listing; otherwise it updates that specific size's
// listing (ownership verified). Lightweight path for the inventory popup.
func (s *Service) SetStock(ctx context.Context, productID, sellerID, listingID string, qty int) error {
	if qty < 0 {
		return httpx.BadRequest("validation_failed", "Stock cannot be negative")
	}
	err := pgx.BeginFunc(ctx, s.prod.Pool(), func(tx pgx.Tx) error {
		var err error
		if listingID != "" {
			// Verify the listing belongs to this seller + product (no IDOR).
			if err = s.prod.VerifyListingOwner(ctx, listingID, productID, sellerID); err != nil {
				return err
			}
		} else {
			listingID, err = s.prod.PrimaryListingID(ctx, productID, sellerID)
			if err != nil {
				return err
			}
		}
		var current int
		if err := tx.QueryRow(ctx,
			`SELECT available_quantity FROM inventory WHERE seller_listing_id = $1 FOR UPDATE`,
			listingID).Scan(&current); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE inventory SET available_quantity = $2 WHERE seller_listing_id = $1`,
			listingID, qty); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO inventory_transactions
				(seller_listing_id, change_type, quantity_delta, balance_after, reference_type)
			VALUES ($1,'adjustment',$2,$3,'manual')`,
			listingID, qty-current, qty)
		return err
	})
	if err == ErrNotFound {
		return httpx.NotFound("product_not_found", "Product not found")
	}
	return err
}

// SubmitForReview submits a seller's product for admin approval.
func (s *Service) SubmitForReview(ctx context.Context, productID, sellerID string) error {
	if err := s.prod.SubmitForReview(ctx, productID, sellerID); err == ErrNotFound {
		return httpx.NotFound("product_not_found", "Product not found or not in a submittable state")
	} else if err != nil {
		return err
	}
	return nil
}

// ListSellerProducts lists a seller's products.
func (s *Service) ListSellerProducts(ctx context.Context, sellerID, status string, limit, offset int) ([]Product, int64, error) {
	return s.prod.ListSellerProducts(ctx, sellerID, status, limit, offset)
}

// GetProduct returns a product by id (admin/internal — unscoped).
func (s *Service) GetProduct(ctx context.Context, id string) (*Product, error) {
	p, err := s.prod.GetProduct(ctx, id)
	if err == ErrNotFound {
		return nil, httpx.NotFound("product_not_found", "Product not found")
	}
	return p, err
}

// GetProductForSeller returns a product only if the seller owns it.
func (s *Service) GetProductForSeller(ctx context.Context, id, sellerID string) (*Product, error) {
	p, err := s.prod.GetProductForSeller(ctx, id, sellerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("product_not_found", "Product not found")
	}
	return p, err
}

// ---- admin ----

// ListForApproval lists products for admin, by status.
func (s *Service) ListForApproval(ctx context.Context, status string, limit, offset int) ([]Product, int64, error) {
	return s.prod.ListForAdmin(ctx, status, limit, offset)
}

// ApproveProduct approves a submitted product.
func (s *Service) ApproveProduct(ctx context.Context, productID, adminID string) error {
	if err := s.prod.SetApprovalStatus(ctx, productID, "approved", "", adminID); err == ErrNotFound {
		return httpx.NotFound("product_not_found", "Product not found")
	} else if err != nil {
		return err
	}
	s.audit(ctx, adminID, "product.approved", productID)
	return nil
}

// RejectProduct rejects a product with a reason.
func (s *Service) RejectProduct(ctx context.Context, productID, reason, adminID string) error {
	if strings.TrimSpace(reason) == "" {
		return httpx.BadRequest("validation_failed", "Rejection reason is required")
	}
	if err := s.prod.SetApprovalStatus(ctx, productID, "rejected", reason, adminID); err == ErrNotFound {
		return httpx.NotFound("product_not_found", "Product not found")
	} else if err != nil {
		return err
	}
	s.audit(ctx, adminID, "product.rejected", productID)
	return nil
}
