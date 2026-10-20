<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Heart, ShoppingBag, Trash2, X } from 'lucide-vue-next'
import type { WishlistItem, Product, Variant } from '@carlitos/types'
import { pub, MEDIA_BASE } from '@/api'
import { formatINR, resolveMediaUrl, extractApiError } from '@carlitos/shared'
import { useWishlistStore } from '@/stores/wishlist'
import { useCartStore } from '@/stores/cart'
import { useToastStore } from '@/stores/toast'
import PageBack from '@/components/PageBack.vue'

const wishlist = useWishlistStore()
const cart = useCartStore()
const toast = useToastStore()

const loading = ref(true)
const items = computed<WishlistItem[]>(() => wishlist.items)

// Size-picker modal (add-to-cart needs a variant / seller_listing_id).
const picking = ref<WishlistItem | null>(null)
const pickProduct = ref<Product | null>(null)
const pickVariant = ref<Variant | null>(null)
const pickLoading = ref(false)
const adding = ref(false)

async function load() {
  loading.value = true
  try {
    await wishlist.refresh()
  } finally {
    loading.value = false
  }
}

async function remove(id: string) {
  try {
    await wishlist.remove(id)
    toast.success('Removed')
  } catch (e) {
    toast.error(extractApiError(e))
  }
}

async function openPicker(item: WishlistItem) {
  picking.value = item
  pickProduct.value = null
  pickVariant.value = null
  pickLoading.value = true
  try {
    const p = await pub.product(item.slug)
    pickProduct.value = p
    pickVariant.value = p.variants?.find(v => (v.available_quantity ?? 0) > 0) || p.variants?.[0] || null
  } catch (e) {
    toast.error(extractApiError(e))
    picking.value = null
  } finally {
    pickLoading.value = false
  }
}

function sizeLabel(v: Variant) { return v.size || v.variant_code }

async function confirmAdd() {
  const p = pickProduct.value
  const v = pickVariant.value
  if (!p || !v) return
  if ((v.available_quantity ?? 0) <= 0) { toast.error('This size is out of stock.'); return }
  if (!v.seller_listing_id) { toast.error('This item is unavailable.'); return }
  adding.value = true
  try {
    await cart.add({
      seller_listing_id: v.seller_listing_id,
      quantity: 1,
      product_title: p.title,
      slug: p.slug,
      image: p.images?.[0]?.file_key || '',
      seller_name: p.brand_name || 'CARLITOS',
      sku: v.variant_code,
      unit_price: v.selling_price,
      available_quantity: v.available_quantity ?? 99,
    })
    toast.success('Added')
    picking.value = null
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    adding.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageBack history />
    <h1 class="text-h2 mb-6">Wishlist</h1>

    <!-- Loading -->
    <div v-if="loading" class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
      <div v-for="i in 8" :key="i" class="overflow-hidden rounded-lg border border-stone-200">
        <div class="skeleton aspect-[3/4] w-full" />
        <div class="space-y-2 p-3"><div class="skeleton h-3 w-3/4 rounded" /><div class="skeleton h-3 w-1/2 rounded" /></div>
      </div>
    </div>

    <!-- Empty -->
    <div v-else-if="!items.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-16">
      <Heart :size="44" class="mb-4 text-stone-300" />
      <p class="text-base font-medium text-stone-700">Your wishlist is empty</p>
      <p class="mt-1 text-sm text-stone-500">Tap the heart on any product to save it here.</p>
      <RouterLink to="/" class="btn-primary btn-sm mt-5">Start browsing</RouterLink>
    </div>

    <!-- Grid -->
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
      <div
        v-for="item in items"
        :key="item.product_id"
        class="group relative overflow-hidden rounded-lg border border-stone-200 bg-white"
      >
        <RouterLink :to="`/product/${item.slug}`" class="block">
          <div class="aspect-[3/4] w-full overflow-hidden bg-stone-100">
            <img
              v-if="item.image"
              :src="resolveMediaUrl(MEDIA_BASE, item.image)"
              :alt="item.title"
              class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
              loading="lazy"
            />
            <div v-else class="flex h-full w-full items-center justify-center text-xs text-stone-400">No image</div>
          </div>
          <div class="p-3">
            <p class="truncate text-sm font-medium text-ink">{{ item.title }}</p>
            <p class="mt-1 text-sm font-semibold">{{ formatINR(item.min_price) }}</p>
            <div v-if="item.rating_avg > 0" class="mt-1 flex items-center gap-1">
              <span class="text-xs text-stone-500">★ {{ item.rating_avg.toFixed(1) }}</span>
            </div>
          </div>
        </RouterLink>

        <!-- Remove button (always visible top-right, touch-friendly) -->
        <button
          class="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full bg-white/90 text-stone-500 shadow-sm hover:text-red-500"
          :disabled="wishlist.busy[item.product_id]"
          aria-label="Remove from wishlist"
          @click="remove(item.product_id)"
        >
          <Trash2 :size="14" />
        </button>

        <!-- Add to cart -->
        <div class="p-3 pt-0">
          <button class="btn-primary btn-sm w-full" @click="openPicker(item)">
            <ShoppingBag :size="13" /> Add to Cart
          </button>
        </div>
      </div>
    </div>

    <!-- ── Size picker modal ────────────────────────────────── -->
    <transition name="fade">
      <div v-if="picking" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="picking = null">
        <div class="w-full max-w-sm rounded-t-2xl bg-white p-5 shadow-lg sm:rounded-2xl">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-h4">Select size</h3>
            <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="picking = null"><X :size="18" /></button>
          </div>
          <p class="mb-4 truncate text-sm text-stone-600">{{ picking.title }}</p>

          <div v-if="pickLoading" class="space-y-2">
            <div class="skeleton h-9 w-full rounded" />
            <div class="skeleton h-9 w-2/3 rounded" />
          </div>

          <template v-else-if="pickProduct">
            <div v-if="pickProduct.variants && pickProduct.variants.length" class="flex flex-wrap gap-2">
              <button
                v-for="v in pickProduct.variants"
                :key="v.id"
                :disabled="(v.available_quantity ?? 0) <= 0"
                :class="[
                  'rounded-md border px-3 py-1.5 text-sm transition-colors',
                  pickVariant?.id === v.id ? 'border-ink bg-ink text-white' : 'border-stone-300 hover:border-stone-500',
                  (v.available_quantity ?? 0) <= 0 ? 'cursor-not-allowed text-stone-300 line-through opacity-60' : ''
                ]"
                @click="pickVariant = v"
              >{{ sizeLabel(v) }}</button>
            </div>
            <p v-else class="text-sm text-stone-500">This product has no available sizes right now.</p>

            <div class="mt-5 flex gap-3">
              <button class="btn-ghost flex-1" @click="picking = null">Cancel</button>
              <button class="btn-primary flex-1" :disabled="adding || !pickVariant" @click="confirmAdd">
                {{ adding ? 'Adding…' : 'Add to Bag' }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from,
.fade-leave-to { opacity: 0; }
</style>
