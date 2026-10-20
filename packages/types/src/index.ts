// Shared TypeScript types for the CARLITOS API. These mirror the Go response
// shapes so all three frontends stay in sync with the backend contract.

// ---------------------------------------------------------------------
// API envelope
// ---------------------------------------------------------------------
export interface ApiMeta {
  page?: number
  page_size?: number
  total?: number
  total_pages?: number
}

export interface ApiEnvelope<T> {
  success: boolean
  data?: T
  meta?: ApiMeta
  message?: string
}

export interface ApiError {
  success: false
  error: { code: string; message: string }
  fields?: Record<string, string>
}

// ---------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------
export type AccountType = 'customer' | 'seller' | 'admin'

export interface AuthUser {
  id: string
  account_type: AccountType
  email: string
  full_name: string
  roles: string[]
  permissions: string[]
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_at: string
  user: AuthUser
}

// ---------------------------------------------------------------------
// Catalog
// ---------------------------------------------------------------------
export interface Gender {
  id: string
  code: string
  name: string
  sort_order: number
  is_active: boolean
}

export interface Category {
  id: string
  parent_id: string | null
  gender_id: string | null
  name: string
  slug: string
  level: number
  description: string
  image_key: string | null
  sort_order: number
  is_active: boolean
  children?: Category[]
}

export interface Brand {
  id: string
  name: string
  slug: string
  logo_key: string | null
  is_active: boolean
}

export interface AttributeValue {
  id: string
  attribute_id: string
  value: string
  display_value: string
  meta: Record<string, unknown>
  sort_order: number
}

export interface Attribute {
  id: string
  code: string
  name: string
  input_type: string
  is_variant: boolean
  is_filterable: boolean
  sort_order: number
  values?: AttributeValue[]
}

export interface ProductImage {
  id: string
  file_key: string
  url?: string
  alt_text: string
  sort_order: number
  is_primary: boolean
}

export interface Variant {
  id: string
  product_id: string
  variant_code: string
  barcode: string | null
  mrp: number
  selling_price: number
  weight_grams: number
  is_active: boolean
  /** Size label (e.g. "M" or "8"). */
  size?: string
  /** The seller listing id for this variant — use this when adding to cart. */
  seller_listing_id?: string
  /** Live available stock for this size. */
  available_quantity?: number
}

export interface Product {
  id: string
  title: string
  slug: string
  description: string
  brand_id: string | null
  brand_name?: string
  gender_id: string | null
  category_id: string | null
  subcategory_id: string | null
  status: string
  rejection_reason?: string
  rating_avg: number
  rating_count: number
  hsn_code: string
  tax_percent: number
  season?: string
  created_at: string
  images?: ProductImage[]
  variants?: Variant[]
  /** Live available stock (seller reads + storefront detail). */
  available_quantity?: number
  /** Seller listing price, surfaced on seller list views. */
  selling_price?: number
  mrp?: number
}

// Lightweight product card returned by the storefront listing endpoint.
export interface ListingCard {
  product_id: string
  title: string
  slug: string
  brand_name: string
  primary_image: string
  min_price: number
  mrp: number
  rating_avg: number
  rating_count: number
  seller_id: string
  seller_name: string
  seller_slug: string
  rank_score: number
  available_quantity?: number
  in_stock?: boolean
}

// ---------------------------------------------------------------------
// Cart & wishlist
// ---------------------------------------------------------------------
export interface CartItem {
  id: string
  seller_listing_id: string
  product_id: string
  product_title: string
  slug: string
  sku: string
  image: string
  seller_name: string
  quantity: number
  unit_price: number
  available_quantity: number
  line_total: number
  /** Whether the seller of this item offers Cash on Delivery. */
  seller_cod_enabled?: boolean
}

export interface WishlistItem {
  product_id: string
  title: string
  slug: string
  image: string
  min_price: number
  rating_avg: number
}

// ---------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------
export type OrderStatus =
  | 'pending_payment' | 'confirmed' | 'processing' | 'packed' | 'ready_to_ship'
  | 'shipped' | 'in_transit' | 'out_for_delivery' | 'delivered' | 'cancelled'
  | 'return_requested' | 'return_approved' | 'return_picked' | 'return_received'
  | 'refund_processing' | 'refunded'

export interface OrderItem {
  id: string
  seller_id: string
  seller_name?: string
  seller_listing_id: string
  product_title: string
  product_slug?: string
  image?: string
  variant_label: string
  sku: string
  item_status: string
  delivered_at?: string | null
  quantity: number
  unit_price: number
  gross_amount: number
  discount_amount: number
  tax_amount: number
  shipping_charge: number
  commission_percent: number
  commission_amount: number
  payment_fee: number
  refund_amount: number
  seller_payable: number
  // UI helpers set by the order detail/list queries.
  has_review?: boolean
  return_id?: string
}

export interface Order {
  id: string
  order_number: string
  customer_id: string
  status: OrderStatus
  payment_method: 'online' | 'cod'
  payment_status: string
  currency: string
  items_subtotal: number
  discount_total: number
  tax_total: number
  shipping_total: number
  grand_total: number
  ship_contact_name: string
  ship_contact_phone: string
  ship_line1: string
  ship_line2: string
  ship_city: string
  ship_state: string
  ship_pincode: string
  ship_country: string
  placed_at: string | null
  created_at: string
  items?: OrderItem[]
}

// ---------------------------------------------------------------------
// Sellers
// ---------------------------------------------------------------------
export type SellerStatus =
  | 'pending' | 'under_review' | 'approved' | 'rejected' | 'suspended' | 'blocked'

export interface Seller {
  id: string
  user_id: string
  store_name: string
  store_slug: string
  store_description: string
  status: SellerStatus
  legal_business_name: string
  business_type: string
  owner_name: string
  contact_email: string
  contact_phone: string
  gst_number: string
  pan_number: string
  commission_percent: number | null
  rating_avg: number
  rating_count: number
  rejection_reason: string
  suspended_reason: string
  resubmit_count: number
  onboarding_completed: boolean
  contract_signed: boolean
  contract_commission: number | null
  notify_order_emails: boolean
  notify_return_emails: boolean
  notify_settlement_emails: boolean
  notify_low_stock: boolean
  notify_marketing: boolean
  /** Whether this seller offers Cash on Delivery on their products. */
  cod_enabled: boolean
  created_at: string
}

export interface SellerOnboarding {
  seller_id: string
  step: string
  business_completed: boolean
  contact_completed: boolean
  pickup_completed: boolean
  bank_completed: boolean
  documents_completed: boolean
  contract_completed: boolean
  review_notes: string
}

export interface SellerBankAccount {
  account_holder: string
  account_number: string
  ifsc: string
  bank_name: string
  branch: string
}

export interface SellerPickupAddress {
  label: string
  contact_name: string
  contact_phone: string
  line1: string
  line2: string
  city: string
  state: string
  pincode: string
  country: string
}

/** Response of GET /seller/onboarding — seller + progress + saved data. */
export interface SellerOnboardingBundle {
  seller: Seller
  onboarding: SellerOnboarding | null
  bank: SellerBankAccount | null
  pickup: SellerPickupAddress | null
  default_commission: number
  max_resubmits: number
}

// ---------------------------------------------------------------------
// Merchandising
// ---------------------------------------------------------------------
export interface Collection {
  id: string
  name: string
  slug: string
  description: string
  type: string
  kind: string
  image_key: string
  is_active: boolean
}

export interface Banner {
  id: string
  title: string
  subtitle: string
  image_key: string
  mobile_image_key: string
  placement: string
  destination_type: string
  targets: Record<string, string>
  sort_order: number
}

export interface TeamMember {
  id: string
  name: string
  role: string
  bio: string
  photo_key: string
  sort_order: number
  is_active: boolean
}

// ---------------------------------------------------------------------
// Returns / exchange / refund
// ---------------------------------------------------------------------
export type ReturnType = 'return' | 'exchange'
export type ReturnStatus =
  | 'requested' | 'approved' | 'rejected' | 'picked'
  | 'received' | 'refund_processing' | 'refunded' | 'completed'

export interface ReturnLineItem {
  order_item_id: string
  product_title: string
  image: string
  variant_label: string
  sku: string
  quantity: number
  refund_amount: number
}

export interface ReturnPhoto {
  file_key: string
}

export interface ReturnEvent {
  status: string
  actor_role: string   // customer | seller | admin | system
  note: string
  created_at: string
}

export interface ReturnRecord {
  id: string
  return_number: string
  order_id: string
  order_number: string
  customer_id: string
  customer_name: string
  seller_id: string
  seller_name: string
  type: ReturnType
  reason_code: string
  reason: string
  reject_reason: string
  status: ReturnStatus
  refund_method: '' | 'upi' | 'bank'
  refund_upi: string
  refund_acc_name: string
  refund_acc_no: string
  refund_ifsc: string
  refund_amount: number
  refund_status: string
  refund_utr: string
  created_at: string
  items: ReturnLineItem[]
  photos: ReturnPhoto[]
  events: ReturnEvent[]
}

// ---------------------------------------------------------------------
// Help & Support (ticketing)
// ---------------------------------------------------------------------
export type SupportCategory =
  | 'product' | 'payment' | 'login' | 'account' | 'refund' | 'other'
export type SupportStatus = 'open' | 'in_progress' | 'solved'
export type SupportAssignee = 'seller' | 'admin'

export interface SupportMessage {
  id: string
  author_role: 'customer' | 'seller' | 'admin' | 'system'
  body: string
  created_at: string
}

export interface SupportPhoto {
  file_key: string
}

export interface SupportTicket {
  id: string
  ticket_number: string
  customer_id: string
  customer_name: string
  category: SupportCategory
  assignee: SupportAssignee
  seller_id: string
  seller_name: string
  // Product context (product tickets only).
  order_id: string
  order_number: string
  order_item_id: string
  product_title: string
  product_image: string
  variant_label: string
  subject: string
  description: string
  status: SupportStatus
  resolution: string
  resolved_at: string | null
  created_at: string
  updated_at: string
  photos: SupportPhoto[]
  messages: SupportMessage[]
}

// ---------------------------------------------------------------------
// Misc
// ---------------------------------------------------------------------
export interface Address {
  id: string
  label: string
  contact_name: string
  contact_phone: string
  line1: string
  line2: string
  landmark: string
  city: string
  state: string
  pincode: string
  country: string
  address_type: string
  is_default: boolean
}

export interface CustomerProfile {
  id: string
  display_name: string
  email: string
  phone: string
  gender: string
  notify_email: boolean
  notify_sms: boolean
  notify_push: boolean
  marketing_opt_in: boolean
}

export interface TrackEvent {
  status: string
  location: string
  description: string
  event_time: string
}

export interface OrderTracking {
  ship_type: string
  courier: string
  awb: string
  status: string
  tracking_url: string
  events: TrackEvent[]
}

export interface AppNotification {
  id: string
  event_type: string
  title: string
  body: string
  data: Record<string, unknown>
  is_read: boolean
  created_at: string
}

// ---------------------------------------------------------------------
// Coupons
// ---------------------------------------------------------------------
export interface Coupon {
  id: string
  code: string
  description: string
  discount_type: 'percent' | 'fixed'
  discount_value: number
  max_discount: number | null
  min_order_amount: number
  usage_limit_total: number | null
  usage_limit_per_user: number | null
  used_count: number
  starts_at: string | null
  ends_at: string | null
  is_active: boolean
  created_at: string
}

export interface CouponRedemption {
  id: string
  coupon_code: string
  customer_id: string
  customer_name: string
  order_id: string | null
  order_number: string
  discount_amount: number
  created_at: string
}

export interface Settlement {
  id: string
  settlement_number: string
  seller_id: string
  period_start: string
  period_end: string
  gross_amount: number
  commission_amount: number
  shipping_deduction: number
  other_fees: number
  refund_amount: number
  net_payable: number
  status: string
  created_at: string
}
