package emailtpl

import "fmt"

// OrderLine is a single line item rendered in order emails.
type OrderLine struct {
	Title    string
	Variant  string
	Qty      int
	Amount   string // pre-formatted, e.g. "₹1,299"
}

func lineRows(lines []OrderLine) string {
	rows := ""
	for _, l := range lines {
		variant := ""
		if l.Variant != "" {
			variant = `<span style="color:` + colorMuted + `;font-size:12px"> · ` + esc(l.Variant) + `</span>`
		}
		rows += fmt.Sprintf(
			`<tr><td style="padding:9px 0;border-bottom:1px solid %s;font-size:13px;color:%s">%s%s<br/><span style="color:%s;font-size:12px">Qty: %d</span></td>`+
				`<td align="right" style="padding:9px 0;border-bottom:1px solid %s;font-size:13px;font-weight:600;color:%s;white-space:nowrap">%s</td></tr>`,
			colorLine, colorInk, esc(l.Title), variant, colorMuted, l.Qty, colorLine, colorInk, esc(l.Amount))
	}
	return `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:10px 0 6px">` + rows + `</table>`
}

// ── Customer: order confirmation ────────────────────────────────────────────

type OrderConfirmationData struct {
	CustomerName string
	OrderNumber  string
	Lines        []OrderLine
	Total        string
	PaymentLabel string // "Cash on Delivery" | "Paid online"
	OrderURL     string
}

func OrderConfirmation(d OrderConfirmationData) string {
	name := d.CustomerName
	if name == "" {
		name = "there"
	}
	content := h2("Order confirmed") +
		p("Hi "+esc(name)+", we've received your order <strong>"+esc(d.OrderNumber)+"</strong>.") +
		lineRows(d.Lines) +
		fmt.Sprintf(`<table role="presentation" width="100%%" style="margin:2px 0 6px"><tr><td style="font-size:14px;font-weight:700;color:%s">Total</td><td align="right" style="font-size:15px;font-weight:700;color:%s">%s</td></tr><tr><td colspan="2" style="font-size:12px;color:%s;padding-top:4px">Payment: %s</td></tr></table>`, colorInk, colorInk, esc(d.Total), colorMuted, esc(d.PaymentLabel)) +
		btn("View your order", d.OrderURL) +
		hr() +
		note("We'll notify you when your order ships. Track it anytime from My Orders.")
	return render("Your CARLITOS order "+d.OrderNumber+" is confirmed", "We've received your order "+d.OrderNumber+".", content)
}

// ── Seller: new order alert ─────────────────────────────────────────────────

type SellerNewOrderData struct {
	StoreName   string
	OrderNumber string
	Lines       []OrderLine
	Payable     string
	DashURL     string
}

func SellerNewOrder(d SellerNewOrderData) string {
	content := h2("New order received") +
		p("Hi "+esc(d.StoreName)+", you have a new order <strong>"+esc(d.OrderNumber)+"</strong>. Please process it promptly.") +
		lineRows(d.Lines) +
		box(colorAccent, "Your payable (after commission): <strong>"+esc(d.Payable)+"</strong>") +
		btn("Manage order", d.DashURL) +
		hr() +
		note("Ship on time to keep your seller rating high.")
	return render("New order "+d.OrderNumber+" on CARLITOS", "You've received a new order: "+d.OrderNumber+".", content)
}

// ── Seller: return / exchange request ───────────────────────────────────────

type SellerReturnData struct {
	StoreName    string
	OrderNumber  string
	ReturnNumber string
	Type         string // "return" | "exchange"
	Reason       string
	Lines        []OrderLine
	DashURL      string
}

func SellerReturnRequest(d SellerReturnData) string {
	kind := "return"
	if d.Type == "exchange" {
		kind = "exchange"
	}
	reason := esc(d.Reason)
	if reason == "" {
		reason = "No reason provided."
	}
	content := h2("New "+kind+" request") +
		p("Hi "+esc(d.StoreName)+", a customer raised a "+kind+" request on order <strong>"+esc(d.OrderNumber)+"</strong>.") +
		lineRows(d.Lines) +
		box(colorAccent, "<strong>Reason:</strong> "+reason) +
		btn("Review "+kind, d.DashURL) +
		hr() +
		note("Respond promptly to keep buyers happy and protect your rating.")
	return render("New "+kind+" request on order "+d.OrderNumber, "A customer raised a "+kind+" on order "+d.OrderNumber+".", content)
}

// ── Seller: payout processed ────────────────────────────────────────────────

type SellerPayoutData struct {
	StoreName        string
	SettlementNumber string
	Amount           string // pre-formatted, e.g. "₹12,340"
	UTR              string
	PeriodStart      string
	PeriodEnd        string
	DashURL          string
}

func SellerPayoutProcessed(d SellerPayoutData) string {
	summary := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:10px 0 6px">` +
		kvRow("Settlement", esc(d.SettlementNumber))
	if d.PeriodStart != "" && d.PeriodEnd != "" {
		summary += kvRow("Period", esc(d.PeriodStart)+" – "+esc(d.PeriodEnd))
	}
	if d.UTR != "" {
		summary += kvRow("UTR reference", esc(d.UTR))
	}
	summary += kvRow("Amount credited", esc(d.Amount)) + `</table>`
	content := h2("Payout processed") +
		p("Hi "+esc(d.StoreName)+", your settlement has been paid to your registered bank account.") +
		summary +
		box("#2f855a", "<strong>"+esc(d.Amount)+"</strong> has been credited. It may take a little time to reflect in your bank.") +
		btn("View settlements", d.DashURL) +
		hr() +
		note("The CARLITOS seller team.")
	return render("Payout processed — "+d.Amount, "Your CARLITOS settlement "+d.SettlementNumber+" has been paid.", content)
}

// ── Customer: shipped / out-for-delivery / delivered ────────────────────────

type OrderStatusData struct {
	CustomerName string
	OrderNumber  string
	Headline     string
	Message      string
	Courier      string
	AWB          string
	TrackURL     string
}

func OrderStatusUpdate(d OrderStatusData) string {
	name := d.CustomerName
	if name == "" {
		name = "there"
	}
	awbBlock := ""
	if d.AWB != "" {
		awbBlock = box(colorAccent,
			"Courier: <strong>"+esc(d.Courier)+"</strong><br/>Tracking (AWB): <strong>"+esc(d.AWB)+"</strong>")
	}
	body := h2(esc(d.Headline)) +
		p("Hi "+esc(name)+", "+d.Message+" (Order <strong>"+esc(d.OrderNumber)+"</strong>).") +
		awbBlock
	if d.TrackURL != "" {
		body += btn("Track your order", d.TrackURL)
	}
	body += hr() + note("Thank you for shopping with CARLITOS.")
	return render("CARLITOS order "+d.OrderNumber+" — "+d.Headline, d.Headline+" for order "+d.OrderNumber+".", body)
}
