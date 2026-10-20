<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft, MapPin, CreditCard } from 'lucide-vue-next'
import type { Order } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, formatDate, orderStatusLabel, extractApiError } from '@carlitos/shared'

const route   = useRoute()
const router  = useRouter()
const order   = ref<Order | null>(null)
const loading = ref(true)
const busy    = ref(false)
const message = ref('')
const error   = ref('')
const newStatus = ref('')

const STATUS_OPTIONS = [
  'confirmed','processing','packed','ready_to_ship','shipped',
  'in_transit','out_for_delivery','delivered','cancelled','refunded',
]

async function load() {
  order.value = await admin.order(route.params.id as string).catch(() => null)
  newStatus.value = order.value?.status || ''
}

async function updateStatus() {
  if (!newStatus.value || newStatus.value === order.value?.status) return
  busy.value = true; error.value = ''; message.value = ''
  try {
    await admin.setOrderStatus(order.value!.id, newStatus.value)
    message.value = 'Order status updated.'
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = false }
}

const o = computed(() => order.value)
function statusClass(s?: string) {
  if (['delivered','refunded'].includes(s || '')) return 'badge-green'
  if (['cancelled'].includes(s || '')) return 'badge-red'
  return 'badge-blue'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="skeleton h-8 w-1/3 rounded" />
    <div class="skeleton h-40 w-full rounded-xl" />
  </div>

  <div v-else-if="!o" class="py-12 text-center">
    <p class="text-stone-600">Order not found.</p>
    <button class="btn-ghost btn-sm mt-4" @click="router.push('/orders')">← All orders</button>
  </div>

  <div v-else class="max-w-4xl">
    <button class="mb-5 flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink" @click="router.push('/orders')">
      <ChevronLeft :size="16" /> All orders
    </button>

    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <div class="flex items-center gap-2">
          <h1 class="page-title">{{ o.order_number }}</h1>
          <span :class="['badge', statusClass(o.status)]">{{ orderStatusLabel(o.status) }}</span>
        </div>
        <p class="mt-0.5 text-sm text-stone-500">
          {{ formatDate(o.placed_at || o.created_at) }} · {{ o.payment_method.toUpperCase() }} · {{ o.payment_status }}
        </p>
      </div>

      <!-- Status update -->
      <div class="flex items-center gap-2">
        <select v-model="newStatus" class="select w-44">
          <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">{{ orderStatusLabel(s) }}</option>
        </select>
        <button class="btn-primary btn-sm" :disabled="busy || newStatus === o.status" @click="updateStatus">Update</button>
      </div>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Items -->
    <div class="card mb-5 overflow-hidden">
      <div class="border-b border-stone-100 px-5 py-4"><h2 class="text-h4">Items ({{ o.items?.length || 0 }})</h2></div>
      <div class="divide-y divide-stone-100">
        <div v-for="it in o.items" :key="it.id" class="flex items-center justify-between px-5 py-4">
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate">{{ it.product_title }}</p>
            <p class="mt-0.5 text-xs text-stone-500">{{ it.seller_name || 'Seller' }} · Qty {{ it.quantity }} · {{ orderStatusLabel(it.item_status) }}</p>
            <p class="mt-0.5 text-xs text-stone-400">Commission {{ formatINR(it.commission_amount) }} · Payable {{ formatINR(it.seller_payable) }}</p>
          </div>
          <p class="ml-4 shrink-0 text-sm font-semibold">{{ formatINR(it.gross_amount) }}</p>
        </div>
      </div>
    </div>

    <div class="grid gap-5 sm:grid-cols-2">
      <!-- Price -->
      <div class="card p-5">
        <div class="flex items-center gap-2 mb-3"><CreditCard :size="16" class="text-stone-500" /><h2 class="text-h4">Payment Summary</h2></div>
        <div class="space-y-2 text-sm">
          <div class="flex justify-between"><span class="text-stone-500">Items</span><span>{{ formatINR(o.items_subtotal) }}</span></div>
          <div v-if="o.discount_total>0" class="flex justify-between"><span class="text-stone-500">Discount</span><span class="text-emerald-600">-{{ formatINR(o.discount_total) }}</span></div>
          <div class="flex justify-between"><span class="text-stone-500">Tax</span><span>{{ formatINR(o.tax_total) }}</span></div>
          <div class="flex justify-between"><span class="text-stone-500">Shipping</span><span>{{ formatINR(o.shipping_total) }}</span></div>
          <div class="flex justify-between border-t border-stone-100 pt-2 font-semibold text-base"><span>Grand Total</span><span>{{ formatINR(o.grand_total) }}</span></div>
        </div>
      </div>

      <!-- Address -->
      <div class="card p-5">
        <div class="flex items-center gap-2 mb-3"><MapPin :size="16" class="text-stone-500" /><h2 class="text-h4">Shipping Address</h2></div>
        <p class="text-sm font-medium">{{ o.ship_contact_name }}</p>
        <p class="text-sm text-stone-600">{{ o.ship_contact_phone }}</p>
        <p class="text-sm text-stone-600 mt-0.5">{{ o.ship_line1 }}<span v-if="o.ship_line2">, {{ o.ship_line2 }}</span></p>
        <p class="text-sm text-stone-600">{{ o.ship_city }}, {{ o.ship_state }} – {{ o.ship_pincode }}</p>
      </div>
    </div>
  </div>
</template>
