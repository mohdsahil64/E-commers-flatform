<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Heart, Plus, Check, Loader2 } from 'lucide-vue-next'
import type { ListingCard } from '@carlitos/types'
import { formatINR, discountPercent, resolveMediaUrl, extractApiError } from '@carlitos/shared'
import { MEDIA_BASE, pub } from '@/api'
import { useWishlistStore } from '@/stores/wishlist'
import { useToastStore } from '@/stores/toast'
import { useCartStore } from '@/stores/cart'

// `addToCart` opts a card into showing a quick add-to-cart button (used by the
// cart "You may also like" slider). Off everywhere else so existing cards are
// unchanged.
const props = defineProps<{ product: ListingCard; compact?: boolean; addToCart?: boolean }>()
const wishlist = useWishlistStore()
const toast = useToastStore()
const cart = useCartStore()

const off     = () => discountPercent(props.product.mrp, props.product.min_price)
const imgLoaded = ref(false)

const wishlisted = computed(() => wishlist.has(props.product.product_id))
const wishlisting = computed(() => !!wishlist.busy[props.product.product_id])

async function toggleWishlist(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  try {
    const now = await wishlist.toggle(wishlist.cardFromListing(props.product))
    toast.success(now ? 'Added to wishlist' : 'Removed from wishlist')
  } catch (err) {
    toast.error(extractApiError(err))
  }
}

// Quick add-to-cart. The listing card has no seller_listing_id (that lives on a
// variant), so we fetch the product's detail and add its first in-stock size —
// the same default ProductView picks. Multi-size products silently use that
// default; the user can still open the product to change size.
const adding = ref(false)
const added  = ref(false)
async function quickAdd(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  if (adding.value || added.value) return
  if (props.product.in_stock === false) { toast.error('This item is out of stock.'); return }
  adding.value = true
  try {
    const detail = await pub.product(props.product.slug)
    const variants = detail.variants || []
    const variant = variants.find(v => (v.available_quantity ?? 0) > 0) || variants[0]
    if (!variant?.seller_listing_id) { toast.error('This item is unavailable.'); return }
    await cart.add({
      seller_listing_id: variant.seller_listing_id,
      quantity: 1,
      product_title: detail.title,
      slug: detail.slug,
      image: detail.images?.[0]?.file_key || props.product.primary_image || '',
      seller_name: detail.brand_name || props.product.seller_name || 'CARLITOS',
      sku: variant.variant_code,
      unit_price: variant.selling_price ?? props.product.min_price,
      available_quantity: variant.available_quantity ?? 99,
    })
    added.value = true
    toast.success('Added to cart')
    setTimeout(() => { added.value = false }, 1500)
  } catch (err) {
    toast.error(extractApiError(err))
  } finally {
    adding.value = false
  }
}
</script>

<template>
  <RouterLink
    :to="`/product/${product.slug}`"
    class="group relative block overflow-hidden rounded-lg border border-stone-200 bg-white transition-shadow hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ink"
    :aria-label="product.title"
  >
    <!-- Image container -->
    <div class="relative aspect-[3/4] w-full overflow-hidden bg-stone-100">
      <!-- Skeleton while loading -->
      <div v-if="!imgLoaded" class="skeleton absolute inset-0" />

      <img
        v-if="product.primary_image"
        :src="resolveMediaUrl(MEDIA_BASE, product.primary_image)"
        :alt="product.title"
        class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
        loading="lazy"
        @load="imgLoaded = true"
        :class="imgLoaded ? 'opacity-100' : 'opacity-0'"
      />
      <div v-else class="flex h-full w-full items-center justify-center text-xs text-stone-400">
        No image
      </div>

      <!-- Out of stock overlay -->
      <div v-if="product.in_stock === false" class="absolute inset-0 flex items-center justify-center bg-white/60">
        <span class="rounded bg-stone-900/85 px-2.5 py-1 text-xs font-semibold text-white">Out of Stock</span>
      </div>

      <!-- Discount badge -->
      <span v-if="off() > 0 && product.in_stock !== false" class="absolute left-2 top-2 rounded bg-accent px-1.5 py-0.5 text-xs font-semibold text-white">
        {{ off() }}% off
      </span>

      <!-- Wishlist button -->
      <button
        class="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full bg-white/90 shadow-sm transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ink disabled:pointer-events-none"
        :class="wishlisted ? 'text-red-500' : 'text-stone-400 hover:text-red-500'"
        :disabled="wishlisting"
        :aria-label="wishlisted ? 'Remove from wishlist' : 'Add to wishlist'"
        :aria-pressed="wishlisted"
        @click="toggleWishlist"
      >
        <Heart :size="16" :class="[wishlisting ? 'animate-pulse' : '', wishlisted ? 'fill-current' : '']" />
      </button>

      <!-- Quick add-to-cart (only when enabled, e.g. cart suggestions) -->
      <button
        v-if="addToCart && product.in_stock !== false"
        class="absolute bottom-2 right-2 flex h-8 w-8 items-center justify-center rounded-full shadow-sm transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:pointer-events-none"
        :class="added ? 'bg-emerald-500 text-white' : 'bg-accent text-white hover:bg-accent-dark'"
        :disabled="adding"
        :aria-label="added ? 'Added to cart' : 'Add to cart'"
        @click="quickAdd"
      >
        <Loader2 v-if="adding" :size="16" class="animate-spin" />
        <Check v-else-if="added" :size="16" />
        <Plus v-else :size="16" />
      </button>
    </div>

    <!-- Info -->
    <div :class="compact ? 'p-2' : 'p-3'">
      <p class="text-xs uppercase tracking-wide text-stone-400 truncate">{{ product.brand_name || '—' }}</p>
      <p class="mt-0.5 truncate text-sm font-medium text-ink leading-snug">{{ product.title }}</p>

      <!-- Rating -->
      <div v-if="product.rating_avg > 0" class="mt-1 flex items-center gap-1">
        <span class="text-xs font-medium text-stone-700">{{ product.rating_avg.toFixed(1) }}</span>
        <div class="flex text-amber-400">
          <svg v-for="i in 5" :key="i" :class="['h-3 w-3', i <= Math.round(product.rating_avg) ? 'fill-current' : 'fill-stone-200 text-stone-200']" viewBox="0 0 20 20" aria-hidden="true">
            <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
          </svg>
        </div>
        <span class="text-xs text-stone-400">({{ product.rating_count }})</span>
      </div>

      <!-- Price row -->
      <div class="mt-1.5 flex items-baseline gap-2 flex-wrap">
        <span class="text-sm font-semibold text-ink">{{ formatINR(product.min_price) }}</span>
        <span v-if="off() > 0" class="text-xs text-stone-400 line-through">{{ formatINR(product.mrp) }}</span>
      </div>

      <p v-if="!compact" class="mt-1 text-xs text-stone-400 truncate">{{ product.seller_name }}</p>
    </div>
  </RouterLink>
</template>
