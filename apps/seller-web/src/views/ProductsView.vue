<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Package, Plus, Search, Edit, CheckCircle, AlertCircle,
  SlidersHorizontal, X, ArrowUpDown,
} from 'lucide-vue-next'
import type { Product } from '@carlitos/types'
import { seller, MEDIA_BASE } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatINR, formatDate, resolveMediaUrl } from '@carlitos/shared'

const auth = useAuthStore()

const products = ref<Product[]>([])
const total    = ref(0)
const loading  = ref(true)
const query    = ref('')
const status   = ref('')
const sort     = ref('newest')      // client-side sort key
const minPrice = ref<string>('')    // client-side price filter
const maxPrice = ref<string>('')
const showFilters = ref(false)      // "More filters" panel toggle
const page     = ref(1)
const PAGE_SIZE = 20

const STATUS_OPTIONS = [
  { value: '',             label: 'All status' },
  { value: 'draft',        label: 'Draft' },
  { value: 'submitted',    label: 'Submitted' },
  { value: 'under_review', label: 'Under Review' },
  { value: 'approved',     label: 'Approved' },
  { value: 'rejected',     label: 'Rejected' },
]

const SORT_OPTIONS = [
  { value: 'newest',      label: 'Newest first' },
  { value: 'oldest',      label: 'Oldest first' },
  { value: 'price_high',  label: 'Price: High to Low' },
  { value: 'price_low',   label: 'Price: Low to High' },
  { value: 'title_az',    label: 'Title: A–Z' },
]

function priceOf(p: Product): number {
  return p.selling_price ?? p.variants?.[0]?.selling_price ?? 0
}
function firstImage(p: Product): string {
  const key = p.images?.[0]?.file_key
  return key ? resolveMediaUrl(MEDIA_BASE, key) : ''
}

async function load() {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (query.value)  params.q      = query.value
    if (status.value) params.status = status.value
    const res    = await seller.products(params)
    products.value = (res.data as Product[]) || []
    total.value    = res.meta?.total || 0
  } finally {
    loading.value = false
  }
}

// Client-side price filter + sort applied on top of the server results.
const visibleProducts = computed(() => {
  let list = [...products.value]
  const min = minPrice.value ? Number(minPrice.value) : null
  const max = maxPrice.value ? Number(maxPrice.value) : null
  if (min != null) list = list.filter(p => priceOf(p) >= min)
  if (max != null) list = list.filter(p => priceOf(p) <= max)

  switch (sort.value) {
    case 'oldest':     list.sort((a, b) => +new Date(a.created_at) - +new Date(b.created_at)); break
    case 'price_high': list.sort((a, b) => priceOf(b) - priceOf(a)); break
    case 'price_low':  list.sort((a, b) => priceOf(a) - priceOf(b)); break
    case 'title_az':   list.sort((a, b) => (a.title || '').localeCompare(b.title || '')); break
    default:           list.sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at)) // newest
  }
  return list
})

// Count of active "advanced" filters (for the badge on the toggle).
const activeFilterCount = computed(() =>
  [status.value, minPrice.value, maxPrice.value].filter(Boolean).length +
  (sort.value !== 'newest' ? 1 : 0)
)
const hasAnyFilter = computed(() =>
  !!(query.value || status.value || minPrice.value || maxPrice.value) || sort.value !== 'newest'
)

async function submit(id: string) {
  await seller.submitProduct(id).catch(() => {})
  await load()
}

function statusClass(s: string) {
  if (s === 'approved')     return 'badge-green'
  if (s === 'submitted')    return 'badge-amber'
  if (s === 'rejected')     return 'badge-red'
  if (s === 'suspended')    return 'badge-red'
  if (s === 'under_review') return 'badge-amber'
  return 'badge-gray'
}

function clearFilters() {
  query.value = ''
  status.value = ''
  sort.value = 'newest'
  minPrice.value = ''
  maxPrice.value = ''
  page.value = 1
  load()
}

onMounted(load)

let searchTimeout: ReturnType<typeof setTimeout>
function onSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => { page.value = 1; load() }, 400)
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Products</h1>
        <p class="mt-0.5 text-sm text-stone-500">{{ total }} product{{ total === 1 ? '' : 's' }} in your catalogue</p>
      </div>
      <RouterLink v-if="auth.isApproved" to="/products/new" class="btn-primary">
        <Plus :size="15" /> Add Product
      </RouterLink>
    </div>

    <!-- Approval-pending notice: listing is blocked until an admin approves -->
    <div
      v-if="!auth.isApproved"
      class="mb-5 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4"
    >
      <AlertCircle :size="18" class="mt-0.5 shrink-0 text-amber-600" />
      <div class="text-sm text-amber-800">
        <p class="font-semibold">You can't add products yet</p>
        <p class="mt-0.5">
          Your store is
          <strong>{{ (auth.sellerStatus || 'pending').replace('_', ' ') }}</strong>.
          Finish your
          <RouterLink to="/onboarding" class="font-medium underline">onboarding</RouterLink>
          and wait for admin approval to start listing products.
        </p>
      </div>
    </div>

    <!-- ── Filter bar ─────────────────────────────────────── -->
    <div class="mb-5 space-y-3">
      <!-- Primary row: search + sort + filter toggle. Wraps cleanly on mobile. -->
      <div class="flex flex-wrap items-center gap-2 sm:gap-3">
        <div class="relative min-w-0 flex-1 basis-full sm:basis-64">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
          <input
            v-model="query"
            class="input pl-9"
            placeholder="Search products…"
            aria-label="Search products"
            @input="onSearch"
          />
        </div>

        <!-- Sort -->
        <div class="relative flex-1 sm:flex-none">
          <ArrowUpDown class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
          <select v-model="sort" class="select w-full pl-9 sm:w-48" aria-label="Sort products">
            <option v-for="o in SORT_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
          </select>
        </div>

        <!-- Advanced filters toggle -->
        <button
          type="button"
          class="btn-ghost relative shrink-0"
          :class="showFilters ? 'border-ink text-ink' : ''"
          @click="showFilters = !showFilters"
        >
          <SlidersHorizontal :size="15" /> Filters
          <span
            v-if="activeFilterCount"
            class="ml-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-ink px-1 text-[10px] font-semibold text-white"
          >{{ activeFilterCount }}</span>
        </button>
      </div>

      <!-- Advanced panel: status + price range. Collapsible. -->
      <transition name="filters">
        <div v-if="showFilters" class="rounded-lg border border-stone-200 bg-stone-50 p-4">
          <div class="grid gap-4 sm:grid-cols-3">
            <div>
              <label class="label text-xs" for="f-status">Status</label>
              <select id="f-status" v-model="status" class="select" @change="() => { page = 1; load() }">
                <option v-for="o in STATUS_OPTIONS" :key="o.value" :value="o.value">{{ o.label }}</option>
              </select>
            </div>
            <div>
              <label class="label text-xs" for="f-min">Min price (₹)</label>
              <input id="f-min" v-model="minPrice" type="number" min="0" inputmode="numeric" class="input" placeholder="0" />
            </div>
            <div>
              <label class="label text-xs" for="f-max">Max price (₹)</label>
              <input id="f-max" v-model="maxPrice" type="number" min="0" inputmode="numeric" class="input" placeholder="Any" />
            </div>
          </div>
          <div class="mt-4 flex items-center justify-between">
            <p class="text-xs text-stone-500">Showing {{ visibleProducts.length }} of {{ products.length }} loaded</p>
            <button v-if="hasAnyFilter" type="button" class="btn-ghost btn-sm" @click="clearFilters">
              <X :size="13" /> Clear all
            </button>
          </div>
        </div>
      </transition>

      <!-- Active filter chips (visible even when panel is collapsed) -->
      <div v-if="hasAnyFilter && !showFilters" class="flex flex-wrap items-center gap-2">
        <span v-if="status" class="badge-gray flex items-center gap-1">
          {{ STATUS_OPTIONS.find(o => o.value === status)?.label }}
          <button aria-label="Clear status" @click="() => { status = ''; page = 1; load() }"><X :size="12" /></button>
        </span>
        <span v-if="minPrice" class="badge-gray flex items-center gap-1">
          Min ₹{{ minPrice }}
          <button aria-label="Clear min price" @click="minPrice = ''"><X :size="12" /></button>
        </span>
        <span v-if="maxPrice" class="badge-gray flex items-center gap-1">
          Max ₹{{ maxPrice }}
          <button aria-label="Clear max price" @click="maxPrice = ''"><X :size="12" /></button>
        </span>
        <button class="text-xs text-stone-500 underline hover:text-ink" @click="clearFilters">Clear all</button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!visibleProducts.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Package :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">
        {{ products.length ? 'No products match your filters' : 'No products yet' }}
      </p>
      <p class="mt-1 text-xs text-stone-500">
        {{ products.length ? 'Try adjusting or clearing the filters.' : 'Add your first product to start selling on CARLITOS.' }}
      </p>
      <button v-if="products.length && hasAnyFilter" class="btn-ghost btn-sm mt-4" @click="clearFilters">
        <X :size="13" /> Clear filters
      </button>
      <RouterLink v-else-if="auth.isApproved" to="/products/new" class="btn-primary btn-sm mt-4">Add Product</RouterLink>
    </div>

    <!-- Table: slides horizontally on small screens so every column stays on one
         line (Product, Brand, Category, Price, Status, Updated, Actions). -->
    <div v-else class="table-wrap">
      <table class="table min-w-[760px]">
        <thead>
          <tr>
            <th class="whitespace-nowrap">Product</th>
            <th class="whitespace-nowrap">Brand</th>
            <th class="whitespace-nowrap">Category</th>
            <th class="whitespace-nowrap">Price</th>
            <th class="whitespace-nowrap">Status</th>
            <th class="whitespace-nowrap">Updated</th>
            <th class="whitespace-nowrap text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in visibleProducts" :key="p.id">
            <td>
              <div class="flex items-center gap-2.5">
                <div class="h-10 w-10 shrink-0 overflow-hidden rounded-md border border-stone-100 bg-stone-50">
                  <img v-if="firstImage(p)" :src="firstImage(p)" :alt="p.title" class="h-full w-full object-cover" />
                  <div v-else class="flex h-full w-full items-center justify-center"><Package :size="14" class="text-stone-300" /></div>
                </div>
                <p class="whitespace-nowrap font-medium text-ink">{{ p.title }}</p>
              </div>
            </td>
            <td class="whitespace-nowrap text-stone-500">{{ p.brand_name || '—' }}</td>
            <td class="whitespace-nowrap text-stone-500">—</td>
            <td class="whitespace-nowrap font-medium">
              {{ priceOf(p) > 0 ? formatINR(priceOf(p)) : '—' }}
            </td>
            <td class="whitespace-nowrap"><span :class="['badge', statusClass(p.status)]">{{ p.status.replace('_', ' ') }}</span></td>
            <td class="whitespace-nowrap text-xs text-stone-500">{{ formatDate(p.created_at) }}</td>
            <td class="whitespace-nowrap">
              <div class="flex items-center justify-end gap-3">
                <RouterLink :to="`/products/${p.id}/edit`" class="flex items-center gap-1 text-xs text-stone-500 hover:text-ink" :aria-label="`Edit ${p.title}`">
                  <Edit :size="13" /> Edit
                </RouterLink>
                <button
                  v-if="p.status === 'draft' || p.status === 'rejected'"
                  class="flex items-center gap-1 text-xs text-emerald-600 hover:text-emerald-700"
                  @click="submit(p.id)"
                >
                  <CheckCircle :size="13" /> Submit
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="Math.ceil(total/PAGE_SIZE) > 1" class="mt-5 flex justify-center gap-2">
      <button class="btn-ghost btn-sm" :disabled="page<=1" @click="() => { page--; load() }">← Prev</button>
      <span class="flex h-8 items-center px-3 text-sm text-stone-500">{{ page }} / {{ Math.ceil(total/PAGE_SIZE) }}</span>
      <button class="btn-ghost btn-sm" :disabled="page>=Math.ceil(total/PAGE_SIZE)" @click="() => { page++; load() }">Next →</button>
    </div>
  </div>
</template>

<style scoped>
.filters-enter-active, .filters-leave-active { transition: opacity 180ms ease, transform 180ms ease; }
.filters-enter-from, .filters-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
