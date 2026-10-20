<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { X, Minus, Plus, Trash2, ShoppingBag, Sparkles } from 'lucide-vue-next'
import type { CartItem, ListingCard } from '@carlitos/types'
import { formatINR, resolveMediaUrl } from '@carlitos/shared'
import { MEDIA_BASE, pub } from '@/api'
import { useCartStore } from '@/stores/cart'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { REDIRECT_KEY } from '@/router'
import ProductCarousel from '@/components/ProductCarousel.vue'

const router = useRouter()
const cart = useCartStore()
const auth = useAuthStore()
const ui = useUiStore()

const items = computed(() => cart.lines)
const busy = computed(() => cart.loading)

// ── Suggestions ("You may also like") — same feed as the full cart page:
// trending products minus whatever is already in the cart.
const suggestions = ref<ListingCard[]>([])
const suggestLoading = ref(false)
async function loadSuggestions() {
  if (suggestLoading.value || suggestions.value.length) return
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

// Refresh the server cart and load suggestions whenever the drawer opens.
watch(() => ui.cartOpen, (open) => {
  if (!open) return
  if (auth.isAuthenticated) cart.refresh()
  if (items.value.length) loadSuggestions()
})
// Once the cart has items (covers the async refresh), load suggestions; drop
// them when the cart empties.
watch(() => items.value.length, (n, prev) => {
  if (n > 0 && ui.cartOpen) loadSuggestions()
  if (n === 0 && prev) suggestions.value = []
})

async function changeQty(item: CartItem, delta: number) {
  const next = item.quantity + delta
  if (next < 1) return
  await cart.updateQty(item.seller_listing_id, next)
}
async function remove(item: CartItem) {
  await cart.remove(item.seller_listing_id)
}

function goCheckout() {
  ui.closeCart()
  if (!auth.isAuthenticated) {
    sessionStorage.setItem(REDIRECT_KEY, '/checkout')
    router.push('/login')
    return
  }
  router.push('/checkout')
}
function viewFullCart() {
  ui.closeCart()
  router.push('/cart')
}
</script>

<template>
  <!-- Overlay -->
  <transition name="overlay">
    <div v-if="ui.cartOpen" class="fixed inset-0 z-40 bg-black/40" aria-hidden="true" @click="ui.closeCart()" />
  </transition>

  <!-- Panel -->
  <transition name="cart-slide">
    <aside
      v-if="ui.cartOpen"
      class="fixed inset-y-0 right-0 z-50 flex w-full max-w-md flex-col bg-white shadow-lg"
      role="dialog"
      aria-label="Shopping cart"
      aria-modal="true"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-stone-200 px-4 py-3">
        <p class="text-base font-semibold text-ink">Your Cart <span v-if="cart.count" class="text-sm font-normal text-stone-500">({{ cart.count }})</span></p>
        <button class="rounded p-1 text-stone-500 hover:bg-stone-50 hover:text-ink" aria-label="Close cart" @click="ui.closeCart()">
          <X :size="20" />
        </button>
      </div>

      <!-- Empty -->
      <div v-if="!items.length" class="flex flex-1 flex-col items-center justify-center px-6 text-center">
        <ShoppingBag :size="40" class="mb-3 text-stone-300" />
        <p class="text-base font-medium text-stone-700">Your cart is empty</p>
        <p class="mt-1 text-sm text-stone-500">Add items to get started.</p>
        <button class="btn-primary btn-sm mt-5" @click="ui.closeCart()">Continue shopping</button>
      </div>

      <!-- Items -->
      <div v-else class="flex-1 overflow-y-auto p-4">
        <div class="space-y-3" :class="{ 'pointer-events-none opacity-60': busy }">
          <div v-for="item in items" :key="item.id" class="flex gap-3 rounded-lg border border-stone-200 p-3">
            <RouterLink :to="`/product/${item.slug}`" class="block shrink-0" @click="ui.closeCart()">
              <div class="h-20 w-16 overflow-hidden rounded-md bg-stone-100">
                <img v-if="item.image" :src="resolveMediaUrl(MEDIA_BASE, item.image)" :alt="item.product_title" class="h-full w-full object-cover" loading="lazy" />
              </div>
            </RouterLink>
            <div class="flex min-w-0 flex-1 flex-col justify-between">
              <div>
                <p class="truncate text-sm font-medium text-ink">{{ item.product_title }}</p>
                <p class="mt-0.5 text-xs text-stone-500">{{ item.seller_name }}</p>
              </div>
              <div class="mt-2 flex items-center justify-between">
                <div class="flex items-center rounded-md border border-stone-300">
                  <button class="flex h-7 w-7 items-center justify-center text-stone-600 disabled:opacity-30" :disabled="item.quantity <= 1" aria-label="Decrease" @click="changeQty(item, -1)"><Minus :size="13" /></button>
                  <span class="w-8 text-center text-sm">{{ item.quantity }}</span>
                  <button class="flex h-7 w-7 items-center justify-center text-stone-600 disabled:opacity-30" :disabled="item.available_quantity > 0 && item.quantity >= item.available_quantity" aria-label="Increase" @click="changeQty(item, 1)"><Plus :size="13" /></button>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-sm font-semibold">{{ formatINR(item.line_total) }}</span>
                  <button class="text-stone-400 hover:text-red-500" aria-label="Remove" @click="remove(item)"><Trash2 :size="15" /></button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Suggestions: horizontal slider with quick add-to-cart -->
        <div v-if="suggestions.length" class="mt-6">
          <div class="mb-2 flex items-center gap-1.5">
            <Sparkles :size="14" class="text-accent" />
            <h3 class="text-xs font-semibold text-stone-700">You may also like</h3>
          </div>
          <ProductCarousel :products="suggestions" :loading="suggestLoading" add-to-cart />
        </div>
      </div>

      <!-- Footer summary -->
      <div v-if="items.length" class="border-t border-stone-200 p-4">
        <div class="mb-3 flex items-center justify-between text-sm">
          <span class="text-stone-500">Subtotal</span>
          <span class="text-base font-semibold text-ink">{{ formatINR(cart.subtotal) }}</span>
        </div>
        <button class="btn-primary w-full" @click="goCheckout">
          {{ auth.isAuthenticated ? 'Proceed to Checkout' : 'Login to Checkout' }}
        </button>
        <button class="mt-2 w-full text-center text-sm text-stone-500 underline hover:text-ink" @click="viewFullCart">
          View full cart
        </button>
      </div>
    </aside>
  </transition>
</template>

<style scoped>
.overlay-enter-active,
.overlay-leave-active { transition: opacity 200ms ease; }
.overlay-enter-from,
.overlay-leave-to { opacity: 0; }

.cart-slide-enter-active,
.cart-slide-leave-active { transition: transform 250ms ease; }
.cart-slide-enter-from,
.cart-slide-leave-to { transform: translateX(100%); }
</style>
