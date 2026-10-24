<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  MapPin, CreditCard, ChevronRight, ChevronLeft, Plus,
  Tag, X, Wallet, BadgePercent, Check, Banknote, Smartphone,
  ShieldCheck, Info,
} from 'lucide-vue-next'
import type { Address, Coupon } from '@carlitos/types'
import { customer, MEDIA_BASE } from '@/api'
import { formatINR, extractApiError, resolveMediaUrl } from '@carlitos/shared'
import { INDIA_STATES, DEFAULT_COUNTRY } from '@/constants/india'
import { useCartStore } from '@/stores/cart'
import { useToastStore } from '@/stores/toast'
import { useAuthStore } from '@/stores/auth'
import { openRazorpayCheckout } from '@/composables/useRazorpay'
import OrderSuccessModal from '@/components/OrderSuccessModal.vue'
import CheckoutPriceSummary from '@/components/CheckoutPriceSummary.vue'

const router = useRouter()
const route = useRoute()
const cart = useCartStore()
const toast = useToastStore()
const authStore = useAuthStore()

// Selected cart lines to check out, passed from the cart page as a
// comma-separated `items` query (seller_listing_ids). Empty => whole cart.
const selectedListingIds = computed<string[]>(() => {
  const raw = route.query.items
  const s = Array.isArray(raw) ? raw[0] : raw
  return s ? String(s).split(',').filter(Boolean) : []
})

/* ── State ──────────────────────────────────────────── */
const step           = ref(1) // 1=address 2=payment 3=review
const addresses      = ref<Address[]>([])
const selectedAddr   = ref('')
const paymentMethod  = ref<'cod'|'online'>('cod')
const placing        = ref(false)
const error          = ref('')
const loading        = ref(true)

/* ── Order-confirmed success modal ──────────────────── */
const showSuccess    = ref(false)
const placedOrder    = ref<{ id: string; order_number: string; grand_total: number } | null>(null)

// Only the lines being checked out (the selection from the cart page). When no
// selection was passed, this is the whole cart.
const cartItems      = computed(() => {
  if (!selectedListingIds.value.length) return cart.lines
  const set = new Set(selectedListingIds.value)
  return cart.lines.filter(l => set.has(l.seller_listing_id))
})
const cartSubtotal   = computed(() => cartItems.value.reduce((s, i) => s + i.line_total, 0))
const shipping       = computed(() => 0)

/* ── Cash on Delivery availability ──────────────────────────────────
   COD is only offered when EVERY seller in the cart allows it. A single
   item from a seller who turned COD off disables the option (the server
   enforces the same rule at checkout). */
const codBlockedBy = computed(() =>
  cartItems.value
    .filter(i => i.seller_cod_enabled === false)
    .map(i => i.seller_name)
    .filter((v, idx, arr) => arr.indexOf(v) === idx),
)
const codAvailable = computed(() => codBlockedBy.value.length === 0)

/* ── Addresses ──────────────────────────────────────── */
const showAddrModal  = ref(false)
const selectedAddrObj = computed(() => addresses.value.find(a => a.id === selectedAddr.value))

/* new-address form (used inside the address modal) */
const showNewAddr    = ref(false)
const newAddr        = ref({ contact_name:'', contact_phone:'', line1:'', line2:'', landmark:'', city:'', state:'', pincode:'', country: DEFAULT_COUNTRY, is_default: false })
const savingAddr     = ref(false)

/* ── Coupons / Offers ───────────────────────────────── */
const offers         = ref<Coupon[]>([])
const showOffersModal = ref(false)
const appliedCoupon  = ref<{ code: string; discount: number } | null>(null)
const couponBusy     = ref(false)
const couponError    = ref('')

const payableTotal   = computed(() =>
  Math.max(0, cartSubtotal.value + shipping.value - (appliedCoupon.value?.discount || 0)),
)

// A coupon is eligible when the current subtotal meets its minimum order.
function isEligible(c: Coupon): boolean {
  return cartSubtotal.value >= (c.min_order_amount || 0)
}

// Short human label for a coupon's benefit.
function couponBenefit(c: Coupon): string {
  if (c.discount_type === 'percent') {
    const cap = c.max_discount ? ` up to ${formatINR(c.max_discount)}` : ''
    return `${c.discount_value}% OFF${cap}`
  }
  return `${formatINR(c.discount_value)} OFF`
}

/* ── Myntra-style unlock progress ───────────────────────────────────
   Finds the next offer the customer hasn't unlocked yet (cheapest min
   order above the current subtotal) and shows how much more to add. If
   everything in view is already unlocked, we celebrate instead. */
const nextOffer = computed<Coupon | null>(() => {
  const locked = offers.value
    .filter(c => (c.min_order_amount || 0) > cartSubtotal.value)
    .sort((a, b) => a.min_order_amount - b.min_order_amount)
  return locked[0] || null
})
const unlockRemaining = computed(() =>
  nextOffer.value ? Math.max(0, nextOffer.value.min_order_amount - cartSubtotal.value) : 0,
)
const unlockProgress = computed(() => {
  if (!nextOffer.value || nextOffer.value.min_order_amount <= 0) return 100
  return Math.min(100, Math.round((cartSubtotal.value / nextOffer.value.min_order_amount) * 100))
})
const hasUnlockedOffer = computed(() =>
  offers.value.some(c => isEligible(c)),
)

async function loadOffers() {
  try {
    offers.value = await customer.coupons()
  } catch {
    offers.value = []
  }
}

async function applyCoupon(code: string) {
  couponError.value = ''
  couponBusy.value = true
  try {
    const q = await customer.previewCoupon(code, cartSubtotal.value)
    appliedCoupon.value = { code: q.code, discount: q.discount }
    showOffersModal.value = false
    toast.success(`Coupon applied — you saved ${formatINR(q.discount)}`)
  } catch (e) {
    couponError.value = extractApiError(e)
    toast.error(couponError.value)
  } finally {
    couponBusy.value = false
  }
}
function removeCoupon() {
  appliedCoupon.value = null
  couponError.value = ''
}

async function saveNewAddress() {
  if (!newAddr.value.contact_name.trim()) { toast.error('Please enter the full name.'); return }
  if (!newAddr.value.line1.trim()) { toast.error('Please enter address line 1.'); return }
  if (!newAddr.value.city.trim()) { toast.error('Please enter the city.'); return }
  if (!/^\d{6}$/.test(newAddr.value.pincode.trim())) { toast.error('Please enter a valid 6-digit pincode.'); return }
  savingAddr.value = true
  try {
    // label/address_type are fixed defaults (no longer shown in the form).
    const res = await customer.addAddress({ ...newAddr.value, label: 'Home', address_type: 'home' })
    addresses.value = (await customer.addresses()) || []
    selectedAddr.value = res.id
    showNewAddr.value = false
    newAddr.value = { contact_name:'', contact_phone:'', line1:'', line2:'', landmark:'', city:'', state:'', pincode:'', country: DEFAULT_COUNTRY, is_default: false }
    toast.success('Address saved')
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    savingAddr.value = false
  }
}

function chooseAddress(id: string) {
  selectedAddr.value = id
}
function confirmAddress() {
  if (!selectedAddr.value) { toast.error('Please select an address.'); return }
  showAddrModal.value = false
}

/* ── Load ───────────────────────────────────────────── */
onMounted(async () => {
  try {
    await cart.refresh()
    addresses.value = (await customer.addresses().catch(() => [])) || []
    const def = addresses.value.find(a => a.is_default) || addresses.value[0]
    if (def) selectedAddr.value = def.id
    await loadOffers()
    if (!cart.lines.length) {
      toast.info('Your cart is empty.')
      router.replace('/cart')
    }
    // If the only cart contents block COD, default the method to online.
    if (!codAvailable.value) paymentMethod.value = 'online'
  } finally {
    loading.value = false
  }
})

/* ── Step navigation ────────────────────────────────── */
function goToPayment() {
  if (!selectedAddr.value) {
    toast.error('Please select a delivery address.')
    showAddrModal.value = true
    return
  }
  step.value = 2
}

/* ── Place order ────────────────────────────────────── */
async function placeOrder() {
  error.value = ''
  if (!selectedAddr.value) { error.value = 'Please select a delivery address.'; step.value = 1; return }
  if (paymentMethod.value === 'cod' && !codAvailable.value) {
    error.value = 'Cash on Delivery isn\u2019t available for your cart. Please pay online.'
    paymentMethod.value = 'online'
    step.value = 2
    return
  }
  placing.value = true
  try {
    const order = await customer.checkout(
      paymentMethod.value,
      selectedAddr.value,
      appliedCoupon.value?.code,
      selectedListingIds.value.length ? selectedListingIds.value : undefined,
    )

    if (paymentMethod.value === 'online') {
      try {
        const gw = await customer.payOrder(order.id)
        const isRazorpay = gw.key_id.startsWith('rzp_')
        if (isRazorpay) {
          const result = await openRazorpayCheckout({
            order: gw,
            name: 'CARLITOS',
            description: `Order ${order.order_number || order.id}`,
            prefill: {
              name: authStore.user?.full_name || '',
              email: authStore.user?.email || '',
              contact: selectedAddrObj.value?.contact_phone || '',
            },
          })
          await customer.verifyPayment(result)
        } else {
          await customer.completeMockPayment(order.id)
        }
      } catch (payErr) {
        try { await customer.abandonOrder(order.id) } catch { /* best-effort */ }
        await cart.refresh()
        error.value = extractApiError(payErr)
        step.value = 2
        placing.value = false
        toast.error(error.value || 'Payment was not completed. Please try again.')
        return
      }
    }

    await cart.refresh()
    cart.clearGuest()
    placedOrder.value = {
      id: order.id,
      order_number: order.order_number,
      grand_total: order.grand_total,
    }
    showSuccess.value = true
  } catch (e) {
    error.value = extractApiError(e)
    placing.value = false
    // COD rejected server-side → bounce back to the payment step.
    step.value = 2
    toast.error(error.value)
  }
}

function goToPlacedOrder() {
  const id = placedOrder.value?.id
  showSuccess.value = false
  if (id) router.push({ path: `/orders/${id}`, query: { placed: '1' } })
}

function back() {
  if (step.value > 1) { step.value -= 1; return }
  router.push('/cart')
}

function addrLine(a: Address): string {
  return [a.line1, a.line2, a.city, a.state].filter(Boolean).join(', ')
}
</script>

<template>
  <div class="mx-auto max-w-2xl">

    <!-- Compact top bar: back + title on one line -->
    <div class="mb-3 flex items-center gap-2">
      <button
        type="button"
        class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-stone-200 text-stone-600 transition-colors hover:bg-stone-50 hover:text-ink"
        aria-label="Go back"
        @click="back"
      >
        <ChevronLeft :size="15" />
      </button>
      <h1 class="text-base font-semibold text-ink">
        {{ step === 1 ? 'Address' : step === 2 ? 'Payment' : 'Review' }}
      </h1>
      <span v-if="cartItems.length" class="ml-auto text-xs text-stone-500">
        {{ cart.count }} item{{ cart.count > 1 ? 's' : '' }}
      </span>
    </div>

    <div v-if="loading" class="space-y-2">
      <div class="skeleton h-24 w-full rounded-lg" />
      <div class="skeleton h-32 w-full rounded-lg" />
    </div>

    <div v-else class="space-y-2.5">

      <!-- ORDER DETAILS (always near the top) -->
      <section class="card p-3">
        <h2 class="mb-2 text-xs font-semibold text-stone-700">Order details</h2>
        <div class="space-y-2">
          <div v-for="item in cartItems" :key="item.id" class="flex items-center gap-2.5">
            <div class="h-12 w-9 shrink-0 overflow-hidden rounded bg-stone-100">
              <img v-if="item.image" :src="resolveMediaUrl(MEDIA_BASE, item.image)" :alt="item.product_title" class="h-full w-full object-cover" loading="lazy" />
            </div>
            <div class="flex-1 min-w-0">
              <p class="truncate text-xs font-medium text-ink">{{ item.product_title }}</p>
              <p class="mt-0.5 text-[11px] text-stone-500">{{ item.seller_name }} · Qty {{ item.quantity }}</p>
              <p v-if="item.seller_cod_enabled === false" class="mt-0.5 inline-flex items-center gap-1 text-[11px] text-amber-600">
                <Wallet :size="11" /> COD not available
              </p>
            </div>
            <span class="shrink-0 text-xs font-semibold">{{ formatINR(item.line_total) }}</span>
          </div>
        </div>
      </section>

      <!-- STEP 1: ADDRESS -->
      <template v-if="step === 1">
        <section class="card p-3">
          <div class="mb-2 flex items-center gap-1.5">
            <MapPin :size="15" class="text-stone-500" />
            <h2 class="text-xs font-semibold text-stone-700">Delivery address</h2>
          </div>

          <!-- Selected summary → tap to change via popup -->
          <button
            v-if="selectedAddrObj"
            type="button"
            class="flex w-full items-start justify-between gap-2.5 rounded-lg border border-stone-200 p-2.5 text-left transition-colors hover:border-stone-300"
            @click="showAddrModal = true"
          >
            <div class="text-xs">
              <div class="flex flex-wrap items-center gap-1.5">
                <span class="font-medium">{{ selectedAddrObj.contact_name }}</span>
                <span class="badge-gray">{{ selectedAddrObj.label || selectedAddrObj.address_type }}</span>
                <span v-if="selectedAddrObj.is_default" class="badge-green">Default</span>
              </div>
              <p class="mt-0.5 text-stone-600">{{ selectedAddrObj.contact_phone }}</p>
              <p class="text-stone-600">{{ addrLine(selectedAddrObj) }} – {{ selectedAddrObj.pincode }}</p>
            </div>
            <span class="shrink-0 text-[11px] font-medium text-ink underline">Change</span>
          </button>

          <!-- No address yet -->
          <button
            v-else
            type="button"
            class="flex w-full items-center justify-center gap-1.5 rounded-lg border border-dashed border-stone-300 p-3 text-xs font-medium text-stone-600 transition-colors hover:border-ink hover:text-ink"
            @click="showAddrModal = true; showNewAddr = !addresses.length"
          >
            <Plus :size="14" /> {{ addresses.length ? 'Select a delivery address' : 'Add a delivery address' }}
          </button>
        </section>

        <!-- Offers -->
        <section class="card p-3">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-1.5">
              <BadgePercent :size="15" class="text-accent" />
              <h2 class="text-xs font-semibold text-stone-700">Offers &amp; coupons</h2>
            </div>
            <button class="text-[11px] font-medium text-ink underline" @click="showOffersModal = true">View all</button>
          </div>

          <!-- Applied coupon -->
          <div v-if="appliedCoupon" class="mt-2 flex items-center justify-between rounded-lg border border-emerald-200 bg-emerald-50 px-2.5 py-1.5">
            <span class="flex items-center gap-1 text-[11px] text-emerald-700">
              <Tag :size="12" /> <span class="font-mono font-semibold">{{ appliedCoupon.code }}</span> · saved {{ formatINR(appliedCoupon.discount) }}
            </span>
            <button class="text-[11px] text-stone-500 underline hover:text-ink" @click="removeCoupon">Remove</button>
          </div>

          <!-- Myntra-style unlock progress -->
          <div v-if="nextOffer" class="mt-2">
            <p class="text-[11px] text-stone-600">
              Add <span class="font-semibold text-ink">{{ formatINR(unlockRemaining) }}</span> more to unlock
              <span class="font-semibold text-accent">{{ couponBenefit(nextOffer) }}</span>
            </p>
            <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-stone-100">
              <div class="h-full rounded-full bg-accent transition-all duration-500" :style="{ width: unlockProgress + '%' }" />
            </div>
          </div>
          <p v-else-if="hasUnlockedOffer && !appliedCoupon" class="mt-2 text-[11px] text-emerald-700">
            🎉 You’ve unlocked offers — tap “View all” to apply one.
          </p>
        </section>

        <CheckoutPriceSummary :subtotal="cartSubtotal" :total="payableTotal" :coupon="appliedCoupon" />

        <button class="btn-primary w-full" @click="goToPayment">
          Continue to payment <ChevronRight :size="16" />
        </button>
      </template>

      <!-- STEP 2: PAYMENT -->
      <template v-else-if="step === 2">
        <section class="card p-3">
          <div class="mb-3 flex items-center gap-1.5">
            <CreditCard :size="15" class="text-stone-500" />
            <h2 class="text-xs font-semibold text-stone-700">Payment method</h2>
          </div>

          <div class="space-y-2.5">
            <!-- Pay Online -->
            <label
              class="group relative block cursor-pointer rounded-xl border p-3 transition-all"
              :class="paymentMethod === 'online'
                ? 'border-ink bg-stone-50 ring-1 ring-ink'
                : 'border-stone-200 hover:border-stone-300'"
            >
              <input v-model="paymentMethod" type="radio" value="online" class="peer sr-only" />
              <div class="flex items-start gap-3">
                <span
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors"
                  :class="paymentMethod === 'online' ? 'bg-ink text-white' : 'bg-stone-100 text-stone-500'"
                >
                  <Smartphone :size="18" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center justify-between gap-2">
                    <p class="text-sm font-semibold text-ink">Pay Online</p>
                    <span
                      class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition-colors"
                      :class="paymentMethod === 'online' ? 'border-ink bg-ink text-white' : 'border-stone-300'"
                    >
                      <Check v-if="paymentMethod === 'online'" :size="11" />
                    </span>
                  </div>
                  <p class="mt-0.5 text-[11px] text-stone-500">UPI, Cards, Net Banking &amp; Wallets</p>
                  <!-- Real payment provider logos -->
                  <div class="mt-2 flex flex-wrap items-center gap-1.5">
                    <img src="/payments/upi.svg" alt="UPI" class="h-6 w-auto rounded border border-stone-200 bg-white" loading="lazy" />
                    <img src="/payments/gpay.svg" alt="Google Pay" class="h-6 w-auto rounded border border-stone-200 bg-white" loading="lazy" />
                    <img src="/payments/phonepe.svg" alt="PhonePe" class="h-6 w-auto rounded border border-stone-200 bg-white" loading="lazy" />
                    <img src="/payments/paytm.svg" alt="Paytm" class="h-6 w-auto rounded border border-stone-200 bg-white" loading="lazy" />
                    <span class="inline-flex items-center gap-1 rounded-md border border-stone-200 bg-white px-1.5 py-1 text-[10px] font-medium text-stone-600">
                      <CreditCard :size="12" class="text-indigo-600" /> Cards
                    </span>
                  </div>
                  <p class="mt-2 inline-flex items-center gap-1 text-[10px] text-stone-400">
                    <ShieldCheck :size="11" class="text-emerald-500" /> Secured by
                    <img src="/payments/razorpay.svg" alt="Razorpay" class="h-3.5 w-auto" loading="lazy" />
                  </p>
                </div>
              </div>
            </label>

            <!-- Cash on Delivery -->
            <label
              class="group relative block rounded-xl border p-3 transition-all"
              :class="!codAvailable
                ? 'cursor-not-allowed border-stone-200 bg-stone-50 opacity-70'
                : paymentMethod === 'cod'
                  ? 'cursor-pointer border-ink bg-stone-50 ring-1 ring-ink'
                  : 'cursor-pointer border-stone-200 hover:border-stone-300'"
            >
              <input v-model="paymentMethod" type="radio" value="cod" class="peer sr-only" :disabled="!codAvailable" />
              <div class="flex items-start gap-3">
                <span
                  class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors"
                  :class="!codAvailable
                    ? 'bg-stone-100 text-stone-400'
                    : paymentMethod === 'cod' ? 'bg-ink text-white' : 'bg-stone-100 text-stone-500'"
                >
                  <Banknote :size="18" />
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center justify-between gap-2">
                    <p class="text-sm font-semibold" :class="codAvailable ? 'text-ink' : 'text-stone-500'">
                      Cash on Delivery
                    </p>
                    <span
                      v-if="codAvailable"
                      class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition-colors"
                      :class="paymentMethod === 'cod' ? 'border-ink bg-ink text-white' : 'border-stone-300'"
                    >
                      <Check v-if="paymentMethod === 'cod'" :size="11" />
                    </span>
                  </div>
                  <template v-if="codAvailable">
                    <p class="mt-0.5 text-[11px] text-stone-500">Pay in cash when your order arrives.</p>
                    <!-- COD badges (mirrors the online card) -->
                    <div class="mt-2 flex flex-wrap items-center gap-1.5">
                      <img src="/payments/cod.svg" alt="Cash on Delivery" class="h-6 w-auto rounded border border-stone-200 bg-white" loading="lazy" />
                      <span class="inline-flex items-center gap-1 rounded-md border border-stone-200 bg-white px-1.5 py-1 text-[10px] font-medium text-stone-600">
                        <Wallet :size="12" class="text-emerald-600" /> Pay at doorstep
                      </span>
                    </div>
                    <p class="mt-2 inline-flex items-center gap-1 text-[10px] text-stone-400">
                      <ShieldCheck :size="11" class="text-emerald-500" /> No advance payment needed
                    </p>
                  </template>
                  <!-- Disabled note when any seller in the cart doesn't offer COD -->
                  <div v-else class="mt-1.5 flex items-start gap-1.5 rounded-lg bg-amber-50 px-2 py-1.5 text-[11px] text-amber-700">
                    <Info :size="12" class="mt-px shrink-0" />
                    <span>
                      Not available — {{ codBlockedBy.join(', ') }}
                      {{ codBlockedBy.length > 1 ? "don't" : "doesn't" }} offer Cash on Delivery for this order.
                    </span>
                  </div>
                </div>
              </div>
            </label>
          </div>

          <p v-if="error" class="mt-2 rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-600">{{ error }}</p>
        </section>

        <CheckoutPriceSummary :subtotal="cartSubtotal" :total="payableTotal" :coupon="appliedCoupon" />

        <button class="btn-primary w-full" @click="step = 3">
          Continue to review <ChevronRight :size="16" />
        </button>
      </template>

      <!-- STEP 3: REVIEW -->
      <template v-else>
        <section class="card p-3">
          <div class="mb-1.5 flex items-center justify-between">
            <p class="text-xs font-semibold text-stone-700">Delivery to</p>
            <button class="text-[11px] font-medium text-ink underline" @click="step = 1">Change</button>
          </div>
          <p class="text-xs text-stone-700">{{ selectedAddrObj?.contact_name }} · {{ selectedAddrObj?.contact_phone }}</p>
          <p class="text-xs text-stone-600">{{ selectedAddrObj ? addrLine(selectedAddrObj) : '' }} – {{ selectedAddrObj?.pincode }}</p>
        </section>

        <section class="card p-3">
          <div class="mb-1.5 flex items-center justify-between">
            <p class="text-xs font-semibold text-stone-700">Payment</p>
            <button class="text-[11px] font-medium text-ink underline" @click="step = 2">Change</button>
          </div>
          <p class="text-xs text-stone-700">{{ paymentMethod === 'cod' ? 'Cash on Delivery' : 'Online Payment' }}</p>
        </section>

        <CheckoutPriceSummary :subtotal="cartSubtotal" :total="payableTotal" :coupon="appliedCoupon" />

        <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-600">{{ error }}</p>

        <button class="btn-primary w-full" :disabled="placing" @click="placeOrder">
          {{ placing ? 'Placing order…' : (paymentMethod === 'online' ? 'Pay & place order' : 'Place order') }}
        </button>
      </template>
    </div>

    <!-- ══════════ Address picker popup ══════════ -->
    <Teleport to="body">
      <div
        v-if="showAddrModal"
        class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
        @click.self="showAddrModal = false"
      >
        <div class="flex max-h-[90vh] w-full flex-col overflow-hidden rounded-t-2xl bg-white sm:max-w-lg sm:rounded-2xl">
          <div class="flex items-center justify-between border-b border-stone-200 p-4">
            <h3 class="text-h4">{{ showNewAddr ? 'Add new address' : 'Select delivery address' }}</h3>
            <button class="text-stone-400 hover:text-ink" aria-label="Close" @click="showAddrModal = false"><X :size="20" /></button>
          </div>

          <div class="flex-1 overflow-y-auto p-4">
            <!-- Address cards -->
            <div v-if="!showNewAddr" class="space-y-2">
              <button
                v-for="a in addresses"
                :key="a.id"
                type="button"
                class="flex w-full items-start gap-3 rounded-lg border p-3 text-left transition-colors"
                :class="selectedAddr === a.id ? 'border-ink bg-stone-50' : 'border-stone-200 hover:border-stone-300'"
                @click="chooseAddress(a.id)"
              >
                <span
                  class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border"
                  :class="selectedAddr === a.id ? 'border-ink bg-ink text-white' : 'border-stone-300'"
                >
                  <Check v-if="selectedAddr === a.id" :size="11" />
                </span>
                <div class="text-sm">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-medium">{{ a.contact_name }}</span>
                    <span class="badge-gray">{{ a.label || a.address_type }}</span>
                    <span v-if="a.is_default" class="badge-green">Default</span>
                  </div>
                  <p class="mt-0.5 text-stone-600">{{ a.contact_phone }}</p>
                  <p class="text-stone-600">{{ addrLine(a) }} – {{ a.pincode }}</p>
                </div>
              </button>

              <button
                type="button"
                class="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-stone-300 p-3 text-sm font-medium text-stone-600 transition-colors hover:border-ink hover:text-ink"
                @click="showNewAddr = true"
              >
                <Plus :size="16" /> Add new address
              </button>
            </div>

            <!-- New address form -->
            <div v-else class="space-y-3">
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
              <label class="flex cursor-pointer items-center gap-2 text-sm">
                <input v-model="newAddr.is_default" type="checkbox" class="accent-ink" />
                Set as default address
              </label>
            </div>
          </div>

          <!-- Footer actions -->
          <div class="flex gap-3 border-t border-stone-200 p-4">
            <template v-if="showNewAddr">
              <button class="btn-ghost flex-1" :disabled="savingAddr" @click="showNewAddr = false">
                {{ addresses.length ? 'Cancel' : 'Back' }}
              </button>
              <button class="btn-primary flex-1" :disabled="savingAddr" @click="saveNewAddress">
                {{ savingAddr ? 'Saving…' : 'Save address' }}
              </button>
            </template>
            <button v-else class="btn-primary w-full" :disabled="!selectedAddr" @click="confirmAddress">
              Deliver here
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- ══════════ Offers popup ══════════ -->
    <Teleport to="body">
      <div
        v-if="showOffersModal"
        class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
        @click.self="showOffersModal = false"
      >
        <div class="flex max-h-[90vh] w-full flex-col overflow-hidden rounded-t-2xl bg-white sm:max-w-lg sm:rounded-2xl">
          <div class="flex items-center justify-between border-b border-stone-200 p-4">
            <h3 class="flex items-center gap-2 text-h4"><BadgePercent :size="18" class="text-accent" /> Available offers</h3>
            <button class="text-stone-400 hover:text-ink" aria-label="Close" @click="showOffersModal = false"><X :size="20" /></button>
          </div>

          <div class="flex-1 overflow-y-auto p-4">
            <p v-if="!offers.length" class="py-8 text-center text-sm text-stone-500">No offers available right now.</p>
            <div v-else class="space-y-2.5">
              <div
                v-for="c in offers"
                :key="c.id"
                class="rounded-lg border p-3 transition-colors"
                :class="isEligible(c) ? 'border-stone-200' : 'border-stone-200 bg-stone-50 opacity-60'"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2">
                      <span class="inline-flex items-center gap-1 rounded border border-dashed border-stone-400 px-2 py-0.5 font-mono text-xs font-semibold text-ink">
                        <Tag :size="12" /> {{ c.code }}
                      </span>
                      <span class="text-sm font-semibold text-emerald-700">{{ couponBenefit(c) }}</span>
                    </div>
                    <p v-if="c.description" class="mt-1 text-xs text-stone-500">{{ c.description }}</p>
                    <p v-if="!isEligible(c)" class="mt-1 text-xs text-amber-600">
                      Add {{ formatINR(c.min_order_amount - cartSubtotal) }} more to use this
                    </p>
                  </div>
                  <button
                    class="btn-sm shrink-0"
                    :class="isEligible(c) ? 'btn-primary' : 'btn-ghost pointer-events-none'"
                    :disabled="!isEligible(c) || couponBusy"
                    @click="applyCoupon(c.code)"
                  >
                    {{ isEligible(c) ? 'Apply' : 'Locked' }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <OrderSuccessModal
      :show="showSuccess"
      :order-number="placedOrder?.order_number"
      :amount="placedOrder ? formatINR(placedOrder.grand_total) : ''"
      @view="goToPlacedOrder"
    />
  </div>
</template>
