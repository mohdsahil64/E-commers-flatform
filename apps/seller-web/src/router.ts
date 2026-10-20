import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Key used to stash the intended destination in sessionStorage so it never
// appears in the browser URL (cleaner UX + hides internal paths from users).
export const REDIRECT_KEY = '__carlitos_seller_redirect__'

// Seller Desk routes. Router runs under the /seller base path.
const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { guestOnly: true } },
  { path: '/register', name: 'register', component: () => import('@/views/RegisterView.vue'), meta: { guestOnly: true } },
  { path: '/forgot-password', name: 'forgot-password', component: () => import('@/views/ForgotPasswordView.vue'), meta: { guestOnly: true } },
  { path: '/reset-password', name: 'reset-password', component: () => import('@/views/ResetPasswordView.vue'), meta: { guestOnly: true } },

  { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { requiresAuth: true } },
  { path: '/onboarding', name: 'onboarding', component: () => import('@/views/OnboardingView.vue'), meta: { requiresAuth: true } },
  { path: '/products', name: 'products', component: () => import('@/views/ProductsView.vue'), meta: { requiresAuth: true } },
  { path: '/products/new', name: 'product-new', component: () => import('@/views/ProductFormView.vue'), meta: { requiresAuth: true, requiresApproved: true } },
  { path: '/products/:id/edit', name: 'product-edit', component: () => import('@/views/ProductFormView.vue'), meta: { requiresAuth: true, requiresApproved: true } },
  { path: '/inventory', name: 'inventory', component: () => import('@/views/InventoryView.vue'), meta: { requiresAuth: true } },
  { path: '/orders', name: 'orders', component: () => import('@/views/OrdersView.vue'), meta: { requiresAuth: true } },
  { path: '/orders/:id', name: 'order-detail', component: () => import('@/views/OrderDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/shipments', name: 'shipments', component: () => import('@/views/ShipmentsView.vue'), meta: { requiresAuth: true } },
  { path: '/returns', name: 'returns', component: () => import('@/views/ReturnsView.vue'), meta: { requiresAuth: true } },
  { path: '/support', name: 'support', component: () => import('@/views/SupportView.vue'), meta: { requiresAuth: true } },
  { path: '/settlements', name: 'settlements', component: () => import('@/views/SettlementsView.vue'), meta: { requiresAuth: true } },
  { path: '/packaging', name: 'packaging', component: () => import('@/views/PackagingView.vue'), meta: { requiresAuth: true } },
  { path: '/profile', name: 'profile', component: () => import('@/views/ProfileView.vue'), meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { requiresAuth: true } },

  { path: '/:pathMatch(.*)*', name: 'not-found', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory('/seller/'),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const store = useAuthStore()
  if (!store.initialized) await store.fetchMe()

  if (to.meta.requiresAuth && !store.isAuthenticated) {
    // Save intended path in sessionStorage — keeps the URL clean.
    sessionStorage.setItem(REDIRECT_KEY, to.fullPath)
    return { name: 'login' }
  }
  if (to.meta.guestOnly && store.isAuthenticated) {
    return { name: 'dashboard' }
  }
  // Selling actions (e.g. listing products) require an approved store. Until an
  // admin approves the seller, bounce these routes back to the dashboard where
  // the approval-pending banner explains the next steps. (The API enforces this
  // server-side too, returning 'seller_not_approved'.)
  if (to.meta.requiresApproved && !store.isApproved) {
    return { name: 'dashboard', query: { needs_approval: '1' } }
  }
  return true
})
