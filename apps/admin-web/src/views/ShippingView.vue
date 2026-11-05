<script setup lang="ts">
// Admin "Shipping": monitor every shipment across all sellers, spot the ones that
// are stuck or failed, and open any of them for the full picture. Cards on
// phones, a table on laptops.
import { onMounted, onBeforeUnmount, ref, computed, watch } from 'vue'
import {
  Truck, Package, Search, RefreshCw, ChevronRight, Clock, CheckCircle2, AlertTriangle,
  Store, XCircle, Layers,
} from 'lucide-vue-next'
import type { ShipmentRow, ShipmentSummary } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, extractApiError, shipmentStatusLabel, shipmentBadgeClass, formatDateTime } from '@carlitos/shared'
import ShippingDrawer from '@/components/ShippingDrawer.vue'

const PAGE_SIZE = 25

const rows     = ref<ShipmentRow[]>([])
const total    = ref(0)
const summary  = ref<ShipmentSummary | null>(null)
const loading  = ref(true)
const refreshing = ref(false)
const error    = ref('')
const page     = ref(1)
const status   = ref('')            // '' | pending | in_transit | delivered | problem | cancelled
const shipType = ref('')            // '' | courier | self
const attention = ref(false)
const query    = ref('')
const openId   = ref<string | null>(null)
let seq = 0

const pages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

async function load(silent = false) {
  const mine = ++seq
  if (silent) refreshing.value = true; else loading.value = true
  error.value = ''
  try {
    const params: Record<string, unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (status.value)        params.status = status.value
    if (shipType.value)      params.type = shipType.value
    if (attention.value)     params.attention = 1
    if (query.value.trim())  params.q = query.value.trim()
    const [res, sm] = await Promise.all([admin.shipments(params), admin.shipmentsSummary().catch(() => null)])
    if (mine !== seq) return // a newer request superseded this one
    rows.value = (res.data as ShipmentRow[]) || []
    total.value = res.meta?.total || 0
    if (sm) summary.value = sm
  } catch (e) {
    if (mine === seq) error.value = extractApiError(e)
  } finally {
    if (mine === seq) { loading.value = false; refreshing.value = false }
  }
}

const chips = computed(() => [
  { v: '',          l: 'All',             n: summary.value?.total },
  { v: 'pending',   l: 'Awaiting pickup', n: summary.value?.pending },
  { v: 'in_transit',l: 'In transit',      n: summary.value?.in_transit },
  { v: 'delivered', l: 'Delivered',       n: summary.value?.delivered },
  { v: 'problem',   l: 'Problems',        n: summary.value?.problem, bad: !!summary.value?.problem },
  { v: 'cancelled', l: 'Cancelled',       n: summary.value?.cancelled },
])

function pickStatus(v: string) { status.value = v; attention.value = false }
function toggleAttention() { attention.value = !attention.value; if (attention.value) status.value = '' }
function clearFilters() { status.value = ''; shipType.value = ''; attention.value = false; query.value = '' }
const hasFilters = computed(() => !!(status.value || shipType.value || attention.value || query.value.trim()))

let t: ReturnType<typeof setTimeout> | undefined
watch(query, () => { clearTimeout(t); t = setTimeout(() => { page.value = 1; load() }, 350) })
watch([status, shipType, attention], () => { page.value = 1; load() })
function go(p: number) { page.value = Math.min(Math.max(1, p), pages.value); load() }

function onVisible() { if (document.visibilityState === 'visible') load(true) }
onMounted(() => { load(); document.addEventListener('visibilitychange', onVisible) })
onBeforeUnmount(() => { clearTimeout(t); document.removeEventListener('visibilitychange', onVisible) })
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Shipping</h1>
        <p class="mt-0.5 text-sm text-stone-500">Every shipment across all sellers, in one place.</p>
      </div>
      <button class="btn-ghost btn-sm" :disabled="loading || refreshing" @click="load(true)">
        <RefreshCw :size="14" :class="{ 'animate-spin': refreshing }" /> Refresh
      </button>
    </div>

    <!-- Summary -->
    <div class="mb-4 grid grid-cols-2 gap-2 md:grid-cols-4 md:gap-3 xl:grid-cols-5">
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickStatus('pending')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Awaiting pickup</p><Clock :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="!summary" class="skeleton mt-1 h-6 w-8" /><p v-else class="mt-0.5 text-xl font-semibold text-ink">{{ summary.pending }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickStatus('in_transit')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">In transit</p><Truck :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="!summary" class="skeleton mt-1 h-6 w-8" /><p v-else class="mt-0.5 text-xl font-semibold text-ink">{{ summary.in_transit }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickStatus('delivered')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Delivered</p><CheckCircle2 :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="!summary" class="skeleton mt-1 h-6 w-8" /><p v-else class="mt-0.5 text-xl font-semibold text-emerald-700">{{ summary.delivered }}</p>
      </button>
      <button type="button" class="stat !p-3 text-left transition-colors hover:border-stone-400" @click="pickStatus('problem')">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Problems</p><XCircle :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="!summary" class="skeleton mt-1 h-6 w-8" /><p v-else :class="['mt-0.5 text-xl font-semibold', summary.problem ? 'text-red-600' : 'text-ink']">{{ summary.problem }}</p>
      </button>
      <button type="button" class="stat col-span-2 !p-3 text-left transition-colors hover:border-stone-400 md:col-span-4 xl:col-span-1" @click="toggleAttention">
        <div class="flex items-center justify-between"><p class="text-label-sm truncate">Needs attention</p><AlertTriangle :size="16" class="shrink-0 text-stone-400" /></div>
        <div v-if="!summary" class="skeleton mt-1 h-6 w-8" /><p v-else :class="['mt-0.5 text-xl font-semibold', summary.attention ? 'text-amber-600' : 'text-ink']">{{ summary.attention }}</p>
      </button>
    </div>

    <!-- Filters -->
    <div class="mb-3 flex gap-2 overflow-x-auto pb-1" role="tablist" aria-label="Shipment status">
      <button v-for="c in chips" :key="c.v || 'all'" role="tab" :aria-selected="!attention && status === c.v"
        :class="['flex shrink-0 items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm transition-colors',
          !attention && status === c.v ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="pickStatus(c.v)">
        {{ c.l }}
        <span v-if="c.n !== undefined" :class="['rounded-full px-1.5 text-[11px] font-medium leading-5',
          !attention && status === c.v ? 'bg-white/20 text-white' : c.bad ? 'bg-red-100 text-red-700' : 'bg-stone-100 text-stone-600']">{{ c.n }}</span>
      </button>
      <button :class="['flex shrink-0 items-center gap-1.5 rounded-full border px-3.5 py-1.5 text-sm transition-colors',
          attention ? 'border-amber-500 bg-amber-500 text-white' : 'border-amber-300 text-amber-700 hover:border-amber-400']"
        :aria-pressed="attention" @click="toggleAttention">
        <AlertTriangle :size="13" /> Needs attention
      </button>
    </div>

    <div class="mb-4 flex flex-wrap items-center gap-2">
      <div class="relative min-w-0 flex-1 basis-60">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
        <input v-model="query" class="input pl-9" placeholder="Search order #, AWB, customer or seller…" aria-label="Search shipments" />
      </div>
      <select v-model="shipType" class="select w-full md:w-44" aria-label="Shipping method">
        <option value="">All methods</option>
        <option value="courier">Courier</option>
        <option value="self">Self-ship</option>
      </select>
      <button v-if="hasFilters" class="btn-ghost btn-sm" @click="clearFilters">Clear filters</button>
    </div>

    <div v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700" role="alert">
      {{ error }} <button class="ml-2 underline" @click="load()">Try again</button>
    </div>

    <div v-if="loading" class="space-y-2.5"><div v-for="i in 6" :key="i" class="skeleton h-16 w-full rounded-lg" /></div>

    <div v-else-if="!rows.length && !error" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-14 text-center">
      <Layers :size="40" class="mb-3 text-stone-300" />
      <p class="text-sm font-medium text-stone-700">{{ hasFilters ? 'No shipments match these filters' : 'No shipments yet' }}</p>
      <p class="mt-1 max-w-sm text-xs text-stone-500">{{ hasFilters ? 'Try removing a filter or searching for something else.' : 'Shipments appear here as soon as sellers book them.' }}</p>
      <button v-if="hasFilters" class="btn-ghost btn-sm mt-4" @click="clearFilters">Clear filters</button>
    </div>

    <template v-else-if="rows.length">
      <!-- Desktop table -->
      <div class="table-wrap hidden rounded-lg border border-stone-200 md:block">
        <table class="table">
          <thead>
            <tr><th>Order</th><th>Seller</th><th>Customer</th><th>Shipping</th><th>Status</th><th class="hidden xl:table-cell">Payment</th><th class="text-right" /></tr>
          </thead>
          <tbody>
            <tr v-for="s in rows" :key="s.id" tabindex="0" @click="openId = s.id" @keydown.enter="openId = s.id">
              <td>
                <p class="font-medium text-ink">{{ s.order_number }}</p>
                <p class="text-xs text-stone-500">{{ formatDateTime(s.created_at) }}</p>
              </td>
              <td><p class="flex items-center gap-1.5 text-ink"><Store :size="13" class="shrink-0 text-stone-400" /><span class="truncate">{{ s.store_name || '—' }}</span></p></td>
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
              <td class="hidden text-sm xl:table-cell">
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
        <button v-for="s in rows" :key="s.id" type="button"
          class="block w-full rounded-lg border border-stone-200 bg-white p-3.5 text-left transition-shadow hover:shadow-sm" @click="openId = s.id">
          <div class="flex items-center justify-between gap-2">
            <p class="truncate font-semibold text-ink">{{ s.order_number }}</p>
            <span :class="['badge shrink-0', shipmentBadgeClass(s.status)]">{{ shipmentStatusLabel(s.status, s.ship_type) }}</span>
          </div>
          <p class="mt-1 flex items-center gap-1.5 text-xs text-stone-500"><Store :size="12" class="shrink-0" /><span class="truncate">{{ s.store_name || '—' }}</span></p>
          <p class="mt-0.5 text-sm text-stone-700">{{ s.customer_name }} <span class="text-stone-400">·</span> <span class="text-stone-500">{{ s.ship_city }}</span></p>
          <p v-if="s.attention && !['delivered','cancelled'].includes(s.status)" class="mt-1 flex items-center gap-1 text-xs text-amber-700"><AlertTriangle :size="12" />{{ s.attention }}</p>
          <div class="mt-2.5 flex items-center justify-between gap-2 border-t border-stone-100 pt-2.5 text-xs text-stone-500">
            <span class="flex min-w-0 items-center gap-1.5">
              <component :is="s.ship_type === 'self' ? Package : Truck" :size="13" class="shrink-0" />
              <span class="truncate">{{ s.ship_type === 'self' ? 'Self-ship' : (s.courier_name || 'Courier') }}<template v-if="s.awb_number"> · {{ s.awb_number }}</template></span>
            </span>
            <span class="flex shrink-0 items-center gap-1">
              <span v-if="s.payment_method === 'cod'" class="font-medium text-amber-700">COD {{ formatINR(s.cod_amount) }}</span><span v-else>Prepaid</span>
              <ChevronRight :size="15" class="text-stone-300" />
            </span>
          </div>
        </button>
      </div>

      <!-- Pagination -->
      <div v-if="pages > 1" class="mt-5 flex items-center justify-center gap-2">
        <button class="btn-ghost btn-sm" :disabled="page <= 1" @click="go(page - 1)">← Prev</button>
        <span class="px-2 text-sm text-stone-500">Page {{ page }} of {{ pages }} <span class="hidden sm:inline">· {{ total }} shipments</span></span>
        <button class="btn-ghost btn-sm" :disabled="page >= pages" @click="go(page + 1)">Next →</button>
      </div>
    </template>

    <ShippingDrawer :id="openId" @close="openId = null" @changed="load(true)" />
  </div>
</template>
