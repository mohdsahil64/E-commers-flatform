package customers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/httpx"
)

// Handler exposes customer profile + address endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a customer Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterCustomer mounts customer account routes (customer auth enforced by caller).
func (h *Handler) RegisterCustomer(r fiber.Router) {
	r.Get("/profile", h.profile)
	r.Put("/profile", h.updateProfile)
	r.Post("/profile/email/request", h.requestEmail)  // add/change email (sends OTP)
	r.Post("/profile/email/confirm", h.confirmEmail)  // verify OTP + set email
	r.Get("/addresses", h.listAddresses)
	r.Post("/addresses", h.addAddress)
	r.Post("/addresses/:id/default", h.setDefault)
	r.Delete("/addresses/:id", h.deleteAddress)
}

type emailRequestBody struct {
	Email string `json:"email"`
}

func (h *Handler) requestEmail(c *fiber.Ctx) error {
	var b emailRequestBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.RequestEmailChange(c.Context(), auth.From(c).UserID, b.Email); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"sent": true})
}

type emailConfirmBody struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handler) confirmEmail(c *fiber.Ctx) error {
	var b emailConfirmBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.ConfirmEmailChange(c.Context(), auth.From(c).UserID, b.Email, b.Code); err != nil {
		return err
	}
	return httpx.Message(c, "Email verified and updated")
}

func (h *Handler) profile(c *fiber.Ctx) error {
	p, err := h.svc.GetProfile(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, p)
}

type updateProfileBody struct {
	DisplayName    string `json:"display_name"`
	Gender         string `json:"gender"`
	NotifyEmail    bool   `json:"notify_email"`
	NotifySMS      bool   `json:"notify_sms"`
	NotifyPush     bool   `json:"notify_push"`
	MarketingOptIn bool   `json:"marketing_opt_in"`
}

func (h *Handler) updateProfile(c *fiber.Ctx) error {
	var b updateProfileBody
	if err := c.BodyParser(&b); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if err := h.svc.UpdateProfile(c.Context(), auth.From(c).UserID, Profile{
		DisplayName: b.DisplayName, Gender: b.Gender, NotifyEmail: b.NotifyEmail,
		NotifySMS: b.NotifySMS, NotifyPush: b.NotifyPush, MarketingOptIn: b.MarketingOptIn,
	}); err != nil {
		return err
	}
	return httpx.Message(c, "Profile updated")
}

func (h *Handler) listAddresses(c *fiber.Ctx) error {
	items, err := h.svc.ListAddresses(c.Context(), auth.From(c).UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, items)
}

func (h *Handler) addAddress(c *fiber.Ctx) error {
	var a Address
	if err := c.BodyParser(&a); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	id, err := h.svc.AddAddress(c.Context(), auth.From(c).UserID, a)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": id})
}

func (h *Handler) setDefault(c *fiber.Ctx) error {
	if err := h.svc.SetDefaultAddress(c.Context(), auth.From(c).UserID, c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Default address set")
}

func (h *Handler) deleteAddress(c *fiber.Ctx) error {
	if err := h.svc.DeleteAddress(c.Context(), auth.From(c).UserID, c.Params("id")); err != nil {
		return err
	}
	return httpx.Message(c, "Address removed")
}
