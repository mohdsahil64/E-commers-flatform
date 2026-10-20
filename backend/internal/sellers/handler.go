package sellers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes seller and admin-seller endpoints.
type Handler struct {
	svc *Service
}

// NewHandler builds a seller Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterSeller mounts seller-portal routes (auth + seller portal enforced by caller).
func (h *Handler) RegisterSeller(r fiber.Router) {
	r.Get("/profile", h.profile)
	r.Put("/profile", h.updateProfile)
	r.Post("/change-password", h.changePassword)
	r.Get("/onboarding", h.getOnboarding)
	// Structured onboarding steps that persist real data.
	r.Put("/onboarding/owner", h.saveOwner)
	r.Put("/onboarding/business", h.saveOwner) // back-compat alias
	r.Put("/onboarding/bank", h.saveBank)
	r.Put("/onboarding/pickup", h.savePickup)
	r.Post("/onboarding/contract/sign", h.signContract)
	r.Post("/onboarding/steps/:step/complete", h.completeStep)
	r.Post("/onboarding/submit", h.submit)
}

// RegisterAdmin mounts admin seller-management routes.
func (h *Handler) RegisterAdmin(r fiber.Router) {
	r.Get("/sellers", auth.RequirePermission(auth.PermSellerView), h.adminList)
	r.Get("/sellers/:id", auth.RequirePermission(auth.PermSellerView), h.adminGet)
	r.Post("/sellers/:id/approve", auth.RequirePermission(auth.PermSellerApprove), h.adminApprove)
	r.Post("/sellers/:id/reject", auth.RequirePermission(auth.PermSellerReject), h.adminReject)
	r.Post("/sellers/:id/suspend", auth.RequirePermission(auth.PermSellerSuspend), h.adminSuspend)
	r.Post("/sellers/:id/unsuspend", auth.RequirePermission(auth.PermSellerActivate), h.adminUnsuspend)
	r.Post("/sellers/:id/activate", auth.RequirePermission(auth.PermSellerActivate), h.adminActivate)
	r.Post("/sellers/:id/block", auth.RequirePermission(auth.PermSellerBlock), h.adminBlock)
	r.Post("/sellers/:id/unblock", auth.RequirePermission(auth.PermSellerBlock), h.adminUnblock)
}

// ---- seller ----

func (h *Handler) profile(c *fiber.Ctx) error {
	p := auth.From(c)
	seller, err := h.svc.GetByUser(c.Context(), p.UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, seller)
}

type profileUpdateBody struct {
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
	NotifyOrderEmails      bool `json:"notify_order_emails"`
	NotifyReturnEmails     bool `json:"notify_return_emails"`
	NotifySettlementEmails bool `json:"notify_settlement_emails"`
	NotifyLowStock         bool `json:"notify_low_stock"`
	NotifyMarketing        bool `json:"notify_marketing"`
	// CODEnabled is a pointer so a client that omits it leaves the current
	// setting untouched (only an explicit true/false changes it).
	CODEnabled *bool `json:"cod_enabled"`
}

func (h *Handler) updateProfile(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	var body profileUpdateBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	seller, err := h.svc.UpdateProfile(c.Context(), sellerID, ProfileUpdate{
		StoreName:              body.StoreName,
		StoreDescription:       body.StoreDescription,
		NotifyOrderEmails:      body.NotifyOrderEmails,
		NotifyReturnEmails:     body.NotifyReturnEmails,
		NotifySettlementEmails: body.NotifySettlementEmails,
		NotifyLowStock:         body.NotifyLowStock,
		NotifyMarketing:        body.NotifyMarketing,
		CODEnabledPtr:          body.CODEnabled,
	})
	if err != nil {
		return err
	}
	return httpx.OK(c, seller)
}

type changePasswordBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) changePassword(c *fiber.Ctx) error {
	p := auth.From(c)
	if p == nil {
		return httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	var body changePasswordBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.ChangePassword(c.Context(), p.UserID, body.CurrentPassword, body.NewPassword); err != nil {
		return err
	}
	return httpx.Message(c, "Password changed")
}

func (h *Handler) getOnboarding(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	b, err := h.svc.GetBundle(c.Context(), sellerID)
	if err != nil {
		return err
	}
	// Include the marketplace default commission so the contract can display it.
	return httpx.OK(c, fiber.Map{
		"seller":              b.Seller,
		"onboarding":          b.Onboarding,
		"bank":                b.Bank,
		"pickup":              b.Pickup,
		"default_commission":  h.svc.DefaultCommission(),
		"max_resubmits":       MaxResubmits,
	})
}

type ownerBody struct {
	LegalBusinessName string `json:"legal_business_name"`
	BusinessType      string `json:"business_type"`
	OwnerName         string `json:"owner_name"`
	ContactEmail      string `json:"contact_email"`
	ContactPhone      string `json:"contact_phone"`
	GSTNumber         string `json:"gst_number"`
	PANNumber         string `json:"pan_number"`
	StoreDescription  string `json:"store_description"`
}

func (h *Handler) saveOwner(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	var body ownerBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.SaveOwnerInfo(c.Context(), sellerID, Seller{
		LegalBusinessName: body.LegalBusinessName, BusinessType: body.BusinessType,
		OwnerName: body.OwnerName, ContactEmail: body.ContactEmail,
		ContactPhone: body.ContactPhone, GSTNumber: body.GSTNumber,
		PANNumber: body.PANNumber, StoreDescription: body.StoreDescription,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Owner details saved")
}

type bankBody struct {
	AccountHolder string `json:"account_holder"`
	AccountNumber string `json:"account_number"`
	IFSC          string `json:"ifsc"`
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
}

func (h *Handler) saveBank(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	var body bankBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.SaveBank(c.Context(), sellerID, BankAccount{
		AccountHolder: body.AccountHolder, AccountNumber: body.AccountNumber,
		IFSC: body.IFSC, BankName: body.BankName, Branch: body.Branch,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Bank details saved")
}

type pickupBody struct {
	Label        string `json:"label"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Line1        string `json:"line1"`
	Line2        string `json:"line2"`
	City         string `json:"city"`
	State        string `json:"state"`
	Pincode      string `json:"pincode"`
	Country      string `json:"country"`
}

func (h *Handler) savePickup(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	var body pickupBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.SavePickup(c.Context(), sellerID, PickupAddress{
		Label: body.Label, ContactName: body.ContactName, ContactPhone: body.ContactPhone,
		Line1: body.Line1, Line2: body.Line2, City: body.City, State: body.State,
		Pincode: body.Pincode, Country: body.Country,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Pickup location saved")
}

type contractBody struct {
	SignedName string `json:"signed_name"`
	Agreed     bool   `json:"agreed"`
}

func (h *Handler) signContract(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	var body contractBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if !body.Agreed {
		return httpx.BadRequest("validation_failed", "You must agree to the commission agreement")
	}
	if err := h.svc.SignContract(c.Context(), sellerID, body.SignedName); err != nil {
		return err
	}
	return httpx.Message(c, "Contract signed")
}

func (h *Handler) completeStep(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.CompleteStep(c.Context(), sellerID, c.Params("step")); err != nil {
		return err
	}
	return httpx.Message(c, "Step marked complete")
}

func (h *Handler) submit(c *fiber.Ctx) error {
	sellerID, err := h.currentSellerID(c)
	if err != nil {
		return err
	}
	if err := h.svc.SubmitApplication(c.Context(), sellerID); err != nil {
		return err
	}
	return httpx.Message(c, "Application submitted for review")
}

// ---- admin ----

func (h *Handler) adminList(c *fiber.Ctx) error {
	page := qInt(c, "page", 1)
	size := qInt(c, "page_size", 20)
	items, total, err := h.svc.List(c.Context(), c.Query("status"), size, (page-1)*size)
	if err != nil {
		return err
	}
	return httpx.Paginated(c, items, httpx.Meta{Page: page, PageSize: size, Total: total,
		TotalPages: int((total + int64(size) - 1) / int64(size))})
}

func (h *Handler) adminGet(c *fiber.Ctx) error {
	s, err := h.svc.Get(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return httpx.OK(c, s)
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) adminApprove(c *fiber.Ctx) error {
	if err := h.svc.Approve(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller approved")
}

func (h *Handler) adminReject(c *fiber.Ctx) error {
	var b reasonBody
	_ = c.BodyParser(&b)
	if err := h.svc.Reject(c.Context(), c.Params("id"), b.Reason, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller rejected")
}

func (h *Handler) adminSuspend(c *fiber.Ctx) error {
	var b reasonBody
	_ = c.BodyParser(&b)
	if err := h.svc.Suspend(c.Context(), c.Params("id"), b.Reason, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller suspended")
}

func (h *Handler) adminActivate(c *fiber.Ctx) error {
	if err := h.svc.Activate(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller activated")
}

func (h *Handler) adminUnsuspend(c *fiber.Ctx) error {
	if err := h.svc.Unsuspend(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller unsuspended")
}

func (h *Handler) adminUnblock(c *fiber.Ctx) error {
	if err := h.svc.Unblock(c.Context(), c.Params("id"), auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller unblocked")
}

func (h *Handler) adminBlock(c *fiber.Ctx) error {
	var b reasonBody
	_ = c.BodyParser(&b)
	if err := h.svc.Block(c.Context(), c.Params("id"), b.Reason, auth.From(c).UserID); err != nil {
		return err
	}
	return httpx.Message(c, "Seller blocked")
}

// ---- helpers ----

func (h *Handler) currentSellerID(c *fiber.Ctx) (string, error) {
	p := auth.From(c)
	if p == nil {
		return "", httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	seller, err := h.svc.GetByUser(c.Context(), p.UserID)
	if err != nil {
		return "", err
	}
	return seller.ID, nil
}

func qInt(c *fiber.Ctx, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
