<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { BarChart3, TrendingUp, ShoppingCart, Store, Package } from 'lucide-vue-next'
import type { Order, Seller } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR } from '@carlitos/shared'

const orders  = ref<Order[]>([])
const sellers = ref<Seller[]>([])
const loading = ref(true)
const range   = ref('30')

onMounted(async () => {
  try {
    const [o, s] = await Promise.all([
      admin.orders({ page_size: 200 }).catch(() => ({ data: [] })),
      admin.sellers({ page_size: 200 }).catch(() => ({ data: [] })),
    ])
    orders.value  = (o as any).data || []
    sellers.value = (s as any).data || []
  } finally { loading.value = false }
})

const gmv          = computed(() => orders.value.reduce((a, o) => a + o.grand_total, 0))
const deliveredGmv = computed(() => orders.value.filter(o => o.status === 'delivered').reduce((a, o) => a + o.grand_total, 0))
const cancelled    = computed(() => orders.value.filter(o => o.status === 'cancelled').length)
const avgOrder     = computed(() => orders.value.length ? gmv.value / orders.value.length : 0)
const activeSellers= computed(() => sellers.value.filter(s => s.status === 'approved').length)

// Status distribution for a simple bar breakdown
const statusDist = computed(() => {
  const map: Record<string, number> = {}
  orders.value.forEach(o => { map[o.status] = (map[o.status] || 0) + 1 })
  const max = Math.max(1, ...Object.values(map))
  return Object.entries(map).map(([status, count]) => ({ status, count, pct: Math.round((count / max) * 100) }))
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Reports</h1>
      <select v-model="range" class="select w-40">
        <option value="7">Last 7 days</option>
        <option value="30">Last 30 days</option>
        <option value="90">Last 90 days</option>
        <option value="all">All time</option>
      </select>
    </div>

    <!-- KPI cards -->
    <div class="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">GMV</p><TrendingUp :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-20 rounded" /><p v-else class="text-lg font-semibold">{{ formatINR(gmv) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Delivered GMV</p><Package :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-20 rounded" /><p v-else class="text-lg font-semibold">{{ formatINR(deliveredGmv) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Avg Order</p><ShoppingCart :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-16 rounded" /><p v-else class="text-lg font-semibold">{{ formatINR(avgOrder) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Cancelled</p><ShoppingCart :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-10 rounded" /><p v-else class="text-lg font-semibold">{{ cancelled }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2"><p class="text-label-sm">Active Sellers</p><Store :size="16" class="text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-6 w-10 rounded" /><p v-else class="text-lg font-semibold">{{ activeSellers }}</p>
      </div>
    </div>

    <!-- Order status distribution -->
    <section class="card p-6">
      <div class="flex items-center gap-2 mb-4">
        <BarChart3 :size="18" class="text-stone-500" />
        <h2 class="text-h4">Order Status Distribution</h2>
      </div>
      <div v-if="loading" class="space-y-3">
        <div v-for="i in 5" :key="i" class="skeleton h-6 w-full rounded" />
      </div>
      <div v-else-if="!statusDist.length" class="text-sm text-stone-500">No order data available.</div>
      <div v-else class="space-y-3">
        <div v-for="s in statusDist" :key="s.status">
          <div class="mb-1 flex items-center justify-between text-xs">
            <span class="capitalize text-stone-600">{{ s.status.replace(/_/g,' ') }}</span>
            <span class="font-medium">{{ s.count }}</span>
          </div>
          <div class="h-2 w-full overflow-hidden rounded-full bg-stone-100">
            <div class="h-full rounded-full bg-ink transition-all" :style="{ width: s.pct + '%' }" />
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
