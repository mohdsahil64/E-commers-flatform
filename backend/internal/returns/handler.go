package returns

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/sellers"
)

// Handler exposes return endpoints across portals.
type Handler struct {
	svc          *Service
	sellerLookup *sellers.Repository
}

// NewHandler builds a returns Handler.
func NewHandler(svc *Service, sellerLookup *sellers.Repository) *Handler {
	return &Handler{svc: svc, sellerLookup: sellerLookup}
}

// RegisterCustomer mounts customer return routes.
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Get("/returns", h.customerList)
	r.Post("/returns", h.request)
}

// RegisterSeller mounts seller return-management routes. The seller owns the
// approve / reject / received decisions.
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/returns", h.sellerList)
	r.Post("/returns/:id/approve", h.sellerApprove)
	r.Post("/returns/:id/reject", h.sellerReject)
	r.Post("/returns/:id/received", h.sellerReceived)
}

// RegisterAdmin mounts admin routes. Admin is READ-ONLY over the return itself
// (it cannot approve/reject/receive — that is the seller's job). Admin only
// pays the refund out-of-band and records the UTR, which is unlocked once the
// seller has approved + received (status = refund_processing).
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/returns", auth.RequirePermission(auth.PermReturnManage), h.adminList)
	r.Post("/returns/:id/refund", auth.RequirePermission(auth.PermRefundManage), h.adminRefund)
}

// ---- customer ----

type requestBody struct {
	OrderItemID   string   `json:"order_item_id"`
	Type          string   `json:"type"`
	ReasonCode    string   `json:"reason_code"`
	Reason        string   `json:"reason"`
	RefundMethod  string   `json:"refund_method"`
	RefundUPI     string   `json:"refund_upi"`
	RefundAccName string   `json:"refund_acc_name"`
	RefundAccNo   string   `json:"refund_acc_no"`
	RefundIFSC    string   `json:"refund_ifsc"`
	PhotoKeys     []string `json:"photo_keys"`
}

func (h *Handler) request(c *fiber.Ctx) error {
	var b requestBody
	if err := c.BodyParser(&b); err != nil || b.OrderItemID == "" {
		return httpx.BadRequest("validation_failed", "order_item_id is required")
	}
	id, err := h.svc.Request(c.Context(), RequestInput{
		UserID: auth.From(c).UserID, OrderItemID: b.OrderItemID, Type: b.Type,
		ReasonCode: b.ReasonCode, Reason: b.Reason,
		RefundMethod: b.RefundMethod, RefundUPI: b.RefundUPI,
		RefundAccName: b.RefundAccName, RefundAccNo: b.RefundAccNo, RefundIFSC: b.RefundIFSC,
		PhotoKeys: b.PhotoKeys,
	})
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) customerList(c *fiber.Ctx) error {
	items, err := h.svc.ListForCustomer(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

// ---- seller ----

func (h *Handler) sellerList(c *fiber.Ctx) error {
	seller, err := h.sellerLookup.FindByUserID(c.Context(), auth.From(c).UserID)
	if err != nil {
		return httpx.Forbidden("no_seller", "Seller profile not found")
	}
	items, err := h.svc.ListForSeller(c.Context(), seller.ID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) sellerApprove(c *fiber.Ctx) error {
	if err := h.svc.Approve(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Return approved")
}

type rejectBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) sellerReject(c *fiber.Ctx) error {
	var b rejectBody
	_ = c.BodyParser(&b)
	if err := h.svc.Reject(c.Context(), c.Params("id"), auth.From(c).UserID, b.Reason); err != nil {
		return err
	}
	return httpx.Message(c, "Return rejected")
}

func (h *Handler) sellerReceived(c *fiber.Ctx) error {
	if err := h.svc.MarkReceived(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Return received; refund pending")
}

// ---- admin ----

func (h *Handler) adminList(c *fiber.Ctx) error {
	items, err := h.svc.ListAll(c.Context(), c.Query("status"))
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

type refundBody struct {
	UTR string `json:"utr"`
}

func (h *Handler) adminRefund(c *fiber.Ctx) error {
	var b refundBody
	_ = c.BodyParser(&b)
	if err := h.svc.RefundPayout(c.Context(), RefundPayoutInput{
		ReturnID: c.Params("id"), AdminID: auth.From(c).UserID, UTR: b.UTR,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Refund marked paid")
}
