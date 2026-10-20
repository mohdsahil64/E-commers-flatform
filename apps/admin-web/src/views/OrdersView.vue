<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ShoppingCart, Search } from 'lucide-vue-next'
import type { Order } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, formatDate, orderStatusLabel } from '@carlitos/shared'

const router  = useRouter()
const orders  = ref<Order[]>([])
const total   = ref(0)
const loading = ref(true)
const status  = ref('')
const query   = ref('')
const page    = ref(1)
const PAGE_SIZE = 25

const tabs = [
  { value: '',          label: 'All' },
  { value: 'confirmed', label: 'Confirmed' },
  { value: 'processing',label: 'Processing' },
  { value: 'shipped',   label: 'Shipped' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'refunded',  label: 'Refunded' },
]

async function load() {
  loading.value = true
  try {
    const params: Record<string,unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (status.value) params.status = status.value
    if (query.value)  params.q      = query.value
    const res     = await admin.orders(params)
    orders.value  = (res.data as Order[]) || []
    total.value   = res.meta?.total || 0
  } finally { loading.value = false }
}

function statusClass(s: string) {
  if (['delivered','refunded'].includes(s)) return 'badge-green'
  if (['cancelled'].includes(s)) return 'badge-red'
  if (['pending_payment'].includes(s)) return 'badge-gray'
  return 'badge-blue'
}

let t: ReturnType<typeof setTimeout>
function onSearch() { clearTimeout(t); t = setTimeout(() => { page.value = 1; load() }, 400) }

onMounted(load)
watch(status, () => { page.value = 1; load() })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Orders</h1>
    </div>

    <div class="mb-4 flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="tb in tabs"
        :key="tb.value || 'all'"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', status === tb.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="status = tb.value"
      >{{ tb.label }}</button>
    </div>

    <div class="mb-5 relative max-w-sm">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
      <input v-model="query" class="input pl-9" placeholder="Search order number…" @input="onSearch" aria-label="Search orders" />
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 10" :key="i" class="skeleton h-12 w-full rounded-lg" />
    </div>

    <div v-else-if="!orders.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <ShoppingCart :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No orders</p>
      <p class="mt-1 text-xs text-stone-500">No orders match this filter.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Order</th>
            <th class="hidden sm:table-cell">Date</th>
            <th class="hidden md:table-cell">Payment</th>
            <th class="hidden lg:table-cell">Items</th>
            <th>Amount</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="o in orders" :key="o.id" @click="router.push(`/orders/${o.id}`)">
            <td class="font-medium">{{ o.order_number }}</td>
            <td class="hidden sm:table-cell text-stone-500 text-xs">{{ formatDate(o.placed_at || o.created_at) }}</td>
            <td class="hidden md:table-cell text-stone-500 uppercase text-xs">{{ o.payment_method }}</td>
            <td class="hidden lg:table-cell text-stone-500">{{ o.items?.length || '—' }}</td>
            <td class="font-semibold">{{ formatINR(o.grand_total) }}</td>
            <td><span :class="['badge', statusClass(o.status)]">{{ orderStatusLabel(o.status) }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="Math.ceil(total/PAGE_SIZE) > 1" class="mt-5 flex justify-center gap-2">
      <button class="btn-ghost btn-sm" :disabled="page<=1" @click="() => { page--; load() }">← Prev</button>
      <span class="flex h-8 items-center px-3 text-sm text-stone-500">{{ page }} / {{ Math.ceil(total/PAGE_SIZE) }}</span>
      <button class="btn-ghost btn-sm" :disabled="page>=Math.ceil(total/PAGE_SIZE)" @click="() => { page++; load() }">Next →</button>
    </div>
  </div>
</template>
