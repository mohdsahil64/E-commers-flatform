package auth

import (
	"testing"
	"time"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("s3cret-password", 10)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword(hash, "s3cret-password") {
		t.Error("expected password to match")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("expected wrong password to fail")
	}
}

func TestJWTIssueAndParse(t *testing.T) {
	m := NewManager("access-secret", "refresh-secret", 15*time.Minute, 24*time.Hour)

	token, exp, err := m.IssueAccess("user-1", "admin", "a@b.com",
		[]string{RoleSuperAdmin}, []string{PermSellerApprove})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !exp.After(time.Now()) {
		t.Error("expiry should be in the future")
	}

	claims, err := m.ParseAccess(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != "user-1" || claims.AccountType != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}

	// A super admin principal implicitly holds every permission.
	p := &Principal{Roles: claims.Roles, Permissions: claims.Permissions}
	if !p.HasPermission(PermProductApprove) {
		t.Error("super admin should implicitly hold all permissions")
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	m := NewManager("a", "b", time.Minute, time.Hour)
	raw, hash, exp, err := m.IssueRefresh()
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatal("expected non-empty token and hash")
	}
	if HashToken(raw) != hash {
		t.Error("hash should be deterministic for the same raw token")
	}
	if !exp.After(time.Now()) {
		t.Error("refresh expiry should be in the future")
	}
}
