package ordernotify

import (
	"context"
	"strings"

	"github.com/carlitos/backend/internal/emailtpl"
)

// categoryLabel maps a ticket category code to a human-readable label for email.
func categoryLabel(code string) string {
	switch code {
	case "product":
		return "Product / Order issue"
	case "payment":
		return "Payment issue"
	case "login":
		return "Login / Account access"
	case "account":
		return "Account issue"
	case "refund":
		return "Refund query"
	default:
		return "Other"
	}
}

// supportTicketEmailData is the row we need to compose a support email.
type supportTicketEmailData struct {
	ticketNumber string
	category     string
	assignee     string // seller | admin
	subject      string
	description  string
	resolution   string
	custEmail    string
	custName     string
}

func (n *Notifier) loadSupportTicket(ctx context.Context, ticketID string) (supportTicketEmailData, error) {
	var d supportTicketEmailData
	err := n.pool.QueryRow(ctx, `
		SELECT t.ticket_number, t.category, t.assignee, coalesce(t.subject,''),
		       coalesce(t.description,''), coalesce(t.resolution,''),
		       coalesce(u.email,''), coalesce(NULLIF(c.display_name,''), u.full_name, '')
		FROM support_tickets t
		JOIN customers c ON c.id = t.customer_id
		JOIN users u ON u.id = c.user_id
		WHERE t.id = $1`, ticketID).
		Scan(&d.ticketNumber, &d.category, &d.assignee, &d.subject,
			&d.description, &d.resolution, &d.custEmail, &d.custName)
	if err != nil {
		return d, err
	}
	// Skip the synthetic phone-login email (not deliverable).
	if strings.HasSuffix(strings.ToLower(d.custEmail), "@phone.carlitos.local") {
		d.custEmail = ""
	}
	return d, nil
}

// SupportTicketOpened emails the customer a confirmation that their help
// request was received. Call from a goroutine.
func (n *Notifier) SupportTicketOpened(ctx context.Context, ticketID string) {
	d, err := n.loadSupportTicket(ctx, ticketID)
	if err != nil {
		n.logger.Error("supportnotify: load ticket", "ticket", ticketID, "error", err)
		return
	}
	if d.custEmail == "" {
		return
	}
	routed := "our support team"
	if d.assignee == "seller" {
		routed = "the seller for your product"
	}
	summary := d.description
	if len(summary) > 500 {
		summary = summary[:500] + "…"
	}
	html := emailtpl.SupportReceived(emailtpl.SupportReceivedData{
		CustomerName: d.custName,
		TicketNumber: d.ticketNumber,
		Category:     categoryLabel(d.category),
		Subject:      d.subject,
		Summary:      summary,
		RoutedTo:     routed,
		ResponseETA:  "24 hours",
		TicketURL:    n.baseURL + "/support",
	})
	n.sendEmail(ctx, d.custEmail, "We've received your request "+d.ticketNumber, html)
}

// SupportTicketResolved emails the customer the resolution when a ticket is
// marked solved. Call from a goroutine.
func (n *Notifier) SupportTicketResolved(ctx context.Context, ticketID string) {
	d, err := n.loadSupportTicket(ctx, ticketID)
	if err != nil {
		n.logger.Error("supportnotify: load ticket", "ticket", ticketID, "error", err)
		return
	}
	if d.custEmail == "" {
		return
	}
	by := "our support team"
	if d.assignee == "seller" {
		by = "the seller"
	}
	html := emailtpl.SupportResolved(emailtpl.SupportResolvedData{
		CustomerName: d.custName,
		TicketNumber: d.ticketNumber,
		Subject:      d.subject,
		Resolution:   d.resolution,
		ResolvedBy:   by,
		TicketURL:    n.baseURL + "/support",
	})
	n.sendEmail(ctx, d.custEmail, "Your request "+d.ticketNumber+" is resolved", html)
}
