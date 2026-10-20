<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChevronLeft, Truck, Printer, Package, CheckCircle, MapPin, FileText,
  X, Check, Loader2, Ban, Copy,
} from 'lucide-vue-next'
import type { Order } from '@carlitos/types'
import { seller, api } from '@/api'
import { formatINR, formatDate, orderStatusLabel, extractApiError, BRAND } from '@carlitos/shared'

interface Shipment {
  id: string; order_id: string; ship_type: string; awb_number: string
  courier_name: string; status: string; tracking_url: string
}
interface HistoryEvent { from_status: string; to_status: string; note: string; created_at: string }

const route   = useRoute()
const router  = useRouter()
const order   = ref<Order | null>(null)
const shipment = ref<Shipment | null>(null)
const history  = ref<HistoryEvent[]>([])
const loading = ref(true)
const message = ref('')
const error   = ref('')
const busy    = ref(false)
const showCancel = ref(false)
const cancelReason = ref('')

// Allowed item-status advances a seller performs (pre-shipment).
const nextStatuses: Record<string, string> = {
  processing: 'packed', packed: 'ready_to_ship',
}
const nextLabel: Record<string, string> = {
  processing: 'Mark Packed', packed: 'Mark Ready to Ship',
}

// Overall fulfilment stage derived from the seller's items (min progress).
const STAGES = ['confirmed', 'processing', 'packed', 'ready_to_ship', 'shipped', 'delivered']
const STAGE_LABEL: Record<string, string> = {
  confirmed: 'New', processing: 'Processing', packed: 'Packed',
  ready_to_ship: 'Ready', shipped: 'Shipped', delivered: 'Delivered',
}

async function load() {
  order.value = await seller.order(route.params.id as string).catch(() => null)
  try {
    const all = await seller.shipments() as Shipment[]
    shipment.value = (all || []).find(s => s.order_id === (route.params.id as string)) || null
  } catch { shipment.value = null }
  try { history.value = (await seller.orderHistory(route.params.id as string)) as HistoryEvent[] } catch { history.value = [] }
}

// The lowest item stage = where the order is in the pipeline.
const items = computed(() => order.value?.items || [])
const currentStage = computed(() => {
  if (!items.value.length) return 'confirmed'
  if (shipment.value?.status === 'delivered' || items.value.every(i => i.item_status === 'delivered')) return 'delivered'
  if (shipment.value) return items.value.every(i => ['shipped','out_for_delivery','delivered','in_transit'].includes(i.item_status)) ? 'shipped' : 'ready_to_ship'
  // Pre-shipment: lowest item stage.
  let idx = STAGES.length
  for (const it of items.value) {
    const i = STAGES.indexOf(it.item_status)
    if (i >= 0 && i < idx) idx = i
  }
  return STAGES[Math.min(idx, STAGES.length - 1)] || 'confirmed'
})
const stageIndex = computed(() => Math.max(0, STAGES.indexOf(currentStage.value)))
const isCancelled = computed(() => items.value.length > 0 && items.value.every(i => i.item_status === 'cancelled'))

// Which bulk action applies to the seller's items right now.
const isNew        = computed(() => items.value.some(i => ['confirmed','pending_payment'].includes(i.item_status)))
const advanceState = computed(() => {
  // If all seller items share a next-able status, offer that bulk advance.
  const statuses = new Set(items.value.map(i => i.item_status))
  for (const s of ['processing', 'packed']) {
    if (statuses.has(s)) return s
  }
  return ''
})
const canShip = computed(() =>
  !shipment.value && items.value.some(i => ['packed', 'ready_to_ship'].includes(i.item_status))
)

async function act(fn: () => Promise<unknown>, okMsg: string) {
  busy.value = true; error.value = ''; message.value = ''
  try {
    await fn()
    message.value = okMsg
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = false }
}

function acceptOrder()  { act(() => seller.acceptOrder(order.value!.id), 'Order accepted — customer notified.') }
function advance()      {
  const to = nextStatuses[advanceState.value]
  const its = items.value.filter(i => i.item_status === advanceState.value)
  act(async () => { for (const it of its) await seller.setItemStatus(it.id, to) },
    `Items moved to ${to.replace('_', ' ')}.`)
}
function shipCourier()  { act(() => seller.createShipment(order.value!.id), 'Shipment booked — AWB assigned & customer notified.') }
function shipSelf()     { act(() => seller.selfShip(order.value!.id), 'Self-ship created. Download & affix the label, then deliver.') }
function markDelivered(){ if (shipment.value) act(() => seller.markDelivered(shipment.value!.id), 'Order marked as delivered.') }

async function submitCancel() {
  if (!cancelReason.value.trim()) { error.value = 'Please enter a cancellation reason.'; return }
  await act(() => seller.cancelOrder(order.value!.id, cancelReason.value.trim()), 'Order cancelled — customer notified.')
  showCancel.value = false
  cancelReason.value = ''
}

async function downloadLabel() {
  if (!shipment.value) return
  busy.value = true; error.value = ''
  try {
    const blob = await api.getBlob(seller.labelURL(shipment.value.id))
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `carlitos-label-${order.value?.order_number || 'order'}.pdf`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = false }
}

function printSlip() { window.print() }
function copyAwb() { if (shipment.value?.awb_number) navigator.clipboard?.writeText(shipment.value.awb_number) }

const sellerTotal = computed(() => items.value.reduce((a, i) => a + i.seller_payable, 0))

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="skeleton h-8 w-1/3 rounded" />
    <div class="skeleton h-40 w-full rounded-xl" />
  </div>

  <div v-else-if="!order" class="py-12 text-center">
    <p class="text-stone-600">Order not found.</p>
    <button class="btn-ghost btn-sm mt-4" @click="router.push('/orders')">← All orders</button>
  </div>

  <div v-else class="mx-auto max-w-3xl">
    <!-- Back (hidden on print) -->
    <button class="no-print mb-4 flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink" @click="router.push('/orders')">
      <ChevronLeft :size="16" /> All orders
    </button>

    <!-- Header -->
    <div class="no-print mb-5 flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 class="text-xl font-semibold text-ink sm:text-2xl">{{ order.order_number }}</h1>
        <p class="mt-0.5 text-sm text-stone-500">Placed {{ formatDate(order.placed_at || order.created_at) }}</p>
      </div>
      <div class="flex items-center gap-2">
        <span :class="['badge px-3 py-1', isCancelled ? 'badge-red' : currentStage === 'delivered' ? 'badge-green' : 'badge-blue']">
          {{ isCancelled ? 'Cancelled' : STAGE_LABEL[currentStage] || orderStatusLabel(order.status) }}
        </span>
        <button class="btn-ghost btn-sm" @click="printSlip"><Printer :size="14" /> <span class="hidden xs:inline">Slip</span></button>
      </div>
    </div>

    <!-- Fulfilment stepper -->
    <div v-if="!isCancelled" class="no-print mb-5 flex items-center gap-1 overflow-x-auto pb-1">
      <template v-for="(st, i) in STAGES" :key="st">
        <div class="flex shrink-0 flex-col items-center gap-1">
          <span :class="[
            'flex h-7 w-7 items-center justify-center rounded-full text-[11px] font-semibold',
            i < stageIndex ? 'bg-emerald-100 text-emerald-700' : i === stageIndex ? 'bg-ink text-white' : 'bg-stone-100 text-stone-400'
          ]">
            <Check v-if="i < stageIndex" :size="13" /><span v-else>{{ i + 1 }}</span>
          </span>
          <span :class="['text-[10px]', i <= stageIndex ? 'text-ink' : 'text-stone-400']">{{ STAGE_LABEL[st] }}</span>
        </div>
        <div v-if="i < STAGES.length - 1" :class="['h-px w-6 shrink-0 sm:flex-1', i < stageIndex ? 'bg-emerald-300' : 'bg-stone-200']" />
      </template>
    </div>

    <!-- Messages -->
    <p v-if="message" class="no-print mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="no-print mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- ── ACTION ZONE (context-aware) ── -->
    <div v-if="!isCancelled" class="no-print mb-5 rounded-xl border border-stone-200 bg-white p-4">
      <p class="mb-3 text-xs font-semibold uppercase tracking-wide text-stone-400">Next step</p>

      <!-- New order: accept or cancel -->
      <template v-if="isNew">
        <p class="mb-3 text-sm text-stone-600">This is a new order. Accept it to start preparing, or cancel with a reason.</p>
        <div class="flex flex-wrap gap-2">
          <button class="btn-primary flex-1 sm:flex-none" :disabled="busy" @click="acceptOrder">
            <Check :size="15" /> Accept Order
          </button>
          <button class="btn-danger flex-1 sm:flex-none" :disabled="busy" @click="showCancel = true">
            <Ban :size="15" /> Cancel
          </button>
        </div>
      </template>

      <!-- Advance packing steps -->
      <template v-else-if="advanceState">
        <p class="mb-3 text-sm text-stone-600">
          {{ advanceState === 'processing' ? 'Prepare and pack the items.' : 'Packed — mark ready so you can ship.' }}
        </p>
        <button class="btn-primary w-full sm:w-auto" :disabled="busy" @click="advance">
          <Package :size="15" /> {{ nextLabel[advanceState] }}
        </button>
      </template>

      <!-- Ready to ship: choose shipping method -->
      <template v-else-if="canShip">
        <p class="mb-3 text-sm text-stone-600">Choose how you'll ship this order.</p>
        <div class="grid gap-2 sm:grid-cols-2">
          <button class="btn-primary" :disabled="busy" @click="shipCourier">
            <Truck :size="15" /> Ship via Courier
          </button>
          <button class="btn-ghost" :disabled="busy" @click="shipSelf">
            <Package :size="15" /> Self-Ship
          </button>
        </div>
        <p class="mt-2 text-xs text-stone-400">Courier assigns an AWB automatically. Self-ship prints a label you affix and deliver yourself.</p>
      </template>

      <!-- Shipment exists -->
      <template v-else-if="shipment">
        <div class="rounded-lg border border-stone-200 bg-stone-50 p-3">
          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
            <span class="font-medium">{{ shipment.ship_type === 'self' ? 'Self-Ship' : shipment.courier_name || 'Courier' }}</span>
            <span v-if="shipment.awb_number" class="inline-flex items-center gap-1 text-stone-600">
              AWB: <span class="font-medium text-ink">{{ shipment.awb_number }}</span>
              <button class="text-stone-400 hover:text-ink" aria-label="Copy AWB" @click="copyAwb"><Copy :size="12" /></button>
            </span>
            <span class="text-stone-600">Status: <span class="font-medium capitalize text-ink">{{ (shipment.status || '').replace(/_/g,' ') }}</span></span>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button v-if="shipment.ship_type === 'self'" class="btn-ghost btn-sm" :disabled="busy" @click="downloadLabel">
              <FileText :size="14" /> Download Label
            </button>
            <a v-if="shipment.tracking_url" :href="shipment.tracking_url" target="_blank" rel="noopener" class="btn-ghost btn-sm">
              <Truck :size="14" /> Track
            </a>
            <button v-if="shipment.status !== 'delivered'" class="btn-primary btn-sm" :disabled="busy" @click="markDelivered">
              <CheckCircle :size="14" /> Mark Delivered
            </button>
          </div>
        </div>
      </template>

      <template v-else>
        <p class="text-sm text-stone-500">No action needed right now.</p>
      </template>
    </div>

    <!-- ── PRINT HEADER ── -->
    <div class="hidden print:block mb-6 border-b border-stone-300 pb-4">
      <div class="flex items-center justify-between">
        <div><p class="text-xl font-bold">{{ BRAND }}</p><p class="text-sm">Packing Slip</p></div>
        <div class="text-right text-sm"><p class="font-semibold">{{ order.order_number }}</p><p>{{ formatDate(order.placed_at || order.created_at) }}</p></div>
      </div>
    </div>

    <!-- Items -->
    <div class="card mb-5 overflow-hidden">
      <div class="border-b border-stone-100 px-4 py-3 no-print"><h2 class="text-h4">Items</h2></div>
      <div class="divide-y divide-stone-100">
        <div v-for="it in items" :key="it.id" class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-ink">{{ it.product_title }}</p>
            <p class="mt-0.5 text-xs text-stone-500">{{ it.variant_label }} · SKU {{ it.sku }} · Qty {{ it.quantity }}</p>
            <p class="mt-0.5 text-xs text-stone-400 no-print">Payable: {{ formatINR(it.seller_payable) }} · Commission: {{ formatINR(it.commission_amount) }}</p>
          </div>
          <span :class="['badge no-print', it.item_status === 'delivered' ? 'badge-green' : it.item_status === 'cancelled' ? 'badge-red' : 'badge-blue']">
            {{ orderStatusLabel(it.item_status) }}
          </span>
        </div>
      </div>
    </div>

    <div class="grid gap-5 sm:grid-cols-2">
      <!-- Deliver to -->
      <div class="card p-4">
        <div class="mb-2 flex items-center gap-2"><MapPin :size="16" class="text-stone-500" /><h2 class="text-h4">Deliver To</h2></div>
        <p class="text-sm font-medium">{{ order.ship_contact_name }}</p>
        <p class="text-sm text-stone-600">{{ order.ship_contact_phone }}</p>
        <p class="mt-0.5 text-sm text-stone-600">{{ order.ship_line1 }}<span v-if="order.ship_line2">, {{ order.ship_line2 }}</span></p>
        <p class="text-sm text-stone-600">{{ order.ship_city }}, {{ order.ship_state }} – {{ order.ship_pincode }}</p>
      </div>

      <!-- Earnings -->
      <div class="card p-4 no-print">
        <div class="mb-2 flex items-center gap-2"><FileText :size="16" class="text-stone-500" /><h2 class="text-h4">Earnings</h2></div>
        <div class="space-y-2 text-sm">
          <div class="flex justify-between"><span class="text-stone-500">Order total</span><span>{{ formatINR(order.grand_total) }}</span></div>
          <div class="flex justify-between border-t border-stone-100 pt-2 font-semibold"><span>Your payable</span><span>{{ formatINR(sellerTotal) }}</span></div>
        </div>
      </div>
    </div>

    <!-- Status timeline -->
    <section v-if="history.length" class="no-print mt-6">
      <h2 class="text-h4 mb-3">Timeline</h2>
      <ol class="relative space-y-4 border-l border-stone-200 pl-4">
        <li v-for="(h, i) in history" :key="i" class="relative">
          <span class="absolute -left-[21px] top-1 flex h-3 w-3 items-center justify-center rounded-full bg-ink" />
          <p class="text-sm font-medium text-ink">{{ orderStatusLabel(h.to_status) }}</p>
          <p v-if="h.note" class="text-xs text-stone-500">{{ h.note }}</p>
          <p class="text-xs text-stone-400">{{ formatDate(h.created_at) }}</p>
        </li>
      </ol>
    </section>

    <!-- ── Cancel modal ── -->
    <transition name="fade">
      <div v-if="showCancel" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="showCancel = false">
        <div class="w-full max-w-sm rounded-t-2xl bg-white p-5 shadow-lg sm:rounded-2xl">
          <div class="mb-3 flex items-start justify-between">
            <h2 class="text-base font-semibold text-ink">Cancel this order?</h2>
            <button class="rounded p-1 text-stone-400 hover:bg-stone-50" @click="showCancel = false"><X :size="18" /></button>
          </div>
          <p class="mb-3 text-sm text-stone-600">The customer will be notified and stock released. Please tell them why.</p>
          <label class="label" for="reason">Reason</label>
          <textarea id="reason" v-model="cancelReason" rows="3" class="textarea" placeholder="e.g. Out of stock, unable to fulfil…" />
          <div class="mt-4 flex gap-2">
            <button class="btn-ghost flex-1" :disabled="busy" @click="showCancel = false">Keep order</button>
            <button class="btn-danger flex-1" :disabled="busy" @click="submitCancel">
              <Loader2 v-if="busy" :size="15" class="animate-spin" /><Ban v-else :size="15" /> Cancel order
            </button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 180ms ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
