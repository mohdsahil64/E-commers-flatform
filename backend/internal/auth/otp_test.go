package auth

import (
	"testing"
	"time"
)

func TestOTPStoreVerifyValidCode(t *testing.T) {
	s := NewOTPStore(time.Minute)
	s.Put("+919876543210", "482913")
	if !s.Verify("+919876543210", "482913") {
		t.Error("expected correct code to verify")
	}
	// Consumed after success.
	if s.Verify("+919876543210", "482913") {
		t.Error("expected code to be single-use")
	}
}

func TestOTPStoreRejectsWrongCode(t *testing.T) {
	s := NewOTPStore(time.Minute)
	s.Put("+919876543210", "482913")
	if s.Verify("+919876543210", "000000") {
		t.Error("expected wrong code to be rejected")
	}
}

func TestOTPStoreExpiry(t *testing.T) {
	s := NewOTPStore(time.Nanosecond)
	s.Put("+919876543210", "482913")
	time.Sleep(time.Millisecond)
	if s.Verify("+919876543210", "482913") {
		t.Error("expected expired code to be rejected")
	}
}

func TestOTPStoreAcceptsDevCode(t *testing.T) {
	s := NewOTPStore(time.Minute)
	// No challenge issued, but the fixed dev code is accepted for testing.
	if !s.Verify("+919999999999", DevOTPCode) {
		t.Error("expected dev code to be accepted")
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+91 98765-43210":  "+919876543210",
		"(987) 654 3210":   "9876543210",
		"98765 43210":      "9876543210",
	}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidPhone(t *testing.T) {
	valid := []string{"9876543210", "+919876543210", "+14155552671"}
	for _, p := range valid {
		if !validPhone(p) {
			t.Errorf("expected %q to be valid", p)
		}
	}
	invalid := []string{"12345", "", "abcdefghij", "+" + "1234567890123456"}
	for _, p := range invalid {
		if validPhone(p) {
			t.Errorf("expected %q to be invalid", p)
		}
	}
}
