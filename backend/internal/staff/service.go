package staff

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/carlitos/backend/internal/auth"
	"github.com/carlitos/backend/internal/emailtpl"
	"github.com/carlitos/backend/internal/httpx"
)

// Mailer delivers one HTML e-mail.
type Mailer func(ctx context.Context, to, subject, htmlBody string) error

// AuditFn records an administrative action.
type AuditFn func(ctx context.Context, actorID, action, entityType, entityID string)

// inviteTTL is how long a temporary password stays usable.
const inviteTTL = 72 * time.Hour

// Service holds the staff / role / account business rules.
type Service struct {
	repo       *Repository
	bcryptCost int
	loginURL   string
	mail       Mailer
	audit      AuditFn
	logger     *slog.Logger
	// devLogger is non-nil ONLY when e-mail is not really delivered (the dev
	// "log" provider); it lets a developer read the temporary password locally.
	devLogger *slog.Logger
}

// NewService builds a staff Service. devLogger must be nil in production.
func NewService(repo *Repository, bcryptCost int, loginURL string, mail Mailer, audit AuditFn, logger, devLogger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, bcryptCost: bcryptCost, loginURL: loginURL, mail: mail, audit: audit, logger: logger, devLogger: devLogger}
}

func (s *Service) record(ctx context.Context, actor *auth.Principal, action, entityType, entityID string) {
	if s.audit != nil && actor != nil {
		s.audit(ctx, actor.UserID, action, entityType, entityID)
	}
}

// ── validation helpers ─────────────────────────────────────────────────────

var (
	nameRe = regexp.MustCompile(`^[\p{L}][\p{L}\p{M} .'\-]{1,78}[\p{L}\p{M}.]$`)
	roleRe = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} &\-_().,/]{1,39}$`)
)

func validUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func cleanName(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// normalizePhone accepts an Indian 10-digit mobile (starting 6-9, optionally
// prefixed with 0 / 91 / +91) or an international number written as +<digits>
// (11–15 digits). It returns the canonical form and whether it is valid.
func normalizePhone(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	plus := strings.HasPrefix(raw, "+")
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case plus && strings.HasPrefix(d, "91") && len(d) == 12:
		d = d[2:]
	case !plus && len(d) == 12 && strings.HasPrefix(d, "91"):
		d = d[2:]
	case !plus && len(d) == 11 && strings.HasPrefix(d, "0"):
		d = d[1:]
	}
	if len(d) == 10 && d[0] >= '6' && d[0] <= '9' {
		return d, true
	}
	if plus && len(d) >= 11 && len(d) <= 15 {
		return "+" + d, true
	}
	return "", false
}

// ── employees ──────────────────────────────────────────────────────────────

// ListEmployees returns every admin-desk account, flagging the caller's own.
func (s *Service) ListEmployees(ctx context.Context, actor *auth.Principal) ([]Employee, error) {
	list, err := s.repo.ListEmployees(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].IsYou = list[i].ID == actor.UserID
	}
	return list, nil
}

// Invite creates a pending employee and e-mails their temporary password. The
// returned bool reports whether the e-mail was handed to the mail system.
func (s *Service) Invite(ctx context.Context, actor *auth.Principal, in InviteInput) (*Employee, bool, error) {
	fields := map[string]any{}
	name := cleanName(in.FullName)
	if !nameRe.MatchString(name) {
		fields["full_name"] = "Enter the employee's full name (letters only, 3–80 characters)"
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email || len(email) > 254 || !strings.Contains(email, ".") {
		fields["email"] = "Enter a valid email address"
	}
	phone, phoneOK := normalizePhone(in.Phone)
	if !phoneOK {
		fields["phone"] = "Enter a valid 10-digit mobile number"
	}
	if !validUUID(in.RoleID) {
		fields["role_id"] = "Choose a role"
	}
	if len(fields) > 0 {
		return nil, false, httpx.BadRequest("validation_failed", "Please fix the highlighted fields").WithFields(fields)
	}

	role, err := s.repo.GetRole(ctx, in.RoleID)
	if errors.Is(err, ErrNotFound) {
		return nil, false, httpx.BadRequest("validation_failed", "That role no longer exists").
			WithFields(map[string]any{"role_id": "Choose a role"})
	}
	if err != nil {
		return nil, false, err
	}
	if !auth.IsAssignableStaffRole(role.Code) {
		return nil, false, httpx.Forbidden("role_not_assignable", "This role can't be given to an employee")
	}
	if !actor.HoldsAll(role.Permissions) {
		return nil, false, httpx.Forbidden("role_exceeds_access",
			"You can only assign roles whose permissions you hold yourself")
	}

	taken, err := s.repo.EmailExists(ctx, email)
	if err != nil {
		return nil, false, err
	}
	if taken {
		return nil, false, httpx.Conflict("email_taken", "An admin account with this email already exists").
			WithFields(map[string]any{"email": "Already registered"})
	}

	temp, err := auth.GenerateTempPassword()
	if err != nil {
		return nil, false, httpx.Internal("could not generate a temporary password")
	}
	hash, err := auth.HashPassword(temp, s.bcryptCost)
	if err != nil {
		return nil, false, httpx.Internal("could not secure the temporary password")
	}
	id, err := s.repo.CreateStaff(ctx, CreateStaffParams{
		FullName: name, Email: email, Phone: phone, PasswordHash: hash,
		RoleID: role.ID, InvitedBy: actor.UserID, ExpiresAt: time.Now().Add(inviteTTL),
	})
	if errors.Is(err, ErrEmailTaken) {
		return nil, false, httpx.Conflict("email_taken", "An admin account with this email already exists")
	}
	if err != nil {
		return nil, false, err
	}
	s.record(ctx, actor, "staff.invited", "staff", id)

	emp, err := s.repo.GetEmployee(ctx, id)
	if err != nil {
		return nil, false, err
	}
	emailed := s.sendInvitation(ctx, actor, emp, role.Name, temp) == nil
	emp.IsYou = false
	return emp, emailed, nil
}

func (s *Service) sendInvitation(ctx context.Context, actor *auth.Principal, e *Employee, roleName, temp string) error {
	if s.devLogger != nil {
		s.devLogger.Warn("DEV ONLY: e-mail is not delivered — employee temporary password",
			"to", e.Email, "temp_password", temp)
	}
	if s.mail == nil {
		return errors.New("mailer not configured")
	}
	invitedBy := ""
	if actor != nil {
		invitedBy = actor.Email
	}
	if emp, err := s.repo.GetEmployee(ctx, actor.UserID); err == nil && emp.FullName != "" {
		invitedBy = emp.FullName
	}
	html := emailtpl.StaffInvitation(emailtpl.StaffInvitationData{
		Name: e.FullName, RoleName: roleName, InvitedBy: invitedBy, Email: e.Email,
		TempPassword: temp, LoginURL: s.loginURL, ExpiryHours: int(inviteTTL / time.Hour),
	})
	if err := s.mail(ctx, e.Email, "You're invited to the CARLITOS admin desk", html); err != nil {
		s.logger.Error("staff invitation email failed", "to", e.Email, "error", err)
		return err
	}
	return nil
}

// loadManageable fetches an employee the caller is allowed to act on.
func (s *Service) loadManageable(ctx context.Context, actor *auth.Principal, id string) (*Employee, error) {
	if !validUUID(id) {
		return nil, httpx.NotFound("staff_not_found", "Employee not found")
	}
	t, err := s.repo.GetEmployee(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NotFound("staff_not_found", "Employee not found")
	}
	if err != nil {
		return nil, err
	}
	if t.ID == actor.UserID {
		return nil, httpx.Forbidden("cannot_modify_self", "You can't do this to your own account")
	}
	if actor.IsSuperAdmin() {
		return t, nil
	}
	if t.IsSuperAdmin {
		return nil, httpx.Forbidden("higher_access", "Only a Super Admin can manage this account")
	}
	_, perms, err := s.repo.UserRolesAndPerms(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	if !actor.HoldsAll(perms) {
		return nil, httpx.Forbidden("higher_access", "This employee has access that you don't have")
	}
	return t, nil
}

// ResendInvite issues a new temporary password to an employee who has not
// accepted yet (or whose invitation expired).
func (s *Service) ResendInvite(ctx context.Context, actor *auth.Principal, id string) (*Employee, bool, error) {
	t, err := s.loadManageable(ctx, actor, id)
	if err != nil {
		return nil, false, err
	}
	switch t.Status {
	case StatusPending, StatusExpired:
	case StatusDisabled:
		if t.AcceptedAt != nil {
			return nil, false, httpx.Conflict("not_pending", "This employee has already joined")
		}
		return nil, false, httpx.Conflict("account_disabled", "Enable this account first, then resend the invitation")
	default:
		return nil, false, httpx.Conflict("not_pending", "This employee has already joined")
	}
	temp, err := auth.GenerateTempPassword()
	if err != nil {
		return nil, false, httpx.Internal("could not generate a temporary password")
	}
	hash, err := auth.HashPassword(temp, s.bcryptCost)
	if err != nil {
		return nil, false, httpx.Internal("could not secure the temporary password")
	}
	if err := s.repo.ResetInvite(ctx, t.ID, hash, time.Now().Add(inviteTTL)); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, httpx.Conflict("not_pending", "This employee has already joined")
		}
		return nil, false, err
	}
	s.record(ctx, actor, "staff.invite_resent", "staff", t.ID)
	emp, err := s.repo.GetEmployee(ctx, t.ID)
	if err != nil {
		return nil, false, err
	}
	emailed := s.sendInvitation(ctx, actor, emp, emp.RoleName, temp) == nil
	return emp, emailed, nil
}

// Disable switches an employee off: sessions are revoked immediately.
func (s *Service) Disable(ctx context.Context, actor *auth.Principal, id string) (*Employee, error) {
	t, err := s.loadManageable(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusDisabled {
		return t, nil
	}
	if t.IsSuperAdmin {
		n, err := s.repo.CountActiveSuperAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, httpx.Conflict("last_super_admin", "You can't disable the last Super Admin")
		}
	}
	if err := s.repo.SetUserStatus(ctx, t.ID, "inactive"); err != nil {
		return nil, err
	}
	s.record(ctx, actor, "staff.disabled", "staff", t.ID)
	return s.repo.GetEmployee(ctx, t.ID)
}

// Enable switches a disabled employee back on.
func (s *Service) Enable(ctx context.Context, actor *auth.Principal, id string) (*Employee, error) {
	t, err := s.loadManageable(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if t.Status != StatusDisabled {
		return t, nil
	}
	if err := s.repo.SetUserStatus(ctx, t.ID, "active"); err != nil {
		return nil, err
	}
	s.record(ctx, actor, "staff.enabled", "staff", t.ID)
	return s.repo.GetEmployee(ctx, t.ID)
}

// ChangeRole gives an employee a different role. The change applies on their
// very next request (AdminGuard re-reads permissions every time).
func (s *Service) ChangeRole(ctx context.Context, actor *auth.Principal, id, roleID string) (*Employee, error) {
	t, err := s.loadManageable(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if t.IsSuperAdmin {
		return nil, httpx.Forbidden("super_admin_locked", "A Super Admin's role can't be changed here")
	}
	if !validUUID(roleID) {
		return nil, httpx.BadRequest("validation_failed", "Choose a role")
	}
	role, err := s.repo.GetRole(ctx, roleID)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.BadRequest("validation_failed", "That role no longer exists")
	}
	if err != nil {
		return nil, err
	}
	if !auth.IsAssignableStaffRole(role.Code) {
		return nil, httpx.Forbidden("role_not_assignable", "This role can't be given to an employee")
	}
	if !actor.HoldsAll(role.Permissions) {
		return nil, httpx.Forbidden("role_exceeds_access", "You can only assign roles whose permissions you hold yourself")
	}
	if err := s.repo.SetRole(ctx, t.ID, role.ID); err != nil {
		return nil, err
	}
	s.record(ctx, actor, "staff.role_changed", "staff", t.ID)
	return s.repo.GetEmployee(ctx, t.ID)
}

// CancelInvite deletes an employee who never accepted their invitation.
func (s *Service) CancelInvite(ctx context.Context, actor *auth.Principal, id string) error {
	t, err := s.loadManageable(ctx, actor, id)
	if err != nil {
		return err
	}
	if t.AcceptedAt != nil || t.IsSuperAdmin || (t.Status == StatusActive) {
		return httpx.Conflict("already_joined", "Only invitations that were never accepted can be removed. Disable the account instead.")
	}
	if err := s.repo.DeletePending(ctx, t.ID); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.Conflict("already_joined", "Only invitations that were never accepted can be removed. Disable the account instead.")
		}
		return err
	}
	s.record(ctx, actor, "staff.invite_cancelled", "staff", t.ID)
	return nil
}

// ── roles ──────────────────────────────────────────────────────────────────

// ListRoles returns the roles annotated with what the caller may do with each.
func (s *Service) ListRoles(ctx context.Context, actor *auth.Principal) ([]Role, error) {
	roles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	canManageRoles := actor.HasPermission(auth.PermRoleManage)
	for i := range roles {
		r := &roles[i]
		sort.Strings(r.Permissions)
		holds := actor.HoldsAll(r.Permissions)
		r.Assignable = auth.IsAssignableStaffRole(r.Code) && holds
		r.Editable = canManageRoles && !r.IsSystem && holds
	}
	return roles, nil
}

// Permissions returns the catalog flagged with what the caller holds.
func (s *Service) Permissions(actor *auth.Principal) []PermissionItem {
	cat := auth.PermissionCatalog()
	out := make([]PermissionItem, 0, len(cat))
	for _, p := range cat {
		out = append(out, PermissionItem{
			Code: p.Code, Label: p.Label, Group: p.Group, Description: p.Description,
			Held: actor.HasPermission(p.Code),
		})
	}
	return out
}

func (s *Service) validateRole(actor *auth.Principal, name, desc string, perms []string) (string, string, []string, error) {
	fields := map[string]any{}
	name = cleanName(name)
	if !roleRe.MatchString(name) {
		fields["name"] = "Use 2–40 characters (letters, numbers, spaces and - _ & ( ) . ,)"
	}
	desc = strings.TrimSpace(desc)
	if len([]rune(desc)) > 200 {
		fields["description"] = "Keep the description under 200 characters"
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(perms))
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		if !auth.IsKnownPermission(p) {
			fields["permissions"] = "Unknown permission: " + p
			break
		}
		seen[p] = true
		clean = append(clean, p)
	}
	if _, bad := fields["permissions"]; !bad && len(clean) == 0 {
		fields["permissions"] = "Choose at least one permission"
	}
	if len(fields) > 0 {
		return "", "", nil, httpx.BadRequest("validation_failed", "Please fix the highlighted fields").WithFields(fields)
	}
	if !actor.HoldsAll(clean) {
		var missing []string
		for _, p := range clean {
			if !actor.HasPermission(p) {
				missing = append(missing, p)
			}
		}
		return "", "", nil, httpx.Forbidden("permission_exceeds_access",
			"You can only grant permissions you hold yourself: "+strings.Join(missing, ", "))
	}
	sort.Strings(clean)
	return name, desc, clean, nil
}

// CreateRole creates a custom role.
func (s *Service) CreateRole(ctx context.Context, actor *auth.Principal, name, desc string, perms []string) (*Role, error) {
	name, desc, perms, err := s.validateRole(actor, name, desc, perms)
	if err != nil {
		return nil, err
	}
	taken, err := s.repo.RoleNameTaken(ctx, name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, httpx.Conflict("role_name_taken", "A role with this name already exists").
			WithFields(map[string]any{"name": "Already in use"})
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return nil, httpx.Internal("could not create the role")
	}
	code := "CUSTOM_" + strings.ToUpper(hex.EncodeToString(b))
	id, err := s.repo.CreateRole(ctx, code, name, desc, perms)
	if err != nil {
		return nil, err
	}
	s.record(ctx, actor, "role.created", "role", id)
	return s.repo.GetRole(ctx, id)
}

// loadEditableRole returns a custom role the caller may change.
func (s *Service) loadEditableRole(ctx context.Context, actor *auth.Principal, id string) (*Role, error) {
	if !validUUID(id) {
		return nil, httpx.NotFound("role_not_found", "Role not found")
	}
	role, err := s.repo.GetRole(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NotFound("role_not_found", "Role not found")
	}
	if err != nil {
		return nil, err
	}
	if role.IsSystem {
		return nil, httpx.Forbidden("system_role", "Built-in roles can't be changed. Create a custom role instead.")
	}
	if !actor.HoldsAll(role.Permissions) {
		return nil, httpx.Forbidden("higher_access", "This role grants access that you don't have")
	}
	return role, nil
}

// UpdateRole changes a custom role's name, description and permissions.
func (s *Service) UpdateRole(ctx context.Context, actor *auth.Principal, id, name, desc string, perms []string) (*Role, error) {
	if _, err := s.loadEditableRole(ctx, actor, id); err != nil {
		return nil, err
	}
	name, desc, perms, err := s.validateRole(actor, name, desc, perms)
	if err != nil {
		return nil, err
	}
	taken, err := s.repo.RoleNameTaken(ctx, name, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, httpx.Conflict("role_name_taken", "A role with this name already exists").
			WithFields(map[string]any{"name": "Already in use"})
	}
	if err := s.repo.UpdateRole(ctx, id, name, desc, perms); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NotFound("role_not_found", "Role not found")
		}
		return nil, err
	}
	s.record(ctx, actor, "role.updated", "role", id)
	return s.repo.GetRole(ctx, id)
}

// DeleteRole removes an unused custom role.
func (s *Service) DeleteRole(ctx context.Context, actor *auth.Principal, id string) error {
	role, err := s.loadEditableRole(ctx, actor, id)
	if err != nil {
		return err
	}
	if role.MemberCount > 0 {
		return httpx.Conflict("role_in_use", fmt.Sprintf(
			"This role is assigned to %d employee(s). Move them to another role first.", role.MemberCount))
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.NotFound("role_not_found", "Role not found")
		}
		return err
	}
	s.record(ctx, actor, "role.deleted", "role", id)
	return nil
}

// ── own account ────────────────────────────────────────────────────────────

// Account returns the caller's own profile, tailored to who they are.
func (s *Service) Account(ctx context.Context, actor *auth.Principal) (*Account, error) {
	me, err := s.repo.GetEmployee(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.Unauthorized("invalid_token", "Invalid or expired token")
	}
	if err != nil {
		return nil, err
	}
	roleCodes, perms, err := s.repo.UserRolesAndPerms(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	allRoles, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, r := range allRoles {
		names[r.Code] = r.Name
	}
	briefs := make([]RoleBrief, 0, len(roleCodes))
	for _, c := range roleCodes {
		n := names[c]
		if n == "" {
			n = c
		}
		briefs = append(briefs, RoleBrief{Code: c, Name: n})
	}
	sort.Strings(perms)

	kind := "employee"
	if me.InvitedBy == "" && me.AcceptedAt == nil && me.InvitedAt == nil {
		kind = "owner" // seeded / platform-provisioned admin, never invited
	}
	acc := &Account{
		ID: me.ID, FullName: me.FullName, Email: me.Email, Phone: me.Phone,
		Status: me.Status, Kind: kind, IsSuperAdmin: me.IsSuperAdmin,
		Roles: briefs, Permissions: perms, InvitedBy: me.InvitedBy,
		AcceptedAt: me.AcceptedAt, LastLoginAt: me.LastLoginAt, CreatedAt: me.CreatedAt,
	}
	if actor.HasPermission(auth.PermUserManage) {
		list, err := s.repo.ListEmployees(ctx)
		if err != nil {
			return nil, err
		}
		t := &TeamSummary{Roles: len(allRoles)}
		for _, e := range list {
			switch e.Status {
			case StatusActive:
				t.Active++
			case StatusPending, StatusExpired:
				t.Pending++
			case StatusDisabled:
				t.Disabled++
			}
		}
		acc.Team = t
	}
	return acc, nil
}

// UpdateAccount changes the caller's display name and phone.
func (s *Service) UpdateAccount(ctx context.Context, actor *auth.Principal, fullName, phone string) (*Account, error) {
	fields := map[string]any{}
	name := cleanName(fullName)
	if !nameRe.MatchString(name) {
		fields["full_name"] = "Enter your full name (letters only, 3–80 characters)"
	}
	p := strings.TrimSpace(phone)
	if p != "" {
		norm, ok := normalizePhone(p)
		if !ok {
			fields["phone"] = "Enter a valid 10-digit mobile number"
		}
		p = norm
	}
	if len(fields) > 0 {
		return nil, httpx.BadRequest("validation_failed", "Please fix the highlighted fields").WithFields(fields)
	}
	if err := s.repo.UpdateAccount(ctx, actor.UserID, name, p); err != nil {
		return nil, err
	}
	return s.Account(ctx, actor)
}

// ChangePassword verifies the current password, applies the staff password
// policy and signs the person out everywhere (they sign in again).
func (s *Service) ChangePassword(ctx context.Context, actor *auth.Principal, current, next string) error {
	hash, err := s.repo.PasswordHash(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return httpx.Unauthorized("invalid_token", "Invalid or expired token")
	}
	if err != nil {
		return err
	}
	if !auth.CheckPassword(hash, current) {
		return httpx.Unauthorized("invalid_password", "Your current password is incorrect")
	}
	if msg := auth.StaffPasswordError(next, actor.Email); msg != "" {
		return httpx.BadRequest("weak_password", msg)
	}
	if auth.CheckPassword(hash, next) {
		return httpx.BadRequest("same_password", "Choose a password you haven't used just now")
	}
	newHash, err := auth.HashPassword(next, s.bcryptCost)
	if err != nil {
		return httpx.Internal("failed to hash password")
	}
	if err := s.repo.SetPassword(ctx, actor.UserID, newHash); err != nil {
		return err
	}
	s.record(ctx, actor, "account.password_changed", "staff", actor.UserID)
	return nil
}
