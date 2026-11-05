package staff

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors translated to HTTP errors by the service.
var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

// Repository provides persistence for staff, roles and accounts.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a staff Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const employeeSelect = `
	SELECT u.id, u.full_name, coalesce(u.email,''), coalesce(u.phone,''), u.status,
	       u.last_login_at, u.created_at,
	       s.status, s.invited_at, s.invite_expires_at, s.accepted_at, coalesce(s.invite_count,0),
	       coalesce(inv.full_name,''),
	       coalesce(r.id::text,''), coalesce(r.code,''), coalesce(r.name,'')
	FROM users u
	LEFT JOIN admin_staff s ON s.user_id = u.id
	LEFT JOIN users inv ON inv.id = s.invited_by
	LEFT JOIN LATERAL (
		SELECT r.id, r.code, r.name
		FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = u.id
		ORDER BY (r.code = 'SUPER_ADMIN') DESC, r.name
		LIMIT 1
	) r ON TRUE
	WHERE u.account_type = 'admin'`

func scanEmployee(row pgx.Row) (*Employee, error) {
	var (
		e           Employee
		userStatus  string
		staffStatus *string
	)
	err := row.Scan(&e.ID, &e.FullName, &e.Email, &e.Phone, &userStatus,
		&e.LastLoginAt, &e.CreatedAt,
		&staffStatus, &e.InvitedAt, &e.InviteExpiresAt, &e.AcceptedAt, &e.InviteCount,
		&e.InvitedBy, &e.RoleID, &e.RoleCode, &e.RoleName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.IsSuperAdmin = e.RoleCode == "SUPER_ADMIN"
	e.Status = computeStatus(userStatus, staffStatus, e.InviteExpiresAt)
	return &e, nil
}

func computeStatus(userStatus string, staffStatus *string, inviteExpires *time.Time) string {
	if userStatus != "active" {
		return StatusDisabled
	}
	if staffStatus != nil && *staffStatus == "invited" {
		if inviteExpires != nil && time.Now().After(*inviteExpires) {
			return StatusExpired
		}
		return StatusPending
	}
	return StatusActive
}

// ListEmployees returns every admin-desk account.
func (r *Repository) ListEmployees(ctx context.Context) ([]Employee, error) {
	rows, err := r.pool.Query(ctx, employeeSelect+`
		ORDER BY (r.code = 'SUPER_ADMIN') DESC NULLS LAST, u.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// GetEmployee returns one admin-desk account.
func (r *Repository) GetEmployee(ctx context.Context, id string) (*Employee, error) {
	return scanEmployee(r.pool.QueryRow(ctx, employeeSelect+` AND u.id = $1`, id))
}

// EmailExists reports whether an admin account already uses the email.
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE account_type = 'admin' AND email = $1)`, email).Scan(&ok)
	return ok, err
}

// UserRolesAndPerms returns the live role codes and permission codes of a user.
func (r *Repository) UserRolesAndPerms(ctx context.Context, userID string) (roles, perms []string, err error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.code FROM roles r JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return nil, nil, err
		}
		roles = append(roles, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
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
		var c string
		if err := prows.Scan(&c); err != nil {
			return nil, nil, err
		}
		perms = append(perms, c)
	}
	return roles, perms, prows.Err()
}

// CountActiveSuperAdmins counts enabled SUPER_ADMIN accounts.
func (r *Repository) CountActiveSuperAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(DISTINCT u.id)
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE u.account_type = 'admin' AND u.status = 'active' AND r.code = 'SUPER_ADMIN'`).Scan(&n)
	return n, err
}

// CreateStaffParams describes a new invited employee.
type CreateStaffParams struct {
	FullName     string
	Email        string
	Phone        string
	PasswordHash string
	RoleID       string
	InvitedBy    string
	ExpiresAt    time.Time
}

// CreateStaff atomically creates the user, the staff record and the role link.
func (r *Repository) CreateStaff(ctx context.Context, p CreateStaffParams) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (account_type, email, phone, password_hash, full_name, status)
			VALUES ('admin', $1, NULLIF($2,''), $3, $4, 'active')
			RETURNING id`, p.Email, p.Phone, p.PasswordHash, p.FullName).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_staff (user_id, status, invited_by, invite_expires_at, must_change_password)
			VALUES ($1, 'invited', NULLIF($2,'')::uuid, $3, TRUE)`,
			id, p.InvitedBy, p.ExpiresAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, id, p.RoleID)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return "", ErrEmailTaken
	}
	return id, err
}

// ResetInvite issues a fresh temporary password for a not-yet-accepted
// employee: new hash + new expiry, any in-progress setup is voided and every
// existing session is revoked.
func (r *Repository) ResetInvite(ctx context.Context, userID, passwordHash string, expiresAt time.Time) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE admin_staff
			SET status = 'invited', must_change_password = TRUE,
			    invite_expires_at = $2, invite_count = invite_count + 1, invited_at = now()
			WHERE user_id = $1 AND status = 'invited'`, userID, expiresAt)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET password_hash = $2 WHERE id = $1`, userID, passwordHash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE admin_setup_challenges SET used_at = now()
			 WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
}

// SetUserStatus switches an account between active and inactive. Disabling also
// revokes every session and voids any in-progress setup.
func (r *Repository) SetUserStatus(ctx context.Context, userID, status string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE users SET status = $2 WHERE id = $1 AND account_type = 'admin'`, userID, status)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		if status != "active" {
			if _, err := tx.Exec(ctx,
				`UPDATE refresh_tokens SET revoked_at = now()
				 WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx,
				`UPDATE admin_setup_challenges SET used_at = now()
				 WHERE user_id = $1 AND used_at IS NULL`, userID)
			return err
		}
		return nil
	})
}

// SetRole replaces the user's role with a single role.
func (r *Repository) SetRole(ctx context.Context, userID, roleID string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID)
		return err
	})
}

// DeletePending removes an employee who never accepted the invitation.
func (r *Repository) DeletePending(ctx context.Context, userID string) error {
	ct, err := r.pool.Exec(ctx, `
		DELETE FROM users
		WHERE id = $1 AND account_type = 'admin'
		  AND EXISTS (SELECT 1 FROM admin_staff WHERE user_id = $1 AND status = 'invited')`, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── roles ──────────────────────────────────────────────────────────────────

const roleSelect = `
	SELECT r.id, r.code, r.name, coalesce(r.description,''), r.is_system,
	       coalesce(array_agg(DISTINCT p.code) FILTER (WHERE p.code IS NOT NULL), '{}'),
	       (SELECT count(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id
	         WHERE ur.role_id = r.id AND u.account_type = 'admin')
	FROM roles r
	LEFT JOIN role_permissions rp ON rp.role_id = r.id
	LEFT JOIN permissions p ON p.id = rp.permission_id
	WHERE r.code NOT IN ('SELLER','SELLER_STAFF','CUSTOMER')`

func scanRole(row pgx.Row) (*Role, error) {
	var ro Role
	err := row.Scan(&ro.ID, &ro.Code, &ro.Name, &ro.Description, &ro.IsSystem, &ro.Permissions, &ro.MemberCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ro, nil
}

// ListRoles returns every admin-desk role with its permissions.
func (r *Repository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.pool.Query(ctx, roleSelect+` GROUP BY r.id ORDER BY r.is_system DESC, r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		ro, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ro)
	}
	return out, rows.Err()
}

// GetRole returns one role.
func (r *Repository) GetRole(ctx context.Context, id string) (*Role, error) {
	return scanRole(r.pool.QueryRow(ctx, roleSelect+` AND r.id = $1 GROUP BY r.id`, id))
}

// RoleNameTaken reports whether another role already uses the name.
func (r *Repository) RoleNameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM roles
		              WHERE lower(name) = lower($1::text) AND ($2::text = '' OR id::text <> $2::text))`,
		name, exceptID).Scan(&ok)
	return ok, err
}

// CreateRole inserts a custom role and its permission links.
func (r *Repository) CreateRole(ctx context.Context, code, name, description string, perms []string) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO roles (code, name, description, is_system)
			VALUES ($1, $2, $3, FALSE) RETURNING id`, code, name, description).Scan(&id); err != nil {
			return err
		}
		return linkPermissions(ctx, tx, id, perms)
	})
	return id, err
}

// UpdateRole renames a custom role and replaces its permissions.
func (r *Repository) UpdateRole(ctx context.Context, id, name, description string, perms []string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE roles SET name = $2, description = $3
			WHERE id = $1 AND is_system = FALSE`, id, name, description)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, id); err != nil {
			return err
		}
		return linkPermissions(ctx, tx, id, perms)
	})
}

// DeleteRole removes a custom role.
func (r *Repository) DeleteRole(ctx context.Context, id string) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1 AND is_system = FALSE`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func linkPermissions(ctx context.Context, tx pgx.Tx, roleID string, perms []string) error {
	for _, code := range perms {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, id FROM permissions WHERE code = $2
			ON CONFLICT DO NOTHING`, roleID, code); err != nil {
			return err
		}
	}
	return nil
}

// ── own account ────────────────────────────────────────────────────────────

// UpdateAccount changes the caller's own display name and phone.
func (r *Repository) UpdateAccount(ctx context.Context, userID, fullName, phone string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET full_name = $2, phone = NULLIF($3,'')
		WHERE id = $1 AND account_type = 'admin'`, userID, fullName, phone)
	return err
}

// PasswordHash returns the stored password hash of a user.
func (r *Repository) PasswordHash(ctx context.Context, userID string) (string, error) {
	var h string
	err := r.pool.QueryRow(ctx,
		`SELECT password_hash FROM users WHERE id = $1 AND account_type = 'admin'`, userID).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return h, err
}

// SetPassword stores a new password hash and revokes every session.
func (r *Repository) SetPassword(ctx context.Context, userID, hash string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
		return err
	})
}
