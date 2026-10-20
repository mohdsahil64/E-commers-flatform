import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { WishlistItem, ListingCard } from '@carlitos/types'
import { customer } from '@/api'
import { useAuthStore } from '@/stores/auth'

const GUEST_KEY = 'carlitos_guest_wishlist'

function readGuest(): WishlistItem[] {
  try {
    return JSON.parse(localStorage.getItem(GUEST_KEY) || '[]')
  } catch {
    return []
  }
}
function writeGuest(items: WishlistItem[]) {
  localStorage.setItem(GUEST_KEY, JSON.stringify(items))
}

// Unified wishlist store: server-backed when authenticated, localStorage for
// guests. Presents one interface (items / ids / has / toggle) and merges the
// guest wishlist onto the server after sign-in.
export const useWishlistStore = defineStore('wishlist', () => {
  const server = ref<WishlistItem[]>([])
  const guest = ref<WishlistItem[]>(readGuest())
  const loading = ref(false)
  const busy = ref<Record<string, boolean>>({})

  const auth = () => useAuthStore()

  const items = computed<WishlistItem[]>(() => (auth().isAuthenticated ? server.value : guest.value))
  const ids = computed(() => new Set(items.value.map((i) => i.product_id)))
  const count = computed(() => items.value.length)

  function has(productId: string): boolean {
    return ids.value.has(productId)
  }

  async function refresh() {
    if (!auth().isAuthenticated) return
    loading.value = true
    try {
      server.value = (await customer.wishlist()) || []
    } finally {
      loading.value = false
    }
  }

  // add stores a product. For guests we need enough to render the wishlist page,
  // so callers pass a card-like snapshot; for the server the product_id suffices.
  async function add(card: Pick<WishlistItem, 'product_id' | 'title' | 'slug' | 'image' | 'min_price' | 'rating_avg'>) {
    if (auth().isAuthenticated) {
      await customer.addWishlist(card.product_id)
      await refresh()
      return
    }
    if (!guest.value.some((i) => i.product_id === card.product_id)) {
      guest.value.push({ ...card })
      writeGuest(guest.value)
    }
  }

  async function remove(productId: string) {
    if (auth().isAuthenticated) {
      await customer.removeWishlist(productId)
      server.value = server.value.filter((i) => i.product_id !== productId)
      return
    }
    guest.value = guest.value.filter((i) => i.product_id !== productId)
    writeGuest(guest.value)
  }

  // toggle returns the new state (true = now wishlisted). Guards busy per product.
  async function toggle(card: Pick<WishlistItem, 'product_id' | 'title' | 'slug' | 'image' | 'min_price' | 'rating_avg'>): Promise<boolean> {
    const id = card.product_id
    if (busy.value[id]) return has(id)
    busy.value[id] = true
    try {
      if (has(id)) {
        await remove(id)
        return false
      }
      await add(card)
      return true
    } finally {
      delete busy.value[id]
    }
  }

  // Build a snapshot from a ListingCard (used by ProductCard / listings).
  function cardFromListing(p: ListingCard): WishlistItem {
    return {
      product_id: p.product_id,
      title: p.title,
      slug: p.slug,
      image: p.primary_image,
      min_price: p.min_price,
      rating_avg: p.rating_avg,
    }
  }

  // mergeGuestIntoServer replays the guest wishlist after login, then clears it.
  async function mergeGuestIntoServer() {
    const local = readGuest()
    if (!local.length) {
      await refresh()
      return
    }
    for (const it of local) {
      try {
        await customer.addWishlist(it.product_id)
      } catch {
        /* skip */
      }
    }
    localStorage.removeItem(GUEST_KEY)
    guest.value = []
    await refresh()
  }

  function clearGuest() {
    localStorage.removeItem(GUEST_KEY)
    guest.value = []
  }

  return {
    server, guest, loading, busy, items, ids, count,
    has, refresh, add, remove, toggle, cardFromListing, mergeGuestIntoServer, clearGuest,
  }
})
