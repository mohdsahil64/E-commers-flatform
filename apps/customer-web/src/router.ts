import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Key used to stash the intended destination in sessionStorage so it never
// appears in the browser URL (cleaner UX + hides internal paths from users).
export const REDIRECT_KEY = '__carlitos_customer_redirect__'

// Customer Store routes. Public routes have no meta guard.
const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', component: () => import('@/views/HomeView.vue') },
  { path: '/men', name: 'men', component: () => import('@/views/ListingView.vue'), props: { gender: 'men' } },
  { path: '/women', name: 'women', component: () => import('@/views/ListingView.vue'), props: { gender: 'women' } },
  { path: '/boys', name: 'boys', component: () => import('@/views/ListingView.vue'), props: { gender: 'boys' } },
  { path: '/girls', name: 'girls', component: () => import('@/views/ListingView.vue'), props: { gender: 'girls' } },
  { path: '/category/:slug', name: 'category', component: () => import('@/views/ListingView.vue') },
  { path: '/product/:slug', name: 'product', component: () => import('@/views/ProductView.vue') },
  { path: '/seller/:slug', name: 'seller-store', component: () => import('@/views/SellerStoreView.vue') },
  { path: '/search', name: 'search', component: () => import('@/views/SearchView.vue'), meta: { title: 'Search' } },
  { path: '/collections', name: 'collections', component: () => import('@/views/CollectionsView.vue'), meta: { title: 'Collections' } },
  { path: '/collections/:slug', name: 'collection', component: () => import('@/views/ListingView.vue') },

  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { guestOnly: true } },
  // Register redirects into the unified mobile-OTP login flow (login = signup).
  { path: '/register', name: 'register', redirect: { name: 'login' } },

  // Guests can view cart & wishlist (stored locally until they sign in).
  { path: '/cart', name: 'cart', component: () => import('@/views/CartView.vue') },
  { path: '/wishlist', name: 'wishlist', component: () => import('@/views/WishlistView.vue') },
  // Checkout requires authentication (placing an order).
  { path: '/checkout', name: 'checkout', component: () => import('@/views/CheckoutView.vue'), meta: { requiresAuth: true } },
  { path: '/orders', name: 'orders', component: () => import('@/views/OrdersView.vue'), meta: { requiresAuth: true } },
  { path: '/orders/:id', name: 'order-detail', component: () => import('@/views/OrderDetailView.vue'), meta: { requiresAuth: true } },
  { path: '/profile', name: 'profile', component: () => import('@/views/ProfileView.vue'), meta: { requiresAuth: true } },
  { path: '/addresses', name: 'addresses', component: () => import('@/views/AddressesView.vue'), meta: { requiresAuth: true } },
  { path: '/returns', name: 'returns', component: () => import('@/views/ReturnsView.vue'), meta: { requiresAuth: true } },
  { path: '/notifications', name: 'notifications', component: () => import('@/views/NotificationsView.vue'), meta: { requiresAuth: true } },
  { path: '/support', name: 'support', component: () => import('@/views/SupportView.vue'), meta: { title: 'Help Centre' } },

  // Legal / info pages (public).
  { path: '/privacy', name: 'privacy', component: () => import('@/views/legal/PrivacyView.vue'), meta: { title: 'Privacy Policy' } },
  { path: '/terms', name: 'terms', component: () => import('@/views/legal/TermsView.vue'), meta: { title: 'Terms & Conditions' } },
  { path: '/refund', name: 'refund', component: () => import('@/views/legal/RefundView.vue'), meta: { title: 'Cancellation & Refund Policy' } },
  { path: '/cookies', name: 'cookies', component: () => import('@/views/legal/CookiesView.vue'), meta: { title: 'Cookie Policy' } },
  { path: '/shipping', name: 'shipping', component: () => import('@/views/legal/ShippingInfoView.vue'), meta: { title: 'Shipping Policy' } },
  { path: '/about', name: 'about', component: () => import('@/views/legal/AboutView.vue'), meta: { title: 'About Us' } },
  { path: '/team', name: 'team', component: () => import('@/views/TeamView.vue'), meta: { title: 'Team & Management' } },
  { path: '/contact', name: 'contact', component: () => import('@/views/legal/ContactView.vue'), meta: { title: 'Contact Us' } },

  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/views/NotFoundView.vue'), meta: { title: 'Page Not Found' } },
]

export const router = createRouter({
  history: createWebHistory(),
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
    return { name: 'home' }
  }
  return true
})

// Baseline SEO title from route meta. Data-driven pages (product, seller,
// collection, search) refine this themselves once their content loads.
router.afterEach((to) => {
  const t = to.meta.title as string | undefined
  document.title = t ? `${t} | CARLITOS` : 'CARLITOS — Premium Fashion & Lifestyle Marketplace'
})
