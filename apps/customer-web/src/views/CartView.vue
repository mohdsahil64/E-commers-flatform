<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { Minus, Plus, Trash2, ShoppingBag, Sparkles, Heart } from 'lucide-vue-next'
import type { CartItem, ListingCard } from '@carlitos/types'
import { formatINR, extractApiError, resolveMediaUrl } from '@carlitos/shared'
import { MEDIA_BASE, pub } from '@/api'
import { useCartStore } from '@/stores/cart'
import { useWishlistStore } from '@/stores/wishlist'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { REDIRECT_KEY } from '@/router'
import PageBack from '@/components/PageBack.vue'
import ProductCarousel from '@/components/ProductCarousel.vue'

const router = useRouter()
const cart   = useCartStore()
const wishlist = useWishlistStore()
const auth   = useAuthStore()
const toast  = useToastStore()

const loading = ref(true)
const error   = ref('')
const busy    = ref<Record<string, boolean>>({})

/* ── Selection (which lines to check out) ───────────────────────────
   Keyed by seller_listing_id (stable server id). Default: everything
   selected. The user can untick items to exclude them from checkout;
   unticked items stay in the cart. */
const selected = ref<Record<string, boolean>>({})

// Keep the selection map in sync with the cart lines: new lines default
// to selected, removed lines are dropped from the map.
watch(() => cart.lines.map(l => l.seller_listing_id).join(','), () => {
  const next: Record<string, boolean> = {}
  for (const l of cart.lines) {
    next[l.seller_listing_id] = selected.value[l.seller_listing_id] ?? true
  }
  selected.value = next
}, { immediate: true })

const selectedLines = computed(() => cart.lines.filter(l => selected.value[l.seller_listing_id]))
const selectedCount = computed(() => selectedLines.value.reduce((s, i) => s + i.quantity, 0))
const selectedSubtotal = computed(() => selectedLines.value.reduce((s, i) => s + i.line_total, 0))
const allSelected = computed(() =>
  cart.lines.length > 0 && cart.lines.every(l => selected.value[l.seller_listing_id]))

function toggleItem(id: string) { selected.value[id] = !selected.value[id] }
function toggleAll() {
  const v = !allSelected.value
  for (const l of cart.lines) selected.value[l.seller_listing_id] = v
}

/* ── Suggestions ("You may also like") ──────────────────────────────
   Small recommendation cards shown under the cart. We pull trending
   products from the public listing endpoint and drop anything already
   in the cart so we only suggest new things. */
const suggestions = ref<ListingCard[]>([])
const suggestLoading = ref(false)

async function loadSuggestions() {
  suggestLoading.value = true
  try {
    const res = await pub.products({ sort: 'popular', page_size: 16 })
    const inCart = new Set(cart.lines.map(l => l.product_id).filter(Boolean))
    suggestions.value = (res.data || [])
      .filter(p => p.in_stock !== false && !inCart.has(p.product_id))
      .slice(0, 12)
  } catch {
    suggestions.value = []
  } finally {
    suggestLoading.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    await cart.refresh()
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}

async function changeQty(item: CartItem, delta: number) {
  const next = item.quantity + delta
  if (next < 1) return
  busy.value[item.id] = true
  try {
    await cart.updateQty(item.seller_listing_id, next)
  } finally {
    delete busy.value[item.id]
  }
}

async function remove(item: CartItem) {
  busy.value[item.id] = true
  try {
    await cart.remove(item.seller_listing_id)
  } finally {
    delete busy.value[item.id]
  }
}

// Move a cart line to the wishlist: add the product to the wishlist, then
// remove it from the cart. product_id is empty for guest lines (the guest cart
// snapshot doesn't carry it), so in that case we resolve it from the product.
async function moveToWishlist(item: CartItem) {
  busy.value[item.id] = true
  try {
    let productId = item.product_id
    let ratingAvg = 0
    if (!productId) {
      const detail = await pub.product(item.slug)
      productId = detail.id
      ratingAvg = detail.rating_avg || 0
    }
    await wishlist.add({
      product_id: productId,
      title: item.product_title,
      slug: item.slug,
      image: item.image,
      min_price: item.unit_price,
      rating_avg: ratingAvg,
    })
    await cart.remove(item.seller_listing_id)
    toast.success('Moved to wishlist')
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    delete busy.value[item.id]
  }
}

function goCheckout() {
  if (!selectedLines.value.length) {
    toast.error('Select at least one item to check out.')
    return
  }
  if (!auth.isAuthenticated) {
    sessionStorage.setItem(REDIRECT_KEY, '/checkout')
    router.push('/login')
    return
  }
  // Pass the selected listing ids to checkout so only those items are ordered;
  // the rest stay in the cart. If all are selected we omit the filter.
  const ids = allSelected.value
    ? []
    : selectedLines.value.map(l => l.seller_listing_id)
  router.push({ path: '/checkout', query: ids.length ? { items: ids.join(',') } : {} })
}

const items = computed(() => cart.lines)
const totalItems = computed(() => cart.count)

onMounted(async () => {
  await load()
  wishlist.refresh().catch(() => {})
  if (items.value.length) loadSuggestions()
})

// Load suggestions once the cart has items (covers the async refresh path).
watch(() => items.value.length, (n, prev) => {
  if (n > 0 && !suggestions.value.length && !suggestLoading.value) loadSuggestions()
  // If the cart just emptied, drop suggestions.
  if (n === 0 && prev) suggestions.value = []
})
</script>

<template>
  <div>
    <PageBack label="Continue shopping" />
    <h1 class="text-lg font-semibold text-ink mb-3">
      Your Cart
      <span v-if="totalItems" class="text-xs font-normal text-stone-500">({{ totalItems }} item{{ totalItems > 1 ? 's' : '' }})</span>
    </h1>

    <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-xs text-red-600">{{ error }}</div>

    <div v-else-if="loading" class="space-y-2">
      <div v-for="i in 3" :key="i" class="flex gap-3 rounded-lg border border-stone-200 p-3">
        <div class="skeleton h-20 w-16 shrink-0 rounded-md" />
        <div class="flex-1 space-y-2 pt-1">
          <div class="skeleton h-3 w-3/4 rounded" />
          <div class="skeleton h-3 w-1/2 rounded" />
          <div class="skeleton h-3 w-1/3 rounded" />
        </div>
      </div>
    </div>

    <div v-else-if="!items.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <ShoppingBag :size="36" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">Your cart is empty</p>
      <p class="mt-1 text-xs text-stone-500">Add some items to get started.</p>
      <RouterLink to="/" class="btn-primary btn-sm mt-4">Continue shopping</RouterLink>
    </div>

    <div v-else class="grid gap-4 lg:grid-cols-3">
      <!-- Items -->
      <div class="space-y-2 lg:col-span-2">
        <!-- Select-all bar -->
        <label class="flex items-center gap-2 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-xs font-medium text-stone-600 cursor-pointer select-none">
          <input
            type="checkbox"
            class="h-4 w-4 shrink-0 rounded border-stone-300 text-accent accent-accent"
            :checked="allSelected"
            @change="toggleAll"
          />
          Select all ({{ selectedCount }}/{{ totalItems }} selected)
        </label>

        <div
          v-for="item in items"
          :key="item.id"
          class="flex gap-2.5 rounded-lg border bg-white p-2.5 transition-colors"
          :class="[
            busy[item.id] ? 'opacity-60 pointer-events-none' : '',
            selected[item.seller_listing_id] ? 'border-stone-300' : 'border-stone-200 opacity-70'
          ]"
        >
          <!-- Selection checkbox -->
          <input
            type="checkbox"
            class="mt-1 h-4 w-4 shrink-0 self-start rounded border-stone-300 text-accent accent-accent"
            :checked="!!selected[item.seller_listing_id]"
            :aria-label="`Select ${item.product_title} for checkout`"
            @change="toggleItem(item.seller_listing_id)"
          />

          <RouterLink :to="`/product/${item.slug}`" class="block shrink-0">
            <div class="h-20 w-16 overflow-hidden rounded-md bg-stone-100">
              <img
                v-if="item.image"
                :src="resolveMediaUrl(MEDIA_BASE, item.image)"
                :alt="item.product_title"
                class="h-full w-full object-cover"
                loading="lazy"
              />
              <div v-else class="flex h-full w-full items-center justify-center text-[11px] text-stone-400">No image</div>
            </div>
          </RouterLink>

          <div class="flex flex-1 flex-col justify-between min-w-0">
            <div>
              <RouterLink :to="`/product/${item.slug}`" class="block">
                <p class="truncate text-xs font-medium text-ink leading-snug hover:underline">{{ item.product_title }}</p>
              </RouterLink>
              <p class="mt-0.5 text-[11px] text-stone-500">by {{ item.seller_name }}</p>
            </div>

            <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center rounded-md border border-stone-300">
                <button
                  class="flex h-7 w-7 items-center justify-center rounded-l-md text-stone-600 hover:bg-stone-50 disabled:opacity-30"
                  :disabled="item.quantity <= 1 || !!busy[item.id]"
                  aria-label="Decrease quantity"
                  @click="changeQty(item, -1)"
                ><Minus :size="13" /></button>
                <span class="w-8 text-center text-xs font-medium">{{ item.quantity }}</span>
                <button
                  class="flex h-7 w-7 items-center justify-center rounded-r-md text-stone-600 hover:bg-stone-50 disabled:opacity-30"
                  :disabled="(item.available_quantity > 0 && item.quantity >= item.available_quantity) || !!busy[item.id]"
                  aria-label="Increase quantity"
                  @click="changeQty(item, 1)"
                ><Plus :size="13" /></button>
              </div>

              <div class="flex items-center gap-2.5">
                <p class="text-xs font-semibold">{{ formatINR(item.line_total) }}</p>
                <button
                  class="flex items-center gap-1 text-[11px] text-stone-400 hover:text-accent transition-colors"
                  :disabled="!!busy[item.id]"
                  aria-label="Move to wishlist"
                  @click="moveToWishlist(item)"
                ><Heart :size="14" /><span class="hidden sm:inline">Wishlist</span></button>
                <button
                  class="text-stone-400 hover:text-red-500 transition-colors"
                  :disabled="!!busy[item.id]"
                  aria-label="Remove item"
                  @click="remove(item)"
                ><Trash2 :size="15" /></button>
              </div>
            </div>

            <p v-if="item.available_quantity > 0 && item.available_quantity <= 3" class="mt-1 text-[11px] text-red-600">
              Only {{ item.available_quantity }} left in stock
            </p>
          </div>
        </div>
      </div>

      <!-- Summary -->
      <aside class="h-fit rounded-lg border border-stone-200 bg-white p-4 lg:sticky lg:top-24">
        <h2 class="text-sm font-semibold text-ink mb-3">Order Summary</h2>
        <div class="space-y-2 text-xs">
          <div class="flex justify-between">
            <span class="text-stone-500">Selected items ({{ selectedCount }})</span>
            <span class="font-medium">{{ formatINR(selectedSubtotal) }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-stone-500">Delivery</span>
            <span class="text-emerald-600 font-medium">FREE</span>
          </div>
        </div>
        <div class="my-3 border-t border-stone-200" />
        <div class="flex justify-between text-sm font-semibold">
          <span>Total</span>
          <span>{{ formatINR(selectedSubtotal) }}</span>
        </div>
        <p class="mt-1 text-[11px] text-stone-500">Inclusive of all taxes</p>

        <button
          class="btn-primary btn-sm mt-4 w-full disabled:opacity-50"
          :disabled="!selectedLines.length"
          @click="goCheckout"
        >
          {{ auth.isAuthenticated ? `Proceed to Checkout (${selectedCount})` : 'Login to Checkout' }}
        </button>
        <RouterLink to="/" class="mt-2 block text-center text-xs text-stone-500 hover:text-ink underline">
          Continue shopping
        </RouterLink>
      </aside>
    </div>

    <!-- Suggestions: single horizontal slider (like home), with quick add-to-cart -->
    <div v-if="items.length && suggestions.length" class="mt-6">
      <div class="mb-2 flex items-center gap-1.5">
        <Sparkles :size="14" class="text-accent" />
        <h2 class="text-xs font-semibold text-stone-700">You may also like</h2>
      </div>
      <ProductCarousel :products="suggestions" :loading="suggestLoading" add-to-cart />
    </div>
  </div>
</template>
