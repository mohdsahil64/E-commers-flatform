package auth

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// systemRoleNames are the built-in roles and their display names.
var systemRoleNames = map[string]string{
	RoleSuperAdmin: "Super Admin", RoleAdmin: "Admin",
	RoleCatalogAdmin: "Catalog Admin", RoleOrderAdmin: "Order Admin",
	RoleSellerAdmin: "Seller Admin", RoleFinanceAdmin: "Finance Admin",
	RoleSupportAdmin: "Support Admin", RoleSeller: "Seller",
	RoleSellerStaff: "Seller Staff", RoleCustomer: "Customer",
}

// SyncRBAC makes sure every permission in the catalog, every built-in role and
// the default role -> permission links exist. It only ever ADDS or refreshes
// descriptions, so it is safe to run on every start: it never removes a
// permission from a role and never touches custom roles. This is what makes a
// newly introduced permission appear on an existing database without a
// re-seed.
func SyncRBAC(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, p := range PermissionCatalog() {
			if _, err := tx.Exec(ctx, `
				INSERT INTO permissions (code, description) VALUES ($1,$2)
				ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description`,
				p.Code, p.Description); err != nil {
				return err
			}
		}
		for code, name := range systemRoleNames {
			if _, err := tx.Exec(ctx, `
				INSERT INTO roles (code, name, is_system) VALUES ($1,$2,TRUE)
				ON CONFLICT (code) DO NOTHING`, code, name); err != nil {
				return err
			}
		}
		for role, perms := range RolePermissionMatrix() {
			for _, perm := range perms {
				if _, err := tx.Exec(ctx, `
					INSERT INTO role_permissions (role_id, permission_id)
					SELECT r.id, p.id FROM roles r, permissions p
					WHERE r.code = $1 AND p.code = $2
					ON CONFLICT DO NOTHING`, role, perm); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
