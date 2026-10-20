<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Store, MapPin, Plus, ChevronRight, X } from 'lucide-vue-next'
import type { ListingCard, Banner, Address } from '@carlitos/types'
import { pub, customer } from '@/api'
import { extractApiError } from '@carlitos/shared'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'
import { INDIA_STATES, DEFAULT_COUNTRY } from '@/constants/india'
import ProductCarousel from '@/components/ProductCarousel.vue'
import BannerSlider from '@/components/BannerSlider.vue'

const auth  = useAuthStore()
const toast = useToastStore()

/* ── Delivery address bar (above the category row) ──────────────────
   Shows the customer's default/first saved address in one line. When none is
   saved (or the user is a guest who then signs in), it offers an "Add address"
   action that opens a popup to capture one inline. */
const addresses    = ref<Address[]>([])
const addrLoading  = ref(false)

const defaultAddress = computed<Address | null>(() => {
  if (!addresses.value.length) return null
  return addresses.value.find(a => a.is_default) || addresses.value[0]
})

// One-line address string for the bar.
const addressLine = computed(() => {
  const a = defaultAddress.value
  if (!a) return ''
  return [a.line1, a.line2, a.landmark, a.city, a.state, a.pincode]
    .filter(Boolean).join(', ')
})

async function loadAddresses() {
  if (!auth.isAuthenticated) { addresses.value = []; return }
  addrLoading.value = true
  try {
    addresses.value = (await customer.addresses()) || []
  } catch {
    addresses.value = []
  } finally {
    addrLoading.value = false
  }
}

/* Add-address popup */
const showAddrModal = ref(false)
const savingAddr    = ref(false)
const addrError     = ref('')
const newAddr = ref({
  contact_name: '', contact_phone: '', line1: '', line2: '',
  landmark: '', city: '', state: '', pincode: '', country: DEFAULT_COUNTRY, is_default: true,
})

function resetNewAddr() {
  newAddr.value = {
    contact_name: '', contact_phone: '', line1: '', line2: '',
    landmark: '', city: '', state: '', pincode: '', country: DEFAULT_COUNTRY, is_default: true,
  }
  addrError.value = ''
}

function openAddrModal() {
  // Guests must sign in before an address can be saved to their account.
  if (!auth.isAuthenticated) {
    toast.info('Please sign in to save a delivery address.')
    return
  }
  resetNewAddr()
  showAddrModal.value = true
}

async function saveNewAddr() {
  if (!newAddr.value.contact_name.trim()) { addrError.value = 'Please enter the full name.'; return }
  if (!newAddr.value.line1.trim())        { addrError.value = 'Please enter address line 1.'; return }
  if (!newAddr.value.city.trim())         { addrError.value = 'Please enter the city.'; return }
  if (!/^\d{6}$/.test(newAddr.value.pincode.trim())) { addrError.value = 'Please enter a valid 6-digit pincode.'; return }
  savingAddr.value = true
  addrError.value = ''
  try {
    // label/address_type are fixed defaults (no longer shown in the form).
    await customer.addAddress({ ...newAddr.value, label: 'Home', address_type: 'home' })
    await loadAddresses()
    showAddrModal.value = false
    resetNewAddr()
    toast.success('Address added')
  } catch (e) {
    addrError.value = extractApiError(e)
  } finally {
    savingAddr.value = false
  }
}

/* ── Data ─────────────────────────────────────────────────── */
const banners      = ref<Banner[]>([])
const newArrivals  = ref<ListingCard[]>([])
const trending     = ref<ListingCard[]>([])
const summer       = ref<ListingCard[]>([])
const winter       = ref<ListingCard[]>([])
const loading      = ref(true)

// Derived seller summaries ("Trending Sellers") built from the popular feed —
// no fabricated numbers, just real sellers that have live products.
interface SellerSummary {
  slug: string
  name: string
  products: ListingCard[]
}
const trendingSellers = computed<SellerSummary[]>(() => {
  const map = new Map<string, SellerSummary>()
  for (const p of trending.value) {
    if (!p.seller_slug) continue
    let s = map.get(p.seller_slug)
    if (!s) { s = { slug: p.seller_slug, name: p.seller_name, products: [] }; map.set(p.seller_slug, s) }
    if (s.products.length < 12) s.products.push(p)
  }
  return Array.from(map.values()).slice(0, 6)
})

onMounted(async () => {
  // Load the delivery address bar in the background (best-effort; never blocks
  // the storefront content).
  loadAddresses()
  try {
    const [b, na, tr, su, wi] = await Promise.all([
      pub.banners('home_hero').catch(() => []),
      pub.products({ sort: 'newest',     page_size: 20 }).catch(() => ({ data: [] })),
      pub.products({ sort: 'popularity', page_size: 20 }).catch(() => ({ data: [] })),
      pub.products({ season: 'summer', sort: 'popularity', page_size: 20 }).catch(() => ({ data: [] })),
      pub.products({ season: 'winter', sort: 'popularity', page_size: 20 }).catch(() => ({ data: [] })),
    ])
    // Only DB-configured banners are shown (the hardcoded house promo image
    // has been removed).
    const apiBanners = (b as Banner[]).slice().sort((x, y) => x.sort_order - y.sort_order)
    banners.value = [...apiBanners]
    newArrivals.value = ((na as { data?: ListingCard[] }).data) || []
    trending.value    = ((tr as { data?: ListingCard[] }).data) || []
    summer.value      = ((su as { data?: ListingCard[] }).data) || []
    winter.value      = ((wi as { data?: ListingCard[] }).data) || []
  } finally {
    loading.value = false
  }
})

/* ── Category circles (Myntra-style quick-shop row) ─────────── */
// Horizontally scrollable circular category chips. On-brand gradients stand in
// for real category art; drop photos into /public/category-*.jpg later.
// Category icons are static PNGs in /public/homeicons (served from the web root).
const categories: { label: string; to: string; icon: string }[] = [
  { label: 'Men',          to: '/men',                       icon: '/homeicons/man.png' },
  { label: 'Women',        to: '/women',                     icon: '/homeicons/woman.png' },
  { label: 'Boys',         to: '/boys',                      icon: '/homeicons/man.png' },
  { label: 'New Arrivals', to: '/collections/new-arrivals',  icon: '/homeicons/new-arrivals.png' },
  { label: 'Collections',  to: '/collections',               icon: '/homeicons/collaction.png' },
]
</script>

<template>
  <div class="space-y-8 sm:space-y-12">

    <!-- ── DELIVERY ADDRESS BAR (above the category row) ──────── -->
    <section aria-label="Delivery address" class="-mt-1 -mb-4 sm:-mb-8">
      <!-- Saved address → one line with a pin -->
      <button
        v-if="defaultAddress"
        type="button"
        class="flex w-full items-center gap-2 rounded-lg px-1 py-1 text-left transition-colors hover:bg-stone-50"
        @click="openAddrModal"
      >
        <MapPin :size="16" class="shrink-0 text-accent" />
        <span class="min-w-0 flex-1 truncate text-xs text-stone-700">
          <span class="font-semibold text-ink">Deliver to {{ defaultAddress.contact_name }}</span>
          <span class="text-stone-500"> · {{ addressLine }}</span>
        </span>
        <ChevronRight :size="15" class="shrink-0 text-stone-400" />
      </button>

      <!-- No saved address → add new -->
      <button
        v-else-if="!addrLoading"
        type="button"
        class="flex w-full items-center gap-2 rounded-lg px-1 py-1 text-left transition-colors hover:bg-stone-50"
        @click="openAddrModal"
      >
        <MapPin :size="16" class="shrink-0 text-accent" />
        <span class="flex-1 text-xs font-medium text-ink">Add a delivery address</span>
        <Plus :size="15" class="shrink-0 text-stone-400" />
      </button>
    </section>

    <!-- ── CATEGORY CIRCLES (Myntra-style quick shop) ─────────── -->
    <section aria-label="Shop by category" class="-mt-1">
      <div class="flex justify-evenly gap-2 overflow-x-auto pb-1 scrollbar-hide sm:justify-start sm:gap-8">
        <RouterLink
          v-for="c in categories"
          :key="c.to"
          :to="c.to"
          class="group flex shrink-0 flex-col items-center gap-1.5"
          :aria-label="`Shop ${c.label}`"
        >
          <span
            class="flex h-12 w-12 items-center justify-center rounded-full bg-white border border-accent/40 transition-transform group-hover:scale-105 group-hover:border-accent sm:h-14 sm:w-14"
          >
            <img :src="c.icon" :alt="c.label" class="h-7 w-7 object-contain sm:h-8 sm:w-8" loading="lazy" />
          </span>
          <span class="max-w-[72px] truncate text-center text-xs font-medium text-stone-700 sm:text-sm">{{ c.label }}</span>
        </RouterLink>
      </div>
    </section>

    <!-- ── HERO BANNER (admin-controlled, auto-sliding) ───────── -->
    <div v-if="loading">
      <div class="skeleton h-40 w-full rounded-2xl xs:h-44 sm:h-64 lg:h-80" />
    </div>
    <BannerSlider v-else-if="banners.length" :banners="banners" />
    <!-- Fallback when no banners are configured -->
    <section v-else class="overflow-hidden rounded-2xl">
      <div class="flex h-40 flex-col items-start justify-end bg-gradient-to-br from-accent to-accent-dark p-5 xs:h-44 sm:h-64 sm:p-8 lg:h-80">
        <p class="mb-2 text-xs font-medium uppercase tracking-widest text-white/70">New Season</p>
        <h2 class="max-w-lg text-3xl font-semibold text-white sm:text-4xl">Everyday fashion, thoughtfully made.</h2>
        <RouterLink to="/women" class="mt-5 inline-flex items-center gap-2 rounded-md bg-white px-5 py-2.5 text-sm font-medium text-ink hover:bg-stone-100">
          Shop now <ArrowRight :size="15" />
        </RouterLink>
      </div>
    </section>

    <!-- ── NEW ARRIVALS ──────────────────────────────────────── -->
    <section aria-labelledby="new-arrivals-heading">
      <div class="mb-4 flex items-center justify-between">
        <h2 id="new-arrivals-heading" class="text-h3">New Arrivals</h2>
        <RouterLink to="/search?sort=newest" class="flex items-center gap-1 text-sm font-medium text-accent hover:text-accent-dark">
          View all <ArrowRight :size="14" />
        </RouterLink>
      </div>
      <div v-if="!loading && !newArrivals.length" class="rounded-xl border border-stone-200 bg-stone-50 p-8 text-center">
        <p class="text-sm text-stone-500">No new arrivals yet. Check back soon.</p>
      </div>
      <ProductCarousel v-else :products="newArrivals" :loading="loading" />
    </section>

    <!-- ── TRENDING NOW ──────────────────────────────────────── -->
    <section v-if="loading || trending.length" aria-labelledby="trending-heading">
      <div class="mb-4 flex items-center justify-between">
        <h2 id="trending-heading" class="text-h3">Trending Now</h2>
        <RouterLink to="/search?sort=popularity" class="flex items-center gap-1 text-sm font-medium text-accent hover:text-accent-dark">
          View all <ArrowRight :size="14" />
        </RouterLink>
      </div>
      <ProductCarousel :products="trending" :loading="loading" />
    </section>

    <!-- ── TRENDING SELLERS ──────────────────────────────────── -->
    <section v-if="trendingSellers.length" aria-labelledby="sellers-heading">
      <h2 id="sellers-heading" class="mb-4 text-h3">Trending Sellers</h2>
      <div class="space-y-6">
        <div
          v-for="s in trendingSellers"
          :key="s.slug"
          class="rounded-xl border border-stone-200 p-3 sm:p-4"
        >
          <div class="mb-3 flex items-center justify-between gap-2">
            <RouterLink :to="`/seller/${s.slug}`" class="group flex min-w-0 items-center gap-2.5">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-stone-100">
                <Store :size="17" class="text-stone-500" />
              </span>
              <span class="min-w-0">
                <span class="block truncate text-sm font-semibold text-ink group-hover:underline">{{ s.name }}</span>
                <span class="block text-xs text-stone-400">Verified seller</span>
              </span>
            </RouterLink>
            <RouterLink :to="`/seller/${s.slug}`" class="flex shrink-0 items-center gap-1 text-xs font-medium text-accent hover:text-accent-dark">
              Visit <ArrowRight :size="13" />
            </RouterLink>
          </div>
          <ProductCarousel :products="s.products" />
        </div>
      </div>
    </section>

    <!-- ── SUMMER COLLECTION ─────────────────────────────────── -->
    <section v-if="summer.length" aria-labelledby="summer-heading">
      <div class="mb-4 flex items-center justify-between">
        <h2 id="summer-heading" class="text-h3">Summer Collection</h2>
        <RouterLink to="/search?season=summer&sort=popularity" class="flex items-center gap-1 text-sm font-medium text-accent hover:text-accent-dark">
          View all <ArrowRight :size="14" />
        </RouterLink>
      </div>
      <ProductCarousel :products="summer" :loading="loading" />
    </section>

    <!-- ── WINTER COLLECTION ─────────────────────────────────── -->
    <section v-if="winter.length" aria-labelledby="winter-heading">
      <div class="mb-4 flex items-center justify-between">
        <h2 id="winter-heading" class="text-h3">Winter Collection</h2>
        <RouterLink to="/search?season=winter&sort=popularity" class="flex items-center gap-1 text-sm font-medium text-accent hover:text-accent-dark">
          View all <ArrowRight :size="14" />
        </RouterLink>
      </div>
      <ProductCarousel :products="winter" :loading="loading" />
    </section>

    <!-- ══════════ Add-address popup ══════════ -->
    <Teleport to="body">
      <div
        v-if="showAddrModal"
        class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
        @click.self="showAddrModal = false"
      >
        <div class="flex max-h-[90vh] w-full flex-col overflow-hidden rounded-t-2xl bg-white sm:max-w-lg sm:rounded-2xl">
          <div class="flex items-center justify-between border-b border-stone-200 p-4">
            <h3 class="text-base font-semibold text-ink">Add delivery address</h3>
            <button class="text-stone-400 hover:text-ink" aria-label="Close" @click="showAddrModal = false"><X :size="20" /></button>
          </div>

          <div class="flex-1 overflow-y-auto p-4">
            <div class="grid gap-3 sm:grid-cols-2">
              <div>
                <label class="label">Full name</label>
                <input v-model="newAddr.contact_name" class="input" placeholder="e.g. Rahul Sharma" />
              </div>
              <div>
                <label class="label">Phone</label>
                <input v-model="newAddr.contact_phone" class="input" type="tel" inputmode="numeric" maxlength="10" placeholder="10-digit mobile" />
              </div>
              <div class="sm:col-span-2">
                <label class="label">Flat / House no, Building, Area</label>
                <input v-model="newAddr.line1" class="input" placeholder="House/flat no, building, street" />
              </div>
              <div class="sm:col-span-2">
                <label class="label">Area / Locality <span class="text-stone-400">(optional)</span></label>
                <input v-model="newAddr.line2" class="input" placeholder="Colony, sector, locality" />
              </div>
              <div class="sm:col-span-2">
                <label class="label">Landmark <span class="text-stone-400">(optional)</span></label>
                <input v-model="newAddr.landmark" class="input" placeholder="Near school, temple, landmark…" />
              </div>
              <div>
                <label class="label">Pincode</label>
                <input v-model="newAddr.pincode" class="input" inputmode="numeric" maxlength="6" placeholder="6-digit pincode" />
              </div>
              <div>
                <label class="label">City</label>
                <input v-model="newAddr.city" class="input" placeholder="City / town" />
              </div>
              <div>
                <label class="label">State</label>
                <select v-model="newAddr.state" class="select">
                  <option value="" disabled>Select state</option>
                  <option v-for="s in INDIA_STATES" :key="s" :value="s">{{ s }}</option>
                </select>
              </div>
              <div>
                <label class="label">Country</label>
                <input :value="newAddr.country" class="input bg-stone-50 text-stone-500" disabled />
              </div>
            </div>
            <label class="mt-3 flex cursor-pointer items-center gap-2 text-sm">
              <input v-model="newAddr.is_default" type="checkbox" class="accent-ink" />
              Set as default address
            </label>
            <p v-if="addrError" class="mt-2 text-sm text-red-600">{{ addrError }}</p>
          </div>

          <div class="flex gap-3 border-t border-stone-200 p-4">
            <button class="btn-ghost flex-1" :disabled="savingAddr" @click="showAddrModal = false">Cancel</button>
            <button class="btn-primary flex-1" :disabled="savingAddr" @click="saveNewAddr">
              {{ savingAddr ? 'Saving…' : 'Save address' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

  </div>
</template>

<style scoped>
.scrollbar-hide { -ms-overflow-style: none; scrollbar-width: none; }
.scrollbar-hide::-webkit-scrollbar { display: none; }
</style>
