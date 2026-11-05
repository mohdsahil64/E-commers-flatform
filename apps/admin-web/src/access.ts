// Which permission(s) open which page of the Admin Desk.
//
// A person may open a page when they hold AT LEAST ONE of the listed
// permissions. An empty list means "any signed-in admin". A route name that is
// not listed here is DENIED (deny-by-default), so a forgotten page can never
// leak to someone who should not see it.
//
// This only decides what the UI shows (nav links, route access). The real
// enforcement is on the server: every API route checks the caller's permissions
// against the database on every request, so editing this file, the browser's
// storage or a request in dev-tools can never grant access to data.

export const PAGE_ACCESS: Record<string, string[]> = {
  dashboard: [],
  settings: [],
  'no-access': [],

  sellers: ['seller.view'],
  'seller-detail': ['seller.view'],
  products: ['product.view'],
  'product-detail': ['product.view'],
  categories: ['category.manage'],
  customers: ['customer.view'],
  'customer-detail': ['customer.view'],

  orders: ['order.view'],
  'order-detail': ['order.view'],
  shipping: ['order.view'],
  returns: ['return.manage', 'refund.manage'],
  support: ['support.manage'],
  'seller-support': ['support.manage'],
  packaging: ['packaging.view'],

  payments: ['settlement.view'],
  settlements: ['settlement.view'],
  reports: ['reports.view'],

  collections: ['collection.manage'],
  banners: ['banner.manage'],
  team: ['banner.manage'],
  coupons: ['coupon.manage'],
  promotions: ['coupon.manage'],

  roles: ['user.manage', 'role.manage'],
  'audit-logs': ['audit.view'],
}

/** Routes in the order we try when sending someone to "their" first page. */
export const LANDING_ORDER = [
  'dashboard', 'orders', 'sellers', 'products', 'customers', 'returns', 'support',
  'settlements', 'reports', 'collections', 'coupons', 'packaging', 'roles', 'settings',
]
