<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import {
  LayoutDashboard, Store, ShieldCheck, Users, Package, PackageCheck,
  FolderTree, Tag, Sliders, ShoppingCart, Truck, RotateCcw, RefreshCcw,
  CreditCard, Wallet, Ticket, Megaphone, Layers, Image, Box, BarChart3,
  UserCog, KeyRound, ScrollText, Settings, Menu, X, LogOut, LifeBuoy,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { BRAND } from '@carlitos/shared'

const store  = useAuthStore()
const router = useRouter()
const route  = useRoute()

const sidebarOpen = ref(false)

const groups = [
  {
    title: 'Overview',
    links: [{ label: 'Dashboard', to: '/', icon: LayoutDashboard, end: true }],
  },
  {
    title: 'Marketplace',
    links: [
      { label: 'Sellers',           to: '/sellers',    icon: Store },
      { label: 'Customers',         to: '/customers',  icon: Users },
      { label: 'Products',          to: '/products',   icon: Package },
      { label: 'Categories',        to: '/categories', icon: FolderTree },
    ],
  },
  {
    title: 'Operations',
    links: [
      { label: 'Orders',     to: '/orders',    icon: ShoppingCart },
      { label: 'Shipping',   to: '/shipping',  icon: Truck },
      { label: 'Returns',    to: '/returns',   icon: RotateCcw },
      { label: 'Refunds',    to: '/refunds',   icon: RefreshCcw },
      { label: 'Help & Support', to: '/support', icon: LifeBuoy },
      { label: 'Packaging',  to: '/packaging', icon: Box },
    ],
  },
  {
    title: 'Finance',
    links: [
      { label: 'Payments',    to: '/payments',    icon: CreditCard },
      { label: 'Settlements', to: '/settlements',  icon: Wallet },
      { label: 'Reports',     to: '/reports',     icon: BarChart3 },
    ],
  },
  {
    title: 'Merchandising',
    links: [
      { label: 'Collections', to: '/collections', icon: Layers },
      { label: 'Banners',     to: '/banners',     icon: Image },
      { label: 'Team & Management', to: '/team',  icon: Users },
      { label: 'Coupons',     to: '/coupons',     icon: Ticket },
      { label: 'Promotions',  to: '/promotions',  icon: Megaphone },
    ],
  },
  {
    title: 'System',
    links: [
      { label: 'Users',      to: '/users',      icon: UserCog },
      { label: 'Roles',      to: '/roles',      icon: KeyRound },
      { label: 'Audit Logs', to: '/audit-logs', icon: ScrollText },
      { label: 'Settings',   to: '/settings',   icon: Settings },
    ],
  },
]

watch(() => route.path, () => { sidebarOpen.value = false })

async function logout() {
  await store.logout()
  router.push('/login')
}
</script>

<template>
  <div class="flex min-h-screen w-full max-w-full overflow-x-hidden">

    <!-- Overlay (mobile) -->
    <transition name="overlay">
      <div v-if="sidebarOpen" class="fixed inset-0 z-30 bg-black/40 lg:hidden" @click="sidebarOpen = false" aria-hidden="true" />
    </transition>

    <!-- Sidebar -->
    <aside
      :class="[
        'fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-stone-200 bg-white transition-transform duration-250 lg:static lg:translate-x-0',
        sidebarOpen ? 'translate-x-0' : '-translate-x-full'
      ]"
      role="navigation"
      aria-label="Admin navigation"
    >
      <div class="flex items-center justify-between border-b border-stone-200 px-5 py-4">
        <div>
          <p class="text-base font-semibold tracking-tight text-ink">{{ BRAND }}</p>
          <p class="text-xs text-stone-400">Admin Desk</p>
        </div>
        <button class="rounded p-1 text-stone-400 hover:bg-stone-50 lg:hidden" @click="sidebarOpen = false" aria-label="Close menu">
          <X :size="18" />
        </button>
      </div>

      <nav class="flex-1 overflow-y-auto p-3">
        <div v-for="g in groups" :key="g.title" class="mb-4">
          <p class="px-3 pb-1.5 text-xs font-semibold uppercase tracking-wide text-stone-400">{{ g.title }}</p>
          <RouterLink
            v-for="l in g.links"
            :key="l.to"
            :to="l.to"
            :end="(l as any).end === true"
            class="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-stone-600 transition-colors hover:bg-stone-50 hover:text-ink"
            active-class="bg-stone-100 text-ink font-medium"
          >
            <component :is="l.icon" :size="15" class="shrink-0" />
            {{ l.label }}
          </RouterLink>
        </div>
      </nav>

      <div class="border-t border-stone-200 p-3">
        <p class="mb-2 truncate px-1 text-xs text-stone-400">{{ store.user?.email }}</p>
        <button class="btn-ghost w-full" @click="logout"><LogOut :size="15" /> Sign out</button>
      </div>
    </aside>

    <!-- Main -->
    <div class="flex flex-1 flex-col min-w-0">
      <header class="flex items-center justify-between border-b border-stone-200 bg-white px-4 py-3 lg:hidden">
        <div class="flex items-center gap-3">
          <button class="rounded-md p-1.5 text-stone-600 hover:bg-stone-50" @click="sidebarOpen = true" aria-label="Open menu">
            <Menu :size="20" />
          </button>
          <p class="font-semibold text-ink">{{ BRAND }}</p>
          <span class="text-xs text-stone-400">Admin</span>
        </div>
      </header>

      <main class="min-w-0 flex-1 overflow-x-hidden p-4 sm:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<style scoped>
.overlay-enter-active, .overlay-leave-active { transition: opacity 200ms ease; }
.overlay-enter-from, .overlay-leave-to { opacity: 0; }
</style>
