package emailtpl

// Support (Help Centre) emails. Two touch points:
//   1. SupportReceived  — sent the instant a customer opens a ticket.
//   2. SupportResolved  — sent when the responder marks it solved, carrying the
//      resolution note (the "what happened / what we did" explanation).

// SupportReceivedData is the payload for the ticket-received confirmation.
type SupportReceivedData struct {
	CustomerName string
	TicketNumber string
	Category     string // human-readable category label
	Subject      string
	Summary      string // the customer's own description (trimmed by caller)
	RoutedTo     string // "our support team" | "the seller"
	ResponseETA  string // e.g. "24 hours"
	TicketURL    string // link to the customer's support requests page
}

// SupportReceived renders the "we've received your request" email.
func SupportReceived(d SupportReceivedData) string {
	name := esc(d.CustomerName)
	if name == "" {
		name = "there"
	}
	eta := d.ResponseETA
	if eta == "" {
		eta = "24 hours"
	}
	routed := d.RoutedTo
	if routed == "" {
		routed = "our support team"
	}

	summaryBlock := ""
	if d.Summary != "" {
		summaryBlock = box(colorAccent, "<strong>Your message:</strong><br/>"+esc(d.Summary))
	}

	detail := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:14px 0 6px">` +
		kvRow("Request ID", esc(d.TicketNumber)) +
		kvRow("Category", esc(d.Category)) +
		kvRow("Subject", esc(d.Subject)) +
		`</table>`

	content := h2("We've got your request") +
		p("Hi "+name+", thanks for reaching out. Your help request has been received and routed to "+esc(routed)+".") +
		detail +
		summaryBlock +
		box("#2f855a", "You can expect a response within <strong>"+esc(eta)+"</strong>. We'll email you the moment it's resolved.") +
		btn("View your request", d.TicketURL) +
		hr() +
		note("Please keep this Request ID handy for any follow-up.")
	return render("We've received your request ("+d.TicketNumber+")",
		"Your CARLITOS help request "+d.TicketNumber+" has been received.", content)
}

// SupportResolvedData is the payload for the resolution email.
type SupportResolvedData struct {
	CustomerName string
	TicketNumber string
	Subject      string
	Resolution   string // the responder's resolution note (why / what was done)
	ResolvedBy   string // "our support team" | "the seller"
	TicketURL    string
}

// SupportResolved renders the "your request is resolved" email.
func SupportResolved(d SupportResolvedData) string {
	name := esc(d.CustomerName)
	if name == "" {
		name = "there"
	}
	by := d.ResolvedBy
	if by == "" {
		by = "our support team"
	}

	detail := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:14px 0 6px">` +
		kvRow("Request ID", esc(d.TicketNumber)) +
		kvRow("Subject", esc(d.Subject)) +
		kvRow("Status", "Solved") +
		`</table>`

	content := h2("Your request is resolved") +
		p("Hi "+name+", good news — your help request has been resolved by "+esc(by)+".") +
		detail +
		box("#2f855a", "<strong>What happened & how we fixed it:</strong><br/>"+esc(d.Resolution)) +
		p("If this didn't fully solve your problem, just reply from your help requests and we'll pick it back up.") +
		btn("View your request", d.TicketURL) +
		hr() +
		note("Thank you for shopping with CARLITOS.")
	return render("Your request "+d.TicketNumber+" is resolved",
		"Your CARLITOS help request "+d.TicketNumber+" has been resolved.", content)
}
