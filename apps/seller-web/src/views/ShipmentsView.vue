<script setup lang="ts">
// Seller "Shipments" hub. One place to see what needs shipping, what is on the
// way and what went wrong — with the actions (ship, dispatch, deliver, cancel,
// label) one tap away. Cards on phones, a table on laptops.
import { onMounted, onBeforeUnmount, ref, computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Truck, Package, Search, RefreshCw, ChevronRight, Banknote, CreditCard, AlertTriangle,
  CheckCircle2, Clock, MapPin, Loader2, PackageCheck, X,
} from 'lucide-vue-next'
import type { ShipmentRow, ReadyOrder } from '@carlitos/types'
import { seller } from '@/api'
import {
  formatINR, extractApiError, shipmentStage, shipmentStatusLabel, shipmentBadgeClass,
  formatDateTime, daysAgo,
} from '@carlitos/shared'
import ShipmentDrawer from '@/components/ShipmentDrawer.vue'

type Tab = 'ready' | 'booked' | 'transit' | 'delivered' | 'issues' | 'all'

const shipments = ref<ShipmentRow[]>([])
const ready     = ref<ReadyOrder[]>([])
const loading   = ref(true)
const loadErr   = ref('')
const refreshing = ref(false)
const tab       = ref<Tab>('ready')
const query     = ref('')
const openId    = ref<string | null>(null)
let tabChosen = false
let lastLoad = 0

const flash = ref<{ kind: 'success' | 'error' | 'info'; text: string; id?: string } | null>(null)
let flashTimer: ReturnType<typeof setTimeout> | undefined
function say(kind: 'success' | 'error' | 'info', text: string, id?: string) {
  flash.value = { kind, text, id }
  clearTimeout(flashTimer)
  flashTimer = setTimeout(() => { flash.value = null }, kind === 'error' ? 10000 : 6000)
}

async function load(silent = false) {
  if (!silent) loading.value = true
  else refreshing.value = true
  loadErr.value = ''
  try {
    const [s, r] = await Promise.all([seller.shipments(), seller.readyToShip()])
    shipments.value = s
    ready.value = r
    lastLoad = Date.now()
    // Choose a sensible starting tab once: whatever needs the seller first.
    if (!tabChosen) {
      tab.value = r.length ? 'ready'
        : counts.value.booked ? 'booked'
        : counts.value.transit ? 'transit'
        : 'all'
    }
    // Forget selections for orders that are no longer ready.
    const still = new Set(r.map((o) => o.order_id))
    selected.value = new Set([...selected.value].filter((id) => still.has(id)))
  } catch (e) {
    loadErr.value = extractApiError(e)
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

function pickTab(t: Tab) { tabChosen = true; tab.value = t }

/* ── counters ────────────────────────────────────────────────── */
const counts = computed(() => {
  const c = { ready: ready.value.length, booked: 0, transit: 0, delivered: 0, issues: 0, all: shipments.value.length }
  for (const s of shipments.value) {
    const st = shipmentStage(s.status)
    if (st === 'booked') c.booked++
    else if (st === 'transit') c.transit++
    else if (st === 'delivered') c.delivered++
    if (st === 'issue' || (s.attention && st !== 'delivered' && st !== 'cancelled')) c.issues++
  }
  return c
})

const tabs = computed(() => [
  { v: 'ready' as Tab,     l: 'Ready to ship', n: counts.value.ready,     hot: counts.value.ready > 0 },
  { v: 'booked' as Tab,    l: 'Awaiting pickup', n: counts.value.booked },
  { v: 'transit' as Tab,   l: 'In transit',    n: counts.value.transit },
  { v: 'delivered' as Tab, l: 'Delivered',     n: counts.value.delivered },
  { v: 'issues' as Tab,    l: 'Issues',        n: counts.value.issues,    bad: counts.value.issues > 0 },
  { v: 'all' as Tab,       l: 'All',           n: counts.value.all },
])

/* ── filtering ───────────────────────────────────────────────── */
const q = computed(() => query.value.trim().toLowerCase())

const visibleReady = computed(() => {
  if (!q.value) return ready.value
  return ready.value.filter((o) =>
    [o.order_number, o.customer_name, o.ship_city, o.ship_state, o.ship_pincode, o.item_title]
      .some((v) => (v || '').toLowerCase().includes(q.value)))
})

const visibleShipments = computed(() => {
  let list = shipments.value
  switch (tab.value) {
    case 'booked':    list = list.filter((s) => shipmentStage(s.status) === 'booked'); break
    case 'transit':   list = list.filter((s) => shipmentStage(s.status) === 'transit'); break
    case 'delivered': list = list.filter((s) => shipmentStage(s.status) === 'delivered'); break
    case 'issues':    list = list.filter((s) => shipmentStage(s.status) === 'issue' ||
                        (!!s.attention && !['delivered', 'cancelled'].includes(s.status))); break
  }
  if (!q.value) return list
  return list.filter((s) =>
    [s.order_number, s.customer_name, s.awb_number, s.ship_city, s.courier_name, s.item_title]
      .some((v) => (v || '').toLowerCase().includes(q.value)))
})

/* ── shipping from the "Ready" queue ─────────────────────────── */
const busy = ref('')
const selected = ref<Set<string>>(new Set())
const bulkRunning = ref(false)
const allSelected = computed(() => visibleReady.value.length > 0 && visibleReady.value.every((o) => selected.value.has(o.order_id)))

function toggle(id: string) {
  const next = new Set(selected.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selected.value = next
}
function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(visibleReady.value.map((o) => o.order_id))
}

async function shipCourier(o: ReadyOrder) {
  busy.value = o.order_id
  try {
    await seller.createShipment(o.order_id)
    say('success', `${o.order_number} booked. The courier will collect it and the customer has been notified.`)
    await load(true)
  } catch (e) { say('error', `${o.order_number}: ${extractApiError(e)}`) } finally { busy.value = '' }
}
async function shipSelf(o: ReadyOrder) {
  busy.value = o.order_id
  try {
    const res = await seller.selfShip(o.order_id) as { id?: string }
    say('success', `${o.order_number} is ready for self-delivery. Print the label, pack it and deliver it.`)
    await load(true)
    if (res?.id) openId.value = res.id
  } catch (e) { say('error', `${o.order_number}: ${extractApiError(e)}`) } finally { busy.value = '' }
}

async function shipSelected() {
  const orders = ready.value.filter((o) => selected.value.has(o.order_id))
  if (!orders.length || bulkRunning.value) return
  bulkRunning.value = true
  let ok = 0
  const failed: string[] = []
  for (const o of orders) {
    busy.value = o.order_id
    try { await seller.createShipment(o.order_id); ok++ }
    catch (e) { failed.push(`${o.order_number} (${extractApiError(e)})`) }
  }
  busy.value = ''
  bulkRunning.value = false
  selected.value = new Set()
  await load(true)
  if (failed.length) say('error', `Booked ${ok} of ${orders.length}. Couldn't book: ${failed.join('; ')}`)
  else say('success', `Booked ${ok} shipment${ok === 1 ? '' : 's'} with the courier. Customers have been notified.`)
}

/* ── helpers ─────────────────────────────────────────────────── */
function more(title: string, count: number) {
  return count > 1 ? `${title} +${count - 1} more` : title
}
function openShipment(id: string) { openId.value = id }
function onChanged() { load(true) }

// Keep the page fresh when the seller comes back to the tab.
function onVisible() {
  if (document.visibilityState === 'visible' && Date.now() - lastLoad > 30_000) load(true)
}
onMounted(() => { load(); document.addEventListener('visibilitychange', onVisible) })
onBeforeUnmount(() => { document.removeEventListener('visibilitychange', onVisible); clearTimeout(flashTimer) })
watch(tab, () => { selected.value = new Set() })
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Shipments</h1>
        <p class="mt-0.5 text-sm text-stone-500">Ship packed orders, follow parcels and close out deliveries.</p>
      </div>
      <button class="btn-ghost btn-sm" :disabled="loading || refreshing" @click="load(true)">
        <RefreshCw :size="14" :class="{ 'animate-spin': refreshing }" /> Refresh
      </button>
    </div>

    <!-- Message -->
    <div v-if="flash" role="status"
      :class="['mb-4 flex items-start justify-between gap-3 rounded-lg border p-3 text-sm', flash.kind === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : flash.kind === 'error' ? 'border-red-200 bg-red-50 text-red-700' : 'border-stone-200 bg-stone-50 text-stone-700']">
      <span class="min-w-0 break-words">{{ flash.text }}</span>
      <button class="shrink-0 opacity-60 hover:opacity-100" aria-label="Dismiss" @click="flash = null"><X :size="15" /></button>
    </div>

    <!-- Summary -->
    <div class="mb-4 grid grid-cols-2 gap-2 md:grid-cols-4 md:gap-3">
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickTab('ready')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Ready to ship</p><PackageCheck :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-8" />
        <p v-else :class="['mt-0.5 text-xl font-semibold', counts.ready ? 'text-amber-600' : 'text-ink']">{{ counts.ready }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickTab('booked')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Awaiting pickup</p><Clock :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-8" />
        <p v-else class="mt-0.5 text-xl font-semibold text-ink">{{ counts.booked }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickTab('transit')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">In transit</p><Truck :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-8" />
        <p v-else class="mt-0.5 text-xl font-semibold text-ink">{{ counts.transit }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickTab('issues')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Needs attention</p><AlertTriangle :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-8" />
        <p v-else :class="['mt-0.5 text-xl font-semibold', counts.issues ? 'text-red-600' : 'text-ink']">{{ counts.issues }}</p>
      </button>
    </div>

    <!-- Tabs -->
    <div class="mb-3 flex gap-2 overflow-x-auto pb-1" role="tablist" aria-label="Shipment stages">
      <button v-for="t in tabs" :key="t.v" role="tab" :aria-selected="tab === t.v"
        :class="['flex shrink-0 items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm transition-colors',
          tab === t.v ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="pickTab(t.v)">
        {{ t.l }}
        <span :class="['rounded-full px-1.5 text-[11px] font-medium leading-5',
          tab === t.v ? 'bg-white/20 text-white' : t.hot ? 'bg-amber-100 text-amber-700' : t.bad ? 'bg-red-100 text-red-700' : 'bg-stone-100 text-stone-600']">{{ t.n }}</span>
      </button>
    </div>

    <!-- Search -->
    <div class="relative mb-4">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
      <input v-model="query" class="input pl-9" placeholder="Search order #, customer, AWB or city…" aria-label="Search shipments" />
    </div>

    <div v-if="loadErr" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
      {{ loadErr }} <button class="ml-2 underline" @click="load()">Try again</button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2.5"><div v-for="i in 5" :key="i" class="skeleton h-24 w-full rounded-lg" /></div>

    <!-- ═══ READY TO SHIP ═══ -->
    <template v-else-if="tab === 'ready'">
      <div v-if="!visibleReady.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-12 text-center">
        <PackageCheck :size="40" class="mb-3 text-stone-300" />
        <p class="text-sm font-medium text-stone-700">{{ query ? 'No orders match your search' : 'Nothing is waiting to be shipped' }}</p>
        <p class="mt-1 max-w-sm text-xs text-stone-500">
          {{ query ? 'Try a different order number or customer.' : 'When you mark an order as packed it appears here, ready to ship in one tap.' }}
        </p>
        <RouterLink v-if="!query" to="/orders" class="btn-ghost btn-sm mt-4">Go to orders</RouterLink>
      </div>

      <template v-else>
        <!-- Bulk bar -->
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-stone-200 bg-white px-3 py-2">
          <label class="flex cursor-pointer items-center gap-2 text-sm text-stone-600">
            <input type="checkbox" class="h-4 w-4 accent-ink" :checked="allSelected" @change="toggleAll" />
            {{ selected.size ? `${selected.size} selected` : `Select all (${visibleReady.length})` }}
          </label>
          <button v-if="selected.size" class="btn-primary btn-sm" :disabled="bulkRunning" @click="shipSelected">
            <Loader2 v-if="bulkRunning" :size="14" class="animate-spin" /><Truck v-else :size="14" />
            {{ bulkRunning ? 'Booking…' : `Ship ${selected.size} via courier` }}
          </button>
          <span v-else class="hidden text-xs text-stone-400 md:inline">Tick orders to book them all with the courier at once.</span>
        </div>

        <div class="space-y-2.5">
          <article v-for="o in visibleReady" :key="o.order_id"
            :class="['rounded-lg border bg-white p-3.5 transition-colors', selected.has(o.order_id) ? 'border-ink' : 'border-stone-200']">
            <div class="flex items-start gap-3">
              <input type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-ink" :checked="selected.has(o.order_id)"
                :aria-label="`Select ${o.order_number}`" @change="toggle(o.order_id)" />
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                  <RouterLink :to="`/orders/${o.order_id}`" class="font-semibold text-ink hover:underline">{{ o.order_number }}</RouterLink>
                  <span v-if="o.payment_method === 'cod'" class="badge badge-amber"><Banknote :size="11" class="mr-1" />COD {{ formatINR(o.cod_amount) }}</span>
                  <span v-else class="badge badge-gray"><CreditCard :size="11" class="mr-1" />Prepaid</span>
                </div>
                <p class="mt-0.5 text-sm text-stone-700">{{ o.customer_name }}</p>
                <p class="mt-0.5 flex items-center gap-1 text-xs text-stone-500">
                  <MapPin :size="12" class="shrink-0" />{{ o.ship_city }}, {{ o.ship_state }} – {{ o.ship_pincode }}
                </p>
                <p class="mt-1.5 text-xs text-stone-500">
                  {{ more(o.item_title, o.item_count) }} · {{ o.item_count }} item{{ o.item_count === 1 ? '' : 's' }} ·
                  placed {{ daysAgo(o.placed_at) }}
                </p>
              </div>
            </div>
            <div class="mt-3 flex flex-wrap gap-2 border-t border-stone-100 pt-3 md:pl-7">
              <button class="btn-primary btn-sm flex-1 md:flex-none" :disabled="busy === o.order_id || bulkRunning" @click="shipCourier(o)">
                <Loader2 v-if="busy === o.order_id" :size="13" class="animate-spin" /><Truck v-else :size="13" /> Ship via courier
              </button>
              <button class="btn-ghost btn-sm flex-1 md:flex-none" :disabled="busy === o.order_id || bulkRunning" @click="shipSelf(o)">
                <Package :size="13" /> Self-ship
              </button>
              <RouterLink :to="`/orders/${o.order_id}`" class="btn-ghost btn-sm hidden md:inline-flex">View order</RouterLink>
            </div>
          </article>
        </div>
        <p class="mt-3 text-xs text-stone-400">Courier: an AWB is assigned and pickup is requested; the courier charge is deducted from your payout. Self-ship: you print a label and deliver the parcel yourself.</p>
      </template>
    </template>

    <!-- ═══ SHIPMENTS (other tabs) ═══ -->
    <template v-else>
      <div v-if="!visibleShipments.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-12 text-center">
        <component :is="tab === 'delivered' ? CheckCircle2 : tab === 'issues' ? AlertTriangle : Truck" :size="40" class="mb-3 text-stone-300" />
        <p class="text-sm font-medium text-stone-700">{{ query ? 'No shipments match your search' : tab === 'issues' ? 'No problems — all good' : 'No shipments here' }}</p>
        <p class="mt-1 max-w-sm text-xs text-stone-500">
          {{ query ? 'Try a different order number, AWB or customer.' : 'Shipments you book appear here with live status.' }}
        </p>
      </div>

      <template v-else>
        <!-- Desktop table -->
        <div class="table-wrap hidden md:block">
          <table class="table">
            <thead>
              <tr><th>Order</th><th>Customer</th><th>Shipping</th><th>Status</th><th>Payment</th><th class="text-right" /></tr>
            </thead>
            <tbody>
              <tr v-for="s in visibleShipments" :key="s.id" class="cursor-pointer" tabindex="0"
                @click="openShipment(s.id)" @keydown.enter="openShipment(s.id)">
                <td>
                  <p class="font-medium text-ink">{{ s.order_number }}</p>
                  <p class="text-xs text-stone-500">{{ formatDateTime(s.created_at) }}</p>
                </td>
                <td>
                  <p class="text-ink">{{ s.customer_name }}</p>
                  <p class="text-xs text-stone-500">{{ s.ship_city }}, {{ s.ship_state }}</p>
                </td>
                <td>
                  <p class="flex items-center gap-1.5 text-ink"><component :is="s.ship_type === 'self' ? Package : Truck" :size="13" class="text-stone-400" />{{ s.ship_type === 'self' ? 'Self-ship' : (s.courier_name || 'Courier') }}</p>
                  <p v-if="s.awb_number" class="text-xs text-stone-500">{{ s.awb_number }}</p>
                </td>
                <td>
                  <span :class="['badge', shipmentBadgeClass(s.status)]">{{ shipmentStatusLabel(s.status, s.ship_type) }}</span>
                  <p v-if="s.attention && !['delivered','cancelled'].includes(s.status)" class="mt-1 flex items-center gap-1 text-[11px] text-amber-700"><AlertTriangle :size="11" />{{ s.attention }}</p>
                </td>
                <td class="text-sm">
                  <span v-if="s.payment_method === 'cod'" class="text-amber-700">COD {{ formatINR(s.cod_amount) }}</span>
                  <span v-else class="text-stone-500">Prepaid</span>
                </td>
                <td class="text-right"><ChevronRight :size="16" class="ml-auto text-stone-300" /></td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Mobile cards -->
        <div class="space-y-2.5 md:hidden">
          <button v-for="s in visibleShipments" :key="s.id" type="button"
            class="block w-full rounded-lg border border-stone-200 bg-white p-3.5 text-left transition-shadow hover:shadow-sm" @click="openShipment(s.id)">
            <div class="flex items-center justify-between gap-2">
              <p class="truncate font-semibold text-ink">{{ s.order_number }}</p>
              <span :class="['badge shrink-0', shipmentBadgeClass(s.status)]">{{ shipmentStatusLabel(s.status, s.ship_type) }}</span>
            </div>
            <p class="mt-1 text-sm text-stone-700">{{ s.customer_name }} <span class="text-stone-400">·</span> <span class="text-stone-500">{{ s.ship_city }}</span></p>
            <p v-if="s.attention && !['delivered','cancelled'].includes(s.status)" class="mt-1 flex items-center gap-1 text-xs text-amber-700"><AlertTriangle :size="12" />{{ s.attention }}</p>
            <div class="mt-2.5 flex items-center justify-between gap-2 border-t border-stone-100 pt-2.5 text-xs text-stone-500">
              <span class="flex min-w-0 items-center gap-1.5">
                <component :is="s.ship_type === 'self' ? Package : Truck" :size="13" class="shrink-0" />
                <span class="truncate">{{ s.ship_type === 'self' ? 'Self-ship' : (s.courier_name || 'Courier') }}<template v-if="s.awb_number"> · {{ s.awb_number }}</template></span>
              </span>
              <span class="flex shrink-0 items-center gap-1">
                <span v-if="s.payment_method === 'cod'" class="font-medium text-amber-700">COD {{ formatINR(s.cod_amount) }}</span>
                <span v-else>Prepaid</span>
                <ChevronRight :size="15" class="text-stone-300" />
              </span>
            </div>
          </button>
        </div>
      </template>
    </template>

    <ShipmentDrawer :id="openId" @close="openId = null" @changed="onChanged" />
  </div>
</template>
