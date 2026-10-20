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
  { path: '/customers', name: 'customers', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Customers' }, meta: { requiresAuth: true } },
  { path: '/shipping', name: 'shipping', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Shipping' }, meta: { requiresAuth: true } },
  { path: '/returns', name: 'returns', component: () => import('@/views/ReturnsView.vue'), meta: { requiresAuth: true } },
  { path: '/refunds', name: 'refunds', component: () => import('@/views/ReturnsView.vue'), meta: { requiresAuth: true } },
  { path: '/support', name: 'support', component: () => import('@/views/SupportView.vue'), meta: { requiresAuth: true } },
  { path: '/payments', name: 'payments', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Payments' }, meta: { requiresAuth: true } },
  { path: '/settlements', name: 'settlements', component: () => import('@/views/SettlementsView.vue'), meta: { requiresAuth: true } },
  { path: '/coupons', name: 'coupons', component: () => import('@/views/CouponsView.vue'), meta: { requiresAuth: true } },
  { path: '/promotions', name: 'promotions', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Promotions' }, meta: { requiresAuth: true } },
  { path: '/collections', name: 'collections', component: () => import('@/views/CollectionsView.vue'), meta: { requiresAuth: true } },
  { path: '/banners', name: 'banners', component: () => import('@/views/BannersView.vue'), meta: { requiresAuth: true } },
  { path: '/team', name: 'team', component: () => import('@/views/TeamView.vue'), meta: { requiresAuth: true } },
  { path: '/packaging', name: 'packaging', component: () => import('@/views/PackagingView.vue'), meta: { requiresAuth: true } },
  { path: '/reports', name: 'reports', component: () => import('@/views/ReportsView.vue'), meta: { requiresAuth: true } },
  { path: '/users', name: 'users', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Users' }, meta: { requiresAuth: true } },
  { path: '/roles', name: 'roles', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Roles' }, meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: () => import('@/views/PlaceholderView.vue'), props: { title: 'Settings' }, meta: { requiresAuth: true } },
  { path: '/audit-logs', name: 'audit-logs', component: () => import('@/views/AuditLogsView.vue'), meta: { requiresAuth: true } },

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
  return true
})
