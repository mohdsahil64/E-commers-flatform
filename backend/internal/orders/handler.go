package orders

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/labelpdf"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes order endpoints for all three portals.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
}

// NewHandler builds an orders Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterCustomer mounts customer order routes.
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Post("/checkout", h.checkout)
	r.Get("/orders", h.customerList)
	r.Get("/orders/:id", h.customerGet)
	r.Get("/orders/:id/history", h.history)
	r.Get("/orders/:id/invoice", h.customerInvoice)
	r.Post("/orders/:id/cancel", h.customerCancel)
	// Called by the storefront when an online payment did not complete (popup
	// closed / verification failed): unwinds the pending order, releases stock,
	// and restores the cart so the customer can retry.
	r.Post("/orders/:id/abandon", h.customerAbandon)
}

// RegisterSeller mounts seller fulfilment routes.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/orders", h.sellerList)
	r.Get("/orders/:id", h.sellerGet)
	r.Get("/orders/:id/history", h.sellerHistory)
	r.Post("/orders/:id/accept", h.sellerAccept)
	r.Post("/orders/:id/cancel", h.sellerCancel)
	r.Post("/order-items/:itemId/status", h.sellerUpdateItemStatus)
}

// RegisterAdmin mounts admin order routes.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/orders", auth.RequirePermission(auth.PermOrderView), h.adminList)
	r.Get("/orders/:id", auth.RequirePermission(auth.PermOrderView), h.adminGet)
	r.Post("/orders/:id/status", auth.RequirePermission(auth.PermOrderManage), h.adminUpdateStatus)
	r.Post("/orders/:id/cancel", auth.RequirePermission(auth.PermOrderManage), h.adminCancel)
}

// ---- customer ----

type checkoutBody struct {
	PaymentMethod string `json:"payment_method"`
	AddressID     string `json:"address_id"`
	CouponCode    string `json:"coupon_code"`
}

func (h *Handler) checkout(c *fiber.Ctx) error {
	var b checkoutBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	order, err := h.svc.Checkout(c.Context(), CheckoutInput{
		UserID: auth.From(c).UserID, PaymentMethod: b.PaymentMethod, AddressID: b.AddressID,
		CouponCode: b.CouponCode,
	})
	if err != nil {
		return err
	}
	// COD orders are confirmed atomically inside Checkout; online orders remain
	// pending_payment until the payments module confirms capture.
	return httpx.Created(c, order)
}

func (h *Handler) customerList(c *fiber.Ctx) error {
	page := pInt(c, "page", 1)
	size := pInt(c, "page_size", 10)
	items, total, err := h.svc.ListForCustomerUser(c.Context(), auth.From(c).UserID, size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, meta(page, size, total))
}

func (h *Handler) customerGet(c *fiber.Ctx) error {
	o, err := h.svc.GetForCustomerUser(c.Context(), c.Params("id"), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, o)
}

func (h *Handler) history(c *fiber.Ctx) error {
	events, err := h.svc.HistoryForCustomerUser(c.Context(), c.Params("id"), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, events)
}

func (h *Handler) customerCancel(c *fiber.Ctx) error {
	// Verify the order belongs to the requesting customer before cancelling.
	if _, err := h.svc.GetForCustomerUser(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	// Optional cancellation reason selected by the customer (best-effort parse).
	var b cancelBody
	_ = c.BodyParser(&b)
	if err := h.svc.Cancel(c.Context(), c.Params("id"), auth.From(c).UserID, "customer", b.Reason); err != nil {
		return err
	}
	return httpx.Message(c, "Order cancelled")
}

func (h *Handler) customerAbandon(c *fiber.Ctx) error {
	if err := h.svc.AbandonOnlineOrder(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Order abandoned and cart restored")
}

// customerInvoice streams a PDF tax invoice for the customer's own order.
// Financials come from the backend order record (never trusted from the client).
func (h *Handler) customerInvoice(c *fiber.Ctx) error {
	o, err := h.svc.GetForCustomerUser(c.Context(), c.Params("id"), auth.From(c).UserID)
	if err != nil {
		return err
	}
	var lines []labelpdf.InvoiceLine
	for _, it := range o.Items {
		lines = append(lines, labelpdf.InvoiceLine{
			Title: it.ProductTitle, Qty: it.Quantity, Amount: rupees(it.GrossAmount),
		})
	}
	payment := "Cash on Delivery"
	if o.PaymentMethod == "online" {
		payment = "Paid online"
	}
	discount := ""
	if o.DiscountTotal > 0 {
		discount = rupees(o.DiscountTotal)
	}
	shipLines := compactStr([]string{
		o.ShipLine1, o.ShipLine2,
		joinCommas(o.ShipCity, o.ShipState, o.ShipPincode),
	})
	pdf := labelpdf.GenerateInvoice(labelpdf.Invoice{
		OrderNumber: o.OrderNumber,
		Date:        formatInvoiceDate(o.PlacedAt, o.CreatedAt),
		BillToName:  o.ShipName,
		BillToPhone: o.ShipPhone,
		ShipLines:   shipLines,
		Lines:       lines,
		Subtotal:    rupees(o.ItemsSubtotal),
		Discount:    discount,
		Tax:         rupees(o.TaxTotal),
		Shipping:    shippingLabel(o.ShippingTotal),
		Total:       rupees(o.GrandTotal),
		PaymentInfo: payment,
	})
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `attachment; filename="carlitos-invoice-`+o.OrderNumber+`.pdf"`)
	return c.Send(pdf)
}

// ---- seller ----

func (h *Handler) sellerList(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	page := pInt(c, "page", 1)
	size := pInt(c, "page_size", 20)
	items, total, err := h.svc.ListForSellerFiltered(
		c.Context(), sellerID, c.Query("status"), c.Query("from"), c.Query("to"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, meta(page, size, total))
}

func (h *Handler) sellerGet(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	o, err := h.svc.GetForSeller(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, o)
}

type itemStatusBody struct {
	Status string `json:"status"`
}

func (h *Handler) sellerHistory(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	events, err := h.svc.HistoryForSeller(c.Context(), c.Params("id"), sellerID)
	if err != nil {
		return err
	}
	return httpx.OK(c, events)
}

func (h *Handler) sellerAccept(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.AcceptForSeller(c.Context(), c.Params("id"), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Order accepted")
}

type cancelBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) sellerCancel(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	var b cancelBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.CancelForSeller(c.Context(), c.Params("id"), sellerID, b.Reason); err != nil {
		return err
	}
	return httpx.Message(c, "Order cancelled")
}

func (h *Handler) sellerUpdateItemStatus(c *fiber.Ctx) error {
	sellerID, err := h.sellerID(c)
	if err != nil {
		return err
	}
	var b itemStatusBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.UpdateItemStatus(c.Context(), c.Params("itemId"), sellerID, Status(b.Status)); err != nil {
		return err
	}
	return httpx.Message(c, "Item status updated")
}

// ---- admin ----

func (h *Handler) adminList(c *fiber.Ctx) error {
	page := pInt(c, "page", 1)
	size := pInt(c, "page_size", 20)
	items, total, err := h.svc.ListAll(c.Context(), c.Query("status"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, meta(page, size, total))
}

func (h *Handler) adminGet(c *fiber.Ctx) error {
	o, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, o)
}

type statusBody struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (h *Handler) adminUpdateStatus(c *fiber.Ctx) error {
	var b statusBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.TransitionOrder(c.Context(), c.Params("id"), Status(b.Status), auth.From(c).UserID, "admin", b.Note); err != nil {
		return err
	}
	return httpx.Message(c, "Order status updated")
}

func (h *Handler) adminCancel(c *fiber.Ctx) error {
	var b cancelBody
	_ = c.BodyParser(&b)
	if err := h.svc.Cancel(c.Context(), c.Params("id"), auth.From(c).UserID, "admin", b.Reason); err != nil {
		return err
	}
	return httpx.Message(c, "Order cancelled")
}

// ---- helpers ----

func (h *Handler) sellerID(c *fiber.Ctx) (string, error) {
	p := auth.From(c)
	seller, err := h.sellerLookup.FindByUserID(c.Context(), p.UserID)
	if err != nil {
		return "", httpx.Forbidden("no_seller", "Seller profile not found")
	}
	return seller.ID, nil
}

func pInt(c *fiber.Ctx, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func meta(page, size int, total int64) httpx.Meta {
	tp := 0
	if size > 0 {
		tp = int((total + int64(size) - 1) / int64(size))
	}
	return httpx.Meta{Page: page, PageSize: size, Total: total, TotalPages: tp}
}

// ── invoice helpers ──────────────────────────────────────────────────────────

// rupees formats a rupee amount for the PDF (Helvetica can't render the ₹ glyph
// reliably, so we use the "Rs" prefix with Indian digit grouping).
func rupees(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(v + 0.5)
	s := strconv.FormatInt(whole, 10)
	n := len(s)
	if n > 3 {
		head, tail := s[:n-3], s[n-3:]
		var parts []string
		for len(head) > 2 {
			parts = append([]string{head[len(head)-2:]}, parts...)
			head = head[:len(head)-2]
		}
		if head != "" {
			parts = append([]string{head}, parts...)
		}
		s = strings.Join(parts, ",") + "," + tail
	}
	if neg {
		return "-Rs " + s
	}
	return "Rs " + s
}

func shippingLabel(v float64) string {
	if v <= 0 {
		return "FREE"
	}
	return rupees(v)
}

func compactStr(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func joinCommas(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

func formatInvoiceDate(placed *time.Time, created time.Time) string {
	t := created
	if placed != nil {
		t = *placed
	}
	return t.Format("02 Jan 2006")
}
