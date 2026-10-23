<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { SlidersHorizontal, X, ChevronDown, ChevronUp, Search } from 'lucide-vue-next'
import type { ListingCard } from '@carlitos/types'
import { pub } from '@/api'
import { useSeo } from '@/composables/useSeo'
import ProductCard from '@/components/ProductCard.vue'
import PageBack from '@/components/PageBack.vue'

const props  = defineProps<{ gender?: string }>()
const route  = useRoute()
const router = useRouter()
const { set: setSeo } = useSeo()

/* ── State ─────────────────────────────────────────────── */
const products   = ref<ListingCard[]>([])
const total      = ref(0)
const loading    = ref(true)
const error      = ref('')
const page       = ref(1)
const PAGE_SIZE  = 24
const filterOpen = ref(false)  // mobile filter drawer

/* ── Filter state (synced with URL) ────────────────────── */
const sortVal      = ref((route.query.sort as string) || 'relevance')
const minPrice     = ref((route.query.min_price as string) || '')
const maxPrice     = ref((route.query.max_price as string) || '')
const inStockOnly  = ref(route.query.in_stock === '1')
const seasonVal    = ref((route.query.season as string) || '')

/* Product-type quick filters — slugs match backend gender-neutral categories. */
const TYPE_FILTERS = [
  { slug: 'upperwear',  label: 'Upperwear' },
  { slug: 'bottomwear', label: 'Bottomwear' },
  { slug: 'innerwear',  label: 'Innerwear' },
  { slug: 'underwear',  label: 'Underwear' },
  { slug: 'shoes',      label: 'Shoes' },
]
const activeTypeSlug = computed(() => route.name === 'category' ? String(route.params.slug || '') : '')
function goType(slug: string) {
  if (activeTypeSlug.value === slug) { router.push('/search'); return }
  router.push(`/category/${slug}`)
}

/* ── Computed page title / breadcrumb ──────────────────── */
const pageTitle = computed(() => {
  if (props.gender) return props.gender.charAt(0).toUpperCase() + props.gender.slice(1)
  if (route.name === 'search')     return `"${route.query.q || ''}"` 
  if (route.name === 'category')   return route.params.slug as string
  if (route.name === 'collection') return route.params.slug as string
  return 'All Products'
})

const breadcrumb = computed(() => {
  const crumbs: { label: string; to?: string }[] = [{ label: 'Home', to: '/' }]
  if (props.gender) crumbs.push({ label: pageTitle.value, to: `/${props.gender}` })
  else if (route.name === 'search')     crumbs.push({ label: 'Search' })
  else if (route.name === 'category')   crumbs.push({ label: 'Category', to: '/search' }, { label: pageTitle.value })
  else if (route.name === 'collection') crumbs.push({ label: 'Collections', to: '/collections' }, { label: pageTitle.value })
  return crumbs
})

/* ── Build API params ──────────────────────────────────── */
function buildParams() {
  const p: Record<string, unknown> = {
    page: page.value,
    page_size: PAGE_SIZE,
    sort: sortVal.value,
  }
  if (props.gender) p.gender = props.gender
  if (route.name === 'category')   p.category = route.params.slug
  if (route.name === 'collection') p.collection = route.params.slug
  if (route.name === 'search')     p.q = route.query.q
  if (minPrice.value)  p.min_price = minPrice.value
  if (maxPrice.value)  p.max_price = maxPrice.value
  if (inStockOnly.value) p.in_stock = '1'
  if (seasonVal.value)   p.season = seasonVal.value

  // Pass through any extra query params banners/collections may have added
  for (const [k, v] of Object.entries(route.query)) {
    if (['gender','category','brand','seller','min_price','max_price','in_stock','q'].includes(k)) {
      p[k] = v
    }
  }
  return p
}

async function load() {
  loading.value = true
  error.value   = ''
  setSeo({
    title: props.gender
      ? `${pageTitle.value}'s Fashion`
      : route.name === 'search'
        ? `Search ${pageTitle.value}`
        : `${pageTitle.value}`,
    description: `Shop ${pageTitle.value} on CARLITOS. Filter by price, season and availability.`,
  })
  try {
    const res    = await pub.products(buildParams())
    products.value = (res.data as ListingCard[]) || []
    total.value    = res.meta?.total || 0
  } catch (e: any) {
    error.value = 'Failed to load products. Please try again.'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  filterOpen.value = false
  // Push filter state to URL
  router.replace({
    query: {
      ...route.query,
      sort:      sortVal.value !== 'relevance' ? sortVal.value : undefined,
      min_price: minPrice.value || undefined,
      max_price: maxPrice.value || undefined,
      in_stock:  inStockOnly.value ? '1' : undefined,
      season:    seasonVal.value || undefined,
    },
  })
  load()
}

function resetFilters() {
  sortVal.value     = 'relevance'
  minPrice.value    = ''
  maxPrice.value    = ''
  inStockOnly.value = false
  seasonVal.value   = ''
  page.value        = 1
  router.replace({ query: { q: route.query.q } })
  load()
}

const totalPages = computed(() => Math.ceil(total.value / PAGE_SIZE))

function goPage(n: number) {
  page.value = n
  window.scrollTo({ top: 0, behavior: 'smooth' })
  load()
}

onMounted(load)
watch(() => [route.fullPath], () => {
  sortVal.value     = (route.query.sort as string) || 'relevance'
  minPrice.value    = (route.query.min_price as string) || ''
  maxPrice.value    = (route.query.max_price as string) || ''
  inStockOnly.value = route.query.in_stock === '1'
  seasonVal.value   = (route.query.season as string) || ''
  page.value = 1
  load()
})

const skeletonCards = Array.from({ length: PAGE_SIZE })

/* ── Filter panel (reused in sidebar + drawer) ─────────── */
</script>

<template>
  <div>
    <PageBack />

    <!-- Breadcrumb -->
    <nav class="mb-4 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <template v-for="(crumb, i) in breadcrumb" :key="i">
        <RouterLink v-if="crumb.to" :to="crumb.to" class="hover:text-ink">{{ crumb.label }}</RouterLink>
        <span v-else class="text-stone-700 font-medium">{{ crumb.label }}</span>
        <span v-if="i < breadcrumb.length - 1" class="text-stone-300">/</span>
      </template>
    </nav>

    <!-- Page heading + toolbar -->
    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-h2 capitalize">{{ pageTitle }}</h1>
        <p class="mt-0.5 text-sm text-stone-500">{{ total.toLocaleString() }} products</p>
      </div>

      <div class="flex items-center gap-2">
        <!-- Mobile filter toggle -->
        <button
          class="btn-ghost btn-sm flex items-center gap-1.5 md:hidden"
          @click="filterOpen = true"
          aria-label="Open filters"
        >
          <SlidersHorizontal :size="15" /> Filters
        </button>

        <!-- Sort dropdown -->
        <div class="relative">
          <select
            v-model="sortVal"
            class="select w-44 pr-8"
            aria-label="Sort by"
            @change="applyFilters"
          >
            <option value="relevance">Relevance</option>
            <option value="newest">Newest</option>
            <option value="price_asc">Price: Low to High</option>
            <option value="price_desc">Price: High to Low</option>
            <option value="rating">Top Rated</option>
            <option value="popularity">Most Popular</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Product-type quick filters -->
    <div class="mb-5 flex gap-2 overflow-x-auto pb-1 scrollbar-hide">
      <button
        v-for="t in TYPE_FILTERS" :key="t.slug"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', activeTypeSlug === t.slug ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="goType(t.slug)"
      >{{ t.label }}</button>
    </div>

    <div class="flex min-w-0 gap-6">

      <!-- ── DESKTOP FILTER SIDEBAR ─────────────────────── -->
      <aside class="hidden w-56 shrink-0 md:block" aria-label="Filters">
        <div class="space-y-5">
          <div class="flex items-center justify-between">
            <p class="text-sm font-semibold">Filters</p>
            <button class="text-xs text-stone-500 hover:text-ink underline" @click="resetFilters">Reset</button>
          </div>

          <!-- Price range -->
          <div>
            <p class="label">Price Range</p>
            <div class="flex items-center gap-2">
              <input v-model="minPrice" type="number" placeholder="Min" class="input w-full" min="0" />
              <span class="text-stone-400 text-sm">–</span>
              <input v-model="maxPrice" type="number" placeholder="Max" class="input w-full" min="0" />
            </div>
          </div>

          <!-- Season -->
          <div>
            <p class="label">Season</p>
            <select v-model="seasonVal" class="select w-full">
              <option value="">All seasons</option>
              <option value="summer">Summer</option>
              <option value="winter">Winter</option>
            </select>
          </div>

          <!-- In stock -->
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <input v-model="inStockOnly" type="checkbox" class="h-4 w-4 rounded border-stone-300 accent-ink" />
            <span class="text-sm text-stone-700">In stock only</span>
          </label>

          <button class="btn-primary w-full" @click="applyFilters">Apply Filters</button>
        </div>
      </aside>

      <!-- ── PRODUCT GRID ──────────────────────────────── -->
      <div class="flex-1 min-w-0">

        <!-- Error -->
        <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-6 text-center">
          <p class="text-sm text-red-600">{{ error }}</p>
          <button class="btn-ghost btn-sm mt-3" @click="load">Try again</button>
        </div>

        <!-- Skeleton -->
        <div v-else-if="loading" class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          <div v-for="(_, i) in skeletonCards" :key="i" class="overflow-hidden rounded-lg border border-stone-200">
            <div class="skeleton aspect-[3/4] w-full" />
            <div class="p-3 space-y-2">
              <div class="skeleton h-3 w-2/3 rounded" />
              <div class="skeleton h-3 w-full rounded" />
              <div class="skeleton h-3 w-1/2 rounded" />
            </div>
          </div>
        </div>

        <!-- Empty state -->
        <div v-else-if="!products.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-16">
          <Search :size="40" class="text-stone-300 mb-3" />
          <p class="text-base font-medium text-stone-700">No products found</p>
          <p class="mt-1 text-sm text-stone-500">Try adjusting your filters or search term.</p>
          <button class="btn-ghost btn-sm mt-4" @click="resetFilters">Clear filters</button>
        </div>

        <!-- Grid -->
        <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          <ProductCard v-for="p in products" :key="p.product_id" :product="p" compact />
        </div>

        <!-- Pagination -->
        <div v-if="totalPages > 1" class="mt-8 flex items-center justify-center gap-2">
          <button
            class="btn-ghost btn-sm"
            :disabled="page <= 1"
            @click="goPage(page - 1)"
            aria-label="Previous page"
          >← Prev</button>

          <template v-for="n in totalPages" :key="n">
            <button
              v-if="Math.abs(n - page) <= 2 || n === 1 || n === totalPages"
              :class="['h-8 w-8 rounded-md text-sm transition-colors', n === page ? 'bg-ink text-white' : 'text-stone-600 hover:bg-stone-100']"
              @click="goPage(n)"
              :aria-label="`Page ${n}`"
              :aria-current="n === page ? 'page' : undefined"
            >{{ n }}</button>
            <span
              v-else-if="Math.abs(n - page) === 3"
              class="text-stone-400 select-none"
            >…</span>
          </template>

          <button
            class="btn-ghost btn-sm"
            :disabled="page >= totalPages"
            @click="goPage(page + 1)"
            aria-label="Next page"
          >Next →</button>
        </div>
      </div>
    </div>

    <!-- ── MOBILE FILTER DRAWER ──────────────────────────── -->
    <transition name="overlay">
      <div v-if="filterOpen" class="fixed inset-0 z-40 bg-black/40 md:hidden" @click="filterOpen = false" />
    </transition>

    <transition name="drawer-bottom">
      <div
        v-if="filterOpen"
        class="fixed bottom-0 left-0 right-0 z-50 rounded-t-2xl bg-white p-5 shadow-lg md:hidden"
        role="dialog"
        aria-label="Filter options"
      >
        <div class="mb-4 flex items-center justify-between">
          <p class="text-base font-semibold">Filters</p>
          <button class="rounded p-1 text-stone-500 hover:bg-stone-50" @click="filterOpen = false" aria-label="Close filters">
            <X :size="20" />
          </button>
        </div>

        <div class="space-y-5">
          <div>
            <p class="label">Price Range</p>
            <div class="flex items-center gap-2">
              <input v-model="minPrice" type="number" placeholder="Min ₹" class="input" min="0" />
              <span class="text-stone-400">–</span>
              <input v-model="maxPrice" type="number" placeholder="Max ₹" class="input" min="0" />
            </div>
          </div>

          <div>
            <p class="label">Season</p>
            <select v-model="seasonVal" class="select w-full">
              <option value="">All seasons</option>
              <option value="summer">Summer</option>
              <option value="winter">Winter</option>
            </select>
          </div>

          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="inStockOnly" type="checkbox" class="h-4 w-4 rounded border-stone-300 accent-ink" />
            <span class="text-sm text-stone-700">In stock only</span>
          </label>

          <div class="flex gap-3">
            <button class="btn-ghost flex-1" @click="resetFilters">Reset</button>
            <button class="btn-primary flex-1" @click="applyFilters">Apply</button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.overlay-enter-active, .overlay-leave-active { transition: opacity 200ms ease; }
.overlay-enter-from, .overlay-leave-to { opacity: 0; }

.drawer-bottom-enter-active, .drawer-bottom-leave-active { transition: transform 250ms ease; }
.drawer-bottom-enter-from, .drawer-bottom-leave-to { transform: translateY(100%); }
</style>
