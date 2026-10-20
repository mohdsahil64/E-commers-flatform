import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { CartItem } from '@carlitos/types'
import { customer } from '@/api'
import { useAuthStore } from '@/stores/auth'

// A guest cart line stored locally before the user signs in. We keep the
// minimum needed to render the cart and to replay adds onto the server cart
// after login. seller_listing_id is the stable server identifier.
export interface GuestLine {
  seller_listing_id: string
  quantity: number
  product_title: string
  slug: string
  image: string
  seller_name: string
  sku: string
  unit_price: number
  available_quantity: number
}

const GUEST_KEY = 'carlitos_guest_cart'

function readGuest(): GuestLine[] {
  try {
    return JSON.parse(localStorage.getItem(GUEST_KEY) || '[]')
  } catch {
    return []
  }
}
function writeGuest(lines: GuestLine[]) {
  localStorage.setItem(GUEST_KEY, JSON.stringify(lines))
}

// The cart store presents one unified interface. When authenticated it proxies
// the server cart; as a guest it keeps items in localStorage.
export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([])
  const guest = ref<GuestLine[]>(readGuest())
  const loading = ref(false)

  const auth = () => useAuthStore()

  const count = computed(() =>
    auth().isAuthenticated
      ? items.value.reduce((s, i) => s + i.quantity, 0)
      : guest.value.reduce((s, i) => s + i.quantity, 0)
  )

  const lines = computed<CartItem[]>(() =>
    auth().isAuthenticated
      ? items.value
      : guest.value.map((g, i) => ({
          id: `guest-${i}`,
          seller_listing_id: g.seller_listing_id,
          product_id: '',
          product_title: g.product_title,
          slug: g.slug,
          sku: g.sku,
          image: g.image,
          seller_name: g.seller_name,
          quantity: g.quantity,
          unit_price: g.unit_price,
          available_quantity: g.available_quantity,
          line_total: g.unit_price * g.quantity,
        }))
  )

  const subtotal = computed(() =>
    lines.value.reduce((s, i) => s + i.line_total, 0)
  )

  async function refresh() {
    if (!auth().isAuthenticated) return
    loading.value = true
    try {
      const res = await customer.cart()
      items.value = res.items || []
    } finally {
      loading.value = false
    }
  }

  async function add(line: GuestLine) {
    if (auth().isAuthenticated) {
      await customer.addToCart(line.seller_listing_id, line.quantity)
      await refresh()
      return
    }
    const existing = guest.value.find(g => g.seller_listing_id === line.seller_listing_id)
    if (existing) {
      existing.quantity = Math.min(existing.quantity + line.quantity, line.available_quantity || 99)
    } else {
      guest.value.push({ ...line })
    }
    writeGuest(guest.value)
  }

  async function updateQty(sellerListingId: string, quantity: number) {
    if (quantity < 1) return
    if (auth().isAuthenticated) {
      await customer.updateCartItem(sellerListingId, quantity)
      await refresh()
      return
    }
    const g = guest.value.find(x => x.seller_listing_id === sellerListingId)
    if (g) { g.quantity = quantity; writeGuest(guest.value) }
  }

  async function remove(sellerListingId: string) {
    if (auth().isAuthenticated) {
      await customer.removeCartItem(sellerListingId)
      await refresh()
      return
    }
    guest.value = guest.value.filter(x => x.seller_listing_id !== sellerListingId)
    writeGuest(guest.value)
  }

  // mergeGuestIntoServer replays the guest cart onto the server cart after a
  // successful login, then clears the local cart. Best-effort per line.
  async function mergeGuestIntoServer() {
    const local = readGuest()
    if (!local.length) { await refresh(); return }
    for (const g of local) {
      try {
        await customer.addToCart(g.seller_listing_id, g.quantity)
      } catch { /* skip lines that can't be added (e.g. out of stock) */ }
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
    items, guest, loading, count, lines, subtotal,
    refresh, add, updateQty, remove, mergeGuestIntoServer, clearGuest,
  }
})
