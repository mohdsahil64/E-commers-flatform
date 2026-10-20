package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user/record does not exist.
var ErrNotFound = errors.New("not found")

// UserRecord is the persisted user row (auth-relevant fields).
type UserRecord struct {
	ID           string
	AccountType  string
	Email        string
	Phone        string
	PasswordHash string
	FullName     string
	Status       string
}

// Repository provides auth-related database access.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds an auth Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindUserByEmail returns a user scoped to a portal (account_type).
func (r *Repository) FindUserByEmail(ctx context.Context, accountType, email string) (*UserRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, account_type, coalesce(email,''), coalesce(phone,''), password_hash, full_name, status
		FROM users
		WHERE account_type = $1 AND email = $2`,
		accountType, email)
	return scanUser(row)
}

// FindUserByID returns a user by id.
func (r *Repository) FindUserByID(ctx context.Context, id string) (*UserRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, account_type, coalesce(email,''), coalesce(phone,''), password_hash, full_name, status
		FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// EmailExists reports whether an email is already registered in a portal.
func (r *Repository) EmailExists(ctx context.Context, accountType, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE account_type=$1 AND email=$2)`,
		accountType, email).Scan(&exists)
	return exists, err
}

// FindUserByPhone returns a user scoped to a portal by phone number.
func (r *Repository) FindUserByPhone(ctx context.Context, accountType, phone string) (*UserRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, account_type, coalesce(email,''), coalesce(phone,''), password_hash, full_name, status
		FROM users
		WHERE account_type = $1 AND phone = $2
		ORDER BY created_at
		LIMIT 1`,
		accountType, phone)
	return scanUser(row)
}

// MarkPhoneVerified flags a user's phone as verified.
func (r *Repository) MarkPhoneVerified(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET phone_verified = TRUE WHERE id = $1`, userID)
	return err
}

// CreateUser inserts a new user and returns its id.
func (r *Repository) CreateUser(ctx context.Context, tx pgx.Tx, u UserRecord) (string, error) {
	var id string
	q := `INSERT INTO users (account_type, email, phone, password_hash, full_name, status)
	      VALUES ($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6) RETURNING id`
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, q, u.AccountType, u.Email, u.Phone, u.PasswordHash, u.FullName, u.Status).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, q, u.AccountType, u.Email, u.Phone, u.PasswordHash, u.FullName, u.Status).Scan(&id)
	}
	return id, err
}

// AssignRoleByCode attaches a role (by code) to a user.
func (r *Repository) AssignRoleByCode(ctx context.Context, tx pgx.Tx, userID, roleCode string) error {
	q := `INSERT INTO user_roles (user_id, role_id)
	      SELECT $1, id FROM roles WHERE code = $2
	      ON CONFLICT DO NOTHING`
	if tx != nil {
		_, err := tx.Exec(ctx, q, userID, roleCode)
		return err
	}
	_, err := r.pool.Exec(ctx, q, userID, roleCode)
	return err
}

// LoadRolesAndPermissions returns the role codes and permission codes for a user.
func (r *Repository) LoadRolesAndPermissions(ctx context.Context, userID string) (roles, perms []string, err error) {
	rrows, err := r.pool.Query(ctx, `
		SELECT r.code FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rrows.Close()
	for rrows.Next() {
		var code string
		if err := rrows.Scan(&code); err != nil {
			return nil, nil, err
		}
		roles = append(roles, code)
	}
	if err := rrows.Err(); err != nil {
		return nil, nil, err
	}

	prows, err := r.pool.Query(ctx, `
		SELECT DISTINCT p.code
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id = p.id
		JOIN user_roles ur ON ur.role_id = rp.role_id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var code string
		if err := prows.Scan(&code); err != nil {
			return nil, nil, err
		}
		perms = append(perms, code)
	}
	return roles, perms, prows.Err()
}

// TouchLastLogin updates the last_login_at timestamp.
func (r *Repository) TouchLastLogin(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	return err
}

// StoreRefreshToken persists a hashed refresh token.
func (r *Repository) StoreRefreshToken(ctx context.Context, userID, tokenHash, ua, ip string, exp time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip, expires_at)
		VALUES ($1,$2,$3,$4,$5)`, userID, tokenHash, ua, ip, exp)
	return err
}

// RevokeAllForUser revokes every active refresh token for a user. Used to
// enforce a single active session (a new login logs out other devices).
func (r *Repository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// FindValidRefreshToken returns the owning user id for an active refresh token.
func (r *Repository) FindValidRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`,
		tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

// RevokeRefreshToken marks a refresh token revoked.
func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}

// CreatePasswordResetToken stores a single-use, hashed reset token. Any
// outstanding (unused, unexpired) tokens for the user are invalidated first so
// only the most recent request is valid.
func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, exp time.Time, ua, ip string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE password_reset_tokens SET used_at = now()
			 WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_ua, created_ip)
			VALUES ($1,$2,$3,$4,$5)`, userID, tokenHash, exp, ua, ip)
		return err
	})
}

// FindValidPasswordResetToken returns the owning user id for an active
// (unused, unexpired) reset token hash.
func (r *Repository) FindValidPasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id FROM password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`,
		tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

// ConsumePasswordReset marks the reset token used and updates the user's
// password hash in a single transaction, then revokes all the user's refresh
// tokens so existing sessions are invalidated after a reset.
func (r *Repository) ConsumePasswordReset(ctx context.Context, tokenHash, userID, newHash string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE password_reset_tokens SET used_at = now()
			 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()`, tokenHash)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
}

// Pool exposes the underlying pool for transactional operations in services
// that need to create a user + profile atomically.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

func scanUser(row pgx.Row) (*UserRecord, error) {
	var u UserRecord
	err := row.Scan(&u.ID, &u.AccountType, &u.Email, &u.Phone, &u.PasswordHash, &u.FullName, &u.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
