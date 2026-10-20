package catalog

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes catalog endpoints across the three portals.
type Handler struct {
	svc         *Service
	sellerLookup *sellers.Repository // to resolve seller_id from user_id
}

// NewHandler builds a catalog Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterPublic mounts unauthenticated storefront routes.
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/genders", h.listGenders)
	r.Get("/categories", h.categoryTree)
	r.Get("/brands", h.listBrands)
	r.Get("/attributes", h.listAttributes)
	r.Get("/products", h.search)
	r.Get("/products/:slug", h.productBySlug)
}

// RegisterSeller mounts seller catalog routes (auth + seller portal enforced by caller).
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/products", h.sellerListProducts)
	r.Post("/products", h.sellerCreateProduct)
	r.Get("/products/:id", h.sellerGetProduct)
	r.Put("/products/:id", h.sellerUpdateProduct)
	r.Patch("/products/:id/stock", h.sellerSetStock)
	r.Post("/products/:id/submit", h.sellerSubmitProduct)
}

// RegisterAdmin mounts admin catalog + approval routes.
func (h *Handler) RegisterAdmin(r fiber.Router, manager *auth.Manager) {
	r.Get("/products", auth.RequirePermission(auth.PermProductView), h.adminListProducts)
	r.Get("/products/:id", auth.RequirePermission(auth.PermProductView), h.adminGetProduct)
	r.Post("/products/:id/approve", auth.RequirePermission(auth.PermProductApprove), h.adminApprove)
	r.Post("/products/:id/reject", auth.RequirePermission(auth.PermProductReject), h.adminReject)
	r.Post("/categories", auth.RequirePermission(auth.PermCategoryManage), h.adminCreateCategory)
}

// ---- public handlers ----

func (h *Handler) listGenders(c *fiber.Ctx) error {
	items, err := h.svc.Genders(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) categoryTree(c *fiber.Ctx) error {
	tree, err := h.svc.CategoryTree(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, tree)
}

func (h *Handler) listBrands(c *fiber.Ctx) error {
	items, err := h.svc.Brands(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) listAttributes(c *fiber.Ctx) error {
	items, err := h.svc.Attributes(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) search(c *fiber.Ctx) error {
	page := queryInt(c, "page", 1)
	size := queryInt(c, "page_size", 24)
	if size <= 0 || size > 100 {
		size = 24
	}
	inStock := c.Query("in_stock")
	f := ListingFilter{
		Search:       c.Query("q"),
		GenderCode:   c.Query("gender"),
		CategorySlug: c.Query("category"),
		BrandSlug:    c.Query("brand"),
		SellerSlug:   c.Query("seller"),
		Season:       c.Query("season"),
		Sort:         c.Query("sort"),
		InStock:      inStock == "true" || inStock == "1",
		Limit:        size,
		Offset:       (page - 1) * size,
	}
	if v := c.Query("min_price"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil {
			f.MinPrice = &p
		}
	}
	if v := c.Query("max_price"); v != "" {
		if p, err := strconv.ParseFloat(v, 64); err == nil {
			f.MaxPrice = &p
		}
	}

	items, total, err := h.svc.Search(c.Context(), f)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{
		Page: page, PageSize: size, Total: total,
		TotalPages: totalPages(total, size),
	})
}

func (h *Handler) productBySlug(c *fiber.Ctx) error {
	p, err := h.svc.ProductBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return httpx.OK(c, p)
}

// ---- seller handlers ----

// productImageBody is a single image reference (storage key from /uploads).
type productImageBody struct {
	Key string `json:"key"`
	Alt string `json:"alt"`
}

type createProductBody struct {
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	BrandID       *string            `json:"brand_id"`
	BrandName     string             `json:"brand_name"`
	GenderID      *string            `json:"gender_id"`
	CategoryID    *string            `json:"category_id"`
	CategorySlug  string             `json:"category_slug"`
	SubcategoryID *string            `json:"subcategory_id"`
	HSNCode       string             `json:"hsn_code"`
	TaxPercent    float64            `json:"tax_percent"`
	Season        string             `json:"season"`
	SKU           string             `json:"sku"`
	MRP           float64            `json:"mrp"`
	SellingPrice  float64            `json:"selling_price"`
	Discount      float64            `json:"discount_percent"`
	InitialStock  int                `json:"initial_stock"`
	VariantCode   string             `json:"variant_code"`
	Barcode       *string            `json:"barcode"`
	Images        []productImageBody `json:"images"`
	Sizes         []sizeStockBody    `json:"sizes"`
}

// sizeStockBody is one size and its starting stock (e.g. {"size":"M","quantity":10}).
type sizeStockBody struct {
	Size     string `json:"size"`
	Quantity int    `json:"quantity"`
}

func toImageInputs(in []productImageBody) []ImageInput {
	out := make([]ImageInput, 0, len(in))
	for _, i := range in {
		if strings.TrimSpace(i.Key) == "" {
			continue
		}
		out = append(out, ImageInput{FileKey: i.Key, AltText: i.Alt})
	}
	return out
}

func (h *Handler) sellerCreateProduct(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	var body createProductBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	id, err := h.svc.CreateProduct(c.Context(), CreateProductRequest{
		SellerID: sellerID, Title: body.Title, Description: body.Description,
		BrandID: body.BrandID, BrandName: body.BrandName, GenderID: body.GenderID, CategoryID: body.CategoryID,
		CategorySlug: body.CategorySlug,
		SubcategoryID: body.SubcategoryID, HSNCode: body.HSNCode, TaxPercent: body.TaxPercent,
		Season: body.Season, SKU: body.SKU, MRP: body.MRP, SellingPrice: body.SellingPrice,
		Discount: body.Discount, InitialStock: body.InitialStock, VariantCode: body.VariantCode,
		Barcode: body.Barcode, Images: toImageInputs(body.Images), Sizes: toSizeInputs(body.Sizes),
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func toSizeInputs(in []sizeStockBody) []SizeStockInput {
	out := make([]SizeStockInput, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s.Size) == "" {
			continue
		}
		q := s.Quantity
		if q < 0 {
			q = 0
		}
		out = append(out, SizeStockInput{Size: s.Size, Quantity: q})
	}
	return out
}

type updateProductBody struct {
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	BrandID      *string            `json:"brand_id"`
	BrandName    string             `json:"brand_name"`
	GenderID     *string            `json:"gender_id"`
	CategoryID   *string            `json:"category_id"`
	CategorySlug string             `json:"category_slug"`
	Season       string             `json:"season"`
	MRP          float64            `json:"mrp"`
	SellingPrice float64            `json:"selling_price"`
	Discount     float64            `json:"discount_percent"`
	Stock        *int               `json:"stock"`
	Images       []productImageBody `json:"images"`
}

func (h *Handler) sellerUpdateProduct(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	var body updateProductBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	req := UpdateProductRequest{
		ProductID: c.Params("id"), SellerID: sellerID,
		Title: body.Title, Description: body.Description, BrandID: body.BrandID, BrandName: body.BrandName,
		GenderID: body.GenderID, CategoryID: body.CategoryID, CategorySlug: body.CategorySlug,
		Season: body.Season,
		MRP: body.MRP, SellingPrice: body.SellingPrice, Discount: body.Discount,
		Stock: body.Stock,
	}
	// Only replace images when the client explicitly sends the field.
	if body.Images != nil {
		req.Images = toImageInputs(body.Images)
	}
	if err := h.svc.UpdateProduct(c.Context(), req); err != nil {
		return err
	}
	return httpx.Message(c, "Product updated")
}

type setStockBody struct {
	Stock           int    `json:"stock"`
	SellerListingID string `json:"seller_listing_id"`
}

func (h *Handler) sellerSetStock(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	var body setStockBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.SetStock(c.Context(), c.Params("id"), sellerID, body.SellerListingID, body.Stock); err != nil {
		return err
	}
	return httpx.Message(c, "Stock updated")
}

func (h *Handler) sellerListProducts(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	page := queryInt(c, "page", 1)
	size := queryInt(c, "page_size", 20)
	items, total, err := h.svc.ListSellerProducts(c.Context(), sellerID, c.Query("status"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{Page: page, PageSize: size, Total: total, TotalPages: totalPages(total, size)})
}

func (h *Handler) sellerGetProduct(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	p, err := h.svc.GetProductForSeller(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, p)
}

func (h *Handler) sellerSubmitProduct(c *fiber.Ctx) error {
	sellerID, err := h.resolveSeller(c)
	if err != nil {
		return err
	}
	if err := h.svc.SubmitForReview(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Product submitted for review")
}

// ---- admin handlers ----

func (h *Handler) adminListProducts(c *fiber.Ctx) error {
	page := queryInt(c, "page", 1)
	size := queryInt(c, "page_size", 20)
	items, total, err := h.svc.ListForApproval(c.Context(), c.Query("status"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{Page: page, PageSize: size, Total: total, TotalPages: totalPages(total, size)})
}

func (h *Handler) adminGetProduct(c *fiber.Ctx) error {
	p, err := h.svc.GetProduct(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, p)
}

func (h *Handler) adminApprove(c *fiber.Ctx) error {
	p := auth.From(c)
	if err := h.svc.ApproveProduct(c.Context(), c.Params("id"), p.UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Product approved")
}

type rejectBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) adminReject(c *fiber.Ctx) error {
	p := auth.From(c)
	var body rejectBody
	_ = c.BodyParser(&body)
	if err := h.svc.RejectProduct(c.Context(), c.Params("id"), body.Reason, p.UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Product rejected")
}

type createCategoryBody struct {
	ParentID  *string `json:"parent_id"`
	GenderID  *string `json:"gender_id"`
	Name      string  `json:"name"`
	Level     int     `json:"level"`
	SortOrder int     `json:"sort_order"`
}

func (h *Handler) adminCreateCategory(c *fiber.Ctx) error {
	var body createCategoryBody
	if err := c.BodyParser(&body); err != nil || body.Name == "" {
		return httpx.BadRequest("validation_failed", "Category name is required")
	}
	id, err := h.svc.tax.CreateCategory(c.Context(), Category{
		ParentID: body.ParentID, GenderID: body.GenderID, Name: body.Name,
		Slug: Slugify(body.Name) + "-" + shortID(), Level: body.Level,
		SortOrder: body.SortOrder, IsActive: true,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

// ---- helpers ----

// resolveSeller resolves the seller_id for the authenticated seller user and
// ensures the seller account is approved before allowing catalog writes.
func (h *Handler) resolveSeller(c *fiber.Ctx) (string, error) {
	p := auth.From(c)
	if p == nil {
		return "", httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	seller, err := h.sellerLookup.FindByUserID(c.Context(), p.UserID)
	if err != nil {
		return "", httpx.Forbidden("no_seller", "Seller profile not found")
	}
	if seller.Status != "approved" {
		return "", httpx.Forbidden("seller_not_approved", "Your seller account is not approved yet")
	}
	return seller.ID, nil
}

func queryInt(c *fiber.Ctx, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func totalPages(total int64, size int) int {
	if size <= 0 {
		return 0
	}
	return int((total + int64(size) - 1) / int64(size))
}
