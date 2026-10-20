<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { Package, ChevronRight, ChevronLeft } from 'lucide-vue-next'
import type { Order } from '@carlitos/types'
import { customer, MEDIA_BASE } from '@/api'
import { formatINR, formatDate, orderStatusLabel, resolveMediaUrl } from '@carlitos/shared'
import { orderStatusStyle } from '@/composables/orderStatus'

const router = useRouter()

const orders  = ref<Order[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const res    = await customer.orders()
    orders.value = (res.data as Order[]) || []
  } finally {
    loading.value = false
  }
})

// Status icon + colours come from the shared, meaning-based mapping.
const statusMeta = orderStatusStyle

// First item of an order drives the card preview (title + image).
function firstItem(o: Order) {
  return o.items && o.items.length ? o.items[0] : null
}
function extraCount(o: Order) {
  const n = o.items?.length || 0
  return n > 1 ? n - 1 : 0
}
const hasOrders = computed(() => orders.value.length > 0)
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <!-- Compact top bar -->
    <div class="mb-3 flex items-center gap-1.5">
      <button
        type="button"
        class="-ml-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-stone-700 transition-colors hover:bg-stone-100"
        aria-label="Go back"
        @click="router.back()"
      >
        <ChevronLeft :size="18" />
      </button>
      <h1 class="text-base font-semibold text-ink">My Orders</h1>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2.5">
      <div v-for="i in 3" :key="i" class="rounded-xl border border-stone-200 p-3">
        <div class="flex gap-2.5">
          <div class="skeleton h-14 w-12 shrink-0 rounded-md" />
          <div class="flex-1 space-y-2 pt-0.5">
            <div class="skeleton h-3 w-2/3 rounded" />
            <div class="skeleton h-2.5 w-1/3 rounded" />
            <div class="skeleton h-2.5 w-1/4 rounded" />
          </div>
        </div>
      </div>
    </div>

    <!-- Empty -->
    <div v-else-if="!hasOrders" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Package :size="36" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No orders yet</p>
      <p class="mt-1 text-xs text-stone-500">Your placed orders will appear here.</p>
      <RouterLink to="/" class="btn-primary btn-sm mt-4">Start shopping</RouterLink>
    </div>

    <!-- Order list -->
    <div v-else class="space-y-3">
      <RouterLink
        v-for="o in orders"
        :key="o.id"
        :to="`/orders/${o.id}`"
        class="block rounded-2xl border border-stone-200 bg-white p-4 transition-all hover:border-stone-300 hover:shadow-sm"
        :aria-label="`Order ${o.order_number}`"
      >
        <div class="flex items-center gap-3.5">
          <!-- Product thumbnail -->
          <div class="h-20 w-16 shrink-0 overflow-hidden rounded-lg bg-stone-100">
            <img
              v-if="firstItem(o)?.image"
              :src="resolveMediaUrl(MEDIA_BASE, firstItem(o)!.image!)"
              :alt="firstItem(o)?.product_title"
              class="h-full w-full object-cover"
              loading="lazy"
            />
            <div v-else class="flex h-full w-full items-center justify-center text-stone-300">
              <Package :size="20" />
            </div>
          </div>

          <!-- Details -->
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium text-ink">{{ firstItem(o)?.product_title || o.order_number }}</p>
            <p v-if="firstItem(o)" class="mt-1 truncate text-xs text-stone-500">
              <span v-if="firstItem(o)?.variant_label">{{ firstItem(o)?.variant_label }} · </span>Qty {{ firstItem(o)?.quantity }}
              <span v-if="extraCount(o)"> · +{{ extraCount(o) }} more</span>
            </p>
            <!-- Status row: single parcel icon, colour by status -->
            <div class="mt-2 flex items-center gap-1.5">
              <span :class="['flex h-5 w-5 shrink-0 items-center justify-center rounded', statusMeta(o.status).chip]">
                <Package :size="12" />
              </span>
              <span :class="['truncate text-xs font-semibold', statusMeta(o.status).text]">
                {{ orderStatusLabel(o.status) }}
              </span>
              <span class="text-[11px] text-stone-400">· {{ formatDate(o.placed_at || o.created_at) }}</span>
            </div>
          </div>

          <!-- Amount + chevron -->
          <div class="flex shrink-0 flex-col items-end gap-2">
            <span class="text-sm font-semibold text-ink">{{ formatINR(o.grand_total) }}</span>
            <ChevronRight :size="18" class="text-stone-300" />
          </div>
        </div>
      </RouterLink>
    </div>
  </div>
</template>
