package auth

import (
	"regexp"
	"strings"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// strongPasswordError returns a human-readable reason if the password does not
// meet the strength policy, or "" when it is strong enough. Policy: at least 8
// characters and at least three of the four character classes (lowercase,
// uppercase, digit, symbol).
func strongPasswordError(pw string) string {
	if len(pw) < 8 {
		return "Password must be at least 8 characters"
	}
	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	classes := 0
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return "Use a mix of uppercase, lowercase, numbers and symbols"
	}
	return ""
}

// validateRegisterCustomer returns a field->message map of validation errors.
func validateRegisterCustomer(b registerCustomerBody) map[string]any {
	fields := map[string]any{}
	if strings.TrimSpace(b.FullName) == "" {
		fields["full_name"] = "Full name is required"
	}
	if !emailRe.MatchString(strings.TrimSpace(b.Email)) {
		fields["email"] = "A valid email is required"
	}
	if msg := strongPasswordError(b.Password); msg != "" {
		fields["password"] = msg
	}
	// Sellers must provide a valid mobile number (it is OTP-verified at signup).
	if strings.TrimSpace(b.Phone) == "" {
		fields["phone"] = "Mobile number is required"
	} else if !isValidPhone(b.Phone) {
		fields["phone"] = "Phone number is invalid"
	}
	return fields
}

// validateRegisterSeller validates the seller registration payload.
func validateRegisterSeller(b registerSellerBody) map[string]any {
	fields := map[string]any{}
	if strings.TrimSpace(b.OwnerName) == "" {
		fields["owner_name"] = "Owner name is required"
	}
	if strings.TrimSpace(b.StoreName) == "" {
		fields["store_name"] = "Store name is required"
	}
	if !emailRe.MatchString(strings.TrimSpace(b.Email)) {
		fields["email"] = "A valid email is required"
	}
	if msg := strongPasswordError(b.Password); msg != "" {
		fields["password"] = msg
	}
	if b.Phone != "" && !isValidPhone(b.Phone) {
		fields["phone"] = "Phone number is invalid"
	}
	return fields
}

func isValidPhone(p string) bool {
	digits := 0
	for _, r := range p {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 7 && digits <= 15
}
