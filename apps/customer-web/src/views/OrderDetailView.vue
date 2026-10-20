<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import {
  MapPin, Package, Truck, CheckCircle, RotateCcw, Download, Star, X, ImagePlus, ChevronLeft, ChevronRight,
} from 'lucide-vue-next'
import type { Order, OrderTracking, OrderItem } from '@carlitos/types'
import { customer, api, MEDIA_BASE } from '@/api'
import {
  formatINR, formatDate, orderStatusLabel, resolveMediaUrl, extractApiError,
  RETURN_REASONS, returnReasonLabel, RETURN_WINDOW_DAYS,
} from '@carlitos/shared'
import { orderStatusStyle } from '@/composables/orderStatus'
import { useToastStore } from '@/stores/toast'

const route   = useRoute()
const router  = useRouter()
const toast   = useToastStore()

// History-aware back: pop the previous entry when there is one (avoids the
// Orders ⇄ OrderDetail ping-pong loop); otherwise fall back to the orders list.
function goBackToOrders() {
  const back = window.history.state?.back
  if (typeof back === 'string' && back.length > 0) router.back()
  else router.push('/orders')
}
const order   = ref<Order | null>(null)
const loading = ref(true)
const tracking = ref<OrderTracking | null>(null)
const cancelling = ref(false)
const cancelError = ref('')

const justPlaced = ref(route.query.placed === '1')

async function reload() {
  order.value = await customer.order(route.params.id as string)
}

onMounted(async () => {
  try {
    const [o, t] = await Promise.all([
      customer.order(route.params.id as string),
      customer.trackOrder(route.params.id as string).catch(() => null),
    ])
    order.value    = o
    tracking.value = t as OrderTracking | null

    // Arrived via a "Re-submit return" link (?return=<orderItemId>): auto-open
    // the return flow for that item once the order has loaded.
    const reItem = route.query.return as string | undefined
    if (reItem) {
      const it = o.items?.find(x => x.id === reItem)
      if (it) openReturn(it)
    }
  } finally {
    loading.value = false
  }
})

/* ── Derived status ─────────────────────────────────────── */
const isDelivered = computed(() => order.value?.status === 'delivered')
const isCancelled = computed(() => ['cancelled', 'return_requested', 'return_approved', 'refunded'].includes(order.value?.status || ''))
const canCancel   = computed(() => ['pending_payment', 'confirmed', 'processing'].includes(order.value?.status || ''))
// Track is offered once the order is on the move, through delivery — and also
// whenever we actually have tracking data for it (shipment created / events).
const canTrack = computed(() => {
  const s = order.value?.status || ''
  if (['shipped', 'in_transit', 'out_for_delivery', 'delivered'].includes(s)) return true
  return !!(tracking.value && (tracking.value.events?.length || tracking.value.awb))
})
// Delivered + at least one item not yet reviewed → offer a top-level Review.
const canReviewOrReturn = computed(() =>
  isDelivered.value && !!order.value?.items?.some(it => !it.has_review),
)
// First item eligible for a NEW return/exchange (skip ones already returned).
const firstReturnable = computed<OrderItem | null>(() =>
  order.value?.items?.find(it => itemReturnable(it) && !it.return_id) || null,
)
const hasReturnable = computed(() => !!firstReturnable.value)
// First item that already has a return/exchange (drives the "view status" button).
const firstExistingReturn = computed<OrderItem | null>(() =>
  order.value?.items?.find(it => !!it.return_id) || null,
)
const hasExistingReturn = computed(() => !!firstExistingReturn.value)

// Navigate to the Returns page, deep-linking the specific return when known.
function viewReturnStatus(returnId?: string) {
  if (returnId) router.push({ path: '/returns', query: { id: returnId } })
  else router.push('/returns')
}

// Top-level Review action: with one item open it directly; with several, scroll
// to the items so the customer picks which one.
function reviewFromActions() {
  const items = (order.value?.items || []).filter(it => !it.has_review)
  if (items.length === 1) openReview(items[0])
  else document.getElementById('order-items')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
// Top-level Return/Exchange action: open the first returnable item, or scroll.
function returnFromActions() {
  const items = (order.value?.items || []).filter(it => itemReturnable(it))
  if (items.length === 1) openReturn(items[0])
  else document.getElementById('order-items')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// Tracking popup (opened from the Track button; no longer an inline section).
const showTracking = ref(false)

// Cancel confirmation popup state + reason.
const showCancelConfirm = ref(false)
const cancelReason = ref('')
const CANCEL_REASONS = [
  'Ordered by mistake',
  'Found a better price elsewhere',
  'Delivery taking too long',
  'Want to change address / payment',
  'No longer needed',
  'Other',
]

function openCancelConfirm() {
  cancelReason.value = ''
  cancelError.value = ''
  showCancelConfirm.value = true
}



const firstItem = computed<OrderItem | null>(() => (order.value?.items?.length ? order.value.items[0] : null))
const extraCount = computed(() => {
  const n = order.value?.items?.length || 0
  return n > 1 ? n - 1 : 0
})

// Friendly heading per status for the big status card.
const STATUS_HEAD: Record<string, string> = {
  pending_payment: 'Awaiting payment',
  confirmed: 'Order confirmed',
  processing: 'Being prepared',
  packed: 'Packed & ready',
  ready_to_ship: 'Ready to ship',
  shipped: 'Shipped',
  in_transit: 'On the way',
  out_for_delivery: 'Out for delivery',
  delivered: 'Order delivered',
  cancelled: 'Order cancelled',
  return_requested: 'Return requested',
  return_approved: 'Return approved',
  return_picked: 'Return picked up',
  return_received: 'Return received',
  refund_processing: 'Refund processing',
  refunded: 'Refunded',
}

// Top status card look + copy, driven by the shared meaning-based colours.
const statusCard = computed(() => {
  const s = order.value?.status || ''
  const style = orderStatusStyle(s)
  return {
    tone: style.card,        // gradient + border
    icon: style.icon,
    iconWrap: style.solid,   // solid icon circle
    accent: style.text,      // headline colour
    head: STATUS_HEAD[s] || orderStatusLabel(s),
  }
})

// Latest tracking event's time (for delivered/shipped date line).
const latestEventTime = computed(() => tracking.value?.events?.[0]?.event_time || order.value?.placed_at || order.value?.created_at || '')

/* ── Order status steps (used when there's no courier timeline) ── */
const STATUS_ORDER = ['pending_payment','confirmed','processing','packed','ready_to_ship','shipped','in_transit','out_for_delivery','delivered']
const ALL_STEPS = [
  { status: 'confirmed',        label: 'Order Confirmed',  icon: CheckCircle },
  { status: 'processing',       label: 'Processing',       icon: Package },
  { status: 'packed',           label: 'Packed',           icon: Package },
  { status: 'shipped',          label: 'Shipped',          icon: Truck },
  { status: 'out_for_delivery', label: 'Out for Delivery', icon: Truck },
  { status: 'delivered',        label: 'Delivered',        icon: CheckCircle },
]
const currentStepIndex = computed(() => order.value ? STATUS_ORDER.indexOf(order.value.status) : -1)
function stepState(status: string): 'done' | 'active' | 'inactive' {
  const idx = STATUS_ORDER.indexOf(status)
  if (isCancelled.value) return 'inactive'
  if (idx < currentStepIndex.value) return 'done'
  if (idx === currentStepIndex.value) return 'active'
  return 'inactive'
}

// Dot colour per step, chosen by the step's state AND the order's overall
// status: completed steps are green; the current step is green once the order
// is delivered, otherwise accent-pink (still in progress); future steps grey.
function stepDotClass(status: string): string {
  const state = stepState(status)
  if (state === 'inactive') return 'bg-stone-300'
  if (state === 'done') return 'bg-emerald-500'
  // active step:
  return isDelivered.value ? 'bg-emerald-500' : 'bg-accent'
}
function stepTextClass(status: string): string {
  const state = stepState(status)
  if (state === 'inactive') return 'text-stone-500'
  if (state === 'active') return isDelivered.value ? 'font-semibold text-emerald-700' : 'font-semibold text-ink'
  return 'text-stone-700'
}
// Prefer a real courier/self-ship timeline when available; fall back to the
// order status steps otherwise.
const hasTrackingTimeline = computed(() => !!tracking.value?.events?.length)

// Colour the latest courier event by what it actually says.
function eventDotClass(status: string): string {
  const s = status.toLowerCase()
  if (s.includes('deliver')) return 'bg-emerald-500'
  if (s.includes('cancel') || s.includes('return') || s.includes('rto') || s.includes('fail')) return 'bg-red-500'
  return 'bg-accent'
}
function eventTextClass(status: string): string {
  const s = status.toLowerCase()
  if (s.includes('deliver')) return 'text-emerald-700'
  if (s.includes('cancel') || s.includes('return') || s.includes('rto') || s.includes('fail')) return 'text-red-600'
  return 'text-ink'
}

/* ── Reviews ─────────────────────────────────────────────── */
const reviewItem = ref<OrderItem | null>(null)
const reviewRating = ref(5)
const reviewTitle = ref('')
const reviewBody = ref('')
const reviewSubmitting = ref(false)
// Review photo upload (up to 5).
const reviewPhotoKeys = ref<string[]>([])
const reviewPhotoPreviews = ref<string[]>([])
const reviewUploading = ref(false)

function openReview(item: OrderItem) {
  reviewItem.value = item
  reviewRating.value = 5
  reviewTitle.value = ''
  reviewBody.value = ''
  reviewPhotoKeys.value = []
  reviewPhotoPreviews.value = []
}
async function onReviewPhoto(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (reviewPhotoKeys.value.length >= 5) { toast.error('You can add up to 5 photos.'); return }
  reviewUploading.value = true
  try {
    const res = await customer.uploadImage(file)
    reviewPhotoKeys.value.push(res.key)
    reviewPhotoPreviews.value.push(res.url || resolveMediaUrl(MEDIA_BASE, res.key))
  } catch (err) { toast.error(extractApiError(err)) }
  finally { reviewUploading.value = false; if (input) input.value = '' }
}
function removeReviewPhoto(i: number) {
  reviewPhotoKeys.value.splice(i, 1)
  reviewPhotoPreviews.value.splice(i, 1)
}
async function submitReview() {
  if (!reviewItem.value) return
  reviewSubmitting.value = true
  try {
    await customer.createReview({
      order_item_id: reviewItem.value.id,
      rating: reviewRating.value,
      title: reviewTitle.value || undefined,
      body: reviewBody.value || undefined,
      photo_keys: reviewPhotoKeys.value.length ? [...reviewPhotoKeys.value] : undefined,
    })
    toast.success('Thanks for your review!')
    reviewItem.value = null
    await reload()   // refresh so the item flips to "Reviewed"
  } catch (e) { toast.error(extractApiError(e)) } finally { reviewSubmitting.value = false }
}

/* ── Return / exchange flow ─────────────────────────────────
   Multi-step: (1) return or exchange + reason, (2) photos (recommended for
   damage/defect/wrong-item), (3) refund destination (UPI / bank). */
const returnItem = ref<OrderItem | null>(null)
const returnStep = ref<1 | 2 | 3>(1)
const returnType = ref<'return' | 'exchange'>('return')
const reasonCode = ref('')
const returnNote = ref('')
const refundMethod = ref<'upi' | 'bank'>('upi')
const refundUpi = ref('')
const refundAccName = ref('')
const refundAccNo = ref('')
const refundIfsc = ref('')
const photoKeys = ref<string[]>([])
const photoPreviews = ref<string[]>([])
const uploadingPhoto = ref(false)
const returnSubmitting = ref(false)
const returnPhotoInput = ref<HTMLInputElement | null>(null)

const selectedReason = computed(() => RETURN_REASONS.find(r => r.code === reasonCode.value) || null)
const photoRecommended = computed(() => !!selectedReason.value?.photo)
// Reasons available for the chosen resolution (exchange has a narrower set).
const reasonOptions = computed(() =>
  returnType.value === 'exchange' ? RETURN_REASONS.filter(r => r.exchange) : RETURN_REASONS,
)

function openReturn(item: OrderItem) {
  returnItem.value = item
  returnStep.value = 1
  returnType.value = 'return'
  reasonCode.value = ''
  returnNote.value = ''
  refundMethod.value = 'upi'
  refundUpi.value = ''; refundAccName.value = ''; refundAccNo.value = ''; refundIfsc.value = ''
  photoKeys.value = []; photoPreviews.value = []
}
function closeReturn() { returnItem.value = null }

async function onReturnPhoto(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (photoKeys.value.length >= 4) { toast.error('You can add up to 4 photos.'); return }
  uploadingPhoto.value = true
  try {
    const res = await customer.uploadImage(file)
    photoKeys.value.push(res.key)
    photoPreviews.value.push(res.url || resolveMediaUrl(MEDIA_BASE, res.key))
  } catch (err) { toast.error(extractApiError(err)) }
  finally { uploadingPhoto.value = false; if (input) input.value = '' }
}
function removePhoto(i: number) {
  photoKeys.value.splice(i, 1)
  photoPreviews.value.splice(i, 1)
}

function returnNext() {
  if (returnStep.value === 1) {
    if (!reasonCode.value) { toast.error('Please choose a reason.'); return }
    returnStep.value = 2
    return
  }
  if (returnStep.value === 2) {
    // Exchange needs no refund destination — submit straight away.
    if (returnType.value === 'exchange') { submitReturn(); return }
    returnStep.value = 3
  }
}
function returnBack() {
  if (returnStep.value > 1) returnStep.value = (returnStep.value - 1) as 1 | 2 | 3
}

async function submitReturn() {
  if (!returnItem.value) return
  if (returnType.value === 'return') {
    if (refundMethod.value === 'upi' && !refundUpi.value.trim()) { toast.error('Enter your UPI ID.'); return }
    if (refundMethod.value === 'bank' && (!refundAccName.value.trim() || !refundAccNo.value.trim() || !refundIfsc.value.trim())) {
      toast.error('Fill all bank details.'); return
    }
  }
  returnSubmitting.value = true
  try {
    await customer.requestReturn({
      order_item_id: returnItem.value.id,
      type: returnType.value,
      reason_code: reasonCode.value,
      reason: returnNote.value || returnReasonLabel(reasonCode.value),
      ...(returnType.value === 'return' ? {
        refund_method: refundMethod.value,
        refund_upi: refundMethod.value === 'upi' ? refundUpi.value.trim() : undefined,
        refund_acc_name: refundMethod.value === 'bank' ? refundAccName.value.trim() : undefined,
        refund_acc_no: refundMethod.value === 'bank' ? refundAccNo.value.trim() : undefined,
        refund_ifsc: refundMethod.value === 'bank' ? refundIfsc.value.trim().toUpperCase() : undefined,
      } : {}),
      photo_keys: photoKeys.value,
    })
    toast.success(returnType.value === 'exchange' ? 'Exchange request submitted' : 'Return request submitted')
    closeReturn()
    await reload()
  } catch (e) { toast.error(extractApiError(e)) } finally { returnSubmitting.value = false }
}

// Days left in the return window (delivery + 7); null if unknown.
function daysLeft(item: OrderItem): number | null {
  if (!item.delivered_at) return null
  const closes = new Date(item.delivered_at).getTime() + RETURN_WINDOW_DAYS * 86400000
  return Math.ceil((closes - Date.now()) / 86400000)
}
function windowOpen(item: OrderItem): boolean {
  const d = daysLeft(item)
  return d === null || d > 0   // if we don't know delivery time, let the server decide
}
function itemReturnable(item: OrderItem): boolean {
  return isDelivered.value
    && windowOpen(item)
    && !['return_requested','return_approved','return_received','refunded','cancelled'].includes(item.item_status)
}

/* ── Invoice + cancel ────────────────────────────────────── */
const downloadingInvoice = ref(false)
async function downloadInvoice() {
  if (!order.value) return
  downloadingInvoice.value = true
  try {
    const blob = await api.getBlob(customer.invoiceURL(order.value.id))
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = `carlitos-invoice-${order.value.order_number}.pdf`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch { /* ignore */ } finally { downloadingInvoice.value = false }
}
async function cancel() {
  if (!order.value) return
  if (!cancelReason.value) { cancelError.value = 'Please select a reason for cancelling.'; return }
  cancelling.value = true; cancelError.value = ''
  try {
    await customer.cancelOrder(order.value.id, cancelReason.value)
    await reload()
    showCancelConfirm.value = false
    toast.success('Order cancelled')
  } catch (e) { cancelError.value = extractApiError(e) } finally { cancelling.value = false }
}
</script>

<template>
  <!-- Loading -->
  <div v-if="loading" class="mx-auto max-w-3xl space-y-4">
    <div class="skeleton h-5 w-24 rounded" />
    <div class="skeleton h-56 w-full rounded-2xl" />
    <div class="skeleton h-40 w-full rounded-2xl" />
  </div>

  <!-- Not found -->
  <div v-else-if="!order" class="py-12 text-center">
    <p class="text-stone-600">Order not found.</p>
    <RouterLink to="/orders" class="btn-ghost btn-sm mt-4 inline-flex">← My Orders</RouterLink>
  </div>

  <div v-else class="mx-auto max-w-2xl">
    <!-- Top bar: back (left corner) + invoice (right corner) -->
    <div class="mb-3 flex items-center justify-between">
      <button
        type="button"
        class="-ml-1 flex items-center gap-1 rounded-full py-1 pr-2 text-xs font-medium text-stone-600 transition-colors hover:text-ink"
        aria-label="Go back"
        @click="goBackToOrders"
      >
        <ChevronLeft :size="16" /> Back
      </button>
      <button class="flex items-center gap-1 text-xs font-medium text-stone-600 transition-colors hover:text-ink disabled:text-stone-300" :disabled="downloadingInvoice" @click="downloadInvoice">
        <Download :size="14" /> {{ downloadingInvoice ? 'Preparing…' : 'Invoice' }}
      </button>
    </div>

    <!-- Order number + placed date -->
    <div class="mb-3">
      <p class="text-xs font-semibold text-ink">Order #{{ order.order_number }}</p>
      <p class="mt-0.5 text-xs text-stone-500">Placed {{ formatDate(order.placed_at || order.created_at) }}</p>
    </div>

    <p v-if="cancelError" class="mb-3 rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-600">{{ cancelError }}</p>

    <!-- Status card (single parcel icon, colour by status) -->
    <div :class="['mb-4 rounded-xl border bg-gradient-to-br p-3.5', statusCard.tone]">
      <div class="flex items-center gap-3">
        <span :class="['flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white', statusCard.iconWrap]">
          <Package :size="18" />
        </span>
        <div class="min-w-0 flex-1">
          <p class="text-xs text-stone-600">{{ statusCard.head }}{{ isDelivered ? ' on' : '' }}</p>
          <p :class="['mt-0.5 text-base font-semibold', statusCard.accent]">{{ formatDate(latestEventTime) }}</p>
        </div>
      </div>
      <div v-if="extraCount" class="mt-3 flex items-center gap-1.5 border-t border-black/5 pt-2.5 text-xs text-stone-600">
        <Package :size="14" class="text-stone-400" /> {{ extraCount }} more item{{ extraCount > 1 ? 's' : '' }} in this order
      </div>
    </div>

    <!-- Situation-based actions (shown right after the status) -->
    <div v-if="canTrack || canCancel || canReviewOrReturn || hasReturnable || hasExistingReturn" class="mb-4 flex flex-wrap gap-2">
      <!-- Track: order is on the move → opens the tracking popup -->
      <button
        v-if="canTrack"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-2.5 text-xs font-semibold text-ink transition-colors hover:border-stone-300 hover:bg-stone-50"
        @click="showTracking = true"
      >
        <Truck :size="15" /> Track order
      </button>

      <!-- Delivered: write a review -->
      <button
        v-if="canReviewOrReturn"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-2.5 text-xs font-semibold text-ink transition-colors hover:border-stone-300 hover:bg-stone-50"
        @click="reviewFromActions"
      >
        <Star :size="15" /> Review
      </button>

      <!-- Delivered + within window + not already returned: return / exchange -->
      <button
        v-if="hasReturnable"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-2.5 text-xs font-semibold text-ink transition-colors hover:border-stone-300 hover:bg-stone-50"
        @click="returnFromActions"
      >
        <RotateCcw :size="15" /> Return / Exchange
      </button>

      <!-- Already returned/exchanged → view its live status on the Returns page -->
      <button
        v-if="hasExistingReturn"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-accent/30 bg-accent-soft px-3 py-2.5 text-xs font-semibold text-accent transition-colors hover:bg-accent-soft/70"
        @click="viewReturnStatus(firstExistingReturn?.return_id)"
      >
        <RotateCcw :size="15" /> View return &amp; exchange status
      </button>

      <!-- Cancel: still cancellable → opens a confirmation popup -->
      <button
        v-if="canCancel"
        class="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-red-200 bg-white px-3 py-2.5 text-xs font-semibold text-red-600 transition-colors hover:bg-red-50"
        @click="openCancelConfirm"
      >
        <X :size="15" /> Cancel order
      </button>
    </div>

    <!-- Placed confirmation -->
    <div v-if="justPlaced && !isCancelled" class="mb-6 flex items-start gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4">
      <CheckCircle :size="22" class="mt-0.5 shrink-0 text-emerald-600" />
      <div class="flex-1">
        <p class="text-sm font-semibold text-emerald-800">Thank you! Your order is confirmed.</p>
        <p class="mt-0.5 text-sm text-emerald-700">
          {{ order.payment_method === 'cod' ? 'Cash on Delivery' : 'Paid online' }}. We'll email you updates.
        </p>
      </div>
    </div>

    <!-- Items (all products in this order, incl. grouped multi-seller) -->
    <div id="order-items" class="card mb-4 scroll-mt-4 overflow-hidden">
      <div class="flex items-center justify-between border-b border-stone-100 px-4 py-3">
        <h2 class="text-sm font-semibold text-ink">Items</h2>
        <span class="text-xs text-stone-400">{{ order.items?.length || 0 }} product{{ (order.items?.length || 0) > 1 ? 's' : '' }}</span>
      </div>
      <div class="divide-y divide-stone-100">
        <component
          :is="it.product_slug ? 'RouterLink' : 'div'"
          v-for="it in order.items"
          :key="it.id"
          :to="it.product_slug ? `/product/${it.product_slug}` : undefined"
          class="flex items-start gap-3 px-4 py-3 transition-colors"
          :class="it.product_slug ? 'cursor-pointer hover:bg-stone-50' : ''"
        >
          <div class="h-16 w-14 shrink-0 overflow-hidden rounded-md bg-stone-100">
            <img v-if="it.image" :src="resolveMediaUrl(MEDIA_BASE, it.image)" :alt="it.product_title" class="h-full w-full object-cover" loading="lazy" />
            <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="18" /></div>
          </div>
          <div class="min-w-0 flex-1">
            <p class="line-clamp-2 text-xs font-medium leading-snug text-ink">{{ it.product_title }}</p>
            <p v-if="it.seller_name" class="mt-0.5 text-[11px] text-stone-400">by {{ it.seller_name }}</p>
            <p class="mt-0.5 text-[11px] text-stone-500">
              <span v-if="it.variant_label">{{ it.variant_label }} · </span>Qty {{ it.quantity }}
              <span v-if="it.sku"> · SKU {{ it.sku }}</span>
            </p>
            <span :class="['mt-1 inline-block rounded px-1.5 py-0.5 text-[10px] font-semibold', orderStatusStyle(it.item_status).chip]">
              {{ orderStatusLabel(it.item_status) }}
            </span>
          </div>
          <div class="flex shrink-0 flex-col items-end gap-1">
            <p class="text-xs font-semibold text-ink">{{ formatINR(it.gross_amount) }}</p>
            <ChevronRight v-if="it.product_slug" :size="14" class="text-stone-300" />
          </div>
        </component>
      </div>
    </div>

    <!-- Price breakdown -->
    <div class="card mb-5 p-5">
      <h2 class="text-h4 mb-4">Price Details</h2>
      <div class="space-y-2.5 text-sm">
        <div class="flex justify-between"><span class="text-stone-500">Items subtotal</span><span>{{ formatINR(order.items_subtotal) }}</span></div>
        <div v-if="order.discount_total > 0" class="flex justify-between"><span class="text-stone-500">Discount</span><span class="text-emerald-600">-{{ formatINR(order.discount_total) }}</span></div>
        <div class="flex justify-between"><span class="text-stone-500">Delivery</span><span>{{ order.shipping_total > 0 ? formatINR(order.shipping_total) : 'FREE' }}</span></div>
        <div class="flex justify-between"><span class="text-stone-500">Tax</span><span>{{ formatINR(order.tax_total) }}</span></div>
        <div class="flex justify-between border-t border-stone-200 pt-2 text-base font-semibold"><span>Total</span><span>{{ formatINR(order.grand_total) }}</span></div>
      </div>
    </div>

    <!-- Delivery address -->
    <div class="card p-5">
      <div class="mb-3 flex items-center gap-2">
        <MapPin :size="16" class="text-stone-500" /><h2 class="text-h4">Delivery Address</h2>
      </div>
      <p class="text-sm font-medium text-stone-700">{{ order.ship_contact_name }}</p>
      <p class="text-sm text-stone-600">{{ order.ship_contact_phone }}</p>
      <p class="mt-0.5 text-sm text-stone-600">{{ order.ship_line1 }}<span v-if="order.ship_line2">, {{ order.ship_line2 }}</span></p>
      <p class="text-sm text-stone-600">{{ order.ship_city }}, {{ order.ship_state }} – {{ order.ship_pincode }}</p>
    </div>
  </div>

  <!-- ── Tracking popup ───────────────────────────────────── -->
  <transition name="fade">
    <div v-if="showTracking" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="showTracking = false">
      <div class="flex max-h-[85vh] w-full max-w-md flex-col rounded-t-2xl bg-white shadow-lg sm:rounded-2xl">
        <div class="flex items-center justify-between border-b border-stone-100 px-5 py-4">
          <h3 class="flex items-center gap-2 text-h4"><Truck :size="18" class="text-stone-500" /> Tracking</h3>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="showTracking = false"><X :size="18" /></button>
        </div>

        <div class="flex-1 overflow-y-auto px-5 py-4">
          <!-- Courier / shipment meta -->
          <div v-if="tracking" class="mb-4 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs">
            <span class="text-stone-600">
              {{ tracking.ship_type === 'self' ? 'Shipped by seller' : 'Courier:' }}
              <span v-if="tracking.ship_type !== 'self'" class="font-medium text-ink">{{ tracking.courier }}</span>
            </span>
            <span v-if="tracking.awb && tracking.ship_type !== 'self'" class="text-stone-600">
              AWB: <span class="font-medium text-ink">{{ tracking.awb }}</span>
            </span>
            <a
              v-if="tracking.tracking_url && tracking.ship_type !== 'self'"
              :href="tracking.tracking_url" target="_blank" rel="noopener"
              class="font-medium text-accent hover:underline"
            >Courier site</a>
          </div>

          <!-- Real courier / self-ship event timeline (latest event on top) -->
          <ol v-if="hasTrackingTimeline" class="relative ml-1 border-l-2 border-stone-100 pl-6">
            <li v-for="(e, i) in tracking!.events" :key="i" class="relative pb-5 last:pb-0">
              <span
                :class="[
                  'absolute -left-[31px] top-0 flex h-5 w-5 items-center justify-center rounded-full ring-4 ring-white',
                  i === 0 ? eventDotClass(e.status) : 'bg-stone-300',
                ]"
              >
                <span class="h-1.5 w-1.5 rounded-full bg-white" />
              </span>
              <p :class="['text-sm capitalize', i === 0 ? ('font-semibold ' + eventTextClass(e.status)) : 'text-stone-700']">
                {{ e.status.replace(/_/g, ' ') }}
              </p>
              <p v-if="e.description || e.location" class="mt-0.5 text-xs text-stone-500">
                {{ e.description }}<span v-if="e.location"> · {{ e.location }}</span>
              </p>
              <p class="mt-0.5 text-xs text-stone-400">{{ formatDate(e.event_time) }}</p>
            </li>
          </ol>

          <!-- Fallback: order status steps -->
          <ol v-else-if="!isCancelled" class="relative ml-1 border-l-2 border-stone-100 pl-6">
            <li v-for="step in ALL_STEPS" :key="step.status" class="relative pb-5 last:pb-0" :class="stepState(step.status) === 'inactive' ? 'opacity-50' : ''">
              <span
                :class="[
                  'absolute -left-[31px] top-0 flex h-5 w-5 items-center justify-center rounded-full ring-4 ring-white transition-colors',
                  stepDotClass(step.status),
                ]"
              >
                <component :is="step.icon" :size="11" class="text-white" />
              </span>
              <p :class="['text-sm', stepTextClass(step.status)]">{{ step.label }}</p>
            </li>
          </ol>

          <p v-else class="text-sm text-stone-500">This order was {{ order?.status.replace(/_/g, ' ') }}.</p>

          <p v-if="!hasTrackingTimeline && !isCancelled" class="mt-1 text-xs text-stone-400">
            Live tracking updates will appear here once your shipment is scanned.
          </p>
        </div>
      </div>
    </div>
  </transition>

  <!-- ── Cancel confirmation popup (with reason dropdown) ─── -->
  <transition name="fade">
    <div v-if="showCancelConfirm" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="showCancelConfirm = false">
      <div class="w-full max-w-sm rounded-t-2xl bg-white p-5 shadow-lg sm:rounded-2xl">
        <div class="mb-3 flex items-center gap-2.5">
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-red-50 text-red-500">
            <X :size="18" />
          </span>
          <div>
            <h3 class="text-sm font-semibold text-ink">Cancel this order?</h3>
            <p class="text-xs text-stone-500">This can't be undone.</p>
          </div>
        </div>

        <label class="label">Reason for cancelling</label>
        <select v-model="cancelReason" class="select">
          <option value="" disabled>Select a reason</option>
          <option v-for="r in CANCEL_REASONS" :key="r" :value="r">{{ r }}</option>
        </select>

        <p v-if="cancelError" class="mt-2 text-xs text-red-600">{{ cancelError }}</p>

        <div class="mt-4 flex gap-3">
          <button class="btn-ghost flex-1" :disabled="cancelling" @click="showCancelConfirm = false">Keep order</button>
          <button
            class="flex-1 rounded-lg bg-red-600 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-red-700 disabled:opacity-60"
            :disabled="cancelling"
            @click="cancel"
          >
            {{ cancelling ? 'Cancelling…' : 'Cancel order' }}
          </button>
        </div>
      </div>
    </div>
  </transition>

  <!-- ── Review modal ─────────────────────────────────────── -->
  <transition name="fade">
    <div v-if="reviewItem" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="reviewItem = null">
      <div class="w-full max-w-md rounded-t-2xl bg-white p-5 shadow-lg sm:rounded-2xl">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-h4">Write a review</h3>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="reviewItem = null"><X :size="18" /></button>
        </div>
        <p class="mb-3 truncate text-sm text-stone-600">{{ reviewItem.product_title }}</p>
        <div class="mb-4 flex items-center gap-1">
          <button v-for="s in 5" :key="s" type="button" :aria-label="`${s} star`" @click="reviewRating = s">
            <Star :size="26" :class="s <= reviewRating ? 'fill-amber-400 text-amber-400' : 'fill-stone-200 text-stone-200'" />
          </button>
        </div>
        <div class="space-y-3">
          <div>
            <label class="label">Title <span class="text-stone-400">(optional)</span></label>
            <input v-model="reviewTitle" class="input" maxlength="120" placeholder="Great quality!" />
          </div>
          <div>
            <label class="label">Your review <span class="text-stone-400">(optional)</span></label>
            <textarea v-model="reviewBody" class="textarea" rows="4" maxlength="1000" placeholder="Share details about fit, fabric, delivery…"></textarea>
          </div>

          <!-- Photos (optional, up to 5) -->
          <div>
            <label class="label">Add photos <span class="text-stone-400">(optional)</span></label>
            <div class="flex flex-wrap gap-2">
              <div v-for="(p, i) in reviewPhotoPreviews" :key="i" class="relative h-16 w-16 overflow-hidden rounded-lg border border-stone-200">
                <img :src="p" alt="Review photo" class="h-full w-full object-cover" />
                <button type="button" class="absolute right-0.5 top-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-white" aria-label="Remove photo" @click="removeReviewPhoto(i)">
                  <X :size="10" />
                </button>
              </div>
              <label
                v-if="reviewPhotoKeys.length < 5"
                class="flex h-16 w-16 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg border border-dashed border-stone-300 text-stone-400 transition-colors hover:border-ink hover:text-ink"
              >
                <ImagePlus :size="16" />
                <span class="text-[10px]">{{ reviewUploading ? '…' : 'Add' }}</span>
                <input type="file" accept="image/*" class="hidden" :disabled="reviewUploading" @change="onReviewPhoto" />
              </label>
            </div>
          </div>
        </div>
        <div class="mt-4 flex gap-3">
          <button class="btn-ghost flex-1" @click="reviewItem = null">Cancel</button>
          <button class="btn-primary flex-1" :disabled="reviewSubmitting || reviewUploading" @click="submitReview">{{ reviewSubmitting ? 'Submitting…' : 'Submit review' }}</button>
        </div>
      </div>
    </div>
  </transition>

  <!-- ── Return / Exchange flow ───────────────────────────── -->
  <transition name="fade">
    <div v-if="returnItem" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="closeReturn">
      <div class="flex max-h-[92vh] w-full max-w-md flex-col rounded-t-2xl bg-white shadow-lg sm:rounded-2xl">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-stone-100 px-5 py-4">
          <div class="flex items-center gap-2">
            <button v-if="returnStep > 1" class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Back" @click="returnBack"><ChevronLeft :size="18" /></button>
            <h3 class="text-h4">{{ returnType === 'exchange' ? 'Exchange item' : 'Return item' }}</h3>
          </div>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="closeReturn"><X :size="18" /></button>
        </div>

        <!-- Step indicator -->
        <div class="flex items-center gap-1.5 px-5 pt-4">
          <span v-for="s in (returnType === 'exchange' ? 2 : 3)" :key="s" :class="['h-1 flex-1 rounded-full', s <= returnStep ? 'bg-accent' : 'bg-stone-200']" />
        </div>

        <div class="flex-1 overflow-y-auto px-5 py-4">
          <div class="mb-4 flex items-center gap-3">
            <div class="h-14 w-12 shrink-0 overflow-hidden rounded-md bg-stone-100">
              <img v-if="returnItem.image" :src="resolveMediaUrl(MEDIA_BASE, returnItem.image)" :alt="returnItem.product_title" class="h-full w-full object-cover" />
              <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="16" /></div>
            </div>
            <p class="line-clamp-2 text-sm font-medium text-ink">{{ returnItem.product_title }}</p>
          </div>

          <!-- STEP 1: resolution + reason -->
          <template v-if="returnStep === 1">
            <label class="label">What would you like to do?</label>
            <div class="mb-4 flex gap-2">
              <button type="button" :class="['flex-1 rounded-lg border px-3 py-2.5 text-sm', returnType === 'return' ? 'border-accent bg-accent-soft/40 font-medium text-accent' : 'border-stone-200 text-stone-600']" @click="returnType = 'return'; reasonCode = ''">Return &amp; refund</button>
              <button type="button" :class="['flex-1 rounded-lg border px-3 py-2.5 text-sm', returnType === 'exchange' ? 'border-accent bg-accent-soft/40 font-medium text-accent' : 'border-stone-200 text-stone-600']" @click="returnType = 'exchange'; reasonCode = ''">Exchange</button>
            </div>

            <label class="label">Reason</label>
            <select v-model="reasonCode" class="select w-full">
              <option value="" disabled>Select a reason…</option>
              <option v-for="r in reasonOptions" :key="r.code" :value="r.code">{{ r.label }}</option>
            </select>

            <div class="mt-3">
              <label class="label">Add a note <span class="text-stone-400">(optional)</span></label>
              <textarea v-model="returnNote" class="textarea" rows="2" maxlength="500" placeholder="Any extra detail that helps the seller…"></textarea>
            </div>
          </template>

          <!-- STEP 2: photos -->
          <template v-else-if="returnStep === 2">
            <label class="label">
              Add photos
              <span :class="photoRecommended ? 'text-accent' : 'text-stone-400'">({{ photoRecommended ? 'recommended for this reason' : 'optional' }})</span>
            </label>
            <p class="mb-3 text-xs text-stone-500">Clear photos of the issue help the seller approve your request faster. Up to 4.</p>
            <input ref="returnPhotoInput" type="file" accept="image/jpeg,image/png,image/webp" class="hidden" @change="onReturnPhoto" />
            <div class="flex flex-wrap gap-2">
              <div v-for="(p, i) in photoPreviews" :key="i" class="relative h-20 w-20 overflow-hidden rounded-lg border border-stone-200">
                <img :src="p" alt="Return proof" class="h-full w-full object-cover" />
                <button type="button" class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white" aria-label="Remove" @click="removePhoto(i)"><X :size="12" /></button>
              </div>
              <button
                v-if="photoKeys.length < 4"
                type="button"
                class="flex h-20 w-20 flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-stone-300 text-stone-400 hover:border-accent hover:text-accent"
                :disabled="uploadingPhoto"
                @click="returnPhotoInput?.click()"
              >
                <ImagePlus :size="20" />
                <span class="text-[10px]">{{ uploadingPhoto ? '…' : 'Add' }}</span>
              </button>
            </div>
          </template>

          <!-- STEP 3: refund destination (return only) -->
          <template v-else>
            <label class="label">Where should we send your refund?</label>
            <div class="mb-4 flex gap-2">
              <button type="button" :class="['flex-1 rounded-lg border px-3 py-2.5 text-sm', refundMethod === 'upi' ? 'border-accent bg-accent-soft/40 font-medium text-accent' : 'border-stone-200 text-stone-600']" @click="refundMethod = 'upi'">UPI</button>
              <button type="button" :class="['flex-1 rounded-lg border px-3 py-2.5 text-sm', refundMethod === 'bank' ? 'border-accent bg-accent-soft/40 font-medium text-accent' : 'border-stone-200 text-stone-600']" @click="refundMethod = 'bank'">Bank account</button>
            </div>

            <div v-if="refundMethod === 'upi'">
              <label class="label">UPI ID</label>
              <input v-model="refundUpi" class="input" placeholder="yourname@upi" autocapitalize="off" />
            </div>
            <div v-else class="space-y-3">
              <div>
                <label class="label">Account holder name</label>
                <input v-model="refundAccName" class="input" placeholder="As per bank records" />
              </div>
              <div>
                <label class="label">Account number</label>
                <input v-model="refundAccNo" class="input" inputmode="numeric" placeholder="Account number" />
              </div>
              <div>
                <label class="label">IFSC code</label>
                <input v-model="refundIfsc" class="input uppercase" placeholder="e.g. HDFC0001234" />
              </div>
            </div>
            <p class="mt-3 text-xs text-stone-400">The refund is paid after the seller approves and receives the item.</p>
          </template>
        </div>

        <!-- Footer -->
        <div class="border-t border-stone-100 p-4">
          <button
            v-if="returnStep < (returnType === 'exchange' ? 2 : 3)"
            class="btn-primary w-full"
            @click="returnNext"
          >Continue</button>
          <button
            v-else
            class="btn-primary w-full"
            :disabled="returnSubmitting"
            @click="submitReturn"
          >{{ returnSubmitting ? 'Submitting…' : (returnType === 'exchange' ? 'Submit exchange' : 'Submit return') }}</button>
        </div>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
