import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Key used to stash the intended destination in sessionStorage so it never
// appears in the browser URL (cleaner UX + hides internal paths from users).
export const REDIRECT_KEY = '__carlitos_admin_redirect__'

// Admin Desk routes served under the /admin base path.
const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { guestOnly: true } },

  { path: '/', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { requiresAuth: true } },
  { path: '/sellers', name: 'sellers', component: () => import('@/views/SellersView.vue'), meta: { requiresAuth: true } },
  { path: '/sellers/:id', name: 'seller-detail', component: () => import('@/views/SellerDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/products', name: 'products', component: () => import('@/views/ProductsView.vue'), meta: { requiresAuth: true } },
  { path: '/products/:id', name: 'product-detail', component: () => import('@/views/ProductDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/categories', name: 'categories', component: () => import('@/views/CategoriesView.vue'), meta: { requiresAuth: true } },
  { path: '/orders', name: 'orders', component: () => import('@/views/OrdersView.vue'), meta: { requiresAuth: true } },
  { path: '/orders/:id', name: 'order-detail', component: () => import('@/views/OrderDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/customers', name: 'customers', component: () => import('@/views/CustomersView.vue'), meta: { requiresAuth: true } },
  { path: '/customers/:id', name: 'customer-detail', component: () => import('@/views/CustomerDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/shipping', name: 'shipping', component: () => import('@/views/ShippingView.vue'), meta: { requiresAuth: true } },
  { path: '/returns', name: 'returns', component: () => import('@/views/ReturnsView.vue'), meta: { requiresAuth: true } },
  // Refunds now live on the combined Returns & Refunds page; keep old links working.
  { path: '/refunds', redirect: '/returns' },
  { path: '/support', name: 'support', component: () => import('@/views/SupportView.vue'), meta: { requiresAuth: true } },
  { path: '/seller-support', name: 'seller-support', component: () => import('@/views/SellerSupportView.vue'), meta: { requiresAuth: true } },
  { path: '/payments', name: 'payments', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Payments' }, meta: { requiresAuth: true } },
  { path: '/settlements', name: 'settlements', component: () => import('@/views/SettlementsView.vue'), meta: { requiresAuth: true } },
  { path: '/coupons', name: 'coupons', component: () => import('@/views/CouponsView.vue'), meta: { requiresAuth: true } },
  { path: '/promotions', name: 'promotions', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Promotions' }, meta: { requiresAuth: true } },
  { path: '/collections', name: 'collections', component: () => import('@/views/CollectionsView.vue'), meta: { requiresAuth: true } },
  { path: '/banners', name: 'banners', component: () => import('@/views/BannersView.vue'), meta: { requiresAuth: true } },
  { path: '/team', name: 'team', component: () => import('@/views/TeamView.vue'), meta: { requiresAuth: true } },
  { path: '/packaging', name: 'packaging', component: () => import('@/views/PackagingView.vue'), meta: { requiresAuth: true } },
  { path: '/reports', name: 'reports', component: () => import('@/views/ReportsView.vue'), meta: { requiresAuth: true } },
  // Employees now live on the Roles page ("Employees" tab); keep old links working.
  { path: '/users', redirect: '/roles' },
  { path: '/roles', name: 'roles', component: () => import('@/views/RolesView.vue'), meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue'), meta: { requiresAuth: true } },
  { path: '/audit-logs', name: 'audit-logs', component: () => import('@/views/AuditLogsView.vue'), meta: { requiresAuth: true } },
  { path: '/no-access', name: 'no-access', component: () => import('@/views/NoAccessView.vue'), meta: { requiresAuth: true } },

  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export const router = createRouter({
  history: createWebHistory('/admin/'),
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

  // Page-level access. Refresh who I am first (cheap, at most once a minute) so
  // a changed role or a disabled account is picked up without signing in again.
  // Deny-by-default: a route that is not in PAGE_ACCESS cannot be opened.
  if (to.meta.requiresAuth) {
    await store.refreshIfStale()
    if (!store.isAuthenticated) {
      sessionStorage.setItem(REDIRECT_KEY, to.fullPath)
      return { name: 'login' }
    }
    if (!store.canAccess(to.name)) {
      return { name: 'no-access', query: { from: to.fullPath } }
    }
  }
  return true
})
