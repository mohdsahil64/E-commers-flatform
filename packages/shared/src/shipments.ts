// Shared shipment helpers used by the Seller "Shipments" page and the Admin
// "Shipping" page, so both show the same labels, colours and progress steps.

/** Coarse stage a shipment is in — drives tabs, counters and colours. */
export type ShipStage = 'booked' | 'transit' | 'delivered' | 'issue' | 'cancelled'

const LABELS: Record<string, string> = {
  created: 'Booked',
  label_generated: 'Booked',
  pickup_scheduled: 'Awaiting pickup',
  picked_up: 'Picked up',
  in_transit: 'In transit',
  out_for_delivery: 'Out for delivery',
  delivered: 'Delivered',
  failed: 'Delivery failed',
  rto: 'Returning to seller',
  cancelled: 'Cancelled',
}

/** Friendly label for a raw shipment status. */
export function shipmentStatusLabel(status: string, shipType?: string): string {
  if (shipType === 'self') {
    if (status === 'pickup_scheduled' || status === 'created' || status === 'label_generated') return 'Ready to dispatch'
    if (status === 'in_transit' || status === 'picked_up') return 'Dispatched'
  }
  return LABELS[status] || (status || '').replace(/_/g, ' ')
}

export function shipmentStage(status: string): ShipStage {
  switch (status) {
    case 'delivered': return 'delivered'
    case 'cancelled': return 'cancelled'
    case 'failed':
    case 'rto': return 'issue'
    case 'picked_up':
    case 'in_transit':
    case 'out_for_delivery': return 'transit'
    default: return 'booked'
  }
}

/** Badge colour class (badge-* exists in both apps' stylesheets). */
export function shipmentBadgeClass(status: string): string {
  switch (shipmentStage(status)) {
    case 'delivered': return 'badge-green'
    case 'issue': return 'badge-red'
    case 'cancelled': return 'badge-gray'
    case 'transit': return 'badge-blue'
    default: return 'badge-amber'
  }
}

export interface ShipStep { key: string; label: string }

/** The journey a shipment goes through, depending on how it is shipped. */
export function shipmentSteps(shipType: string): ShipStep[] {
  if (shipType === 'self') {
    return [
      { key: 'ready', label: 'Ready' },
      { key: 'dispatched', label: 'Dispatched' },
      { key: 'delivered', label: 'Delivered' },
    ]
  }
  return [
    { key: 'booked', label: 'Booked' },
    { key: 'picked_up', label: 'Picked up' },
    { key: 'in_transit', label: 'In transit' },
    { key: 'out_for_delivery', label: 'Out for delivery' },
    { key: 'delivered', label: 'Delivered' },
  ]
}

/** Index of the step a shipment is currently on, or -1 when it left the normal path. */
export function shipmentStepIndex(shipType: string, status: string): number {
  if (['failed', 'rto', 'cancelled'].includes(status)) return -1
  if (shipType === 'self') {
    if (status === 'delivered') return 2
    if (['picked_up', 'in_transit', 'out_for_delivery'].includes(status)) return 1
    return 0
  }
  switch (status) {
    case 'delivered': return 4
    case 'out_for_delivery': return 3
    case 'in_transit': return 2
    case 'picked_up': return 1
    default: return 0
  }
}

/** "12 Mar, 4:35 pm" */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const date = d.toLocaleDateString('en-IN', { day: 'numeric', month: 'short' })
  const time = d.toLocaleTimeString('en-IN', { hour: 'numeric', minute: '2-digit', hour12: true }).toLowerCase()
  return `${date}, ${time}`
}

/** "today", "yesterday", "3 days ago" */
export function daysAgo(iso: string | null | undefined): string {
  if (!iso) return ''
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const days = Math.floor((Date.now() - t) / 86_400_000)
  if (days <= 0) return 'today'
  if (days === 1) return 'yesterday'
  return `${days} days ago`
}
