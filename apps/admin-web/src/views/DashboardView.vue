<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  TrendingUp, ShoppingCart, Store, Package, ShieldCheck,
  RotateCcw, Wallet, ArrowRight, AlertCircle, Users,
} from 'lucide-vue-next'
import type { Seller, Product, Order } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, formatDate, orderStatusLabel } from '@carlitos/shared'

const pendingSellers  = ref<Seller[]>([])
const pendingProducts = ref<Product[]>([])
const recentOrders    = ref<Order[]>([])
const allOrders       = ref<Order[]>([])
const returns         = ref<any[]>([])
const totals          = ref({ sellers: 0, products: 0, orders: 0 })
const loading         = ref(true)

onMounted(async () => {
  try {
    const emptyRes = { data: [], meta: {} as { total?: number } }
    const [ps, pp, ao, allO, rt] = await Promise.all([
      admin.sellers({ status: 'under_review' }).catch(() => emptyRes),
      admin.products({ status: 'submitted' }).catch(() => emptyRes),
      admin.orders({ page_size: 6 }).catch(() => emptyRes),
      admin.orders({ page_size: 100 }).catch(() => emptyRes),
      admin.returns().catch(() => []),
    ])
    pendingSellers.value  = (ps.data as Seller[]) || []
    pendingProducts.value = (pp.data as Product[]) || []
    recentOrders.value    = (ao.data as Order[]) || []
    allOrders.value       = (allO.data as Order[]) || []
    returns.value         = (rt as any[]) || []
    totals.value = {
      sellers:  ps.meta?.total ?? pendingSellers.value.length,
      products: pp.meta?.total ?? pendingProducts.value.length,
      orders:   allO.meta?.total ?? allOrders.value.length,
    }
  } finally {
    loading.value = false
  }
})

const gmv = computed(() => allOrders.value.reduce((a, o) => a + (o.grand_total || 0), 0))
const pendingReturns = computed(() => returns.value.filter(r => ['requested','pending'].includes(r.status)).length)

function statusClass(s: string) {
  if (['delivered'].includes(s)) return 'badge-green'
  if (['cancelled'].includes(s)) return 'badge-red'
  return 'badge-blue'
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Dashboard</h1>
        <p class="mt-0.5 text-sm text-stone-500">Marketplace operations overview</p>
      </div>
    </div>

    <!-- KPI grid -->
    <div class="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">GMV</p><TrendingUp :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-20 rounded" />
        <p v-else class="text-lg font-semibold">{{ formatINR(gmv) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Orders</p><ShoppingCart :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-12 rounded" />
        <p v-else class="text-lg font-semibold">{{ totals.orders }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Seller Approvals</p><Store :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-10 rounded" />
        <p v-else class="text-lg font-semibold" :class="totals.sellers ? 'text-amber-600' : ''">{{ totals.sellers }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Product Approvals</p><Package :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-10 rounded" />
        <p v-else class="text-lg font-semibold" :class="totals.products ? 'text-amber-600' : ''">{{ totals.products }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Returns</p><RotateCcw :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-10 rounded" />
        <p v-else class="text-lg font-semibold">{{ pendingReturns }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Settlements</p><Wallet :size="16" class="text-stone-400" /></div>
        <RouterLink to="/settlements" class="text-sm text-stone-500 hover:text-ink underline">View</RouterLink>
      </div>
    </div>

    <!-- Operational alerts -->
    <div v-if="!loading && (totals.sellers > 0 || totals.products > 0)" class="mb-6 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4">
      <AlertCircle :size="18" class="shrink-0 text-amber-600 mt-0.5" />
      <div class="text-sm text-amber-800">
        <p class="font-semibold">Action required</p>
        <p class="mt-0.5">
          <span v-if="totals.sellers">{{ totals.sellers }} seller{{ totals.sellers > 1 ? 's' : '' }} awaiting review</span>
          <span v-if="totals.sellers && totals.products"> · </span>
          <span v-if="totals.products">{{ totals.products }} product{{ totals.products > 1 ? 's' : '' }} pending approval</span>
        </p>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Pending sellers -->
      <section>
        <div class="section-header mb-3">
          <h2 class="text-h4">Sellers Awaiting Review</h2>
          <RouterLink to="/sellers" class="flex items-center gap-1 text-sm text-stone-500 hover:text-ink">View all <ArrowRight :size="13" /></RouterLink>
        </div>
        <div v-if="loading" class="space-y-2"><div v-for="i in 3" :key="i" class="skeleton h-12 w-full rounded-lg" /></div>
        <div v-else-if="!pendingSellers.length" class="rounded-xl border border-stone-200 bg-stone-50 p-6 text-center text-sm text-stone-500">Nothing pending. All caught up.</div>
        <div v-else class="card divide-y divide-stone-100">
          <RouterLink v-for="s in pendingSellers.slice(0,5)" :key="s.id" :to="`/sellers/${s.id}`" class="flex items-center justify-between p-3 hover:bg-stone-50">
            <span class="text-sm font-medium">{{ s.store_name }}</span>
            <span class="status-under_review">Under Review</span>
          </RouterLink>
        </div>
      </section>

      <!-- Pending products -->
      <section>
        <div class="section-header mb-3">
          <h2 class="text-h4">Products Pending Approval</h2>
          <RouterLink to="/products" class="flex items-center gap-1 text-sm text-stone-500 hover:text-ink">View all <ArrowRight :size="13" /></RouterLink>
        </div>
        <div v-if="loading" class="space-y-2"><div v-for="i in 3" :key="i" class="skeleton h-12 w-full rounded-lg" /></div>
        <div v-else-if="!pendingProducts.length" class="rounded-xl border border-stone-200 bg-stone-50 p-6 text-center text-sm text-stone-500">No products pending approval.</div>
        <div v-else class="card divide-y divide-stone-100">
          <RouterLink v-for="p in pendingProducts.slice(0,5)" :key="p.id" :to="`/products/${p.id}`" class="flex items-center justify-between p-3 hover:bg-stone-50">
            <span class="text-sm font-medium truncate">{{ p.title }}</span>
            <span class="status-submitted">Submitted</span>
          </RouterLink>
        </div>
      </section>
    </div>

    <!-- Recent orders -->
    <section class="mt-6">
      <div class="section-header mb-3">
        <h2 class="text-h4">Recent Orders</h2>
        <RouterLink to="/orders" class="flex items-center gap-1 text-sm text-stone-500 hover:text-ink">View all <ArrowRight :size="13" /></RouterLink>
      </div>
      <div v-if="loading" class="space-y-2"><div v-for="i in 5" :key="i" class="skeleton h-12 w-full rounded-lg" /></div>
      <div v-else-if="!recentOrders.length" class="rounded-xl border border-stone-200 bg-stone-50 p-6 text-center text-sm text-stone-500">No orders yet.</div>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr><th>Order</th><th class="hidden sm:table-cell">Date</th><th>Amount</th><th>Status</th></tr>
          </thead>
          <tbody>
            <tr v-for="o in recentOrders" :key="o.id" @click="$router.push(`/orders/${o.id}`)">
              <td class="font-medium">{{ o.order_number }}</td>
              <td class="hidden sm:table-cell text-stone-500">{{ formatDate(o.placed_at || o.created_at) }}</td>
              <td class="font-semibold">{{ formatINR(o.grand_total) }}</td>
              <td><span :class="['badge', statusClass(o.status)]">{{ orderStatusLabel(o.status) }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
