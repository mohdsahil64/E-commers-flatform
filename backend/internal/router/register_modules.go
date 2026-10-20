package router

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/carlitos/backend/internal/app"
	"github.com/carlitos/backend/internal/audit"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/cart"
	"github.com/carlitos/backend/internal/catalog"
	"github.com/carlitos/backend/internal/coupons"
	"github.com/carlitos/backend/internal/customers"
	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/fulfillment"
	"github.com/carlitos/backend/internal/inventory"
	"github.com/carlitos/backend/internal/merchandising"
	"github.com/carlitos/backend/internal/notifications"
	"github.com/carlitos/backend/internal/notify"
	"github.com/carlitos/backend/internal/ordernotify"
	"github.com/carlitos/backend/internal/orders"
	"github.com/carlitos/backend/internal/packaging"
	"github.com/carlitos/backend/internal/payments"
	"github.com/carlitos/backend/internal/returns"
	"github.com/carlitos/backend/internal/reviews"
	"github.com/carlitos/backend/internal/sellers"
	"github.com/carlitos/backend/internal/settlements"
	"github.com/carlitos/backend/internal/shipping"
	"github.com/carlitos/backend/internal/storage"
	"github.com/carlitos/backend/internal/support"
	"github.com/carlitos/backend/internal/team"
	"github.com/carlitos/backend/internal/uploads"
)

func registerModules(a *app.App, v1, public, seller, admin, authGroup fiber.Router) {
	pool := a.DB.Pool
	cfg := a.Config
	ctx := context.Background()

	storageProvider, err := storage.New(cfg.Storage)
	if err != nil {
		a.Logger.Error("storage init failed; falling back", "error", err)
	}
	paymentProvider := payments.NewProvider(cfg.Payment)
	courierProvider := shipping.NewProvider(cfg.Shipping)
	dispatcher := notify.NewDispatcher(cfg, a.Logger)

	auditLog := audit.New(pool)
	notifStore := notifications.NewStore(pool)

	tokenManager := auth.NewManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	authRepo := auth.NewRepository(pool)
	// Mobile-OTP (seller registration + customer login): in-memory challenge
	// store + a sender selected by config. Default "log" is the dev/test mode —
	// OTPs are logged and the fixed test code 123456 is accepted (no real SMS).
	// Set SMS_PROVIDER=apihome with SMS_API_KEY to send real OTP SMS via the
	// apihome.in bulksms gateway; the rest of the flow is unchanged.
	otpStore := auth.NewOTPStore(5 * time.Minute)
	var otpSender auth.OTPSender
	if strings.EqualFold(cfg.SMS.Provider, "apihome") && cfg.SMS.APIKey != "" {
		otpSender = auth.NewAPIHomeOTPSender(cfg.SMS.APIKey, cfg.SMS.BaseURL, a.Logger)
		a.Logger.Info("otp sender: apihome (real SMS)")
	} else {
		otpSender = auth.NewLogOTPSender(a.Logger)
		a.Logger.Info("otp sender: log (dev/test — code 123456)")
	}
	authService := auth.NewService(authRepo, tokenManager, cfg.JWT.BcryptCost).
		WithOTP(otpStore, otpSender).
		WithEmailVerifySender(func(ctx context.Context, to, subject, html string) error {
			return dispatcher.Email.Send(ctx, notify.EmailMessage{To: to, Subject: subject, Body: html, IsHTML: true})
		}).
		WithPasswordResetSender(func(ctx context.Context, e auth.PasswordResetEmail) {
			// Build the reset link into the relevant portal's web app. Both the
			// seller and admin SPAs read the token from the ?token= query param
			// on their /reset-password route.
			portalPath := "/seller/reset-password"
			if e.Portal == "admin" {
				portalPath = "/admin/reset-password"
			}
			resetURL := cfg.App.BaseURL + portalPath + "?token=" + url.QueryEscape(e.Token)
			html := emailtpl.PasswordReset(emailtpl.PasswordResetData{
				Name:       e.Name,
				Portal:     e.Portal,
				ResetURL:   resetURL,
				ExpiryMins: e.ExpiryMins,
			})
			// Best-effort send on a background context so a slow SMTP send never
			// delays the (already-generic) HTTP response.
			go func() {
				_ = dispatcher.Email.Send(context.Background(), notify.EmailMessage{
					To:      e.To,
					Subject: "Reset your CARLITOS password",
					Body:    html,
					IsHTML:  true,
				})
			}()
		})

	customerAuth := auth.Middleware(tokenManager, "customer")
	sellerAuth := auth.Middleware(tokenManager, "seller")
	adminAuth := auth.Middleware(tokenManager, "admin")
	anyAuth := auth.Middleware(tokenManager, "")

	sellerRepo := sellers.NewRepository(pool)
	sellerHook := func(ctx context.Context, sellerID, event, actorID string) {
		a.Logger.Info("seller lifecycle", "seller", sellerID, "event", event)
		auditLog.Record(ctx, actorID, "admin", event, "seller", sellerID, nil, nil, "", "")
		s, err := sellerRepo.FindByID(ctx, sellerID)
		if err != nil {
			return
		}
		title, plainBody := sellerEventMessage(event)
		_ = notifStore.Create(ctx, s.UserID, event, title, plainBody, map[string]any{"seller_id": sellerID})
		htmlBody, subject := sellerEventHTML(event, s.OwnerName, s.StoreName, cfg.App.BaseURL)
		if htmlBody == "" {
			_ = dispatcher.Email.Send(ctx, notify.EmailMessage{To: s.ContactEmail, Subject: title, Body: plainBody})
			return
		}
		_ = dispatcher.Email.Send(ctx, notify.EmailMessage{To: s.ContactEmail, Subject: subject, Body: htmlBody, IsHTML: true})
	}
	sellerService := sellers.NewService(sellerRepo, sellerHook).
		WithDefaultCommission(cfg.DefaultCommissionPercent).
		WithBcryptCost(cfg.JWT.BcryptCost)

	invRepo := inventory.NewRepository(pool)

	taxRepo := catalog.NewTaxonomyRepository(pool)
	prodRepo := catalog.NewProductRepository(pool)
	storeRepo := catalog.NewStorefrontRepository(pool)
	catalogService := catalog.NewService(taxRepo, prodRepo, storeRepo, invRepo).
		WithAuditHook(func(ctx context.Context, actorID, action, productID string) {
			auditLog.Action(ctx, actorID, "admin", action, "product", productID)
		})

	cartRepo := cart.NewRepository(pool)

	// Order notifications (email now; WhatsApp deferred). Best-effort, off the
	// request path — the hook fires goroutines with a background context so a
	// slow SMTP send never delays the order response.
	orderNotifier := ordernotify.New(pool, dispatcher, a.Logger, cfg.App.BaseURL)

	orderRepo := orders.NewRepository(pool)
	orderHook := func(ctx context.Context, orderID, event, actorID string) {
		a.Logger.Info("order lifecycle", "order", orderID, "event", event)
		// Record consequential lifecycle events (status changes, cancellations)
		// to the audit log. Order creation/confirmation are high-volume and
		// already captured in order_status_history, so we focus on admin/seller
		// driven transitions and cancellations here.
		switch event {
		case "order.cancelled", "order.refunded", "order.confirmed":
			role := "system"
			if actorID != "" {
				role = "actor"
			}
			auditLog.Action(ctx, actorID, role, event, "order", orderID)
		}
		// Send lifecycle emails off the request path (background context so a
		// cancelled request context never aborts the send).
		switch event {
		case "order.created":
			go orderNotifier.OrderPlaced(context.Background(), orderID)
		case "order.processing":
			go orderNotifier.OrderProcessing(context.Background(), orderID)
		case "order.seller_cancelled":
			go orderNotifier.OrderCancelledBySeller(context.Background(), orderID)
		}
	}
	couponRepo := coupons.NewRepository(pool)
	couponService := coupons.NewService(couponRepo)

	orderService := orders.NewService(orderRepo, cartRepo, invRepo, cfg.DefaultCommissionPercent, orderHook).
		WithCoupons(couponService)

	paymentService := payments.NewService(pool, paymentProvider, orderService, cfg.Payment.Currency)

	shipRepo := fulfillment.NewRepository(pool)
	shipService := fulfillment.NewService(shipRepo, courierProvider).
		WithBaseURL(cfg.App.BaseURL).
		WithNotifier(func(ctx context.Context, orderID, headline, message, courier, awb string) {
			orderNotifier.StatusUpdate(ctx, orderID, headline, message, courier, awb)
		})

	returnRepo := returns.NewRepository(pool)
	returnService := returns.NewService(returnRepo).
		WithNotifier(func(ctx context.Context, returnID string) {
			go orderNotifier.ReturnRequested(context.Background(), returnID)
		})

	// Help & Support ticketing. On create/resolve we email the customer off the
	// request path (background context so a slow SMTP send never blocks the API).
	supportRepo := support.NewRepository(pool)
	supportService := support.NewService(supportRepo).
		WithNotifier(func(ctx context.Context, ticketID, event string) {
			switch event {
			case "created":
				go orderNotifier.SupportTicketOpened(context.Background(), ticketID)
			case "resolved":
				go orderNotifier.SupportTicketResolved(context.Background(), ticketID)
			}
		})

	reviewRepo := reviews.NewRepository(pool)
	reviewService := reviews.NewService(reviewRepo)

	merchRepo := merchandising.NewRepository(pool)
	teamRepo := team.NewRepository(pool)

	packRepo := packaging.NewRepository(pool)
	packService := packaging.NewService(packRepo)

	settleRepo := settlements.NewRepository(pool)
	settleService := settlements.NewService(settleRepo).
		WithAuditHook(func(ctx context.Context, actorID, action, settlementID string) {
			auditLog.Action(ctx, actorID, "admin", action, "settlement", settlementID)
		}).
		WithPayoutHook(func(ctx context.Context, settlementID string) {
			go orderNotifier.PayoutProcessed(context.Background(), settlementID)
		})

	custRepo := customers.NewRepository(pool)
	custService := customers.NewService(custRepo).
		WithEmailVerification(
			otpStore, // reuse the auth OTP store (dev code 123456 accepted)
			func(ctx context.Context, to, subject, html string) error {
				return dispatcher.Email.Send(ctx, notify.EmailMessage{To: to, Subject: subject, Body: html, IsHTML: true})
			},
			auth.GenerateOTPCode,
		)

	uploadHandler := uploads.NewHandler(storageProvider)

	sellerLoginURL := cfg.App.BaseURL + "/seller/login"
	authHandler := auth.NewHandler(authService, tokenManager).
		WithSellerRegisteredHook(func(ownerName, email, storeName string) {
			htmlBody := emailtpl.SellerWelcome(emailtpl.SellerWelcomeData{
				OwnerName: ownerName,
				StoreName: storeName,
				LoginURL:  sellerLoginURL,
			})
			_ = dispatcher.Email.Send(ctx, notify.EmailMessage{
				To:      email,
				Subject: "Welcome to CARLITOS — let's start selling!",
				Body:    htmlBody,
				IsHTML:  true,
			})
		})
	// Rate-limit authentication endpoints (login/refresh/register) to blunt
	// brute-force and credential-stuffing. Keyed by client IP; a 429 is returned
	// with the standard error envelope when the limit is exceeded.
	authGroup.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return httpx.Fail(c, fiber.StatusTooManyRequests, "rate_limited",
				"Too many attempts. Please try again in a minute.")
		},
	}))
	authHandler.Register(authGroup)

	fulfillmentHandler := fulfillment.NewHandler(shipService, sellerRepo)

	catalog.NewHandler(catalogService, sellerRepo).RegisterPublic(public)
	merchandising.NewHandler(merchRepo).RegisterPublic(public)
	team.NewHandler(teamRepo).RegisterPublic(public)
	reviews.NewHandler(reviewService).RegisterPublic(public)
	payments.NewHandler(paymentService).RegisterPublic(public)
	fulfillmentHandler.RegisterPublic(public, cfg.Shipping.WebhookToken)

	public.Get("/sellers/:slug", func(c *fiber.Ctx) error {
		return publicSellerStore(c, sellerRepo)
	})

	couponHandler := coupons.NewHandler(couponService)

	customerGrp := v1.Group("/customer", customerAuth)
	customers.NewHandler(custService).RegisterCustomer(customerGrp)
	cart.NewHandler(cartRepo).Register(customerGrp)
	couponHandler.RegisterCustomer(customerGrp, func(c *fiber.Ctx) (string, error) {
		return cartRepo.CustomerID(c.Context(), auth.From(c).UserID)
	})
	orders.NewHandler(orderService, sellerRepo).RegisterCustomer(customerGrp)
	fulfillmentHandler.RegisterCustomer(customerGrp)
	payments.NewHandler(paymentService).RegisterCustomer(customerGrp)
	returns.NewHandler(returnService, sellerRepo).RegisterCustomer(customerGrp)
	reviews.NewHandler(reviewService).RegisterCustomer(customerGrp)
	support.NewHandler(supportService, sellerRepo).RegisterCustomer(customerGrp)
	notifications.NewHandler(notifStore).Register(customerGrp)
	uploadHandler.Register(customerGrp)

	seller.Use(sellerAuth)
	sellers.NewHandler(sellerService).RegisterSeller(seller)
	catalog.NewHandler(catalogService, sellerRepo).RegisterSeller(seller)
	orders.NewHandler(orderService, sellerRepo).RegisterSeller(seller)
	fulfillmentHandler.RegisterSeller(seller)
	returns.NewHandler(returnService, sellerRepo).RegisterSeller(seller)
	support.NewHandler(supportService, sellerRepo).RegisterSeller(seller)
	packaging.NewHandler(packService, sellerRepo).RegisterSeller(seller)
	settlements.NewHandler(settleService, sellerRepo).RegisterSeller(seller)
	notifications.NewHandler(notifStore).Register(seller)
	uploadHandler.Register(seller)

	admin.Use(adminAuth)
	sellers.NewHandler(sellerService).RegisterAdmin(admin)
	catalog.NewHandler(catalogService, sellerRepo).RegisterAdmin(admin, tokenManager)
	orders.NewHandler(orderService, sellerRepo).RegisterAdmin(admin)
	merchandising.NewHandler(merchRepo).RegisterAdmin(admin)
	team.NewHandler(teamRepo).RegisterAdmin(admin)
	couponHandler.RegisterAdmin(admin)
	returns.NewHandler(returnService, sellerRepo).RegisterAdmin(admin)
	support.NewHandler(supportService, sellerRepo).RegisterAdmin(admin)
	reviews.NewHandler(reviewService).RegisterAdmin(admin)
	packaging.NewHandler(packService, sellerRepo).RegisterAdmin(admin)
	settlements.NewHandler(settleService, sellerRepo).RegisterAdmin(admin)
	audit.NewHandler(auditLog).RegisterAdmin(admin)
	notifications.NewHandler(notifStore).Register(admin)
	uploadHandler.Register(admin) // admin image uploads (banners, etc.)

	v1.Get("/me/notifications", anyAuth, func(c *fiber.Ctx) error {
		return notificationsPassthrough(c, notifStore)
	})

	_ = ctx
}

func sellerEventMessage(event string) (title, body string) {
	switch event {
	case "seller.approved":
		return "Seller account approved", "Congratulations! Your CARLITOS seller account has been approved. You can now start listing products."
	case "seller.rejected":
		return "Seller application rejected", "Your seller application was not approved. Please review the notes and re-apply."
	case "seller.suspended":
		return "Seller account suspended", "Your seller account has been suspended. Please contact support."
	case "seller.blocked":
		return "Seller account blocked", "Your seller account has been blocked."
	case "seller.activated", "seller.unsuspended", "seller.unblocked":
		return "Seller account reactivated", "Your seller account is active again."
	case "seller.submitted":
		return "Application submitted", "Your onboarding application has been submitted and is under review."
	default:
		return "Account update", "There is an update on your seller account."
	}
}

func sellerEventHTML(event, ownerName, storeName, baseURL string) (string, string) {
	supportURL := "mailto:seller-support@carlitos.in"
	dashURL := baseURL + "/seller/dashboard"
	reApplyURL := baseURL + "/seller/onboarding"

	switch event {
	case "seller.approved":
		return emailtpl.SellerApproved(emailtpl.SellerApprovedData{
			OwnerName: ownerName, StoreName: storeName,
			DashURL: dashURL, ListingURL: dashURL + "/products/new",
		}), "Your CARLITOS seller account is approved — start selling today!"
	case "seller.rejected":
		return emailtpl.SellerRejected(emailtpl.SellerRejectedData{
			OwnerName: ownerName, StoreName: storeName, ReApplyURL: reApplyURL,
		}), "CARLITOS — Update on your seller application"
	case "seller.suspended":
		return emailtpl.SellerSuspended(emailtpl.SellerSuspendedData{
			OwnerName: ownerName, StoreName: storeName, SupportURL: supportURL,
		}), "Your CARLITOS seller account has been suspended"
	case "seller.submitted":
		return emailtpl.SellerSubmitted(emailtpl.SellerSubmittedData{
			OwnerName: ownerName, StoreName: storeName,
		}), "CARLITOS — Your seller application is under review"
	default:
		return "", ""
	}
}
