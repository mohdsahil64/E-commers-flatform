// Central place that turns an order status into the icons + Tailwind colour
// classes used across the customer order views, so the list, the detail status
// card and the timeline all agree on what each status looks like.
//
// Colour is chosen by the status's MEANING (see orderStatusTone in @carlitos/shared):
//   neutral  grey    — nothing happened yet (pending payment)
//   info     blue    — accepted / being prepared (confirmed, processing, packed, ready_to_ship)
//   progress pink    — on the move (shipped, in_transit)
//   active   amber   — out for delivery (arriving today)
//   success  green   — delivered / refunded (good terminal state)
//   danger   red     — cancelled
//   warning  amber   — in a return/refund flow
import {
  Package, PackageCheck, PackageX, Truck, Clock, RotateCcw, Boxes, PackageOpen,
  BadgeCheck, type LucideIcon,
} from 'lucide-vue-next'
import { orderStatusTone, type OrderStatusTone } from '@carlitos/shared'

export interface OrderStatusStyle {
  icon: LucideIcon
  /** text colour for the status label */
  text: string
  /** small rounded icon chip (bg + text) used in the list */
  chip: string
  /** solid dot/badge colour used on timelines + detail card icon */
  solid: string
  /** soft gradient + border for the big detail status card */
  card: string
}

// Icon per concrete status (more specific than tone).
function iconFor(status: string): LucideIcon {
  switch (status) {
    case 'pending_payment': return Clock
    case 'confirmed': return BadgeCheck
    case 'processing': return Boxes
    case 'packed':
    case 'ready_to_ship': return PackageOpen
    case 'shipped':
    case 'in_transit':
    case 'out_for_delivery': return Truck
    case 'delivered': return PackageCheck
    case 'refunded': return BadgeCheck
    case 'cancelled': return PackageX
    case 'return_requested':
    case 'return_approved':
    case 'return_picked':
    case 'return_received':
    case 'refund_processing': return RotateCcw
    default: return Package
  }
}

const TONE_STYLES: Record<OrderStatusTone, Omit<OrderStatusStyle, 'icon'>> = {
  neutral: {
    text: 'text-stone-500', chip: 'bg-stone-100 text-stone-500', solid: 'bg-stone-400',
    card: 'from-stone-50 to-white border-stone-200',
  },
  info: {
    text: 'text-blue-600', chip: 'bg-blue-50 text-blue-600', solid: 'bg-blue-500',
    card: 'from-blue-50 to-blue-50/40 border-blue-100',
  },
  progress: {
    text: 'text-accent', chip: 'bg-accent-soft text-accent', solid: 'bg-accent',
    card: 'from-accent-soft to-accent-soft/40 border-accent/20',
  },
  active: {
    text: 'text-amber-600', chip: 'bg-amber-50 text-amber-600', solid: 'bg-amber-500',
    card: 'from-amber-50 to-amber-50/40 border-amber-100',
  },
  success: {
    text: 'text-emerald-600', chip: 'bg-emerald-50 text-emerald-600', solid: 'bg-emerald-500',
    card: 'from-emerald-50 to-emerald-50/40 border-emerald-100',
  },
  danger: {
    text: 'text-red-500', chip: 'bg-red-50 text-red-500', solid: 'bg-red-500',
    card: 'from-red-50 to-red-50/40 border-red-100',
  },
  warning: {
    text: 'text-amber-600', chip: 'bg-amber-50 text-amber-600', solid: 'bg-amber-500',
    card: 'from-amber-50 to-amber-50/40 border-amber-100',
  },
}

export function orderStatusStyle(status: string): OrderStatusStyle {
  const tone = orderStatusTone(status)
  return { icon: iconFor(status), ...TONE_STYLES[tone] }
}
