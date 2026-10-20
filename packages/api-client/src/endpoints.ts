import type {
  ApiEnvelope, TokenPair, Gender, Category, Brand, Attribute, Product, ListingCard,
  CartItem, WishlistItem, Order, Seller, SellerOnboardingBundle, Collection, Banner,
  TeamMember, Address, CustomerProfile, AppNotification, Settlement, AccountType,
  OrderTracking, Coupon, CouponRedemption, ReturnRecord, SupportTicket,
} from '@carlitos/types'
import { ApiClient } from './client'

// ---------------------------------------------------------------------
// Auth (all portals)
// ---------------------------------------------------------------------
export const authApi = (c: ApiClient) => ({
  // Customer mobile-OTP login (primary customer auth).
  requestOtp: (phone: string) =>
    c.post<{ sent: boolean }>('/auth/customer/otp/request', { phone }),
  verifyOtp: (phone: string, code: string) =>
    c.post<TokenPair>('/auth/customer/otp/verify', { phone, code }),

  loginCustomer: (email: string, password: string) =>
    c.post<TokenPair>('/auth/customer/login', { email, password }),
  loginSeller: (email: string, password: string) =>
    c.post<TokenPair>('/auth/seller/login', { email, password }),
  loginAdmin: (email: string, password: string) =>
    c.post<TokenPair>('/auth/admin/login', { email, password }),
  registerCustomer: (body: { full_name: string; email: string; phone?: string; password: string }) =>
    c.post<TokenPair>('/auth/customer/register', body),
  registerSeller: (body: { owner_name: string; email: string; phone?: string; password: string; store_name: string }) =>
    c.post<TokenPair>('/auth/seller/register', body),

  // Forgot / reset password (email-password portals: seller & admin).
  // The request endpoint always resolves successfully — the server never
  // reveals whether an email is registered (no account enumeration).
  forgotPasswordSeller: (email: string) =>
    c.post<{ sent: boolean }>('/auth/seller/forgot-password', { email }),
  forgotPasswordAdmin: (email: string) =>
    c.post<{ sent: boolean }>('/auth/admin/forgot-password', { email }),
  // resetPassword consumes the single-use token from the emailed link.
  resetPassword: (token: string, password: string) =>
    c.post<void>('/auth/reset-password', { token, password }),

  logout: (refresh_token: string) => c.post<void>('/auth/logout', { refresh_token }),
  me: () => c.get<{ id: string; account_type: AccountType; email: string; full_name: string; roles: string[]; permissions: string[] }>('/auth/me'),
})

// ---------------------------------------------------------------------
// Public storefront
// ---------------------------------------------------------------------
export const publicApi = (c: ApiClient) => ({
  genders: () => c.get<Gender[]>('/public/genders'),
  categories: () => c.get<Category[]>('/public/categories'),
  brands: () => c.get<Brand[]>('/public/brands'),
  attributes: () => c.get<Attribute[]>('/public/attributes'),
  products: (params?: Record<string, unknown>) =>
    c.getWithMeta<ListingCard[]>('/public/products', params),
  product: (slug: string) => c.get<Product>(`/public/products/${slug}`),
  productReviews: (productId: string) => c.get<unknown[]>(`/public/products/${productId}/reviews`),
  collections: () => c.get<Collection[]>('/public/collections'),
  collection: (slug: string) => c.get<{ collection: Collection; rules: unknown[] }>(`/public/collections/${slug}`),
  collectionProducts: (slug: string) => c.get<unknown[]>(`/public/collections/${slug}/products`),
  banners: (placement?: string) => c.get<Banner[]>('/public/banners', placement ? { placement } : undefined),
  team: () => c.get<TeamMember[]>('/public/team'),
  sellerStore: (slug: string) => c.get<Partial<Seller>>(`/public/sellers/${slug}`),
})

// ---------------------------------------------------------------------
// Customer (authenticated)
// ---------------------------------------------------------------------
export const customerApi = (c: ApiClient) => ({
  profile: () => c.get<CustomerProfile>('/customer/profile'),
  updateProfile: (body: Partial<CustomerProfile>) => c.put<void>('/customer/profile', body),
  requestEmailChange: (email: string) => c.post<{ sent: boolean }>('/customer/profile/email/request', { email }),
  confirmEmailChange: (email: string, code: string) => c.post<void>('/customer/profile/email/confirm', { email, code }),
  addresses: () => c.get<Address[]>('/customer/addresses').then((r) => r ?? []),
  addAddress: (body: Partial<Address>) => c.post<{ id: string }>('/customer/addresses', body),
  setDefaultAddress: (id: string) => c.post<void>(`/customer/addresses/${id}/default`),
  deleteAddress: (id: string) => c.del<void>(`/customer/addresses/${id}`),

  cart: () => c.get<{ items: CartItem[]; subtotal: number; count: number }>('/customer/cart'),
  addToCart: (seller_listing_id: string, quantity = 1) =>
    c.post<void>('/customer/cart/items', { seller_listing_id, quantity }),
  updateCartItem: (listingId: string, quantity: number) =>
    c.put<void>(`/customer/cart/items/${listingId}`, { quantity }),
  removeCartItem: (listingId: string) => c.del<void>(`/customer/cart/items/${listingId}`),

  wishlist: () => c.get<WishlistItem[]>('/customer/wishlist').then((r) => r ?? []),
  addWishlist: (product_id: string) => c.post<void>('/customer/wishlist/items', { product_id }),
  removeWishlist: (productId: string) => c.del<void>(`/customer/wishlist/items/${productId}`),

  checkout: (payment_method: 'online' | 'cod', address_id?: string, coupon_code?: string) =>
    c.post<Order>('/customer/checkout', { payment_method, address_id, coupon_code }),
  // List active offers/coupons available to the customer. Eligibility against
  // the current cart is computed client-side from min_order_amount; the actual
  // discount is always re-validated and recomputed server-side.
  coupons: () => c.get<Coupon[]>('/customer/coupons').then((r) => r ?? []),
  // Validate a coupon against the current cart subtotal (preview the discount
  // before placing the order). Server computes the discount.
  previewCoupon: (code: string, subtotal: number) =>
    c.post<{ code: string; discount: number; message: string }>('/customer/coupons/preview', { code, subtotal }),
  orders: () => c.getWithMeta<Order[]>('/customer/orders'),
  order: (id: string) => c.get<Order>(`/customer/orders/${id}`),
  orderHistory: (id: string) => c.get<unknown[]>(`/customer/orders/${id}/history`),
  trackOrder: (id: string) => c.get<OrderTracking>(`/customer/orders/${id}/track`),
  invoiceURL: (id: string) => `/customer/orders/${id}/invoice`,
  cancelOrder: (id: string, reason?: string) => c.post<void>(`/customer/orders/${id}/cancel`, reason ? { reason } : undefined),
  // Called when an online payment did not complete: unwinds the pending order,
  // releases stock, and restores the cart so the customer can retry.
  abandonOrder: (id: string) => c.post<void>(`/customer/orders/${id}/abandon`),

  payOrder: (orderId: string) => c.post<{ provider_order_id: string; amount: number; currency: string; key_id: string }>(`/customer/orders/${orderId}/pay`),
  verifyPayment: (body: { provider_order_id: string; provider_payment_id: string; signature: string }) =>
    c.post<void>('/customer/payments/verify', body),
  // Dev-only: complete a mock-gateway payment end-to-end (no-op on real providers).
  completeMockPayment: (orderId: string) => c.post<void>(`/customer/orders/${orderId}/pay/mock-complete`),

  returns: () => c.get<ReturnRecord[]>('/customer/returns').then((r) => r ?? []),
  requestReturn: (body: {
    order_item_id: string
    type?: string
    reason_code?: string
    reason?: string
    refund_method?: 'upi' | 'bank'
    refund_upi?: string
    refund_acc_name?: string
    refund_acc_no?: string
    refund_ifsc?: string
    photo_keys?: string[]
  }) => c.post<{ id: string }>('/customer/returns', body),
  // Upload return proof photos; returns the storage key to attach to the request.
  uploadImage: (file: File) => c.uploadFile<{ key: string; url: string; size: number }>('/customer/uploads', file),
  createReview: (body: { order_item_id: string; rating: number; title?: string; body?: string; photo_keys?: string[] }) =>
    c.post<{ id: string }>('/customer/reviews', body),

  // ── Help & Support (ticketing) ──────────────────────────────────────
  supportTickets: () => c.get<SupportTicket[]>('/customer/support/tickets').then((r) => r ?? []),
  openSupportTicket: (body: {
    category: string
    subject?: string
    description: string
    order_item_id?: string
    photo_keys?: string[]
  }) => c.post<{ id: string }>('/customer/support/tickets', body),
  replySupportTicket: (id: string, body: string) =>
    c.post<void>(`/customer/support/tickets/${id}/messages`, { body }),

  notifications: () => c.get<AppNotification[]>('/customer/notifications').then((r) => r ?? []),
})

// ---------------------------------------------------------------------
// Seller (authenticated)
// ---------------------------------------------------------------------
export const sellerApi = (c: ApiClient) => ({
  profile: () => c.get<Seller>('/seller/profile'),
  // Seller self-service: edit store description + notification prefs (store name
  // is server-locked once the store is approved/under review). Returns updated seller.
  updateProfile: (body: Record<string, unknown>) => c.put<Seller>('/seller/profile', body),
  changePassword: (current_password: string, new_password: string) =>
    c.post<void>('/seller/change-password', { current_password, new_password }),
  onboarding: () => c.get<SellerOnboardingBundle>('/seller/onboarding'),
  saveOwner: (body: Record<string, unknown>) => c.put<void>('/seller/onboarding/owner', body),
  saveBank: (body: Record<string, unknown>) => c.put<void>('/seller/onboarding/bank', body),
  savePickup: (body: Record<string, unknown>) => c.put<void>('/seller/onboarding/pickup', body),
  signContract: (signed_name: string) => c.post<void>('/seller/onboarding/contract/sign', { signed_name, agreed: true }),
  completeStep: (step: string) => c.post<void>(`/seller/onboarding/steps/${step}/complete`),
  submitOnboarding: () => c.post<void>('/seller/onboarding/submit'),
  // Seller signup verification (pre-account): email + mobile codes.
  requestEmailCode: (email: string) => c.post<{ sent: boolean }>('/auth/seller/verify/email/request', { email }),
  confirmEmailCode: (email: string, code: string) => c.post<{ verified: boolean }>('/auth/seller/verify/email/confirm', { email, code }),
  requestPhoneCode: (phone: string) => c.post<{ sent: boolean }>('/auth/seller/verify/phone/request', { phone }),
  confirmPhoneCode: (phone: string, code: string) => c.post<{ verified: boolean }>('/auth/seller/verify/phone/confirm', { phone, code }),

  products: (params?: Record<string, unknown>) => c.getWithMeta<Product[]>('/seller/products', params),
  product: (id: string) => c.get<Product>(`/seller/products/${id}`),
  createProduct: (body: Record<string, unknown>) => c.post<{ id: string }>('/seller/products', body),
  updateProduct: (id: string, body: Record<string, unknown>) => c.put<void>(`/seller/products/${id}`, body),
  setProductStock: (id: string, stock: number, sellerListingId?: string) =>
    c.patch<void>(`/seller/products/${id}/stock`, { stock, seller_listing_id: sellerListingId }),
  submitProduct: (id: string) => c.post<void>(`/seller/products/${id}/submit`),
  // Upload an image file; returns the storage key + URL to attach to a product.
  uploadImage: (file: File) => c.uploadFile<{ key: string; url: string; size: number }>('/seller/uploads', file),

  orders: (params?: Record<string, unknown>) => c.getWithMeta<Order[]>('/seller/orders', params),
  order: (id: string) => c.get<Order>(`/seller/orders/${id}`),
  orderHistory: (id: string) => c.get<unknown[]>(`/seller/orders/${id}/history`),
  // Accept a new order (moves the seller's items to processing).
  acceptOrder: (id: string) => c.post<void>(`/seller/orders/${id}/accept`),
  // Cancel the seller's part of an order with a reason.
  cancelOrder: (id: string, reason: string) => c.post<void>(`/seller/orders/${id}/cancel`, { reason }),
  setItemStatus: (itemId: string, status: string) =>
    c.post<void>(`/seller/order-items/${itemId}/status`, { status }),

  shipments: () => c.get<unknown[]>('/seller/shipments'),
  createShipment: (orderId: string) => c.post<unknown>(`/seller/orders/${orderId}/ship`),
  selfShip: (orderId: string) => c.post<unknown>(`/seller/orders/${orderId}/self-ship`),
  markShipped: (id: string) => c.post<void>(`/seller/shipments/${id}/mark-shipped`),
  markDelivered: (id: string) => c.post<void>(`/seller/shipments/${id}/mark-delivered`),
  labelURL: (id: string) => `/seller/shipments/${id}/label`,
  trackShipment: (id: string) => c.get<unknown[]>(`/seller/shipments/${id}/track`),

  returns: () => c.get<ReturnRecord[]>('/seller/returns').then((r) => r ?? []),
  approveReturn: (id: string) => c.post<void>(`/seller/returns/${id}/approve`),
  rejectReturn: (id: string, reason: string) => c.post<void>(`/seller/returns/${id}/reject`, { reason }),
  receiveReturn: (id: string) => c.post<void>(`/seller/returns/${id}/received`),

  // Help & Support — seller handles only its own product tickets.
  supportTickets: () => c.get<SupportTicket[]>('/seller/support/tickets').then((r) => r ?? []),
  replySupportTicket: (id: string, body: string) =>
    c.post<void>(`/seller/support/tickets/${id}/messages`, { body }),
  resolveSupportTicket: (id: string, resolution: string) =>
    c.post<void>(`/seller/support/tickets/${id}/resolve`, { resolution }),

  packagingItems: () => c.get<unknown[]>('/seller/packaging/items'),
  packagingBalances: () => c.get<unknown[]>('/seller/packaging/balances'),
  packagingRequests: () => c.get<unknown[]>('/seller/packaging/requests'),
  requestPackaging: (packaging_item_id: string, quantity: number) =>
    c.post<{ id: string }>('/seller/packaging/requests', { packaging_item_id, quantity }),

  settlements: () => c.get<Settlement[]>('/seller/settlements'),
})

// ---------------------------------------------------------------------
// Admin (authenticated + permissioned)
// ---------------------------------------------------------------------
export const adminApi = (c: ApiClient) => ({
  sellers: (params?: Record<string, unknown>) => c.getWithMeta<Seller[]>('/admin/sellers', params),
  seller: (id: string) => c.get<Seller>(`/admin/sellers/${id}`),
  approveSeller: (id: string) => c.post<void>(`/admin/sellers/${id}/approve`),
  rejectSeller: (id: string, reason: string) => c.post<void>(`/admin/sellers/${id}/reject`, { reason }),
  suspendSeller: (id: string, reason: string) => c.post<void>(`/admin/sellers/${id}/suspend`, { reason }),
  unsuspendSeller: (id: string) => c.post<void>(`/admin/sellers/${id}/unsuspend`),
  activateSeller: (id: string) => c.post<void>(`/admin/sellers/${id}/activate`),
  blockSeller: (id: string, reason: string) => c.post<void>(`/admin/sellers/${id}/block`, { reason }),
  unblockSeller: (id: string) => c.post<void>(`/admin/sellers/${id}/unblock`),

  products: (params?: Record<string, unknown>) => c.getWithMeta<Product[]>('/admin/products', params),
  product: (id: string) => c.get<Product>(`/admin/products/${id}`),
  approveProduct: (id: string) => c.post<void>(`/admin/products/${id}/approve`),
  rejectProduct: (id: string, reason: string) => c.post<void>(`/admin/products/${id}/reject`, { reason }),
  createCategory: (body: Record<string, unknown>) => c.post<{ id: string }>('/admin/categories', body),

  orders: (params?: Record<string, unknown>) => c.getWithMeta<Order[]>('/admin/orders', params),
  order: (id: string) => c.get<Order>(`/admin/orders/${id}`),
  setOrderStatus: (id: string, status: string, note?: string) =>
    c.post<void>(`/admin/orders/${id}/status`, { status, note }),

  collections: () => c.get<Collection[]>('/admin/collections'),
  createCollection: (body: Record<string, unknown>) => c.post<{ id: string }>('/admin/collections', body),
  banners: () => c.get<Banner[]>('/admin/banners'),
  createBanner: (body: Record<string, unknown>) => c.post<{ id: string }>('/admin/banners', body),
  updateBanner: (id: string, body: Record<string, unknown>) => c.put<void>(`/admin/banners/${id}`, body),
  team: () => c.get<TeamMember[]>('/admin/team'),
  updateTeamMember: (id: string, body: Record<string, unknown>) => c.put<void>(`/admin/team/${id}`, body),
  // Upload an image file; returns the storage key + URL to attach to a banner.
  uploadImage: (file: File) => c.uploadFile<{ key: string; url: string; size: number }>('/admin/uploads', file),

  // Coupons & offers management.
  coupons: () => c.get<Coupon[]>('/admin/coupons').then((r) => r ?? []),
  createCoupon: (body: Record<string, unknown>) => c.post<{ id: string }>('/admin/coupons', body),
  updateCoupon: (id: string, body: Record<string, unknown>) => c.put<void>(`/admin/coupons/${id}`, body),
  setCouponActive: (id: string, active: boolean) => c.post<void>(`/admin/coupons/${id}/active`, { active }),
  deleteCoupon: (id: string) => c.del<void>(`/admin/coupons/${id}`),
  couponRedemptions: () => c.get<CouponRedemption[]>('/admin/coupons/redemptions').then((r) => r ?? []),

  // Admin is read-only over returns; it only pays the refund + records the UTR
  // (unlocked once the seller has approved + received → refund_processing).
  returns: (status?: string) => c.get<ReturnRecord[]>('/admin/returns', status ? { status } : undefined).then((r) => r ?? []),
  payRefund: (id: string, utr: string) => c.post<void>(`/admin/returns/${id}/refund`, { utr }),

  // Help & Support — admin handles non-product tickets and can read product
  // (seller) tickets with scope='all' (read-only; actions are rejected server-side).
  supportTickets: (params?: { scope?: 'all'; status?: string }) =>
    c.get<SupportTicket[]>('/admin/support/tickets', params as Record<string, unknown> | undefined).then((r) => r ?? []),
  replySupportTicket: (id: string, body: string) =>
    c.post<void>(`/admin/support/tickets/${id}/messages`, { body }),
  resolveSupportTicket: (id: string, resolution: string) =>
    c.post<void>(`/admin/support/tickets/${id}/resolve`, { resolution }),

  reviews: (params?: Record<string, unknown>) => c.get<unknown[]>('/admin/reviews', params),
  moderateReview: (id: string, status: string) => c.post<void>(`/admin/reviews/${id}/moderate`, { status }),

  packagingItems: () => c.get<unknown[]>('/admin/packaging/items'),
  createPackagingItem: (body: Record<string, unknown>) => c.post<{ id: string }>('/admin/packaging/items', body),
  packagingRequests: () => c.get<unknown[]>('/admin/packaging/requests'),
  approvePackaging: (id: string, approved_quantity?: number) =>
    c.post<void>(`/admin/packaging/requests/${id}/approve`, { approved_quantity }),

  settlements: (params?: Record<string, unknown>) => c.get<Settlement[]>('/admin/settlements', params),
  generateSettlement: (seller_id: string) => c.post<{ id: string }>('/admin/settlements/generate', { seller_id }),
  auditLogs: () => c.get<unknown[]>('/admin/audit-logs'),
})

export type { ApiEnvelope }
