// Package ordernotify orchestrates customer/seller notifications for order
// lifecycle events. It loads the recipients and line items from the database
// and dispatches emails via the notify.Dispatcher. WhatsApp is intentionally
// deferred: the WhatsApp hooks are left as no-ops until a sender is wired.
//
// All sends are best-effort and run off the request path (the caller invokes
// these from a goroutine). Failures are logged, never surfaced to the user, and
// never block the order flow.
package ordernotify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/notify"
)

// Notifier sends order notifications. It owns only read access to the DB (for
// recipients/line items) plus the dispatcher.
type Notifier struct {
	pool       *pgxpool.Pool
	dispatcher *notify.Dispatcher
	logger     *slog.Logger
	baseURL    string // storefront base URL for links
	sellerURL  string // seller dashboard base URL for links
}

// New builds an order Notifier.
func New(pool *pgxpool.Pool, d *notify.Dispatcher, logger *slog.Logger, baseURL string) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{
		pool:       pool,
		dispatcher: d,
		logger:     logger,
		baseURL:    strings.TrimRight(baseURL, "/"),
		sellerURL:  strings.TrimRight(baseURL, "/") + "/seller",
	}
}

func inr(v float64) string {
	// Simple grouped INR formatting: ₹1,23,456 (Indian grouping).
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(v + 0.5)
	s := fmt.Sprintf("%d", whole)
	// Indian grouping: last 3 digits, then groups of 2.
	n := len(s)
	if n > 3 {
		head := s[:n-3]
		tail := s[n-3:]
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
		return "-₹" + s
	}
	return "₹" + s
}

type recipient struct {
	email string
	name  string
}

// loadCustomer returns the customer's email + display name for an order. The
// synthetic phone-login email (…@phone.carlitos.local) is skipped so we don't
// email a non-deliverable address.
func (n *Notifier) loadCustomer(ctx context.Context, orderID string) (recipient, error) {
	var r recipient
	err := n.pool.QueryRow(ctx, `
		SELECT coalesce(u.email,''), coalesce(NULLIF(c.display_name,''), u.full_name, '')
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		JOIN users u ON u.id = c.user_id
		WHERE o.id = $1`, orderID).Scan(&r.email, &r.name)
	if err != nil {
		return r, err
	}
	if strings.HasSuffix(strings.ToLower(r.email), "@phone.carlitos.local") {
		r.email = "" // not a real, deliverable address
	}
	return r, nil
}

// loadOrderLinesForCustomer returns all line items of an order (all sellers).
func (n *Notifier) loadOrderLines(ctx context.Context, orderID, sellerID string) ([]emailtpl.OrderLine, error) {
	q := `SELECT product_title, coalesce(variant_label,''), quantity, gross_amount
	      FROM order_items WHERE order_id = $1`
	args := []any{orderID}
	if sellerID != "" {
		q += ` AND seller_id = $2`
		args = append(args, sellerID)
	}
	q += ` ORDER BY created_at`
	rows, err := n.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []emailtpl.OrderLine
	for rows.Next() {
		var title, variant string
		var qty int
		var amount float64
		if err := rows.Scan(&title, &variant, &qty, &amount); err != nil {
			return nil, err
		}
		out = append(out, emailtpl.OrderLine{Title: title, Variant: variant, Qty: qty, Amount: inr(amount)})
	}
	return out, rows.Err()
}

type orderMeta struct {
	number        string
	grandTotal    float64
	paymentMethod string
}

func (n *Notifier) loadOrderMeta(ctx context.Context, orderID string) (orderMeta, error) {
	var m orderMeta
	err := n.pool.QueryRow(ctx,
		`SELECT order_number, grand_total, payment_method FROM orders WHERE id = $1`, orderID).
		Scan(&m.number, &m.grandTotal, &m.paymentMethod)
	return m, err
}

// sellerRecipient holds a seller's contact + their payable + lines for an order.
type sellerRecipient struct {
	sellerID  string
	email     string
	storeName string
	payable   float64
}

func (n *Notifier) loadOrderSellers(ctx context.Context, orderID string) ([]sellerRecipient, error) {
	rows, err := n.pool.Query(ctx, `
		SELECT oi.seller_id, coalesce(s.contact_email,''), coalesce(s.store_name,''),
		       coalesce(sum(oi.seller_payable),0)
		FROM order_items oi
		JOIN sellers s ON s.id = oi.seller_id
		WHERE oi.order_id = $1
		GROUP BY oi.seller_id, s.contact_email, s.store_name`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sellerRecipient
	for rows.Next() {
		var sr sellerRecipient
		if err := rows.Scan(&sr.sellerID, &sr.email, &sr.storeName, &sr.payable); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

func (n *Notifier) sendEmail(ctx context.Context, to, subject, html string) {
	if to == "" {
		return
	}
	if err := n.dispatcher.Email.Send(ctx, notify.EmailMessage{
		To: to, Subject: subject, Body: html, IsHTML: true,
	}); err != nil {
		n.logger.Error("order email failed", "to", to, "subject", subject, "error", err)
	}
}

// ── Public events ───────────────────────────────────────────────────────────

// OrderPlaced sends the confirmation to the customer and a new-order alert to
// each seller in the order. Call from a goroutine.
func (n *Notifier) OrderPlaced(ctx context.Context, orderID string) {
	meta, err := n.loadOrderMeta(ctx, orderID)
	if err != nil {
		n.logger.Error("ordernotify: load meta", "order", orderID, "error", err)
		return
	}

	// Customer confirmation.
	cust, err := n.loadCustomer(ctx, orderID)
	if err == nil && cust.email != "" {
		lines, _ := n.loadOrderLines(ctx, orderID, "")
		payLabel := "Cash on Delivery"
		if meta.paymentMethod == "online" {
			payLabel = "Paid online"
		}
		html := emailtpl.OrderConfirmation(emailtpl.OrderConfirmationData{
			CustomerName: cust.name,
			OrderNumber:  meta.number,
			Lines:        lines,
			Total:        inr(meta.grandTotal),
			PaymentLabel: payLabel,
			OrderURL:     n.baseURL + "/orders",
		})
		n.sendEmail(ctx, cust.email, "Your CARLITOS order "+meta.number+" is confirmed", html)
	}

	// Seller alerts.
	sellers, err := n.loadOrderSellers(ctx, orderID)
	if err != nil {
		n.logger.Error("ordernotify: load sellers", "order", orderID, "error", err)
		return
	}
	for _, sr := range sellers {
		if sr.email == "" {
			continue
		}
		lines, _ := n.loadOrderLines(ctx, orderID, sr.sellerID)
		html := emailtpl.SellerNewOrder(emailtpl.SellerNewOrderData{
			StoreName:   sr.storeName,
			OrderNumber: meta.number,
			Lines:       lines,
			Payable:     inr(sr.payable),
			DashURL:     n.sellerURL + "/orders",
		})
		n.sendEmail(ctx, sr.email, "New order "+meta.number+" on CARLITOS", html)
	}
}

// OrderProcessing tells the customer the seller has accepted the order and is
// preparing it for shipment. Call from a goroutine.
func (n *Notifier) OrderProcessing(ctx context.Context, orderID string) {
	n.StatusUpdate(ctx, orderID,
		"Your order is confirmed & being prepared",
		"the seller has accepted your order and is getting it ready to ship", "", "")
}

// OrderCancelledBySeller tells the customer their order was cancelled by the
// seller. Call from a goroutine.
func (n *Notifier) OrderCancelledBySeller(ctx context.Context, orderID string) {
	n.StatusUpdate(ctx, orderID,
		"Your order was cancelled",
		"the seller could not fulfil this order and it has been cancelled; any payment will be refunded", "", "")
}

// ReturnRequested emails the affected seller when a customer raises a return or
// exchange. reqType is "return" or "exchange". Call from a goroutine.
func (n *Notifier) ReturnRequested(ctx context.Context, returnID string) {
	var (
		orderNumber, returnNumber, reqType, reason string
		sellerID, storeName, sellerEmail, orderID  string
	)
	err := n.pool.QueryRow(ctx, `
		SELECT coalesce(o.order_number,''), coalesce(r.return_number,''), r.type,
		       coalesce(r.reason,''), r.seller_id, coalesce(s.store_name,''),
		       coalesce(s.contact_email,''), r.order_id
		FROM returns r
		JOIN orders o ON o.id = r.order_id
		JOIN sellers s ON s.id = r.seller_id
		WHERE r.id = $1`, returnID).
		Scan(&orderNumber, &returnNumber, &reqType, &reason, &sellerID, &storeName, &sellerEmail, &orderID)
	if err != nil {
		n.logger.Error("ordernotify: load return", "return", returnID, "error", err)
		return
	}
	if sellerEmail == "" {
		return
	}
	lines, _ := n.loadOrderLines(ctx, orderID, sellerID)
	html := emailtpl.SellerReturnRequest(emailtpl.SellerReturnData{
		StoreName:    storeName,
		OrderNumber:  orderNumber,
		ReturnNumber: returnNumber,
		Type:         reqType,
		Reason:       reason,
		Lines:        lines,
		DashURL:      n.sellerURL + "/returns",
	})
	kind := "return"
	if reqType == "exchange" {
		kind = "exchange"
	}
	n.sendEmail(ctx, sellerEmail, "New "+kind+" request on order "+orderNumber, html)
}

// PayoutProcessed emails the seller when a settlement is marked settled (paid).
// Call from a goroutine.
func (n *Notifier) PayoutProcessed(ctx context.Context, settlementID string) {
	var (
		number, utr, storeName, sellerEmail string
		periodStart, periodEnd              string
		netPayable                          float64
	)
	err := n.pool.QueryRow(ctx, `
		SELECT coalesce(ss.settlement_number,''), coalesce(ss.utr_reference,''),
		       coalesce(s.store_name,''), coalesce(s.contact_email,''),
		       ss.period_start::text, ss.period_end::text, ss.net_payable
		FROM seller_settlements ss
		JOIN sellers s ON s.id = ss.seller_id
		WHERE ss.id = $1`, settlementID).
		Scan(&number, &utr, &storeName, &sellerEmail, &periodStart, &periodEnd, &netPayable)
	if err != nil {
		n.logger.Error("ordernotify: load settlement", "settlement", settlementID, "error", err)
		return
	}
	if sellerEmail == "" {
		return
	}
	html := emailtpl.SellerPayoutProcessed(emailtpl.SellerPayoutData{
		StoreName:        storeName,
		SettlementNumber: number,
		Amount:           inr(netPayable),
		UTR:              utr,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		DashURL:          n.sellerURL + "/settlements",
	})
	n.sendEmail(ctx, sellerEmail, "Payout processed — "+inr(netPayable), html)
}

// StatusUpdate notifies the customer of a shipment/delivery status change.
// headline/message are human-readable; awb/courier are optional.
func (n *Notifier) StatusUpdate(ctx context.Context, orderID, headline, message, courier, awb string) {
	meta, err := n.loadOrderMeta(ctx, orderID)
	if err != nil {
		return
	}
	cust, err := n.loadCustomer(ctx, orderID)
	if err != nil || cust.email == "" {
		return
	}
	html := emailtpl.OrderStatusUpdate(emailtpl.OrderStatusData{
		CustomerName: cust.name,
		OrderNumber:  meta.number,
		Headline:     headline,
		Message:      message,
		Courier:      courier,
		AWB:          awb,
		TrackURL:     n.baseURL + "/orders",
	})
	n.sendEmail(ctx, cust.email, "CARLITOS order "+meta.number+" — "+headline, html)
}
