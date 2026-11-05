package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// StaffGate is the first-login state of an invited admin employee.
type StaffGate struct {
	Status             string // invited | active
	MustChangePassword bool
	InviteExpiresAt    *time.Time
}

// GetStaffGate returns the employee-invitation state for an admin user, or nil
// when the user has no staff row (e.g. the seeded SUPER_ADMIN).
func (r *Repository) GetStaffGate(ctx context.Context, userID string) (*StaffGate, error) {
	var g StaffGate
	err := r.pool.QueryRow(ctx, `
		SELECT status, must_change_password, invite_expires_at
		FROM admin_staff WHERE user_id = $1`, userID).
		Scan(&g.Status, &g.MustChangePassword, &g.InviteExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// SetupChallenge is a first-login challenge joined with its user.
type SetupChallenge struct {
	ID            string
	UserID        string
	CodeHash      string
	Attempts      int
	ResendCount   int
	ExpiresAt     time.Time
	CodeExpiresAt time.Time
	LastSentAt    time.Time

	Email        string
	FullName     string
	PasswordHash string
	AccountType  string
}

// CreateSetupChallenge stores a new challenge, invalidating any earlier unused
// ones for the same user so only the latest sign-in attempt is valid.
func (r *Repository) CreateSetupChallenge(ctx context.Context, userID, tokenHash, codeHash string, expiresAt, codeExpiresAt time.Time, ip string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE admin_setup_challenges SET used_at = now()
			 WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO admin_setup_challenges
				(user_id, token_hash, code_hash, expires_at, code_expires_at, created_ip)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			userID, tokenHash, codeHash, expiresAt, codeExpiresAt, ip)
		return err
	})
}

// FindSetupChallenge returns an unused, unexpired challenge by token hash.
func (r *Repository) FindSetupChallenge(ctx context.Context, tokenHash string) (*SetupChallenge, error) {
	var c SetupChallenge
	err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.user_id, c.code_hash, c.attempts, c.resend_count,
		       c.expires_at, c.code_expires_at, c.last_sent_at,
		       coalesce(u.email,''), u.full_name, u.password_hash, u.account_type
		FROM admin_setup_challenges c
		JOIN users u ON u.id = c.user_id
		WHERE c.token_hash = $1 AND c.used_at IS NULL AND c.expires_at > now()`,
		tokenHash).
		Scan(&c.ID, &c.UserID, &c.CodeHash, &c.Attempts, &c.ResendCount,
			&c.ExpiresAt, &c.CodeExpiresAt, &c.LastSentAt,
			&c.Email, &c.FullName, &c.PasswordHash, &c.AccountType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// BumpSetupAttempts records a wrong code and returns the new attempt count.
func (r *Repository) BumpSetupAttempts(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`UPDATE admin_setup_challenges SET attempts = attempts + 1
		 WHERE id = $1 RETURNING attempts`, id).Scan(&n)
	return n, err
}

// InvalidateSetupChallenge burns a challenge so it can never be used again.
func (r *Repository) InvalidateSetupChallenge(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE admin_setup_challenges SET used_at = now() WHERE id = $1`, id)
	return err
}

// ReplaceSetupCode stores a freshly issued code for an existing challenge.
func (r *Repository) ReplaceSetupCode(ctx context.Context, id, codeHash string, codeExpiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE admin_setup_challenges
		SET code_hash = $2, code_expires_at = $3, attempts = 0,
		    resend_count = resend_count + 1, last_sent_at = now()
		WHERE id = $1`, id, codeHash, codeExpiresAt)
	return err
}

// CompleteStaffSetup atomically finishes the employee's account setup: it sets
// the new password, activates the staff record, burns the challenge and revokes
// every existing session for the user.
func (r *Repository) CompleteStaffSetup(ctx context.Context, challengeID, userID, newHash string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE admin_setup_challenges SET used_at = now()
			WHERE id = $1 AND used_at IS NULL AND expires_at > now()`, challengeID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `
			UPDATE users SET password_hash = $2, email_verified = TRUE WHERE id = $1`,
			userID, newHash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE admin_staff
			SET status = 'active', must_change_password = FALSE,
			    accepted_at = now(), invite_expires_at = NULL
			WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE admin_setup_challenges SET used_at = now()
			WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE refresh_tokens SET revoked_at = now()
			WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
}
