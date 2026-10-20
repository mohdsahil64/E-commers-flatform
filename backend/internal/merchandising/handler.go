package merchandising

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes collection + banner endpoints.
type Handler struct{ repo *Repository }

// NewHandler builds a merchandising Handler.
func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

// RegisterPublic mounts storefront reads.
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/collections", h.listCollections)
	r.Get("/collections/:slug", h.getCollection)
	r.Get("/collections/:slug/products", h.collectionProducts)
	r.Get("/banners", h.listBanners)
}

// RegisterAdmin mounts admin management.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/collections", auth.RequirePermission(auth.PermCollectionManage), h.adminListCollections)
	r.Post("/collections", auth.RequirePermission(auth.PermCollectionManage), h.createCollection)
	r.Post("/collections/:id/products", auth.RequirePermission(auth.PermCollectionManage), h.addProduct)
	r.Get("/banners", auth.RequirePermission(auth.PermBannerManage), h.adminListBanners)
	r.Post("/banners", auth.RequirePermission(auth.PermBannerManage), h.createBanner)
	r.Put("/banners/:id", auth.RequirePermission(auth.PermBannerManage), h.updateBanner)
}

func (h *Handler) listCollections(c *fiber.Ctx) error {
	items, err := h.repo.ListCollections(c.Context(), true)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) adminListCollections(c *fiber.Ctx) error {
	items, err := h.repo.ListCollections(c.Context(), false)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) getCollection(c *fiber.Ctx) error {
	col, rules, err := h.repo.GetCollectionBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"collection": col, "rules": rules})
}

func (h *Handler) collectionProducts(c *fiber.Ctx) error {
	col, _, err := h.repo.GetCollectionBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	page := mi(c, "page", 1)
	size := mi(c, "page_size", 24)
	items, err := h.repo.ProductsForCollection(c.Context(), col.ID, size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) listBanners(c *fiber.Ctx) error {
	items, err := h.repo.ListBanners(c.Context(), c.Query("placement"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) adminListBanners(c *fiber.Ctx) error {
	items, err := h.repo.AdminListBanners(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type createCollectionBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Kind        string `json:"kind"`
}

func (h *Handler) createCollection(c *fiber.Ctx) error {
	var b createCollectionBody
	if err := c.BodyParser(&b); err != nil || b.Name == "" {
		return httpx.BadRequest("validation_failed", "Collection name is required")
	}
	id, err := h.repo.CreateCollection(c.Context(), Collection{
		Name: b.Name, Slug: slugify(b.Name), Description: b.Description, Type: b.Type, Kind: b.Kind,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

type addProductBody struct {
	ProductID string `json:"product_id"`
	SortOrder int    `json:"sort_order"`
}

func (h *Handler) addProduct(c *fiber.Ctx) error {
	var b addProductBody
	if err := c.BodyParser(&b); err != nil || b.ProductID == "" {
		return httpx.BadRequest("validation_failed", "product_id is required")
	}
	if err := h.repo.AddProductToCollection(c.Context(), c.Params("id"), b.ProductID, b.SortOrder); err != nil {
		return err
	}
	return httpx.Message(c, "Product added to collection")
}

type createBannerBody struct {
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle"`
	ImageKey        string            `json:"image_key"`
	MobileImageKey  string            `json:"mobile_image_key"`
	Placement       string            `json:"placement"`
	DestinationType string            `json:"destination_type"`
	SortOrder       int               `json:"sort_order"`
	Targets         map[string]string `json:"targets"`
}

func (h *Handler) createBanner(c *fiber.Ctx) error {
	var b createBannerBody
	if err := c.BodyParser(&b); err != nil || b.Title == "" {
		return httpx.BadRequest("validation_failed", "Banner title is required")
	}
	id, err := h.repo.CreateBanner(c.Context(), Banner{
		Title: b.Title, Subtitle: b.Subtitle, ImageKey: b.ImageKey, MobileImageKey: b.MobileImageKey,
		Placement: b.Placement, DestinationType: b.DestinationType, SortOrder: b.SortOrder,
		Targets: b.Targets,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) updateBanner(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return httpx.BadRequest("validation_failed", "Banner id is required")
	}
	var b createBannerBody
	if err := c.BodyParser(&b); err != nil || b.Title == "" {
		return httpx.BadRequest("validation_failed", "Banner title is required")
	}
	if err := h.repo.UpdateBanner(c.Context(), Banner{
		ID: id, Title: b.Title, Subtitle: b.Subtitle, ImageKey: b.ImageKey, MobileImageKey: b.MobileImageKey,
		Placement: b.Placement, DestinationType: b.DestinationType, SortOrder: b.SortOrder,
		Targets: b.Targets,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Banner updated")
}

func mi(c *fiber.Ctx, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func slugify(s string) string {
	out := make([]rune, 0, len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+32)
			prevDash = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out = append(out, r)
			prevDash = false
		default:
			if !prevDash {
				out = append(out, '-')
				prevDash = true
			}
		}
	}
	s = string(out)
	for len(s) > 0 && s[0] == '-' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == '-' {
		s = s[:len(s)-1]
	}
	if s == "" {
		s = "collection"
	}
	return s
}
