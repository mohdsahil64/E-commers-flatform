<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import {
  LayoutDashboard, Package, Boxes, ShoppingCart, Truck, RotateCcw,
  Wallet, Box, Store, Settings, Menu, X, LogOut, LifeBuoy,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import BrandLogo from '@/components/BrandLogo.vue'

const store  = useAuthStore()
const router = useRouter()
const route  = useRoute()

const sidebarOpen = ref(false)

const nav = [
  { label: 'Dashboard',   to: '/',           icon: LayoutDashboard, end: true },
  { label: 'Products',    to: '/products',   icon: Package },
  { label: 'Inventory',   to: '/inventory',  icon: Boxes },
  { label: 'Orders',      to: '/orders',     icon: ShoppingCart },
  { label: 'Shipments',   to: '/shipments',  icon: Truck },
  { label: 'Returns',     to: '/returns',    icon: RotateCcw },
  { label: 'Help & Support', to: '/support', icon: LifeBuoy },
  { label: 'Settlements', to: '/settlements',icon: Wallet },
  { label: 'Packaging',   to: '/packaging',  icon: Box },
]

const bottomNav = [
  { label: 'Store Profile', to: '/profile',  icon: Store },
  { label: 'Settings',      to: '/settings', icon: Settings },
]

watch(() => route.path, () => { sidebarOpen.value = false })

async function logout() {
  await store.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex min-h-screen">

    <!-- ── OVERLAY (mobile) ──────────────────────────────── -->
    <transition name="overlay">
      <div
        v-if="sidebarOpen"
        class="fixed inset-0 z-30 bg-black/40 lg:hidden"
        aria-hidden="true"
        @click="sidebarOpen = false"
      />
    </transition>

    <!-- ── SIDEBAR ───────────────────────────────────────── -->
    <aside
      :class="[
        'fixed inset-y-0 left-0 z-40 flex w-60 flex-col border-r border-stone-200 bg-white transition-transform duration-250 lg:static lg:translate-x-0',
        sidebarOpen ? 'translate-x-0' : '-translate-x-full'
      ]"
      role="navigation"
      aria-label="Seller desk navigation"
    >
      <!-- Logo -->
      <div class="flex items-center justify-between border-b border-stone-200 px-5 py-4">
        <BrandLogo :height="26" label />
        <button class="rounded p-1 text-stone-400 hover:bg-stone-50 lg:hidden" @click="sidebarOpen = false" aria-label="Close menu">
          <X :size="18" />
        </button>
      </div>

      <!-- Seller info -->
      <div v-if="store.user" class="border-b border-stone-100 px-4 py-3">
        <div class="flex items-center gap-3">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-stone-900">
            <Store :size="14" class="text-white" />
          </div>
          <div class="min-w-0">
            <p class="truncate text-xs font-semibold text-ink">{{ store.user.full_name }}</p>
            <p class="truncate text-xs text-stone-400">{{ store.user.email }}</p>
          </div>
        </div>
      </div>

      <!-- Main nav -->
      <nav class="flex-1 overflow-y-auto p-2" aria-label="Main navigation">
        <RouterLink
          v-for="n in nav"
          :key="n.to"
          :to="n.to"
          :end="n.end"
          class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-stone-600 transition-colors hover:bg-stone-50 hover:text-ink"
          active-class="bg-stone-100 text-ink font-medium"
        >
          <component :is="n.icon" :size="16" class="shrink-0" />
          {{ n.label }}
        </RouterLink>
      </nav>

      <!-- Bottom nav -->
      <div class="border-t border-stone-200 p-2">
        <RouterLink
          v-for="n in bottomNav"
          :key="n.to"
          :to="n.to"
          class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-stone-600 transition-colors hover:bg-stone-50 hover:text-ink"
          active-class="bg-stone-100 text-ink font-medium"
        >
          <component :is="n.icon" :size="16" class="shrink-0" />
          {{ n.label }}
        </RouterLink>
        <button
          class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm text-stone-600 hover:bg-stone-50 hover:text-ink"
          @click="logout"
        >
          <LogOut :size="16" class="shrink-0" /> Sign out
        </button>
      </div>
    </aside>

    <!-- ── MAIN CONTENT ──────────────────────────────────── -->
    <div class="flex flex-1 flex-col min-w-0">
      <!-- Mobile / tablet top bar -->
      <header class="flex items-center gap-3 border-b border-stone-200 bg-white px-4 py-3 lg:hidden">
        <button
          class="shrink-0 rounded-md p-1.5 text-stone-600 hover:bg-stone-50"
          aria-label="Open menu"
          @click="sidebarOpen = true"
        >
          <Menu :size="22" />
        </button>
        <div class="flex min-w-0 items-center gap-2.5">
          <BrandLogo :height="34" />
          <div class="min-w-0 border-l border-stone-200 pl-2.5">
            <p class="truncate text-sm font-semibold leading-tight text-ink">CARLITOS</p>
            <p class="truncate text-xs leading-tight text-stone-400">Seller Dashboard</p>
          </div>
        </div>
      </header>

      <!-- Page content -->
      <main class="flex-1 p-4 sm:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.overlay-enter-active, .overlay-leave-active { transition: opacity 200ms ease; }
.overlay-enter-from, .overlay-leave-to { opacity: 0; }
</style>
