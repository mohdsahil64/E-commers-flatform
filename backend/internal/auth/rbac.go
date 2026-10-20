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
)

// AllPermissions lists every permission code (used by the seeder).
func AllPermissions() []string {
	return []string{
		PermSellerView, PermSellerApprove, PermSellerReject, PermSellerSuspend,
		PermSellerActivate, PermSellerBlock,
		PermProductView, PermProductApprove, PermProductReject, PermProductManage,
		PermOrderView, PermOrderManage,
		PermSettlementView, PermSettlementManage,
		PermPackagingView, PermPackagingManage,
		PermBannerManage, PermCategoryManage, PermCollectionManage, PermCouponManage,
		PermReturnManage, PermRefundManage, PermReviewModerate, PermSupportManage,
		PermUserManage, PermRoleManage, PermAuditView, PermReportsView, PermSettingsManage,
	}
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
		},
	}
}
