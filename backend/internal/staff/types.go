// Package staff manages the people who work in the admin desk: inviting
// employees, their lifecycle (pending -> active -> disabled), the roles they
// hold and what each role may do, plus the signed-in person's own account.
//
// Security model (the browser is never trusted):
//   - every route is guarded server-side by permission (see handler.go) and the
//     auth.AdminGuard middleware re-reads the caller's live roles from the DB;
//   - nobody can grant access they do not hold themselves (no privilege
//     escalation through invites, role edits or role assignment);
//   - a person cannot act on their own account, nor on someone more privileged;
//   - the last active SUPER_ADMIN can never be disabled or demoted.
package staff

import "time"

// Employee status values shown in the UI.
const (
	StatusActive   = "active"   // accepted the invitation and set a password
	StatusPending  = "pending"  // invited, has not completed first sign-in yet
	StatusExpired  = "expired"  // invited, but the temporary password timed out
	StatusDisabled = "disabled" // switched off by an administrator
)

// Employee is one admin-desk account as listed on the Employees tab.
type Employee struct {
	ID              string     `json:"id"`
	FullName        string     `json:"full_name"`
	Email           string     `json:"email"`
	Phone           string     `json:"phone"`
	Status          string     `json:"status"`
	RoleID          string     `json:"role_id"`
	RoleCode        string     `json:"role_code"`
	RoleName        string     `json:"role_name"`
	IsSuperAdmin    bool       `json:"is_super_admin"`
	IsYou           bool       `json:"is_you"`
	InvitedBy       string     `json:"invited_by"`
	InvitedAt       *time.Time `json:"invited_at"`
	InviteExpiresAt *time.Time `json:"invite_expires_at"`
	InviteCount     int        `json:"invite_count"`
	AcceptedAt      *time.Time `json:"accepted_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Role is a role with the permissions it grants.
type Role struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"`
	MemberCount int      `json:"member_count"`
	// Editable is true when the caller may change or delete this role.
	Editable bool `json:"editable"`
	// Assignable is true when the caller may give this role to an employee.
	Assignable bool `json:"assignable"`
}

// PermissionItem is a catalog permission, flagged with whether the caller holds
// it (a caller can only hand out permissions they hold).
type PermissionItem struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Group       string `json:"group"`
	Description string `json:"description"`
	Held        bool   `json:"held"`
}

// RoleBrief is a short role reference.
type RoleBrief struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// TeamSummary counts employees by status (shown to people who manage staff).
type TeamSummary struct {
	Active   int `json:"active"`
	Pending  int `json:"pending"`
	Disabled int `json:"disabled"`
	Roles    int `json:"roles"`
}

// Account is the signed-in person's own profile (Settings page).
type Account struct {
	ID           string       `json:"id"`
	FullName     string       `json:"full_name"`
	Email        string       `json:"email"`
	Phone        string       `json:"phone"`
	Status       string       `json:"status"`
	Kind         string       `json:"kind"` // owner | employee
	IsSuperAdmin bool         `json:"is_super_admin"`
	Roles        []RoleBrief  `json:"roles"`
	Permissions  []string     `json:"permissions"`
	InvitedBy    string       `json:"invited_by"`
	AcceptedAt   *time.Time   `json:"accepted_at"`
	LastLoginAt  *time.Time   `json:"last_login_at"`
	CreatedAt    time.Time    `json:"created_at"`
	Team         *TeamSummary `json:"team,omitempty"`
}

// InviteInput is the admin's input when inviting an employee.
type InviteInput struct {
	FullName string
	Email    string
	Phone    string
	RoleID   string
}
