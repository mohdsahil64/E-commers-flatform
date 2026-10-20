<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import {
  Search, Heart, ShoppingBag, User, Bell, ChevronDown,
  LogOut, Package, MapPin, RotateCcw, HelpCircle, Home,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { useWishlistStore } from '@/stores/wishlist'
import { useUiStore } from '@/stores/ui'
import { BRAND, BUSINESS } from '@carlitos/shared'
import Toaster from '@/components/Toaster.vue'
import BrandLogo from '@/components/BrandLogo.vue'
import CartDrawer from '@/components/CartDrawer.vue'

const store = useAuthStore()
const cart = useCartStore()
const wishlist = useWishlistStore()
const ui = useUiStore()
const router = useRouter()
const route = useRoute()

const profileOpen = ref(false)

// Cart + wishlist counts come from the unified stores (guest or server).
const cartCount = computed(() => cart.count)
const wishlistCount = computed(() => wishlist.count)

// The mobile bottom nav only appears on the home page (per product decision) so
// deeper pages get the full viewport without a persistent bar.
const showBottomNav = computed(() => route.name === 'home')

onMounted(() => { if (store.isAuthenticated) { cart.refresh(); wishlist.refresh() } })
watch(() => store.isAuthenticated, (v) => { if (v) { cart.refresh(); wishlist.refresh() } })

const navItems = [
  { label: 'Home', to: '/' },
  { label: 'Men', to: '/men' },
  { label: 'Women', to: '/women' },
  { label: 'Boys', to: '/boys' },
  { label: 'Girls', to: '/girls' },
  { label: 'New Arrivals', to: '/collections/new-arrivals' },
  { label: 'Collections', to: '/collections' },
]

function closeProfile() { profileOpen.value = false }

// Close dropdowns on route change
watch(() => route.path, () => {
  profileOpen.value = false
})

function handleOutsideClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.closest('[data-profile-dropdown]')) profileOpen.value = false
}
onMounted(() => document.addEventListener('click', handleOutsideClick))

async function logout() {
  await store.logout()
  cart.items = []
  wishlist.server = []
  router.push('/')
}
</script>

<template>
  <div class="flex min-h-screen flex-col">

    <!-- ── HEADER ─────────────────────────────────────────────── -->
    <header class="sticky top-0 z-30 border-b border-stone-200 bg-white/98 backdrop-blur-sm">

      <!-- Top row: logo + (desktop nav) + cart -->
      <div class="mx-auto flex max-w-7xl items-center gap-3 px-4 py-2.5 sm:px-6 lg:px-8">

        <!-- Logo + wordmark (left-aligned on mobile; no hamburger) -->
        <BrandLogo size="sm" class="md:hidden" />
        <BrandLogo size="md" class="hidden md:inline-flex" />

        <!-- Right icons -->
        <div class="ml-auto flex items-center gap-1">

          <!-- Search (opens dedicated search page) -->
          <RouterLink
            to="/search"
            class="flex h-9 w-9 items-center justify-center rounded-md text-stone-600 hover:bg-stone-50 hover:text-ink"
            aria-label="Search"
          >
            <Search :size="20" />
          </RouterLink>

          <!-- Wishlist (desktop/tablet only — mobile uses bottom nav) -->
          <RouterLink
            to="/wishlist"
            class="relative hidden h-9 w-9 items-center justify-center rounded-md text-stone-600 hover:bg-stone-50 hover:text-ink md:flex"
            aria-label="Wishlist"
          >
            <Heart :size="20" />
            <span
              v-if="wishlistCount > 0"
              class="absolute -right-0.5 -top-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-accent text-white"
              style="font-size:10px; font-weight:600;"
            >{{ wishlistCount > 9 ? '9+' : wishlistCount }}</span>
          </RouterLink>

          <!-- Cart (opens slide-in drawer) -->
          <button
            type="button"
            class="relative flex h-9 w-9 items-center justify-center rounded-md text-stone-600 hover:bg-stone-50 hover:text-ink"
            aria-label="Cart"
            @click="ui.openCart()"
          >
            <ShoppingBag :size="20" />
            <span
              v-if="cartCount > 0"
              class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-white"
              style="font-size:10px; font-weight:600;"
            >{{ cartCount > 99 ? '99+' : cartCount }}</span>
          </button>

          <!-- Authenticated user menu (desktop/tablet) -->
          <template v-if="store.isAuthenticated">
            <RouterLink
              to="/notifications"
              class="hidden h-9 w-9 items-center justify-center rounded-md text-stone-600 hover:bg-stone-50 hover:text-ink md:flex"
              aria-label="Notifications"
            >
              <Bell :size="20" />
            </RouterLink>

            <div class="relative hidden md:block" data-profile-dropdown>
              <button
                class="flex h-9 items-center gap-1.5 rounded-md px-2 text-stone-600 hover:bg-stone-50 hover:text-ink"
                aria-label="Account"
                @click.stop="profileOpen = !profileOpen"
              >
                <User :size="20" />
                <ChevronDown :size="14" />
              </button>

              <transition name="fade">
                <div
                  v-if="profileOpen"
                  class="absolute right-0 top-full mt-1 w-52 rounded-lg border border-stone-200 bg-white py-1 shadow-md"
                  role="menu"
                >
                  <div class="border-b border-stone-100 px-4 py-2.5">
                    <p class="text-sm font-medium text-ink">{{ store.user?.full_name }}</p>
                    <p class="truncate text-xs text-stone-500">{{ store.user?.email }}</p>
                  </div>
                  <RouterLink to="/profile" class="flex items-center gap-3 px-4 py-2 text-sm text-stone-700 hover:bg-stone-50" @click="closeProfile">
                    <User :size="15" /> My Profile
                  </RouterLink>
                  <RouterLink to="/orders" class="flex items-center gap-3 px-4 py-2 text-sm text-stone-700 hover:bg-stone-50" @click="closeProfile">
                    <Package :size="15" /> My Orders
                  </RouterLink>
                  <RouterLink to="/addresses" class="flex items-center gap-3 px-4 py-2 text-sm text-stone-700 hover:bg-stone-50" @click="closeProfile">
                    <MapPin :size="15" /> Addresses
                  </RouterLink>
                  <RouterLink to="/returns" class="flex items-center gap-3 px-4 py-2 text-sm text-stone-700 hover:bg-stone-50" @click="closeProfile">
                    <RotateCcw :size="15" /> Returns
                  </RouterLink>
                  <RouterLink to="/support" class="flex items-center gap-3 px-4 py-2 text-sm text-stone-700 hover:bg-stone-50" @click="closeProfile">
                    <HelpCircle :size="15" /> Support
                  </RouterLink>
                  <div class="mt-1 border-t border-stone-100">
                    <button
                      class="flex w-full items-center gap-3 px-4 py-2 text-sm text-stone-600 hover:bg-stone-50"
                      @click="logout"
                    >
                      <LogOut :size="15" /> Sign out
                    </button>
                  </div>
                </div>
              </transition>
            </div>
          </template>

          <RouterLink
            v-else
            to="/login"
            class="btn-primary btn-sm hidden md:inline-flex"
          >
            Sign in
          </RouterLink>
        </div>
      </div>

      <!-- Desktop gender nav bar -->
      <nav class="hidden border-t border-stone-100 md:block" aria-label="Category navigation">
        <div class="mx-auto flex max-w-7xl items-center gap-6 px-4 sm:px-6 lg:px-8">
          <RouterLink
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="border-b-2 border-transparent py-2.5 text-sm text-stone-600 transition-colors hover:text-accent"
            :active-class="item.to === '/' ? '' : 'border-accent text-accent font-medium'"
            :exact-active-class="item.to === '/' ? 'border-accent text-accent font-medium' : ''"
          >
            {{ item.label }}
          </RouterLink>
        </div>
      </nav>
    </header>

    <!-- ── MAIN ───────────────────────────────────────────────── -->
    <main
      class="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8"
      :class="showBottomNav ? 'pb-20 md:pb-6' : 'pb-6'"
    >
      <slot />
    </main>

    <!-- ── MOBILE BOTTOM NAV (home only) — icons only ── -->
    <nav
      v-if="showBottomNav"
      class="fixed bottom-0 left-0 right-0 z-20 grid grid-cols-5 items-stretch border-t border-stone-200 bg-white/98 pb-[env(safe-area-inset-bottom)] backdrop-blur-sm md:hidden"
      aria-label="Primary"
    >
      <RouterLink to="/" class="flex items-center justify-center py-3.5 text-stone-500" exact-active-class="text-accent" aria-label="Home">
        <Home :size="24" />
      </RouterLink>
      <RouterLink to="/search" class="flex items-center justify-center py-3.5 text-stone-500" active-class="text-accent" aria-label="Search">
        <Search :size="24" />
      </RouterLink>
      <RouterLink to="/wishlist" class="relative flex items-center justify-center py-3.5 text-stone-500" active-class="text-accent" aria-label="Wishlist">
        <Heart :size="24" />
        <span
          v-if="wishlistCount > 0"
          class="absolute right-[calc(50%-16px)] top-2 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-white"
          style="font-size:9px; font-weight:700;"
        >{{ wishlistCount > 9 ? '9+' : wishlistCount }}</span>
      </RouterLink>
      <button type="button" class="relative flex items-center justify-center py-3.5 text-stone-500" aria-label="Cart" @click="ui.openCart()">
        <ShoppingBag :size="24" />
        <span
          v-if="cartCount > 0"
          class="absolute right-[calc(50%-16px)] top-2 flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-white"
          style="font-size:9px; font-weight:700;"
        >{{ cartCount > 99 ? '99+' : cartCount }}</span>
      </button>
      <RouterLink :to="store.isAuthenticated ? '/profile' : '/login'" class="flex items-center justify-center py-3.5 text-stone-500" active-class="text-accent" :aria-label="store.isAuthenticated ? 'Account' : 'Login'">
        <User :size="24" />
      </RouterLink>
    </nav>

    <!-- ── FOOTER (hidden on mobile; links reused elsewhere later) ─ -->
    <footer class="hidden border-t border-stone-200 bg-stone-50 md:block">
      <div class="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <div class="grid grid-cols-2 gap-6 sm:gap-8 lg:grid-cols-4">

          <!-- Brand (full width on mobile) -->
          <div class="col-span-2 lg:col-span-1">
            <BrandLogo size="md" :to="null" />
            <p class="mt-3 text-sm leading-relaxed text-stone-500">
              A premium multi-vendor fashion &amp; lifestyle marketplace.
            </p>
            <div class="mt-4 space-y-1 text-xs text-stone-500">
              <p>{{ BUSINESS.legalName }}</p>
              <p>{{ BUSINESS.addressLine }}</p>
              <p>
                <a :href="`mailto:${BUSINESS.email}`" class="hover:text-ink">{{ BUSINESS.email }}</a>
                ·
                <a :href="`tel:${BUSINESS.phoneHref}`" class="hover:text-ink">{{ BUSINESS.phone }}</a>
              </p>
            </div>
          </div>

          <!-- Customer support -->
          <div>
            <p class="mb-3 text-xs font-semibold uppercase tracking-wide text-stone-400">Customer Support</p>
            <ul class="space-y-2">
              <li><RouterLink to="/support" class="text-sm text-stone-600 hover:text-ink">Help Centre</RouterLink></li>
              <li><RouterLink to="/orders" class="text-sm text-stone-600 hover:text-ink">Track Order</RouterLink></li>
              <li><RouterLink to="/returns" class="text-sm text-stone-600 hover:text-ink">Returns &amp; Exchanges</RouterLink></li>
              <li><RouterLink to="/shipping" class="text-sm text-stone-600 hover:text-ink">Shipping Info</RouterLink></li>
            </ul>
          </div>

          <!-- Company -->
          <div>
            <p class="mb-3 text-xs font-semibold uppercase tracking-wide text-stone-400">Company</p>
            <ul class="space-y-2">
              <li><RouterLink to="/about" class="text-sm text-stone-600 hover:text-ink">About Us</RouterLink></li>
              <li><RouterLink to="/contact" class="text-sm text-stone-600 hover:text-ink">Contact</RouterLink></li>
              <li><RouterLink to="/privacy" class="text-sm text-stone-600 hover:text-ink">Privacy Policy</RouterLink></li>
              <li><RouterLink to="/terms" class="text-sm text-stone-600 hover:text-ink">Terms &amp; Conditions</RouterLink></li>
              <li><RouterLink to="/refund" class="text-sm text-stone-600 hover:text-ink">Cancellation &amp; Refund</RouterLink></li>
            </ul>
          </div>

          <!-- Shop -->
          <div>
            <p class="mb-3 text-xs font-semibold uppercase tracking-wide text-stone-400">Shop</p>
            <ul class="space-y-2">
              <li><RouterLink to="/men" class="text-sm text-stone-600 hover:text-ink">Men</RouterLink></li>
              <li><RouterLink to="/women" class="text-sm text-stone-600 hover:text-ink">Women</RouterLink></li>
              <li><RouterLink to="/collections/new-arrivals" class="text-sm text-stone-600 hover:text-ink">New Arrivals</RouterLink></li>
              <li><RouterLink to="/collections" class="text-sm text-stone-600 hover:text-ink">Collections</RouterLink></li>
            </ul>
          </div>
        </div>

        <div class="mt-8 flex flex-col items-start justify-between gap-4 border-t border-stone-200 pt-6 sm:flex-row sm:items-center">
          <p class="text-xs text-stone-400">© {{ new Date().getFullYear() }} {{ BRAND }}. All rights reserved.</p>
          <div class="flex gap-4">
            <RouterLink to="/privacy" class="text-xs text-stone-500 hover:text-ink">Privacy Policy</RouterLink>
            <RouterLink to="/terms" class="text-xs text-stone-500 hover:text-ink">Terms &amp; Conditions</RouterLink>
            <RouterLink to="/refund" class="text-xs text-stone-500 hover:text-ink">Cancellation &amp; Refund</RouterLink>
            <RouterLink to="/shipping" class="text-xs text-stone-500 hover:text-ink">Shipping</RouterLink>
            <RouterLink to="/cookies" class="text-xs text-stone-500 hover:text-ink">Cookie Policy</RouterLink>
          </div>
        </div>
      </div>
    </footer>

    <!-- Global cart drawer + toast stack -->
    <CartDrawer />
    <Toaster />
  </div>
</template>

<style scoped>
.drawer-enter-active,
.drawer-leave-active { transition: transform 250ms ease; }
.drawer-enter-from,
.drawer-leave-to { transform: translateX(-100%); }

.overlay-enter-active,
.overlay-leave-active { transition: opacity 200ms ease; }
.overlay-enter-from,
.overlay-leave-to { opacity: 0; }

.fade-enter-active,
.fade-leave-active { transition: opacity 150ms ease, transform 150ms ease; }
.fade-enter-from,
.fade-leave-to { opacity: 0; transform: translateY(-4px); }
</style>
