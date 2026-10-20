// Command emailtest renders every email template and sends each one to a test
// recipient using the app's configured email provider (SMTP in this project).
//
// Usage:
//
//	go run ./cmd/emailtest                       # sends to the default test inbox
//	go run ./cmd/emailtest you@example.com       # sends to a custom address
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/carlitos/backend/internal/config"
	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/notify"
	"github.com/carlitos/backend/internal/applog"
)

func main() {
	to := "mohdsahil6464@gmail.com"
	if len(os.Args) > 1 && os.Args[1] != "" {
		to = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	logger := applog.New(cfg.App.LogLevel, cfg.App.Env)
	dispatcher := notify.NewDispatcher(cfg, logger)
	fmt.Printf("email provider: %s  →  sending all templates to %s\n\n", dispatcher.Email.Name(), to)

	owner := "Mohd Sahil"
	store := "Sahil Store"
	dash := cfg.App.BaseURL + "/seller"

	lines := []emailtpl.OrderLine{
		{Title: "Cotton Casual Shirt", Variant: "M / Blue", Qty: 2, Amount: "₹1,998"},
		{Title: "Slim-Fit Jeans", Variant: "32 / Black", Qty: 1, Amount: "₹1,499"},
	}

	// name -> (subject, html)
	type mail struct {
		subject string
		html    string
	}
	mails := []struct {
		key string
		m   mail
	}{
		{"1. Email verification (OTP)", mail{
			"Your CARLITOS verification code: 482913",
			emailtpl.SellerOTP(emailtpl.SellerOTPData{OwnerName: owner, OTP: "482913", ExpiryMins: 5, Purpose: "verify your email address"}),
		}},
		{"2. Welcome", mail{
			"Welcome to CARLITOS",
			emailtpl.SellerWelcome(emailtpl.SellerWelcomeData{OwnerName: owner, StoreName: store, LoginURL: dash}),
		}},
		{"3. Application submitted (under review)", mail{
			"Your CARLITOS application is under review",
			emailtpl.SellerSubmitted(emailtpl.SellerSubmittedData{OwnerName: owner, StoreName: store}),
		}},
		{"4. Approved", mail{
			"Your CARLITOS seller account is approved",
			emailtpl.SellerApproved(emailtpl.SellerApprovedData{OwnerName: owner, StoreName: store, DashURL: dash, ListingURL: dash + "/products/new"}),
		}},
		{"5. Rejected", mail{
			"Update on your CARLITOS seller application",
			emailtpl.SellerRejected(emailtpl.SellerRejectedData{OwnerName: owner, StoreName: store, Reason: "PAN details did not match our records.", ReApplyURL: dash + "/onboarding"}),
		}},
		{"6. Suspended", mail{
			"Your CARLITOS seller account has been suspended",
			emailtpl.SellerSuspended(emailtpl.SellerSuspendedData{OwnerName: owner, StoreName: store, Reason: "Multiple late dispatches.", SupportURL: "mailto:seller-support@carlitos.in"}),
		}},
		{"7. New order (seller)", mail{
			"New order CRL-2026-000123 on CARLITOS",
			emailtpl.SellerNewOrder(emailtpl.SellerNewOrderData{StoreName: store, OrderNumber: "CRL-2026-000123", Lines: lines, Payable: "₹3,077", DashURL: dash + "/orders"}),
		}},
		{"8. Return request (seller)", mail{
			"New return request on order CRL-2026-000123",
			emailtpl.SellerReturnRequest(emailtpl.SellerReturnData{StoreName: store, OrderNumber: "CRL-2026-000123", ReturnNumber: "RET-2026-0007", Type: "return", Reason: "Item did not fit.", Lines: lines[:1], DashURL: dash + "/returns"}),
		}},
		{"9. Exchange request (seller)", mail{
			"New exchange request on order CRL-2026-000123",
			emailtpl.SellerReturnRequest(emailtpl.SellerReturnData{StoreName: store, OrderNumber: "CRL-2026-000123", ReturnNumber: "RET-2026-0008", Type: "exchange", Reason: "Wrong colour delivered.", Lines: lines[:1], DashURL: dash + "/returns"}),
		}},
		{"10. Payout processed (seller)", mail{
			"Payout processed — ₹12,340",
			emailtpl.SellerPayoutProcessed(emailtpl.SellerPayoutData{StoreName: store, SettlementNumber: "STL-2026-0031", Amount: "₹12,340", UTR: "HDFCN12345678", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-15", DashURL: dash + "/settlements"}),
		}},
		{"11. Password reset", mail{
			"Reset your CARLITOS password",
			emailtpl.PasswordReset(emailtpl.PasswordResetData{Name: owner, Portal: "seller", ResetURL: dash + "/reset-password?token=demo-token", ExpiryMins: 30}),
		}},
		{"12. Order confirmation (customer)", mail{
			"Your CARLITOS order CRL-2026-000123 is confirmed",
			emailtpl.OrderConfirmation(emailtpl.OrderConfirmationData{CustomerName: owner, OrderNumber: "CRL-2026-000123", Lines: lines, Total: "₹3,497", PaymentLabel: "Cash on Delivery", OrderURL: cfg.App.BaseURL + "/orders"}),
		}},
		{"13. Order shipped (customer)", mail{
			"CARLITOS order CRL-2026-000123 — Your order has shipped",
			emailtpl.OrderStatusUpdate(emailtpl.OrderStatusData{CustomerName: owner, OrderNumber: "CRL-2026-000123", Headline: "Your order has shipped", Message: "your order is on its way", Courier: "Delhivery", AWB: "DLV123456789", TrackURL: cfg.App.BaseURL + "/orders"}),
		}},
	}

	ctx := context.Background()
	ok, fail := 0, 0
	for _, item := range mails {
		err := dispatcher.Email.Send(ctx, notify.EmailMessage{
			To: to, Subject: item.m.subject, Body: item.m.html, IsHTML: true,
		})
		if err != nil {
			fail++
			fmt.Printf("  ✗ %-42s  %v\n", item.key, err)
		} else {
			ok++
			fmt.Printf("  ✓ %-42s  sent\n", item.key)
		}
		// Small gap to be gentle on the SMTP relay's rate limits.
		time.Sleep(700 * time.Millisecond)
	}
	fmt.Printf("\nDone: %d sent, %d failed → %s\n", ok, fail, to)
}
