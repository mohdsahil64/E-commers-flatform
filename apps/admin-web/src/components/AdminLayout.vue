<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { RouterLink, useRouter, useRoute } from 'vue-router'
import {
  LayoutDashboard, Store, ShieldCheck, Users, Package, PackageCheck,
  FolderTree, Tag, Sliders, ShoppingCart, Truck, RotateCcw,
  CreditCard, Wallet, Ticket, Megaphone, Layers, Image, Box, BarChart3,
  KeyRound, Settings, Menu, X, LogOut, LifeBuoy, Headset,
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
      { label: 'Returns & Refunds', to: '/returns', icon: RotateCcw },
      { label: 'Customer Support', to: '/support', icon: LifeBuoy },
      { label: 'Seller Support', to: '/seller-support', icon: Headset },
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
      { label: 'Employees & Roles', to: '/roles', icon: KeyRound },
      { label: 'Settings',   to: '/settings',   icon: Settings },
    ],
  },
]

// Only show the pages this person is allowed to open. Each link is resolved to
// its route name and checked against PAGE_ACCESS, so the sidebar can never
// advertise a page the router would refuse. (A group with no visible link
// disappears entirely.) Real enforcement is on the server for every API call.
const visibleGroups = computed(() =>
  groups
    .map((g) => ({
      ...g,
      links: g.links.filter((l) => store.canAccess(router.resolve(l.to).name)),
    }))
    .filter((g) => g.links.length > 0),
)

// "ORDER_ADMIN" -> "Order Admin" (shown under the email).
const roleLabel = computed(() => {
  const r = store.user?.roles?.[0]
  if (!r) return ''
  return r.startsWith('CUSTOM_') ? 'Employee' : r.toLowerCase().split('_').map((w) => w[0].toUpperCase() + w.slice(1)).join(' ')
})

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
        <div v-for="g in visibleGroups" :key="g.title" class="mb-4">
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
        <div class="mb-2 px-1">
          <p class="truncate text-xs font-medium text-stone-600">{{ store.user?.full_name || store.user?.email }}</p>
          <p class="truncate text-xs text-stone-400">{{ roleLabel }}<span v-if="roleLabel && store.user?.full_name"> · </span><span v-if="store.user?.full_name">{{ store.user?.email }}</span></p>
        </div>
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
