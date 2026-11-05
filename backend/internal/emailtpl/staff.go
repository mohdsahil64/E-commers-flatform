package emailtpl

import "fmt"

// StaffInvitationData feeds the employee invitation email.
type StaffInvitationData struct {
	Name         string
	RoleName     string
	InvitedBy    string
	Email        string
	TempPassword string
	LoginURL     string
	ExpiryHours  int
}

// StaffInvitation welcomes a new employee and carries their one-time temporary
// password. The password only works for the first sign-in, which then requires
// an emailed verification code and a new password.
func StaffInvitation(d StaffInvitationData) string {
	name := esc(d.Name)
	if name == "" {
		name = "there"
	}
	hours := d.ExpiryHours
	if hours <= 0 {
		hours = 72
	}
	by := ""
	if d.InvitedBy != "" {
		by = " by <strong>" + esc(d.InvitedBy) + "</strong>"
	}
	creds := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:16px 0;background:#fbfbfb;border:1px solid ` + colorLine + `;border-radius:8px">` +
		`<tr><td style="padding:16px 18px">` +
		fmt.Sprintf(`<p style="margin:0 0 4px;font-size:11px;font-weight:600;color:%s;letter-spacing:1.2px;text-transform:uppercase">Login email</p>`, colorMuted) +
		fmt.Sprintf(`<p style="margin:0 0 14px;font-size:14px;font-weight:600;color:%s">%s</p>`, colorInk, esc(d.Email)) +
		fmt.Sprintf(`<p style="margin:0 0 4px;font-size:11px;font-weight:600;color:%s;letter-spacing:1.2px;text-transform:uppercase">Temporary password</p>`, colorMuted) +
		fmt.Sprintf(`<p style="margin:0;font-size:20px;font-weight:700;letter-spacing:2px;color:%s;font-family:'Courier New',Courier,monospace">%s</p>`, colorInk, esc(d.TempPassword)) +
		`</td></tr></table>`
	steps := `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:6px 0 4px">` +
		stepRow("1", colorAccent, "Sign in with the temporary password", "Use the email and password above on the admin sign-in page.") +
		stepRow("2", colorAccent, "Verify with a code", "We'll email a 6-digit code to this address to confirm it's you.") +
		stepRow("3", colorAccent, "Choose your own password", "Pick a strong new password. The temporary one stops working.") +
		`</table>`
	content := h2("Welcome to the CARLITOS team, "+name) +
		p("You've been invited"+by+" to join the CARLITOS admin desk as <strong>"+esc(d.RoleName)+"</strong>. You will only see the sections your role allows.") +
		creds +
		steps +
		btn("Sign in to Admin Desk", d.LoginURL) +
		box("#c0392b", fmt.Sprintf("This temporary password is <strong>single-use</strong> and expires in <strong>%d hours</strong>. Never share it with anyone, including CARLITOS staff.", hours)) +
		hr() +
		note("If you weren't expecting this invitation, ignore this email and let your administrator know.")
	return render("You're invited to the CARLITOS admin desk", "Your temporary password is inside. Sign in to finish setting up your account.", content)
}

// StaffSetupCodeData feeds the first-login verification email.
type StaffSetupCodeData struct {
	Name       string
	Code       string
	ExpiryMins int
}

// StaffSetupCode carries the one-time code an invited employee must enter
// (together with a new password) to complete account setup.
func StaffSetupCode(d StaffSetupCodeData) string {
	name := esc(d.Name)
	if name == "" {
		name = "there"
	}
	mins := d.ExpiryMins
	if mins <= 0 {
		mins = 10
	}
	block := fmt.Sprintf(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%%" style="margin:18px 0"><tr><td align="center" style="background:#fbfbfb;border:1px solid %s;border-radius:8px;padding:20px 16px"><p style="margin:0 0 8px;font-size:11px;font-weight:600;color:%s;letter-spacing:1.5px;text-transform:uppercase">Verification code</p><p style="margin:0;font-size:32px;font-weight:700;letter-spacing:8px;color:%s;font-family:'Courier New',Courier,monospace">%s</p></td></tr></table>`, colorLine, colorMuted, colorInk, esc(d.Code))
	content := h2("Verify your account") +
		p("Hi "+name+", enter this code on the sign-in screen, then choose your new password to finish setting up your CARLITOS admin account.") +
		block +
		note(fmt.Sprintf("The code is valid for %d minutes and can be used once. If you didn't just try to sign in, ignore this email and tell your administrator.", mins))
	return render("Your CARLITOS verification code", "Use this code to finish setting up your admin account.", content)
}
