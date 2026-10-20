// Shared, framework-agnostic utilities and constants used by all three
// CARLITOS frontends.

export const BRAND = 'CARLITOS'

/**
 * Legal / business entity details shown on the storefront's policy pages
 * (Contact, Privacy, Terms, Refund, Shipping). These are required for payment
 * gateway (Razorpay) approval — the operating entity, a reachable support
 * email + phone, and a physical business address must be publicly visible.
 * Keep this the single source of truth; the policy pages read from here.
 */
export const BUSINESS = {
  /** Public store / brand name shown to customers. */
  brand: 'CARLITOS',
  /** Legal operating name of the business behind the store. */
  legalName: 'Carlitos Fashion Hub',
  /** Business constitution (sole proprietorship, since it is run by an individual). */
  entityType: 'Sole Proprietorship',
  /** Live website domain. */
  domain: 'carlitosfashion.com',
  website: 'https://carlitosfashion.com',
  /** Customer support email. */
  email: 'support@carlitosfashion.com',
  /** Customer support phone (display + tel: link forms). */
  phone: '+91 98969 63414',
  phoneHref: '+919896963414',
  /** Registered / operating business address. */
  address: {
    line1: 'Bassdalla',
    city: 'Punhana',
    district: 'Mewat',
    state: 'Haryana',
    pincode: '122508',
    country: 'India',
  },
  /** Single-line formatted address for compact display. */
  addressLine: 'Bassdalla, Punhana, Mewat, Haryana - 122508, India',
  /** Days after delivery within which a return/refund can be requested. */
  returnWindowDays: 7,
  /** Business days in which an approved refund is credited back. */
  refundProcessingDays: '3-4',
  /** Support response time shown to customers. */
  supportResponseHours: '24-48',
} as const

/** formatINR formats a number as Indian Rupees. */
export function formatINR(amount: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(amount)
}

/** formatDate renders an ISO date as a readable short date. */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' })
}

/** discountPercent computes the % off from MRP to price. */
export function discountPercent(mrp: number, price: number): number {
  if (mrp <= 0 || price >= mrp) return 0
  return Math.round(((mrp - price) / mrp) * 100)
}

/** Human-readable labels for order statuses. */
export const ORDER_STATUS_LABELS: Record<string, string> = {
  pending_payment: 'Pending Payment',
  confirmed: 'Confirmed',
  processing: 'Processing',
  packed: 'Packed',
  ready_to_ship: 'Ready to Ship',
  shipped: 'Shipped',
  in_transit: 'In Transit',
  out_for_delivery: 'Out for Delivery',
  delivered: 'Delivered',
  cancelled: 'Cancelled',
  return_requested: 'Return Requested',
  return_approved: 'Return Approved',
  return_picked: 'Return Picked',
  return_received: 'Return Received',
  refund_processing: 'Refund Processing',
  refunded: 'Refunded',
}

export function orderStatusLabel(status: string): string {
  return ORDER_STATUS_LABELS[status] ?? status
}

/** Return/exchange reason options (Myntra-style). `photo: true` = proof
 *  recommended for that reason. */
export interface ReturnReason {
  code: string
  label: string
  photo: boolean
  /** Reasons valid for an exchange (vs refund-only). */
  exchange: boolean
}
export const RETURN_REASONS: ReturnReason[] = [
  { code: 'size_small',     label: 'Size too small',                 photo: false, exchange: true },
  { code: 'size_large',     label: 'Size too large',                 photo: false, exchange: true },
  { code: 'wrong_item',     label: 'Wrong item delivered',           photo: true,  exchange: true },
  { code: 'defective',      label: 'Item defective / not working',   photo: true,  exchange: true },
  { code: 'damaged',        label: 'Item damaged in transit',        photo: true,  exchange: true },
  { code: 'quality',        label: 'Quality not as expected',        photo: true,  exchange: false },
  { code: 'not_as_desc',    label: 'Not as described / shown',       photo: true,  exchange: false },
  { code: 'missing_parts',  label: 'Missing parts / accessories',    photo: true,  exchange: true },
  { code: 'changed_mind',   label: 'Changed my mind',                photo: false, exchange: false },
  { code: 'other',          label: 'Other',                          photo: false, exchange: false },
]
export function returnReasonLabel(code: string): string {
  return RETURN_REASONS.find((r) => r.code === code)?.label ?? code
}

/** Labels for return lifecycle statuses. */
export const RETURN_STATUS_LABELS: Record<string, string> = {
  requested: 'Requested',
  approved: 'Approved',
  rejected: 'Rejected',
  picked: 'Picked Up',
  received: 'Received',
  refund_processing: 'Refund Processing',
  refunded: 'Refunded',
  completed: 'Completed',
}
export function returnStatusLabel(status: string): string {
  return RETURN_STATUS_LABELS[status] ?? status
}

/** How many days after delivery a return/exchange can be raised. Mirrors the
 *  backend ReturnWindowDays constant. */
export const RETURN_WINDOW_DAYS = 7

/**
 * Semantic colour "tone" for each order status, chosen by what the status
 * MEANS to the customer:
 *  - neutral  : nothing has happened yet / terminal non-event (grey)
 *  - info     : accepted & being prepared by the seller (blue)
 *  - progress : moving toward the customer / almost there (accent pink + amber)
 *  - success  : a good terminal state — delivered / money back (green)
 *  - danger   : cancelled / failed (red)
 *  - warning  : in a return/refund flow, needs attention (amber)
 * The exact Tailwind classes live in each app so they can match its palette;
 * this keeps the mapping itself in one shared place.
 */
export type OrderStatusTone = 'neutral' | 'info' | 'progress' | 'active' | 'success' | 'danger' | 'warning'

export function orderStatusTone(status: string): OrderStatusTone {
  switch (status) {
    case 'pending_payment':
      return 'neutral'
    case 'confirmed':
    case 'processing':
    case 'packed':
    case 'ready_to_ship':
      return 'info'
    case 'shipped':
    case 'in_transit':
      return 'progress'
    case 'out_for_delivery':
      return 'active'
    case 'delivered':
    case 'refunded':
      return 'success'
    case 'cancelled':
      return 'danger'
    case 'return_requested':
    case 'return_approved':
    case 'return_picked':
    case 'return_received':
    case 'refund_processing':
      return 'warning'
    default:
      return 'neutral'
  }
}

/** Human-readable labels for seller statuses. */
export const SELLER_STATUS_LABELS: Record<string, string> = {
  pending: 'Pending',
  under_review: 'Under Review',
  approved: 'Approved',
  rejected: 'Rejected',
  suspended: 'Suspended',
  blocked: 'Blocked',
}

/** resolveMediaUrl turns a storage key into a full URL using a base. */
export function resolveMediaUrl(base: string, keyOrUrl: string): string {
  if (!keyOrUrl) return ''
  if (keyOrUrl.startsWith('http')) return keyOrUrl
  return `${base.replace(/\/$/, '')}/${keyOrUrl}`
}

/** extractApiError pulls a user-facing message out of an axios error. */
export function extractApiError(err: unknown): string {
  const anyErr = err as { response?: { data?: { error?: { message?: string }; message?: string } } }
  return (
    anyErr?.response?.data?.error?.message ||
    anyErr?.response?.data?.message ||
    'Something went wrong. Please try again.'
  )
}
