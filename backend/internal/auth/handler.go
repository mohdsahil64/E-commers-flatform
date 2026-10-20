package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/carlitos/backend/internal/httpx"
)

type SellerRegisteredHook func(ownerName, email, storeName string)

type Handler struct {
	svc                *Service
	manager            *Manager
	onSellerRegistered SellerRegisteredHook
}

func NewHandler(svc *Service, manager *Manager) *Handler {
	return &Handler{svc: svc, manager: manager}
}

func (h *Handler) WithSellerRegisteredHook(hook SellerRegisteredHook) *Handler {
	h.onSellerRegistered = hook
	return h
}

type registerCustomerBody struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type registerSellerBody struct {
	OwnerName string `json:"owner_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Password  string `json:"password"`
	StoreName string `json:"store_name"`
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Register(auth fiber.Router) {
	// Customer mobile-OTP login (primary customer auth).
	auth.Post("/customer/otp/request", h.requestOTP)
	auth.Post("/customer/otp/verify", h.verifyOTP)
	// Legacy email/password customer auth kept for compatibility.
	auth.Post("/customer/register", h.registerCustomer)
	auth.Post("/customer/login", h.loginFor("customer"))
	// Seller signup verification (email + mobile codes before account creation).
	auth.Post("/seller/verify/email/request", h.sellerEmailOTPRequest)
	auth.Post("/seller/verify/email/confirm", h.sellerEmailOTPConfirm)
	auth.Post("/seller/verify/phone/request", h.sellerPhoneOTPRequest)
	auth.Post("/seller/verify/phone/confirm", h.sellerPhoneOTPConfirm)
	// Seller & admin remain email/password.
	auth.Post("/seller/register", h.registerSeller)
	auth.Post("/seller/login", h.loginFor("seller"))
	auth.Post("/admin/login", h.loginFor("admin"))
	// Forgot / reset password (email-password portals only).
	auth.Post("/seller/forgot-password", h.forgotPasswordFor("seller"))
	auth.Post("/admin/forgot-password", h.forgotPasswordFor("admin"))
	auth.Post("/reset-password", h.resetPassword)
	auth.Post("/refresh", h.refresh)
	auth.Post("/logout", h.logout)
	auth.Get("/me", Middleware(h.manager, ""), h.me)
}

type forgotPasswordBody struct {
	Email string `json:"email"`
}

type resetPasswordBody struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// forgotPasswordFor returns a handler that starts the reset flow for the given
// portal. It always responds 200 with a generic message so callers cannot use
// it to discover which emails have accounts.
func (h *Handler) forgotPasswordFor(accountType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body forgotPasswordBody
		if err := c.BodyParser(&body); err != nil {
			return httpx.BadRequest("invalid_body", "Invalid request body")
		}
		ua, ip := clientMeta(c)
		if err := h.svc.RequestPasswordReset(c.Context(), accountType, body.Email, ua, ip); err != nil {
			return err
		}
		return httpx.OK(c, fiber.Map{"sent": true})
	}
}

func (h *Handler) resetPassword(c *fiber.Ctx) error {
	var body resetPasswordBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if strings.TrimSpace(body.Token) == "" {
		return httpx.BadRequest("validation_failed", "Reset token is required")
	}
	if err := h.svc.ResetPassword(c.Context(), body.Token, body.Password); err != nil {
		return err
	}
	return httpx.Message(c, "Your password has been reset. You can now sign in.")
}

type otpRequestBody struct {
	Phone string `json:"phone"`
}

type otpVerifyBody struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (h *Handler) requestOTP(c *fiber.Ctx) error {
	var body otpRequestBody
	if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.Phone) == "" {
		return httpx.BadRequest("validation_failed", "Mobile number is required")
	}
	if err := h.svc.RequestOTP(c.Context(), body.Phone); err != nil {
		return err
	}
	// Never echo the code. Client proceeds to the verify step.
	return httpx.OK(c, fiber.Map{"sent": true})
}

func (h *Handler) verifyOTP(c *fiber.Ctx) error {
	var body otpVerifyBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if strings.TrimSpace(body.Phone) == "" || strings.TrimSpace(body.Code) == "" {
		return httpx.BadRequest("validation_failed", "Mobile number and code are required")
	}
	ua, ip := clientMeta(c)
	pair, err := h.svc.VerifyOTP(c.Context(), body.Phone, body.Code, ua, ip)
	if err != nil {
		return err
	}
	return httpx.OK(c, pair)
}

func (h *Handler) registerCustomer(c *fiber.Ctx) error {
	var body registerCustomerBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if fields := validateRegisterCustomer(body); len(fields) > 0 {
		return httpx.BadRequest("validation_failed", "Please fix the errors").WithFields(fields)
	}
	ua, ip := clientMeta(c)
	pair, err := h.svc.RegisterCustomer(c.Context(), RegisterCustomerInput{
		FullName: strings.TrimSpace(body.FullName),
		Email:    body.Email,
		Phone:    strings.TrimSpace(body.Phone),
		Password: body.Password,
	}, ua, ip)
	if err != nil {
		return err
	}
	return httpx.Created(c, pair)
}

type verifyEmailBody struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type verifyPhoneBody struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (h *Handler) sellerEmailOTPRequest(c *fiber.Ctx) error {
	var b verifyEmailBody
	if err := c.BodyParser(&b); err != nil || strings.TrimSpace(b.Email) == "" {
		return httpx.BadRequest("validation_failed", "Email is required")
	}
	if err := h.svc.RequestSellerEmailOTP(c.Context(), b.Email); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"sent": true})
}

func (h *Handler) sellerEmailOTPConfirm(c *fiber.Ctx) error {
	var b verifyEmailBody
	if err := c.BodyParser(&b); err != nil || strings.TrimSpace(b.Email) == "" || strings.TrimSpace(b.Code) == "" {
		return httpx.BadRequest("validation_failed", "Email and code are required")
	}
	if err := h.svc.VerifySellerEmailOTP(c.Context(), b.Email, b.Code); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"verified": true})
}

func (h *Handler) sellerPhoneOTPRequest(c *fiber.Ctx) error {
	var b verifyPhoneBody
	if err := c.BodyParser(&b); err != nil || strings.TrimSpace(b.Phone) == "" {
		return httpx.BadRequest("validation_failed", "Mobile number is required")
	}
	if err := h.svc.RequestSellerPhoneOTP(c.Context(), b.Phone); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"sent": true})
}

func (h *Handler) sellerPhoneOTPConfirm(c *fiber.Ctx) error {
	var b verifyPhoneBody
	if err := c.BodyParser(&b); err != nil || strings.TrimSpace(b.Phone) == "" || strings.TrimSpace(b.Code) == "" {
		return httpx.BadRequest("validation_failed", "Mobile number and code are required")
	}
	if err := h.svc.VerifySellerPhoneOTP(c.Context(), b.Phone, b.Code); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"verified": true})
}

func (h *Handler) registerSeller(c *fiber.Ctx) error {
	var body registerSellerBody
	if err := c.BodyParser(&body); err != nil {
		return httpx.BadRequest("invalid_body", "Invalid request body")
	}
	if fields := validateRegisterSeller(body); len(fields) > 0 {
		return httpx.BadRequest("validation_failed", "Please fix the errors").WithFields(fields)
	}
	ua, ip := clientMeta(c)
	ownerName := strings.TrimSpace(body.OwnerName)
	storeName := strings.TrimSpace(body.StoreName)
	pair, err := h.svc.RegisterSeller(c.Context(), RegisterSellerInput{
		OwnerName: ownerName,
		Email:     body.Email,
		Phone:     strings.TrimSpace(body.Phone),
		Password:  body.Password,
		StoreName: storeName,
	}, ua, ip)
	if err != nil {
		return err
	}
	if h.onSellerRegistered != nil {
		go h.onSellerRegistered(ownerName, normalizeEmail(body.Email), storeName)
	}
	return httpx.Created(c, pair)
}

func (h *Handler) loginFor(accountType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body loginBody
		if err := c.BodyParser(&body); err != nil {
			return httpx.BadRequest("invalid_body", "Invalid request body")
		}
		if strings.TrimSpace(body.Email) == "" || body.Password == "" {
			return httpx.BadRequest("validation_failed", "Email and password are required")
		}
		ua, ip := clientMeta(c)
		pair, err := h.svc.Login(c.Context(), accountType, body.Email, body.Password, ua, ip)
		if err != nil {
			return err
		}
		return httpx.OK(c, pair)
	}
}

func (h *Handler) refresh(c *fiber.Ctx) error {
	var body refreshBody
	if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.RefreshToken) == "" {
		return httpx.BadRequest("invalid_body", "refresh_token is required")
	}
	ua, ip := clientMeta(c)
	pair, err := h.svc.Refresh(c.Context(), body.RefreshToken, ua, ip)
	if err != nil {
		return err
	}
	return httpx.OK(c, pair)
}

func (h *Handler) logout(c *fiber.Ctx) error {
	var body refreshBody
	if err := c.BodyParser(&body); err == nil && body.RefreshToken != "" {
		_ = h.svc.Logout(c.Context(), body.RefreshToken)
	}
	return httpx.Message(c, "Logged out")
}

func (h *Handler) me(c *fiber.Ctx) error {
	p := From(c)
	if p == nil {
		return httpx.Unauthorized("unauthenticated", "Authentication required")
	}
	user, err := h.svc.Me(c.Context(), p.UserID)
	if err != nil {
		return err
	}
	return httpx.OK(c, user)
}
