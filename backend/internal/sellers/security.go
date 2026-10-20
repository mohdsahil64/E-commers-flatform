package sellers

import "golang.org/x/crypto/bcrypt"

// hashPassword returns a bcrypt hash of the plaintext password.
func hashPassword(plain string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// checkPassword reports whether plain matches the stored bcrypt hash.
func checkPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// StrongPasswordError returns a human-readable reason if the password does not
// meet the strength policy, or "" when strong enough: at least 8 characters and
// at least three of the four character classes (lower, upper, digit, symbol).
func StrongPasswordError(pw string) string {
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
