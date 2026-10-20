<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { Store, Star, Package, SlidersHorizontal } from 'lucide-vue-next'
import type { Seller, ListingCard } from '@carlitos/types'
import { pub } from '@/api'
import { useSeo } from '@/composables/useSeo'
import ProductCard from '@/components/ProductCard.vue'
import PageBack from '@/components/PageBack.vue'

const route = useRoute()
const { set: setSeo } = useSeo()

const sellerInfo  = ref<Partial<Seller> | null>(null)
const products    = ref<ListingCard[]>([])
const total       = ref(0)
const loading     = ref(true)
const prodLoading = ref(false)
const sort        = ref('popularity')
const activeCat   = ref('')          // '' = all
const page        = ref(1)
const PAGE_SIZE   = 24

// Category chips to filter within this seller's store (gender-neutral slugs).
const storeCategories = [
  { label: 'Upperwear', slug: 'upperwear' },
  { label: 'Bottomwear', slug: 'bottomwear' },
  { label: 'Shirts', slug: 'shirts' },
  { label: 'Jeans', slug: 'jeans' },
  { label: 'Shoes', slug: 'shoes' },
  { label: 'Innerwear', slug: 'innerwear' },
]

function setCategory(slug: string) {
  activeCat.value = activeCat.value === slug ? '' : slug
  page.value = 1
  loadProducts()
}

onMounted(async () => {
  const slug = route.params.slug as string
  loading.value = true
  try {
    sellerInfo.value = await pub.sellerStore(slug).catch(() => null)
    if (sellerInfo.value?.store_name) {
      setSeo({
        title: `${sellerInfo.value.store_name} — Store`,
        description: sellerInfo.value.store_description || `Shop products from ${sellerInfo.value.store_name} on CARLITOS.`,
      })
    }
  } finally {
    loading.value = false
  }
  await loadProducts()
})

async function loadProducts() {
  prodLoading.value = true
  try {
    const params: Record<string, unknown> = {
      seller: route.params.slug,
      sort: sort.value,
      page: page.value,
      page_size: PAGE_SIZE,
    }
    if (activeCat.value) params.category = activeCat.value
    const res = await pub.products(params)
    products.value = (res.data as ListingCard[]) || []
    total.value    = res.meta?.total || 0
  } finally {
    prodLoading.value = false
  }
}

watch(sort, () => { page.value = 1; loadProducts() })

const totalPages = computed(() => Math.ceil(total.value / PAGE_SIZE))
</script>

<template>
  <div>
    <PageBack />
    <!-- Loading -->
    <div v-if="loading" class="space-y-4">
      <div class="skeleton h-32 w-full rounded-xl" />
      <div class="skeleton h-5 w-1/3 rounded" />
    </div>

    <template v-else>
      <!-- Store Header -->
      <div class="relative overflow-hidden rounded-xl bg-stone-900 p-8 mb-8">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
          <div class="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-white/10">
            <Store :size="28" class="text-white" />
          </div>
          <div>
            <h1 class="text-2xl font-semibold text-white">
              {{ sellerInfo?.store_name || route.params.slug }}
            </h1>
            <div v-if="sellerInfo?.rating_avg" class="mt-1 flex items-center gap-2">
              <div class="flex gap-0.5">
                <svg v-for="i in 5" :key="i" :class="['h-4 w-4', i <= Math.round(sellerInfo.rating_avg || 0) ? 'fill-amber-400 text-amber-400' : 'fill-white/20 text-white/20']" viewBox="0 0 20 20"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/></svg>
              </div>
              <span class="text-sm text-white/80">{{ sellerInfo.rating_avg.toFixed(1) }} ({{ sellerInfo.rating_count }} reviews)</span>
            </div>
            <p v-if="sellerInfo?.store_description" class="mt-2 text-sm text-white/70 max-w-lg">
              {{ sellerInfo.store_description }}
            </p>
          </div>
        </div>

        <!-- Stats strip -->
        <div class="mt-6 flex flex-wrap gap-6 border-t border-white/10 pt-5">
          <div class="flex items-center gap-2">
            <Package :size="16" class="text-white/60" />
            <span class="text-sm text-white/80">{{ total }} products</span>
          </div>
          <div v-if="sellerInfo?.status === 'approved'" class="flex items-center gap-2">
            <Star :size="16" class="text-amber-400" />
            <span class="text-sm text-white/80">Verified Seller</span>
          </div>
        </div>
      </div>

      <!-- Category filter chips (within this seller's store) -->
      <div class="mb-5 flex gap-2 overflow-x-auto pb-1 scrollbar-hide">
        <button
          :class="['shrink-0 whitespace-nowrap rounded-full border px-4 py-1.5 text-sm transition-colors', !activeCat ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-ink']"
          @click="setCategory('')"
        >All</button>
        <button
          v-for="c in storeCategories"
          :key="c.slug"
          :class="['shrink-0 whitespace-nowrap rounded-full border px-4 py-1.5 text-sm transition-colors', activeCat === c.slug ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-700 hover:border-ink']"
          @click="setCategory(c.slug)"
        >{{ c.label }}</button>
      </div>

      <!-- Toolbar -->
      <div class="mb-5 flex items-center justify-between">
        <p class="text-sm text-stone-500">{{ total }} products</p>
        <select v-model="sort" class="select w-44" aria-label="Sort products">
          <option value="popularity">Most Popular</option>
          <option value="newest">Newest First</option>
          <option value="price_asc">Price: Low to High</option>
          <option value="price_desc">Price: High to Low</option>
          <option value="rating">Top Rated</option>
        </select>
      </div>

      <!-- Product grid -->
      <div v-if="prodLoading" class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        <div v-for="i in 8" :key="i" class="overflow-hidden rounded-lg border border-stone-200">
          <div class="skeleton aspect-[3/4] w-full" />
          <div class="p-3 space-y-2">
            <div class="skeleton h-3 w-2/3 rounded" />
            <div class="skeleton h-3 w-full rounded" />
            <div class="skeleton h-3 w-1/2 rounded" />
          </div>
        </div>
      </div>

      <div v-else-if="!products.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-16">
        <Package :size="40" class="text-stone-300 mb-3" />
        <p class="text-base font-medium text-stone-700">No products yet</p>
        <p class="mt-1 text-sm text-stone-500">This seller hasn't listed any products.</p>
      </div>

      <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        <ProductCard v-for="p in products" :key="p.product_id" :product="p" compact />
      </div>

      <!-- Pagination -->
      <div v-if="totalPages > 1" class="mt-8 flex justify-center gap-2">
        <button class="btn-ghost btn-sm" :disabled="page<=1" @click="() => { page--; loadProducts() }">← Prev</button>
        <template v-for="n in totalPages" :key="n">
          <button
            v-if="Math.abs(n-page) <= 2 || n===1 || n===totalPages"
            :class="['h-8 w-8 rounded-md text-sm', n===page ? 'bg-ink text-white' : 'text-stone-600 hover:bg-stone-100']"
            @click="() => { page=n; loadProducts() }"
          >{{ n }}</button>
          <span v-else-if="Math.abs(n-page)===3" class="text-stone-400">…</span>
        </template>
        <button class="btn-ghost btn-sm" :disabled="page>=totalPages" @click="() => { page++; loadProducts() }">Next →</button>
      </div>
    </template>
  </div>
</template>
