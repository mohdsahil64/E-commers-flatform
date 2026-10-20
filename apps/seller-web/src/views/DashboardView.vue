<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ShoppingCart, Package, AlertCircle,
  ArrowRight, IndianRupee, PackageCheck, RotateCcw, Repeat, Boxes,
} from 'lucide-vue-next'
import type { Seller, Order, Product } from '@carlitos/types'
import { seller } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatINR, formatDate, orderStatusLabel, SELLER_STATUS_LABELS } from '@carlitos/shared'

const auth      = useAuthStore()
const profile   = ref<Seller | null>(null)
const orders    = ref<Order[]>([])
const returns   = ref<any[]>([])
const inventory = ref(0)
const loading   = ref(true)

onMounted(async () => {
  try {
    const [p, o, r, inv] = await Promise.all([
      seller.profile().catch(() => null),
      // Pull a wider window so the last-30-days analytics are accurate.
      seller.orders({ page_size: 100 }).catch(() => ({ data: [] as Order[] })),
      seller.returns().catch(() => [] as any[]),
      // Live inventory = approved (live) product listings.
      seller.products({ status: 'approved', page_size: 1 }).catch(() => ({ data: [] as Product[], meta: { total: 0 } })),
    ])
    profile.value   = p
    orders.value    = (o as any).data || []
    returns.value   = Array.isArray(r) ? r : []
    inventory.value = (inv as any)?.meta?.total ?? ((inv as any)?.data?.length || 0)
  } finally {
    loading.value = false
  }
})

// Most recent orders for the table (top 5).
const recentOrders = computed(() => orders.value.slice(0, 5))

function placedAt(o: Order) { return new Date(o.placed_at || o.created_at).getTime() }

// ── Today window (since local midnight) ───────────────────────────
const todayStart = new Date(); todayStart.setHours(0, 0, 0, 0)
const todayMs = todayStart.getTime()
const todayOrders = computed(() => orders.value.filter(o => placedAt(o) >= todayMs))
const todayOrderCount = computed(() => todayOrders.value.length)
// Today's revenue = total value of ALL orders placed today (not just delivered).
const todayRevenue = computed(() =>
  todayOrders.value.reduce((a, o) => a + o.grand_total, 0)
)

// ── Last 30 days window ──────────────────────────────────────────
const monthAgo = Date.now() - 30 * 24 * 60 * 60 * 1000
const monthOrders = computed(() => orders.value.filter(o => placedAt(o) >= monthAgo))

// One-month analytics: total orders, revenue (delivered), delivered count.
const monthOrderCount = computed(() => monthOrders.value.length)
const monthDelivered  = computed(() => monthOrders.value.filter(o => o.status === 'delivered').length)
const monthRevenue    = computed(() =>
  monthOrders.value.filter(o => o.status === 'delivered').reduce((a, o) => a + o.grand_total, 0)
)

// Returns vs exchanges raised in the last 30 days. The type field distinguishes
// an exchange from a plain return; anything not marked 'exchange' counts as a return.
const monthReturnsList = computed(() =>
  returns.value.filter(r => new Date(r.created_at).getTime() >= monthAgo)
)
const monthExchanges = computed(() =>
  monthReturnsList.value.filter(r => r.type === 'exchange').length
)
const monthReturns = computed(() =>
  monthReturnsList.value.filter(r => r.type !== 'exchange').length
)

// Store status helpers for the header banner + Add Product gating.
const storeStatus      = computed(() => profile.value?.status ?? auth.sellerStatus ?? 'pending')
const storeStatusLabel = computed(() => SELLER_STATUS_LABELS[storeStatus.value] || storeStatus.value)
const isApproved       = computed(() => storeStatus.value === 'approved')

// Lifecycle detail used by the status banners.
const MAX_RESUBMITS    = 3
const rejectionReason  = computed(() => profile.value?.rejection_reason || '')
const suspendedReason  = computed(() => profile.value?.suspended_reason || '')
const resubmitCount    = computed(() => profile.value?.resubmit_count ?? 0)
const attemptsLeft     = computed(() => Math.max(0, MAX_RESUBMITS - resubmitCount.value))
const isRejected       = computed(() => storeStatus.value === 'rejected')
const isSuspended      = computed(() => storeStatus.value === 'suspended')
const isBlocked        = computed(() => storeStatus.value === 'blocked')
// A rejected seller who has run out of attempts is effectively permanently out.
const canResubmit      = computed(() => isRejected.value && attemptsLeft.value > 0)
// "Pending onboarding" = created but not yet submitted for review.
const isPendingOnboarding = computed(() => storeStatus.value === 'pending')
const isUnderReview    = computed(() => storeStatus.value === 'under_review')
const CONTACT_ADMIN    = 'mailto:seller-support@carlitos.in'
function statusBadgeClass(s: string) {
  if (s === 'approved') return 'badge-green'
  if (['rejected','suspended','blocked'].includes(s)) return 'badge-red'
  return 'badge-amber'
}

function statusClass(s: string) {
  if (['delivered'].includes(s))  return 'badge-green'
  if (['cancelled'].includes(s))  return 'badge-red'
  if (['pending_payment'].includes(s)) return 'badge-gray'
  return 'badge-blue'
}

</script>

<template>
  <div>
    <!-- Page header: store name + live approval status.
         (The "Dashboard" title lives in the top bar, so it's not repeated here.) -->
    <div class="page-header">
      <div class="min-w-0">
        <p class="text-label-sm">Store Name</p>
        <div class="mt-0.5 flex flex-wrap items-center gap-2">
          <h1 class="page-title truncate">{{ profile?.store_name || 'Your store' }}</h1>
          <span :class="['badge', statusBadgeClass(storeStatus)]">{{ storeStatusLabel }}</span>
        </div>
      </div>

      <!-- Actions: Add Product + View Orders. On mobile they stretch edge to
           edge (one hugging the left, one the right); auto width on desktop.
           Add Product is gated (disabled) until the store is approved. -->
      <div class="flex w-full items-center gap-3 sm:w-auto">
        <RouterLink v-if="isApproved" to="/products/new" class="btn-primary flex-1 sm:flex-none">
          <Package :size="15" /> Add Product
        </RouterLink>
        <button
          v-else
          type="button"
          class="btn-primary flex-1 sm:flex-none"
          disabled
          title="Your store must be approved before you can add products"
        >
          <Package :size="15" /> Add Product
        </button>

        <RouterLink to="/orders" class="btn-ghost flex-1 sm:flex-none">
          <ShoppingCart :size="15" /> View Orders
        </RouterLink>

        <RouterLink v-if="isPendingOnboarding || canResubmit" to="/onboarding" class="btn-ghost flex-1 sm:flex-none">
          Onboarding
        </RouterLink>
      </div>
    </div>

    <!-- ── Status banners ─────────────────────────────────────────────
         Shown until the store is approved. Adding products is disabled
         (see the header button) in every non-approved state. -->

    <!-- Pending onboarding: finish onboarding to submit for review. -->
    <div v-if="!loading && isPendingOnboarding" class="mb-6 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4">
      <AlertCircle :size="18" class="shrink-0 text-amber-600 mt-0.5" />
      <div class="text-sm text-amber-800">
        <p class="font-semibold">Complete your onboarding</p>
        <p class="mt-0.5">
          Add your pickup location and sign the commission agreement to submit your store for review.
          You can add products only once an admin approves your store.
        </p>
        <RouterLink to="/onboarding" class="btn-primary btn-sm mt-3">Complete onboarding</RouterLink>
      </div>
    </div>

    <!-- Under review: submitted, waiting on admin. -->
    <div v-else-if="!loading && isUnderReview" class="mb-6 flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4">
      <AlertCircle :size="18" class="shrink-0 text-blue-600 mt-0.5" />
      <div class="text-sm text-blue-800">
        <p class="font-semibold">Application under review</p>
        <p class="mt-0.5">Your store is being reviewed by our team. We'll notify you once it's approved — this usually takes 1–2 business days.</p>
      </div>
    </div>

    <!-- Rejected: show reason + resubmit (up to 3 attempts). -->
    <div v-else-if="!loading && isRejected" class="mb-6 flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-4">
      <AlertCircle :size="18" class="shrink-0 text-red-600 mt-0.5" />
      <div class="text-sm text-red-800">
        <p class="font-semibold">Application rejected</p>
        <p v-if="rejectionReason" class="mt-1 rounded border border-red-200 bg-white/60 p-2">
          <span class="font-medium">Reason:</span> {{ rejectionReason }}
        </p>
        <template v-if="canResubmit">
          <p class="mt-2">You can review your details and resubmit. <strong>{{ attemptsLeft }}</strong> of {{ MAX_RESUBMITS }} attempt(s) remaining.</p>
          <RouterLink to="/onboarding" class="btn-primary btn-sm mt-3">Review & resubmit</RouterLink>
        </template>
        <p v-else class="mt-2 font-medium">You've reached the maximum number of resubmissions. Please contact support.</p>
        <a v-if="!canResubmit" :href="CONTACT_ADMIN" class="btn-ghost btn-sm mt-3">Contact administrator</a>
      </div>
    </div>

    <!-- Suspended: hidden from customers; contact admin to restore. -->
    <div v-else-if="!loading && isSuspended" class="mb-6 flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-4">
      <AlertCircle :size="18" class="shrink-0 text-red-600 mt-0.5" />
      <div class="text-sm text-red-800">
        <p class="font-semibold">Your store is suspended</p>
        <p v-if="suspendedReason" class="mt-1 rounded border border-red-200 bg-white/60 p-2">
          <span class="font-medium">Reason:</span> {{ suspendedReason }}
        </p>
        <p class="mt-2">Your products are hidden from customers while suspended. Please contact the administrator to resolve this.</p>
        <a :href="CONTACT_ADMIN" class="btn-primary btn-sm mt-3">Contact administrator</a>
      </div>
    </div>

    <!-- Blocked: account closed; contact admin. -->
    <div v-else-if="!loading && isBlocked" class="mb-6 flex items-start gap-3 rounded-lg border border-red-300 bg-red-100 p-4">
      <AlertCircle :size="18" class="shrink-0 text-red-700 mt-0.5" />
      <div class="text-sm text-red-900">
        <p class="font-semibold">Your account is blocked</p>
        <p v-if="suspendedReason" class="mt-1 rounded border border-red-300 bg-white/60 p-2">
          <span class="font-medium">Reason:</span> {{ suspendedReason }}
        </p>
        <p class="mt-2">Your store and products are no longer visible to customers. Contact the administrator if you believe this is a mistake.</p>
        <a :href="CONTACT_ADMIN" class="btn-primary btn-sm mt-3">Contact administrator</a>
      </div>
    </div>

    <!-- ── Today ── -->
    <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-stone-400">Today</p>
    <div class="mb-6 grid grid-cols-2 gap-3 sm:gap-4">
      <div class="stat ring-1 ring-ink/5">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Today Orders</p>
          <ShoppingCart :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ todayOrderCount }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">Placed today</p>
      </div>

      <div class="stat ring-1 ring-ink/5">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Today Revenue</p>
          <IndianRupee :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ formatINR(todayRevenue) }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">All orders placed</p>
      </div>
    </div>

    <!-- ── Last 30 days ── -->
    <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-stone-400">1 Month</p>
    <div class="mb-8 grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-3">
      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Orders</p>
          <ShoppingCart :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ monthOrderCount }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">1 month</p>
      </div>

      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Revenue</p>
          <IndianRupee :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ formatINR(monthRevenue) }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">Delivered</p>
      </div>

      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Delivered</p>
          <PackageCheck :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-emerald-600 sm:text-2xl">{{ monthDelivered }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">1 month</p>
      </div>

      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Returns</p>
          <RotateCcw :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ monthReturns }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">1 month</p>
      </div>

      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Exchanges</p>
          <Repeat :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ monthExchanges }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">1 month</p>
      </div>

      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3">
          <p class="text-label-sm truncate">Inventory</p>
          <Boxes :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-16 rounded" />
        <p v-else class="text-xl font-semibold text-ink sm:text-2xl">{{ inventory }}</p>
        <p class="mt-1 text-xs text-stone-500 truncate">Live products</p>
      </div>
    </div>

    <!-- Recent orders -->
    <section>
      <div class="mb-4 flex items-center justify-between">
        <h2 class="text-h3">Recent Orders</h2>
        <RouterLink to="/orders" class="flex items-center gap-1 text-sm text-stone-500 hover:text-ink">
          View all <ArrowRight :size="14" />
        </RouterLink>
      </div>

      <!-- Skeleton -->
      <div v-if="loading" class="space-y-2">
        <div v-for="i in 5" :key="i" class="skeleton h-14 w-full rounded-lg" />
      </div>

      <!-- Empty -->
      <div v-else-if="!orders.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-10">
        <ShoppingCart :size="36" class="text-stone-300 mb-3" />
        <p class="text-sm font-medium text-stone-700">No orders yet</p>
        <p class="mt-1 text-xs text-stone-500">Orders will appear here when customers purchase your products.</p>
      </div>

      <!-- Table: scrolls horizontally on small screens so every column (Order ID,
           Date, Items, Amount, Status, action) stays on one line instead of
           wrapping onto multiple rows. Swipe sideways to see the rest. -->
      <div v-else class="table-wrap">
        <table class="table min-w-[640px]">
          <thead>
            <tr>
              <th class="whitespace-nowrap">Order ID</th>
              <th class="whitespace-nowrap">Date</th>
              <th class="whitespace-nowrap">Items</th>
              <th class="whitespace-nowrap">Amount</th>
              <th class="whitespace-nowrap">Status</th>
              <th class="whitespace-nowrap"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="o in recentOrders" :key="o.id">
              <td class="whitespace-nowrap font-medium">{{ o.order_number }}</td>
              <td class="whitespace-nowrap text-stone-500">{{ formatDate(o.placed_at || o.created_at) }}</td>
              <td class="whitespace-nowrap text-stone-500">{{ o.items?.length || '—' }}</td>
              <td class="whitespace-nowrap font-semibold">{{ formatINR(o.grand_total) }}</td>
              <td class="whitespace-nowrap"><span :class="['badge', statusClass(o.status)]">{{ orderStatusLabel(o.status) }}</span></td>
              <td class="whitespace-nowrap">
                <RouterLink :to="`/orders/${o.id}`" class="text-xs text-stone-500 hover:text-ink underline">View</RouterLink>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
