<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import {
  Heart, ShoppingBag, ChevronLeft, ChevronRight,
  Star, Truck, RotateCcw, Store, ShieldCheck, BadgeCheck,
} from 'lucide-vue-next'
import type { Product, Variant, ListingCard } from '@carlitos/types'
import { pub, customer, MEDIA_BASE } from '@/api'
import { formatINR, resolveMediaUrl, extractApiError, discountPercent } from '@carlitos/shared'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { useWishlistStore } from '@/stores/wishlist'
import { useToastStore } from '@/stores/toast'
import { useSeo } from '@/composables/useSeo'
import ProductCarousel from '@/components/ProductCarousel.vue'
import PageBack from '@/components/PageBack.vue'

const route  = useRoute()
const router = useRouter()
const store  = useAuthStore()
const cart   = useCartStore()
const wishlist = useWishlistStore()
const toast  = useToastStore()
const { set: setSeo } = useSeo()

interface ReviewRow {
  rating?: number
  title?: string
  body?: string
  review_text?: string
  is_verified_purchase?: boolean
  author_name?: string
  customer_name?: string
  created_at?: string
  photos?: string[]
}

const product     = ref<Product | null>(null)
const reviews     = ref<ReviewRow[]>([])
const related     = ref<ListingCard[]>([])
const loading     = ref(true)
const addingCart  = ref(false)
const selectedVariant = ref<Variant | null>(null)
const activeImg   = ref(0)
const pincode     = ref('')
const deliveryMsg = ref('')

// Real seller identity for this product (resolved from the storefront listing,
// since the product-detail payload only carries created_by_seller as a UUID).
const seller = ref<{ name: string; slug: string } | null>(null)

async function loadProduct(slug: string) {
  loading.value = true
  seller.value = null
  reviews.value = []
  related.value = []
  activeImg.value = 0
  try {
    const p = await pub.product(slug)
    product.value = p
    // SEO: title reads like a real marketplace product page.
    setSeo({
      title: `${p.brand_name ? p.brand_name + ' ' : ''}${p.title} - Buy Online`,
      description: (p.description || `Buy ${p.title} online at CARLITOS.`).slice(0, 160),
    })
    if (p.variants?.length) {
      // Default to the first in-stock size, else the first size.
      selectedVariant.value = p.variants.find(v => (v.available_quantity ?? 0) > 0) || p.variants[0]
    } else {
      selectedVariant.value = null
    }

    await Promise.all([
      // Reviews for this product.
      p.id
        ? pub.productReviews(p.id).then(r => { reviews.value = (r as ReviewRow[]) || [] }).catch(() => {})
        : Promise.resolve(),
      // Resolve the real seller by matching this product in the listing feed
      // (the product-detail payload carries only created_by_seller UUID).
      pub.products({ q: p.title, page_size: 10 })
        .then(res => {
          const card = (res.data as ListingCard[] | undefined)?.find(c => c.product_id === p.id)
          if (card) seller.value = { name: card.seller_name, slug: card.seller_slug }
        })
        .catch(() => {}),
      // Related / "You might also like": popular products, current one removed.
      pub.products({ sort: 'popularity', page_size: 12 })
        .then(res => {
          related.value = ((res.data as ListingCard[]) || []).filter(c => c.product_id !== p.id).slice(0, 8)
        })
        .catch(() => {}),
    ])
  } catch {
    product.value = null
  } finally {
    loading.value = false
  }
}

onMounted(() => loadProduct(route.params.slug as string))
watch(() => route.params.slug, (slug) => { if (slug) loadProduct(slug as string) })

const wishlisted  = computed(() => (product.value ? wishlist.has(product.value.id) : false))
const wishlisting = computed(() => (product.value ? !!wishlist.busy[product.value.id] : false))

/* ── Images ─────────────────────────────────────────────── */
const images = computed(() => product.value?.images?.sort((a, b) => a.sort_order - b.sort_order) || [])
const primaryImage = computed(() => {
  const img = images.value[activeImg.value]
  return img ? resolveMediaUrl(MEDIA_BASE, img.file_key) : ''
})
function nextImg() { if (images.value.length) activeImg.value = (activeImg.value + 1) % images.value.length }
function prevImg() { if (images.value.length) activeImg.value = (activeImg.value - 1 + images.value.length) % images.value.length }
function selectImg(i: number) { activeImg.value = i }

// Swipe support for the main image on touch devices.
let touchStartX = 0
function onImgTouchStart(e: TouchEvent) { touchStartX = e.changedTouches[0].screenX }
function onImgTouchEnd(e: TouchEvent) {
  const dx = e.changedTouches[0].screenX - touchStartX
  if (Math.abs(dx) > 40) (dx < 0 ? nextImg() : prevImg())
}

/* ── Price ──────────────────────────────────────────────── */
const displayPrice = computed(() => selectedVariant.value?.selling_price ?? product.value?.variants?.[0]?.selling_price ?? 0)
const displayMRP   = computed(() => selectedVariant.value?.mrp ?? product.value?.variants?.[0]?.mrp ?? 0)
const off          = computed(() => discountPercent(displayMRP.value, displayPrice.value))

/* ── Stock ──────────────────────────────────────────────── */
// Below this (inclusive) a size is "low stock" and shows a small red count.
const LOW_STOCK_THRESHOLD = 3
// Product-level out of stock = every variant has zero stock.
const outOfStock = computed(() => (product.value?.available_quantity ?? 1) <= 0)
// Selected size's own stock.
const selectedStock = computed(() => selectedVariant.value?.available_quantity ?? 0)
const selectedOutOfStock = computed(() => selectedStock.value <= 0)
function sizeLabel(v: Variant): string {
  return v.size || v.variant_code
}
// A size shows a red low-stock number when it has 1..3 units left.
function lowStockCount(v: Variant): number {
  const q = v.available_quantity ?? 0
  return q > 0 && q <= LOW_STOCK_THRESHOLD ? q : 0
}

/* ── Actions ────────────────────────────────────────────── */
async function addToCart(buyNow = false) {
  const variant = selectedVariant.value
  if (!variant) { toast.error('Please select a size.'); return }
  if (selectedOutOfStock.value) { toast.error('This size is out of stock.'); return }
  if (!variant.seller_listing_id) { toast.error('This item is unavailable.'); return }
  addingCart.value = true
  try {
    await cart.add({
      seller_listing_id: variant.seller_listing_id,
      quantity: 1,
      product_title: product.value!.title,
      slug: product.value!.slug,
      image: images.value[0] ? images.value[0].file_key : '',
      seller_name: product.value!.brand_name || 'CARLITOS',
      sku: variant.variant_code,
      unit_price: displayPrice.value,
      available_quantity: variant.available_quantity ?? 99,
    })
    if (buyNow) {
      router.push('/checkout')
      return
    }
    toast.success('Added to bag')
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    addingCart.value = false
  }
}

async function addToWishlist() {
  if (!product.value) return
  try {
    const now = await wishlist.toggle({
      product_id: product.value.id,
      title: product.value.title,
      slug: product.value.slug,
      image: images.value[0]?.file_key || '',
      min_price: displayPrice.value,
      rating_avg: product.value.rating_avg || 0,
    })
    toast.success(now ? 'Added to wishlist' : 'Removed from wishlist')
  } catch (e) {
    toast.error(extractApiError(e))
  }
}

async function checkDelivery() {
  if (!pincode.value.trim() || !/^\d{6}$/.test(pincode.value)) {
    deliveryMsg.value = 'Enter a valid 6-digit pincode.'
    return
  }
  deliveryMsg.value = 'Estimated delivery in 3–5 business days.'
}

/* ── Rating helpers ─────────────────────────────────────── */
const ratingStars = (r: number) => Math.round(r)
const avgRating   = computed(() => product.value?.rating_avg || 0)

/* ── Reviews preview + "view all" modal ─────────────────── */
// Show at least 5 reviews inline (6 when available); the rest open in a modal.
const PREVIEW_REVIEWS = 5
const previewReviews = computed(() => reviews.value.slice(0, PREVIEW_REVIEWS))
const hasMoreReviews = computed(() => reviews.value.length > PREVIEW_REVIEWS)
const allReviewsOpen = ref(false)
function openAllReviews() { allReviewsOpen.value = true }
function closeAllReviews() { allReviewsOpen.value = false }
</script>

<template>
  <!-- Loading skeleton -->
  <div v-if="loading" class="grid gap-10 md:grid-cols-2">
    <div class="skeleton aspect-[3/4] w-full rounded-xl" />
    <div class="space-y-4 pt-2">
      <div class="skeleton h-4 w-1/3 rounded" />
      <div class="skeleton h-7 w-2/3 rounded" />
      <div class="skeleton h-5 w-1/4 rounded" />
      <div class="skeleton h-10 w-full rounded-md" />
      <div class="skeleton h-10 w-full rounded-md" />
    </div>
  </div>

  <!-- Not found -->
  <div v-else-if="!product" class="py-20 text-center">
    <p class="text-base font-medium text-stone-700">Product not found.</p>
    <RouterLink to="/" class="btn-ghost btn-sm mt-4 inline-flex">Back to Home</RouterLink>
  </div>

  <!-- Product page -->
  <div v-else>
    <PageBack />

    <div class="grid gap-6 lg:grid-cols-2 lg:gap-8">

      <!-- ── LEFT: Image Gallery (Myntra-style) ─────────── -->
      <div class="flex flex-col-reverse gap-3 sm:flex-row">
        <!-- Thumbnail rail: horizontal on mobile, vertical on desktop -->
        <div
          v-if="images.length > 1"
          class="flex gap-2 overflow-x-auto pb-1 sm:flex-col sm:overflow-y-auto sm:pb-0"
        >
          <button
            v-for="(img, i) in images"
            :key="img.id"
            :class="['h-16 w-12 shrink-0 overflow-hidden rounded-md border-2 transition-all', i === activeImg ? 'border-ink opacity-100' : 'border-transparent opacity-70 hover:opacity-100']"
            :aria-label="`View image ${i + 1}`"
            @mouseenter="selectImg(i)"
            @click="selectImg(i)"
          >
            <img
              :src="resolveMediaUrl(MEDIA_BASE, img.file_key)"
              :alt="img.alt_text || product.title"
              class="h-full w-full object-cover"
              loading="lazy"
            />
          </button>
        </div>

        <!-- Main image -->
        <div
          class="relative aspect-[3/4] flex-1 overflow-hidden rounded-xl bg-stone-100"
          @touchstart.passive="onImgTouchStart"
          @touchend.passive="onImgTouchEnd"
        >
          <transition name="img-fade" mode="out-in">
            <img
              v-if="primaryImage"
              :key="activeImg"
              :src="primaryImage"
              :alt="images[activeImg]?.alt_text || product.title"
              class="h-full w-full object-cover"
              loading="eager"
            />
            <div v-else class="flex h-full w-full items-center justify-center text-sm text-stone-400">No image</div>
          </transition>

          <!-- Nav arrows (desktop) -->
          <template v-if="images.length > 1">
            <button
              class="absolute left-2 top-1/2 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/90 text-ink shadow hover:bg-white sm:flex"
              aria-label="Previous image"
              @click="prevImg"
            ><ChevronLeft :size="18" /></button>
            <button
              class="absolute right-2 top-1/2 hidden h-9 w-9 -translate-y-1/2 items-center justify-center rounded-full bg-white/90 text-ink shadow hover:bg-white sm:flex"
              aria-label="Next image"
              @click="nextImg"
            ><ChevronRight :size="18" /></button>

            <!-- Position dots (mobile) + counter -->
            <div class="absolute bottom-3 left-1/2 flex -translate-x-1/2 gap-1.5 sm:hidden">
              <span
                v-for="(_, i) in images"
                :key="i"
                :class="['h-1.5 rounded-full transition-all', i === activeImg ? 'w-4 bg-ink' : 'w-1.5 bg-stone-300']"
              />
            </div>
            <span class="absolute right-2 top-2 rounded-full bg-black/50 px-2 py-0.5 text-xs font-medium text-white">
              {{ activeImg + 1 }}/{{ images.length }}
            </span>
          </template>
        </div>
      </div>

      <!-- ── RIGHT: Product Info ──────────────────────── -->
      <div class="flex flex-col gap-3">
        <!-- Brand + title + seller -->
        <div>
          <div class="flex items-center gap-2">
            <p class="text-xs font-medium uppercase tracking-widest text-stone-400">{{ product.brand_name || 'CARLITOS' }}</p>
            <RouterLink v-if="seller" :to="`/seller/${seller.slug}`" class="text-xs text-accent hover:underline">· {{ seller.name }}</RouterLink>
          </div>
          <h1 class="mt-1 text-lg font-semibold leading-snug text-ink lg:text-2xl">{{ product.title }}</h1>
        </div>

        <!-- Rating -->
        <div v-if="product.rating_count > 0" class="flex items-center gap-2">
          <div class="flex items-center gap-0.5">
            <svg
              v-for="i in 5"
              :key="i"
              :class="['h-4 w-4', i <= ratingStars(avgRating) ? 'fill-amber-400 text-amber-400' : 'fill-stone-200 text-stone-200']"
              viewBox="0 0 20 20"
              aria-hidden="true"
            >
              <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/>
            </svg>
          </div>
          <span class="text-sm font-medium">{{ avgRating.toFixed(1) }}</span>
          <span class="text-sm text-stone-500">({{ product.rating_count }} reviews)</span>
        </div>

        <!-- Price -->
        <div class="flex items-baseline gap-3">
          <span class="text-2xl font-semibold text-ink">{{ formatINR(displayPrice) }}</span>
          <span v-if="off > 0" class="text-base text-stone-400 line-through">{{ formatINR(displayMRP) }}</span>
          <span v-if="off > 0" class="text-sm font-semibold text-emerald-600">{{ off }}% off</span>
        </div>

        <!-- Stock status (reflects the selected size) -->
        <div v-if="outOfStock" class="inline-flex w-fit items-center gap-1.5 rounded-md bg-red-50 px-2.5 py-1 text-sm font-medium text-red-600">
          Out of Stock
        </div>
        <template v-else>
          <div v-if="selectedOutOfStock" class="inline-flex w-fit items-center gap-1.5 rounded-md bg-red-50 px-2.5 py-1 text-sm font-medium text-red-600">
            Selected size out of stock
          </div>
          <div v-else-if="selectedStock > 0 && selectedStock <= LOW_STOCK_THRESHOLD" class="inline-flex w-fit items-center gap-1.5 rounded-md bg-red-50 px-2.5 py-1 text-sm font-medium text-red-600">
            Only {{ selectedStock }} left
          </div>
        </template>

        <!-- Size selector -->
        <div v-if="product.variants && product.variants.length">
          <p class="label">Select Size</p>
          <div class="flex flex-wrap gap-2.5 pt-1">
            <button
              v-for="v in product.variants"
              :key="v.id"
              :disabled="(v.available_quantity ?? 0) <= 0"
              :class="[
                'relative rounded-md border px-3 py-1.5 text-sm transition-colors',
                selectedVariant?.id === v.id ? 'border-ink bg-ink text-white' : 'border-stone-300 hover:border-stone-500',
                (v.available_quantity ?? 0) <= 0 ? 'cursor-not-allowed text-stone-300 line-through opacity-60' : ''
              ]"
              @click="selectedVariant = v"
            >
              {{ sizeLabel(v) }}
              <!-- Low-stock indicator: small red number when 1..3 left for this size. -->
              <span
                v-if="lowStockCount(v)"
                class="absolute -right-1.5 -top-1.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold leading-none text-white shadow-sm"
                :title="`${lowStockCount(v)} left`"
                aria-hidden="true"
              >{{ lowStockCount(v) }}</span>
            </button>
          </div>
        </div>

        <!-- Delivery checker (compact) -->
        <div class="rounded-lg border border-stone-200 px-3 py-2">
          <div class="flex items-center gap-2">
            <Truck :size="15" class="shrink-0 text-stone-400" />
            <input
              v-model="pincode"
              type="text"
              inputmode="numeric"
              placeholder="Check delivery pincode"
              maxlength="6"
              class="input h-8 flex-1 py-1 text-sm"
              aria-label="Pincode"
            />
            <button class="btn-ghost btn-sm h-8 shrink-0 py-1" @click="checkDelivery">Check</button>
          </div>
          <p v-if="deliveryMsg" class="mt-1.5 text-xs text-stone-600">{{ deliveryMsg }}</p>
        </div>

        <!-- Actions -->
        <div class="flex gap-3">
          <button
            class="btn-ghost flex-1"
            :disabled="addingCart || outOfStock || selectedOutOfStock"
            @click="addToCart(false)"
          >
            <ShoppingBag :size="16" />
            {{ outOfStock ? 'Out of Stock' : (addingCart ? 'Adding…' : 'Add to Bag') }}
          </button>
          <button
            class="btn-primary flex-1"
            :disabled="addingCart || outOfStock || selectedOutOfStock"
            @click="addToCart(true)"
          >
            Buy Now
          </button>
          <button
            class="btn-ghost !w-11 shrink-0 !px-0"
            :class="wishlisted ? '!text-red-500 !border-red-200' : ''"
            :disabled="wishlisting"
            :aria-label="wishlisted ? 'Remove from wishlist' : 'Add to wishlist'"
            :aria-pressed="wishlisted"
            @click="addToWishlist"
          >
            <Heart :size="16" :class="[wishlisting ? 'animate-pulse' : '', wishlisted ? 'fill-current' : '']" />
          </button>
        </div>

        <!-- Trusted badges -->
        <div class="mt-1 grid grid-cols-2 gap-2 sm:grid-cols-4">
          <div class="flex items-center gap-2 rounded-lg border border-stone-200 px-3 py-2">
            <ShieldCheck :size="18" class="shrink-0 text-emerald-600" />
            <span class="text-[11px] font-medium leading-tight text-stone-600">100% Secure Payments</span>
          </div>
          <div class="flex items-center gap-2 rounded-lg border border-stone-200 px-3 py-2">
            <RotateCcw :size="18" class="shrink-0 text-stone-500" />
            <span class="text-[11px] font-medium leading-tight text-stone-600">7-Day Easy Returns</span>
          </div>
          <div class="flex items-center gap-2 rounded-lg border border-stone-200 px-3 py-2">
            <BadgeCheck :size="18" class="shrink-0 text-stone-500" />
            <span class="text-[11px] font-medium leading-tight text-stone-600">100% Genuine Product</span>
          </div>
          <div class="flex items-center gap-2 rounded-lg border border-stone-200 px-3 py-2">
            <Truck :size="18" class="shrink-0 text-stone-500" />
            <span class="text-[11px] font-medium leading-tight text-stone-600">Free Delivery ₹499+</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ── PRODUCT DETAILS SECTION ──────────────────────── -->
    <div class="mt-10 grid gap-8 lg:grid-cols-3">

      <div class="lg:col-span-3 space-y-6">

        <!-- Description -->
        <section>
          <h2 class="text-h4 mb-2">Product Description</h2>
          <div class="whitespace-pre-line text-sm leading-relaxed text-stone-600">
            {{ product.description || 'No description available.' }}
          </div>
        </section>

        <!-- Seller card (ABOVE reviews) — compact -->
        <section>
          <h2 class="text-h4 mb-2">Sold By</h2>
          <div class="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-stone-200 px-3 py-2.5">
            <component
              :is="seller ? RouterLink : 'div'"
              :to="seller ? `/seller/${seller.slug}` : undefined"
              class="group flex min-w-0 items-center gap-2"
            >
              <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-stone-100">
                <Store :size="16" class="text-stone-500" />
              </div>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-ink group-hover:underline">
                  {{ seller ? seller.name : 'A CARLITOS seller' }}
                </p>
                <span class="text-xs text-stone-500">{{ seller ? 'Visit store →' : 'Verified Seller' }}</span>
              </div>
            </component>
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-stone-500 sm:ml-auto">
              <span class="flex items-center gap-1"><Truck :size="13" class="text-stone-400" /> Free ₹499+</span>
              <span class="flex items-center gap-1"><RotateCcw :size="13" class="text-stone-400" /> 7-day returns</span>
              <span class="flex items-center gap-1"><ShieldCheck :size="13" class="text-stone-400" /> Protected</span>
            </div>
          </div>
        </section>

        <!-- Reviews (BELOW seller) — compact -->
        <section>
          <div class="mb-2 flex items-center justify-between">
            <h2 class="text-h4">Reviews &amp; Ratings</h2>
            <button
              v-if="reviews.length"
              class="text-xs font-medium text-accent hover:underline"
              @click="openAllReviews"
            >View all ({{ reviews.length }})</button>
          </div>

          <div v-if="!reviews.length" class="rounded-lg border border-stone-200 bg-stone-50 p-4 text-center">
            <Star :size="22" class="mx-auto mb-1.5 text-stone-300" />
            <p class="text-sm text-stone-500">No reviews yet. Buy this product to leave the first review.</p>
          </div>

          <div v-else class="space-y-2.5">
            <!-- Summary bar (compact, horizontal) -->
            <div class="flex items-center gap-3 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2">
              <span class="text-xl font-semibold text-ink">{{ avgRating.toFixed(1) }}</span>
              <div class="flex gap-0.5">
                <svg v-for="i in 5" :key="i" :class="['h-3.5 w-3.5', i <= ratingStars(avgRating) ? 'fill-amber-400 text-amber-400' : 'fill-stone-200 text-stone-200']" viewBox="0 0 20 20"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/></svg>
              </div>
              <span class="text-xs text-stone-500">{{ product.rating_count }} reviews</span>
            </div>

            <!-- Review preview list (at least 5) -->
            <div class="space-y-2.5">
              <div v-for="(rv, i) in previewReviews" :key="i" class="rounded-lg border border-stone-200 p-3">
                <div class="mb-1 flex items-center gap-2">
                  <div class="flex gap-0.5">
                    <svg v-for="s in 5" :key="s" :class="['h-3 w-3', s <= (rv.rating || 0) ? 'fill-amber-400 text-amber-400' : 'fill-stone-200 text-stone-200']" viewBox="0 0 20 20"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/></svg>
                  </div>
                  <span v-if="rv.is_verified_purchase" class="badge-green text-xs">Verified Purchase</span>
                </div>
                <p v-if="rv.title" class="text-sm font-medium text-ink">{{ rv.title }}</p>
                <p class="mt-0.5 text-sm text-stone-600">{{ rv.body || rv.review_text || '' }}</p>
                <div v-if="rv.photos && rv.photos.length" class="mt-2 flex flex-wrap gap-2">
                  <a
                    v-for="(ph, pi) in rv.photos"
                    :key="pi"
                    :href="resolveMediaUrl(MEDIA_BASE, ph)"
                    target="_blank"
                    rel="noopener"
                    class="block h-14 w-14 overflow-hidden rounded-md border border-stone-200"
                  >
                    <img :src="resolveMediaUrl(MEDIA_BASE, ph)" alt="Review photo" class="h-full w-full object-cover" loading="lazy" />
                  </a>
                </div>
                <p v-if="rv.customer_name || rv.author_name" class="mt-1.5 text-xs text-stone-400">— {{ rv.customer_name || rv.author_name }}</p>
              </div>
            </div>

            <!-- View all reviews button (opens modal with the full list) -->
            <button
              v-if="hasMoreReviews"
              class="btn-ghost btn-sm w-full"
              @click="openAllReviews"
            >View all {{ reviews.length }} reviews</button>
          </div>
        </section>
      </div>
    </div>

    <!-- Related products -->
    <section v-if="related.length" class="mt-12">
      <h2 class="text-h3 mb-4">You Might Also Like</h2>
      <ProductCarousel :products="related" />
    </section>

    <!-- ── All reviews modal ────────────────────────────── -->
    <transition name="fade">
      <div
        v-if="allReviewsOpen"
        class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
        @click.self="closeAllReviews"
      >
        <div class="flex max-h-[90vh] w-full max-w-lg flex-col rounded-t-2xl bg-white shadow-lg sm:rounded-2xl">
          <!-- Header -->
          <div class="flex items-center justify-between border-b border-stone-100 px-5 py-4">
            <div class="flex items-center gap-2">
              <h3 class="text-base font-semibold text-ink">Reviews &amp; Ratings</h3>
              <span class="flex items-center gap-1 text-sm text-stone-500">
                <svg class="h-3.5 w-3.5 fill-amber-400 text-amber-400" viewBox="0 0 20 20"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/></svg>
                {{ avgRating.toFixed(1) }} · {{ product.rating_count }}
              </span>
            </div>
            <button class="rounded px-2 py-1 text-xl leading-none text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="closeAllReviews">&times;</button>
          </div>

          <!-- Scrollable full list -->
          <div class="flex-1 space-y-2.5 overflow-y-auto px-5 py-4">
            <div v-for="(rv, i) in reviews" :key="i" class="rounded-lg border border-stone-200 p-3">
              <div class="mb-1 flex items-center gap-2">
                <div class="flex gap-0.5">
                  <svg v-for="s in 5" :key="s" :class="['h-3 w-3', s <= (rv.rating || 0) ? 'fill-amber-400 text-amber-400' : 'fill-stone-200 text-stone-200']" viewBox="0 0 20 20"><path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"/></svg>
                </div>
                <span v-if="rv.is_verified_purchase" class="badge-green text-xs">Verified Purchase</span>
              </div>
              <p v-if="rv.title" class="text-sm font-medium text-ink">{{ rv.title }}</p>
              <p class="mt-0.5 text-sm text-stone-600">{{ rv.body || rv.review_text || '' }}</p>
              <div v-if="rv.photos && rv.photos.length" class="mt-2 flex flex-wrap gap-2">
                <a
                  v-for="(ph, pi) in rv.photos"
                  :key="pi"
                  :href="resolveMediaUrl(MEDIA_BASE, ph)"
                  target="_blank"
                  rel="noopener"
                  class="block h-14 w-14 overflow-hidden rounded-md border border-stone-200"
                >
                  <img :src="resolveMediaUrl(MEDIA_BASE, ph)" alt="Review photo" class="h-full w-full object-cover" loading="lazy" />
                </a>
              </div>
              <p v-if="rv.customer_name || rv.author_name" class="mt-1.5 text-xs text-stone-400">— {{ rv.customer_name || rv.author_name }}</p>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
.img-fade-enter-active, .img-fade-leave-active { transition: opacity 0.2s ease; }
.img-fade-enter-from, .img-fade-leave-to { opacity: 0; }
</style>
