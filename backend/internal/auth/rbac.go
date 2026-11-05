package auth

// Role codes (RBAC). These match the roles seeded into the database.
const (
	RoleSuperAdmin   = "SUPER_ADMIN"
	RoleAdmin        = "ADMIN"
	RoleCatalogAdmin = "CATALOG_ADMIN"
	RoleOrderAdmin   = "ORDER_ADMIN"
	RoleSellerAdmin  = "SELLER_ADMIN"
	RoleFinanceAdmin = "FINANCE_ADMIN"
	RoleSupportAdmin = "SUPPORT_ADMIN"

	RoleSeller      = "SELLER"
	RoleSellerStaff = "SELLER_STAFF"

	RoleCustomer = "CUSTOMER"
)

// Permission codes. Granular actions enforced by the backend.
const (
	PermSellerView      = "seller.view"
	PermSellerApprove   = "seller.approve"
	PermSellerReject    = "seller.reject"
	PermSellerSuspend   = "seller.suspend"
	PermSellerActivate  = "seller.activate"
	PermSellerBlock     = "seller.block"

	PermProductView    = "product.view"
	PermProductApprove = "product.approve"
	PermProductReject  = "product.reject"
	PermProductManage  = "product.manage"

	PermOrderView   = "order.view"
	PermOrderManage = "order.manage"

	PermSettlementView   = "settlement.view"
	PermSettlementManage = "settlement.manage"

	PermPackagingView   = "packaging.view"
	PermPackagingManage = "packaging.manage"

	PermBannerManage    = "banner.manage"
	PermCategoryManage  = "category.manage"
	PermCollectionManage = "collection.manage"
	PermCouponManage    = "coupon.manage"

	PermReturnManage   = "return.manage"
	PermRefundManage   = "refund.manage"
	PermReviewModerate = "review.moderate"
	PermSupportManage  = "support.manage"

	PermUserManage  = "user.manage"
	PermRoleManage  = "role.manage"
	PermAuditView   = "audit.view"
	PermReportsView = "reports.view"
	PermSettingsManage = "settings.manage"

	PermCustomerView  = "customer.view"
	PermCustomerBlock = "customer.block"
)

// PermissionInfo describes one permission for the Roles & Employees UI.
type PermissionInfo struct {
	Code        string `json:"code"`
	Label       string `json:"label"`
	Group       string `json:"group"`
	Description string `json:"description"`
}

// PermissionCatalog is the single source of truth for every permission: its
// human label, the group it is shown under, and what it allows. The Roles page
// renders this list; the seeder/sync writes it to the permissions table.
func PermissionCatalog() []PermissionInfo {
	return []PermissionInfo{
		{PermSellerView, "View sellers", "Sellers", "See the seller list and seller details"},
		{PermSellerApprove, "Approve sellers", "Sellers", "Approve seller applications"},
		{PermSellerReject, "Reject sellers", "Sellers", "Reject seller applications"},
		{PermSellerSuspend, "Suspend sellers", "Sellers", "Suspend a seller store"},
		{PermSellerActivate, "Reactivate sellers", "Sellers", "Reactivate or unsuspend a seller store"},
		{PermSellerBlock, "Block sellers", "Sellers", "Block or unblock a seller account"},

		{PermProductView, "View products", "Catalog", "See products submitted by sellers"},
		{PermProductApprove, "Approve products", "Catalog", "Approve seller products so they go live"},
		{PermProductReject, "Reject products", "Catalog", "Reject seller products"},
		{PermProductManage, "Manage products", "Catalog", "Edit and manage product listings"},
		{PermCategoryManage, "Manage categories", "Catalog", "Create and edit categories"},
		{PermCollectionManage, "Manage collections", "Catalog", "Create collections and curate their products"},
		{PermBannerManage, "Manage banners & team page", "Catalog", "Manage homepage banners and the team page"},

		{PermOrderView, "View orders", "Orders & Returns", "See orders and shipping status"},
		{PermOrderManage, "Manage orders", "Orders & Returns", "Change order status and cancel orders"},
		{PermReturnManage, "Manage returns", "Orders & Returns", "See return and exchange requests"},
		{PermRefundManage, "Process refunds", "Orders & Returns", "Pay out refunds and record the UTR"},

		{PermSettlementView, "View settlements", "Finance", "See seller settlements and payments"},
		{PermSettlementManage, "Manage settlements", "Finance", "Generate settlements and mark them paid"},
		{PermReportsView, "View reports", "Finance", "Open sales and finance reports"},

		{PermSupportManage, "Handle support tickets", "Support & Reviews", "Reply to and resolve customer and seller tickets"},
		{PermReviewModerate, "Moderate reviews", "Support & Reviews", "Approve or hide customer reviews"},
		{PermCustomerView, "View customers", "Support & Reviews", "See the customer list and customer details"},
		{PermCustomerBlock, "Block customers", "Support & Reviews", "Block or unblock a customer account"},

		{PermCouponManage, "Manage coupons & promotions", "Marketing", "Create and manage coupons and offers"},

		{PermPackagingView, "View packaging", "Packaging", "See packaging stock and seller requests"},
		{PermPackagingManage, "Manage packaging", "Packaging", "Add stock and approve packaging requests"},

		{PermUserManage, "Manage employees", "Administration", "Invite, disable and change the role of employees"},
		{PermRoleManage, "Manage roles", "Administration", "Create roles and choose what each role can do"},
		{PermAuditView, "View audit logs", "Administration", "See the log of administrative actions"},
		{PermSettingsManage, "Manage platform settings", "Administration", "Change platform-wide settings"},
	}
}

// AllPermissions lists every permission code (used by the seeder).
func AllPermissions() []string {
	cat := PermissionCatalog()
	out := make([]string, 0, len(cat))
	for _, p := range cat {
		out = append(out, p.Code)
	}
	return out
}

// IsKnownPermission reports whether code is a real permission.
func IsKnownPermission(code string) bool {
	for _, p := range PermissionCatalog() {
		if p.Code == code {
			return true
		}
	}
	return false
}

// IsAssignableStaffRole reports whether a role may be given to an employee.
// SUPER_ADMIN is never assignable through the UI, and the marketplace roles
// (seller / customer) belong to other portals.
func IsAssignableStaffRole(code string) bool {
	switch code {
	case RoleSuperAdmin, RoleSeller, RoleSellerStaff, RoleCustomer, "":
		return false
	}
	return true
}

// RolePermissionMatrix maps each admin role to the permissions it holds.
// SUPER_ADMIN implicitly has all permissions (handled in the middleware).
func RolePermissionMatrix() map[string][]string {
	return map[string][]string{
		RoleSuperAdmin: AllPermissions(),
		RoleAdmin: {
			PermSellerView, PermSellerApprove, PermSellerReject, PermSellerSuspend,
			PermSellerActivate, PermSellerBlock,
			PermProductView, PermProductApprove, PermProductReject, PermProductManage,
			PermOrderView, PermOrderManage, PermSettlementView, PermSettlementManage,
			PermPackagingView, PermPackagingManage, PermBannerManage, PermCategoryManage,
			PermCollectionManage, PermCouponManage, PermReturnManage, PermRefundManage,
			PermReviewModerate, PermSupportManage, PermReportsView, PermAuditView,
			PermCustomerView, PermCustomerBlock,
		},
		RoleCatalogAdmin: {
			PermProductView, PermProductApprove, PermProductReject, PermProductManage,
			PermCategoryManage, PermCollectionManage, PermBannerManage, PermReviewModerate,
		},
		RoleOrderAdmin: {
			PermOrderView, PermOrderManage, PermReturnManage, PermRefundManage,
		},
		RoleSellerAdmin: {
			PermSellerView, PermSellerApprove, PermSellerReject, PermSellerSuspend,
			PermSellerActivate, PermSellerBlock,
		},
		RoleFinanceAdmin: {
			PermSettlementView, PermSettlementManage, PermRefundManage, PermReportsView,
		},
		RoleSupportAdmin: {
			PermOrderView, PermReturnManage, PermReviewModerate, PermSupportManage,
			PermCustomerView,
		},
	}
}
