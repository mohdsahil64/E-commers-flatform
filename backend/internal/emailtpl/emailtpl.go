package emailtpl

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// Design language: clean & classic. A slim wordmark header (no external image,
// so it always renders), generous whitespace, small readable text, a single
// accent colour used sparingly, and a quiet footer. Fully responsive.
const (
	colorInk    = "#1a1a1a"
	colorText   = "#4b5563"
	colorMuted  = "#9ca3af"
	colorLine   = "#ececec"
	colorBg     = "#f6f6f6"
	colorAccent = "#b8860b" // classic gold — matches the CARLITOS mark
)

const baseLayout = `<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta charset="UTF-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
  <meta http-equiv="X-UA-Compatible" content="IE=edge"/>
  <meta name="format-detection" content="telephone=no"/>
  <title>{{.Subject}}</title>
  <!--[if mso]>
  <noscript><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript>
  <![endif]-->
  <style type="text/css">
    body,table,td,a{-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%}
    table,td{mso-table-lspace:0pt;mso-table-rspace:0pt}
    img{-ms-interpolation-mode:bicubic;border:0;height:auto;line-height:100%;outline:none;text-decoration:none}
    body{margin:0;padding:0;width:100%!important;min-width:100%;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1a1a1a;background:` + colorBg + `}
    a{color:` + colorAccent + `}
    @media only screen and (max-width:600px){
      .email-container{width:100%!important;max-width:100%!important;border-radius:0!important}
      .px-mobile{padding-left:22px!important;padding-right:22px!important}
      .py-mobile{padding-top:28px!important;padding-bottom:28px!important}
    }
  </style>
</head>
<body style="margin:0;padding:0;background-color:` + colorBg + `;width:100%">
<div style="display:none;font-size:1px;color:` + colorBg + `;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden">{{.PreviewText}}&nbsp;&zwnj;&nbsp;&zwnj;</div>
<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="background:` + colorBg + `">
<tr><td align="center" style="padding:28px 12px">
  <table class="email-container" role="presentation" border="0" cellpadding="0" cellspacing="0"
         width="560" style="max-width:560px;background:#ffffff;border:1px solid ` + colorLine + `;border-radius:10px;overflow:hidden">
    <tr>
      <td style="padding:26px 40px 22px;text-align:center;border-bottom:1px solid ` + colorLine + `">
        <span style="font-size:22px;font-weight:700;letter-spacing:3px;color:` + colorInk + `">CAR<span style="color:` + colorAccent + `">LITOS</span></span>
      </td>
    </tr>
    <tr>
      <td class="px-mobile py-mobile" style="padding:36px 40px">{{.Content}}</td>
    </tr>
    <tr>
      <td style="background:#fbfbfb;border-top:1px solid ` + colorLine + `;padding:20px 40px;text-align:center">
        <p style="margin:0 0 4px;font-size:12px;color:` + colorMuted + `;line-height:1.6">&copy; {{.Year}} CARLITOS Marketplace</p>
        <p style="margin:0;font-size:11px;color:` + colorMuted + `;line-height:1.6">This is an automated message about your CARLITOS account.</p>
      </td>
    </tr>
  </table>
</td></tr>
</table>
</body>
</html>`

type baseData struct {
	Subject     string
	PreviewText string
	Content     template.HTML
	Year        string
}

func render(subject, preview, content string) string {
	tmpl := template.Must(template.New("base").Parse(baseLayout))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, baseData{
		Subject:     subject,
		PreviewText: preview,
		Content:     template.HTML(content),
		Year:        "2026",
	}); err != nil {
		return fmt.Sprintf("<p>%s</p>", subject)
	}
	return buf.String()
}

func btn(label, href string) string {
	return fmt.Sprintf(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" style="margin:24px 0 6px"><tr><td align="center" style="border-radius:6px" bgcolor="`+colorInk+`"><a href="%s" target="_blank" style="display:inline-block;background:`+colorInk+`;color:#ffffff;font-size:14px;font-weight:600;letter-spacing:.2px;text-decoration:none;padding:12px 30px;border-radius:6px">%s</a></td></tr></table>`, href, label)
}

func hr() string {
	return `<hr style="border:none;border-top:1px solid ` + colorLine + `;margin:24px 0"/>`
}

func h2(text string) string {
	return fmt.Sprintf(`<h2 style="margin:0 0 12px;font-size:18px;font-weight:700;color:%s;line-height:1.35">%s</h2>`, colorInk, text)
}

func p(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 14px;font-size:14px;color:%s;line-height:1.65">%s</p>`, colorText, text)
}

// note renders a small muted line (used for fine print).
func note(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 6px;font-size:12px;color:%s;line-height:1.6">%s</p>`, colorMuted, text)
}

// box renders a quiet, bordered callout. accent is the left-border colour.
func box(accent, text string) string {
	return fmt.Sprintf(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%%" style="margin:16px 0"><tr><td style="background:#fbfbfb;border:1px solid %s;border-left:3px solid %s;border-radius:6px;padding:13px 16px;font-size:13px;color:%s;line-height:1.6">%s</td></tr></table>`, colorLine, accent, colorInk, text)
}

// kvRow renders a compact label / value row for summaries.
func kvRow(label, value string) string {
	return fmt.Sprintf(`<tr><td style="padding:7px 0;border-bottom:1px solid %s;font-size:13px;color:%s">%s</td><td align="right" style="padding:7px 0;border-bottom:1px solid %s;font-size:13px;font-weight:600;color:%s;white-space:nowrap">%s</td></tr>`, colorLine, colorText, label, colorLine, colorInk, value)
}

func stepRow(num, color, title, desc string) string {
	return fmt.Sprintf(`<tr><td style="padding:8px 0"><table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%%"><tr><td valign="top" width="34" style="padding-right:12px"><div style="width:26px;height:26px;background:%s;border-radius:50%%;text-align:center;line-height:26px;color:#fff;font-size:13px;font-weight:700">%s</div></td><td valign="top"><p style="margin:0 0 2px;font-size:13px;font-weight:700;color:%s">%s</p><p style="margin:0;font-size:12px;color:%s;line-height:1.5">%s</p></td></tr></table></td></tr>`, color, num, colorInk, title, colorMuted, desc)
}

type SellerWelcomeData struct {
	OwnerName string
	StoreName string
	LoginURL  string
}

func SellerWelcome(d SellerWelcomeData) string {
	name := esc(d.OwnerName)
	store := esc(d.StoreName)
	steps := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:14px 0 6px">` +
		stepRow("1", colorAccent, "Add owner & bank details", "Your KYC and payout account.") +
		stepRow("2", colorAccent, "Set your pickup location", "Where couriers collect your orders.") +
		stepRow("3", colorAccent, "Sign the agreement & submit", "We review applications in 1–2 business days.") +
		`</table>`
	content := h2("Welcome, "+name+"") +
		p("Your seller account for <strong>"+store+"</strong> is ready. Complete a short onboarding to go live.") +
		steps +
		btn("Open Seller Dashboard", d.LoginURL) +
		hr() +
		note("Need help? Write to seller-support@carlitos.in")
	return render("Welcome to CARLITOS", "Your seller account is ready. Complete onboarding to go live.", content)
}

type SellerOTPData struct {
	OwnerName  string
	OTP        string
	ExpiryMins int
	Purpose    string
}

func SellerOTP(d SellerOTPData) string {
	name := esc(d.OwnerName)
	otp := esc(d.OTP)
	purpose := d.Purpose
	if purpose == "" {
		purpose = "verify your email address"
	}
	expiry := d.ExpiryMins
	if expiry == 0 {
		expiry = 10
	}
	otpBlock := fmt.Sprintf(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%%" style="margin:18px 0"><tr><td align="center" style="background:#fbfbfb;border:1px solid %s;border-radius:8px;padding:20px 16px"><p style="margin:0 0 8px;font-size:11px;font-weight:600;color:%s;letter-spacing:1.5px;text-transform:uppercase">Verification code</p><p style="margin:0;font-size:32px;font-weight:700;letter-spacing:8px;color:%s;font-family:'Courier New',Courier,monospace">%s</p></td></tr></table>`, colorLine, colorMuted, colorInk, otp)
	content := h2("Verify your email") +
		p("Hi "+name+", use this code to "+purpose+".") +
		otpBlock +
		note("Valid for "+fmt.Sprintf("%d", expiry)+" minutes. Don't share this code. If you didn't request it, ignore this email.")
	return render("Your CARLITOS verification code: "+otp, "Use code "+otp+" to verify your CARLITOS seller account.", content)
}

type SellerSubmittedData struct {
	OwnerName string
	StoreName string
}

func SellerSubmitted(d SellerSubmittedData) string {
	name := esc(d.OwnerName)
	store := esc(d.StoreName)
	content := h2("Application received") +
		p("Hi "+name+", thanks for completing onboarding for <strong>"+store+"</strong>.") +
		box(colorAccent, "Your application is <strong>under review</strong>. We usually respond within 1–2 business days.") +
		p("We'll email you as soon as there's a decision.") +
		hr() +
		note("Questions? seller-support@carlitos.in")
	return render("Your CARLITOS application is under review", "We've received your application for "+d.StoreName+".", content)
}

type SellerApprovedData struct {
	OwnerName  string
	StoreName  string
	DashURL    string
	ListingURL string
}

func SellerApproved(d SellerApprovedData) string {
	name := esc(d.OwnerName)
	store := esc(d.StoreName)
	content := h2("You're approved") +
		p("Hi "+name+", <strong>"+store+"</strong> is now live on CARLITOS. Customers can discover and buy your products.") +
		box("#2f855a", "Your store is <strong>active</strong>. You can start listing products right away.") +
		btn("Add your first product", d.DashURL) +
		hr() +
		note("Welcome aboard — the CARLITOS team.")
	return render("Your CARLITOS seller account is approved", d.StoreName+" is now live on CARLITOS.", content)
}

type SellerRejectedData struct {
	OwnerName  string
	StoreName  string
	Reason     string
	ReApplyURL string
}

func SellerRejected(d SellerRejectedData) string {
	name := esc(d.OwnerName)
	store := esc(d.StoreName)
	reason := esc(d.Reason)
	if reason == "" {
		reason = "Please review our seller eligibility guidelines and re-apply."
	}
	content := h2("Application not approved") +
		p("Hi "+name+", we've reviewed <strong>"+store+"</strong> and can't approve it at this time.") +
		box("#c0392b", "<strong>Reason:</strong> "+reason) +
		p("You can fix the issue above and resubmit your application.") +
		btn("Review & resubmit", d.ReApplyURL) +
		hr() +
		note("Questions? seller-support@carlitos.in")
	return render("Update on your CARLITOS seller application", "An update on your seller application for "+d.StoreName+".", content)
}

type SellerSuspendedData struct {
	OwnerName  string
	StoreName  string
	Reason     string
	SupportURL string
}

func SellerSuspended(d SellerSuspendedData) string {
	name := esc(d.OwnerName)
	store := esc(d.StoreName)
	reason := esc(d.Reason)
	if reason == "" {
		reason = "Your account did not comply with our seller policy."
	}
	content := h2("Your store has been suspended") +
		p("Hi "+name+", your store <strong>"+store+"</strong> has been temporarily suspended.") +
		box("#c0392b", "<strong>Reason:</strong> "+reason) +
		p("Your listings are hidden from buyers while suspended. Pending payouts continue on schedule.") +
		btn("Contact support", d.SupportURL) +
		hr() +
		note("The CARLITOS seller team.")
	return render("Your CARLITOS seller account has been suspended", "Important notice about your seller account.", content)
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// PasswordResetData is the payload for the password-reset email.
type PasswordResetData struct {
	Name       string // recipient's name (owner name / admin name); may be empty
	Portal     string // "seller" or "admin" — used only for wording
	ResetURL   string // full link to the reset page including the token
	ExpiryMins int    // link validity in minutes
}

// PasswordReset renders the "reset your password" email with a single-use link.
func PasswordReset(d PasswordResetData) string {
	name := esc(d.Name)
	if name == "" {
		name = "there"
	}
	expiry := d.ExpiryMins
	if expiry == 0 {
		expiry = 30
	}
	portal := "account"
	switch d.Portal {
	case "seller":
		portal = "seller account"
	case "admin":
		portal = "admin account"
	}
	content := h2("Reset your password") +
		p("Hi "+name+", we received a request to reset the password for your CARLITOS "+portal+". Click below to choose a new one — this link works once.") +
		btn("Reset password", d.ResetURL) +
		note("This link expires in "+fmt.Sprintf("%d", expiry)+" minutes. If you didn't request this, ignore this email — your password won't change.") +
		hr() +
		note("Trouble with the button? Paste this link into your browser:") +
		p(`<span style="word-break:break-all;font-size:12px;color:`+colorMuted+`">`+esc(d.ResetURL)+`</span>`)
	return render("Reset your CARLITOS password", "Use the secure link to reset your CARLITOS password.", content)
}
