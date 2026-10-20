<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ShoppingCart, Search, SlidersHorizontal, X, ChevronRight,
  IndianRupee, Clock, CreditCard, Banknote,
} from 'lucide-vue-next'
import type { Order } from '@carlitos/types'
import { seller } from '@/api'
import { formatINR, formatDate, orderStatusLabel } from '@carlitos/shared'

const orders   = ref<Order[]>([])
const loading  = ref(true)
const status   = ref('')
const query    = ref('')
const dateFrom = ref('')
const dateTo   = ref('')
const showDates = ref(false)

const tabs = [
  { value: '',              label: 'All' },
  { value: 'confirmed',     label: 'New' },
  { value: 'processing',    label: 'Processing' },
  { value: 'packed',        label: 'Packed' },
  { value: 'ready_to_ship', label: 'Ready' },
  { value: 'shipped',       label: 'Shipped' },
  { value: 'delivered',     label: 'Delivered' },
  { value: 'cancelled',     label: 'Cancelled' },
]

// Statuses that need the seller to act (fulfil the order).
const ACTION_STATUSES = ['confirmed', 'processing', 'packed', 'ready_to_ship']

async function load() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (status.value) params.status = status.value
    if (dateFrom.value) params.from = dateFrom.value
    if (dateTo.value)   params.to   = dateTo.value
    const res    = await seller.orders(params)
    orders.value = (res.data as Order[]) || []
  } finally {
    loading.value = false
  }
}

function clearDates() {
  dateFrom.value = ''
  dateTo.value = ''
  load()
}

// Client-side search on top of the server list (by order number).
const visibleOrders = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return orders.value
  return orders.value.filter(o =>
    o.order_number.toLowerCase().includes(q) ||
    (o.ship_contact_name || '').toLowerCase().includes(q),
  )
})

// Summary of the current (filtered) list.
const totalValue    = computed(() => visibleOrders.value.reduce((a, o) => a + o.grand_total, 0))
const needsAction   = computed(() => visibleOrders.value.filter(o => ACTION_STATUSES.includes(o.status)).length)

function statusClass(s: string) {
  if (['delivered'].includes(s))  return 'badge-green'
  if (['cancelled'].includes(s))  return 'badge-red'
  if (['pending_payment'].includes(s)) return 'badge-gray'
  return 'badge-blue'
}
function itemCount(o: Order) { return o.items?.length || 0 }

onMounted(load)
watch(status, load)
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Orders</h1>
    </div>

    <!-- Summary strip -->
    <div class="mb-4 grid grid-cols-3 gap-2 sm:gap-3">
      <div class="stat !p-3">
        <div class="flex items-center justify-between">
          <p class="text-label-sm truncate">Orders</p>
          <ShoppingCart :size="16" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-10 rounded" />
        <p v-else class="mt-0.5 text-lg font-semibold text-ink sm:text-xl">{{ visibleOrders.length }}</p>
      </div>
      <div class="stat !p-3">
        <div class="flex items-center justify-between">
          <p class="text-label-sm truncate">Value</p>
          <IndianRupee :size="16" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-16 rounded" />
        <p v-else class="mt-0.5 text-lg font-semibold text-ink sm:text-xl">{{ formatINR(totalValue) }}</p>
      </div>
      <div class="stat !p-3">
        <div class="flex items-center justify-between">
          <p class="text-label-sm truncate">To fulfil</p>
          <Clock :size="16" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton mt-1 h-6 w-10 rounded" />
        <p v-else :class="['mt-0.5 text-lg font-semibold sm:text-xl', needsAction ? 'text-amber-600' : 'text-ink']">{{ needsAction }}</p>
      </div>
    </div>

    <!-- Search + date filter toggle -->
    <div class="mb-3 flex items-center gap-2">
      <div class="relative min-w-0 flex-1">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
        <input v-model="query" class="input pl-9" placeholder="Search order # or customer…" aria-label="Search orders" />
      </div>
      <button
        type="button"
        class="btn-ghost btn-sm shrink-0"
        :class="showDates ? 'border-ink text-ink' : ''"
        @click="showDates = !showDates"
      >
        <SlidersHorizontal :size="15" /> <span class="hidden xs:inline">Dates</span>
        <span v-if="dateFrom || dateTo" class="ml-1 h-1.5 w-1.5 rounded-full bg-ink" />
      </button>
    </div>

    <!-- Date range (collapsible) -->
    <transition name="dates">
      <div v-if="showDates" class="mb-3 rounded-lg border border-stone-200 bg-stone-50 p-3">
        <div class="flex flex-wrap items-end gap-3">
          <div class="min-w-0 flex-1 xs:flex-none">
            <label class="label text-xs" for="from">From date</label>
            <input id="from" v-model="dateFrom" type="date" class="input py-1.5" />
          </div>
          <div class="min-w-0 flex-1 xs:flex-none">
            <label class="label text-xs" for="to">To date</label>
            <input id="to" v-model="dateTo" type="date" class="input py-1.5" />
          </div>
          <button class="btn-primary btn-sm" @click="load">Apply</button>
          <button v-if="dateFrom || dateTo" class="btn-ghost btn-sm" @click="clearDates">
            <X :size="13" /> Clear
          </button>
        </div>
      </div>
    </transition>

    <!-- Status tabs -->
    <div class="mb-4 flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="t in tabs"
        :key="t.value || 'all'"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', status === t.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="status = t.value"
      >{{ t.label }}</button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-20 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!visibleOrders.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <ShoppingCart :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No orders here</p>
      <p class="mt-1 text-xs text-stone-500">
        {{ query ? 'No orders match your search.' : 'Orders matching this filter will appear here.' }}
      </p>
    </div>

    <!-- Order cards (fully responsive — no cramped table columns) -->
    <div v-else class="space-y-2.5">
      <RouterLink
        v-for="o in visibleOrders" :key="o.id"
        :to="`/orders/${o.id}`"
        class="block rounded-lg border border-stone-200 bg-white p-3.5 transition-shadow hover:shadow-sm"
      >
        <!-- Top row: order # + status -->
        <div class="flex items-center justify-between gap-2">
          <p class="truncate font-semibold text-ink">{{ o.order_number }}</p>
          <span :class="['badge shrink-0', statusClass(o.status)]">{{ orderStatusLabel(o.status) }}</span>
        </div>

        <!-- Meta row: customer + date -->
        <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-stone-500">
          <span class="truncate">{{ o.ship_contact_name || 'Customer' }}</span>
          <span>·</span>
          <span class="whitespace-nowrap">{{ formatDate(o.placed_at || o.created_at) }}</span>
        </div>

        <!-- Bottom row: items, payment, amount, arrow -->
        <div class="mt-2.5 flex items-center justify-between gap-2 border-t border-stone-100 pt-2.5">
          <div class="flex items-center gap-3 text-xs text-stone-500">
            <span class="whitespace-nowrap">{{ itemCount(o) }} item{{ itemCount(o) === 1 ? '' : 's' }}</span>
            <span class="inline-flex items-center gap-1 whitespace-nowrap">
              <component :is="o.payment_method === 'cod' ? Banknote : CreditCard" :size="13" />
              {{ o.payment_method === 'cod' ? 'COD' : 'Prepaid' }}
            </span>
          </div>
          <div class="flex items-center gap-1">
            <span class="font-semibold text-ink">{{ formatINR(o.grand_total) }}</span>
            <ChevronRight :size="16" class="text-stone-300" />
          </div>
        </div>
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.dates-enter-active, .dates-leave-active { transition: opacity 160ms ease, transform 160ms ease; }
.dates-enter-from, .dates-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
