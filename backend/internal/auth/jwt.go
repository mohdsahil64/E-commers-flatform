package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes access and refresh tokens.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims is the JWT payload for CARLITOS access tokens.
type Claims struct {
	UserID      string   `json:"uid"`
	AccountType string   `json:"act"` // customer | seller | admin
	Email       string   `json:"email"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"perms,omitempty"`
	TokenType   string   `json:"typ"`
	jwt.RegisteredClaims
}

// Manager issues and verifies JWTs using separate access/refresh secrets.
type Manager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
}

// NewManager builds a token Manager.
func NewManager(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		issuer:        "carlitos",
	}
}

// IssueAccess creates a signed access token embedding roles and permissions.
func (m *Manager) IssueAccess(userID, accountType, email string, roles, perms []string) (string, time.Time, error) {
	exp := time.Now().Add(m.accessTTL)
	claims := Claims{
		UserID:      userID,
		AccountType: accountType,
		Email:       email,
		Roles:       roles,
		Permissions: perms,
		TokenType:   string(AccessToken),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.accessSecret)
	return signed, exp, err
}

// IssueRefresh creates an opaque refresh token (random string). The caller
// stores only its hash. Returns (rawToken, hash, expiry).
func (m *Manager) IssueRefresh() (raw string, hash string, exp time.Time, err error) {
	return m.IssueRefreshWithTTL(m.refreshTTL)
}

// IssueRefreshWithTTL creates an opaque refresh token with an explicit TTL.
// Used to give customers effectively non-expiring sessions (long TTL + silent
// rotation) while keeping the standard TTL for seller/admin.
func (m *Manager) IssueRefreshWithTTL(ttl time.Duration) (raw string, hash string, exp time.Time, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", time.Time{}, err
	}
	raw = hex.EncodeToString(b)
	hash = HashToken(raw)
	exp = time.Now().Add(ttl)
	return raw, hash, exp, nil
}

// HashToken returns a stable SHA-256 hex hash of a token (for storage/lookup).
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ParseAccess verifies and parses an access token.
func (m *Manager) ParseAccess(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.accessSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.TokenType != string(AccessToken) {
		return nil, errors.New("invalid access token")
	}
	return claims, nil
}
