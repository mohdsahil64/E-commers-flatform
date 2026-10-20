<script setup lang="ts">
import { computed, onMounted, ref, nextTick } from 'vue'
import {
  Package, CreditCard, LogIn, RotateCcw, HelpCircle, UserCog,
  Search, X, ImagePlus, Send, CheckCircle2, Clock, ChevronRight,
  MessageSquare, ArrowLeft, ShieldCheck, Store,
} from 'lucide-vue-next'
import type { Order, OrderItem, SupportTicket } from '@carlitos/types'
import { customer, MEDIA_BASE } from '@/api'
import { resolveMediaUrl, extractApiError } from '@carlitos/shared'
import { useToastStore } from '@/stores/toast'
import { useSeo } from '@/composables/useSeo'
import PageBack from '@/components/PageBack.vue'

const toast = useToastStore()
useSeo({ title: 'Help Centre', description: 'Raise a help request and track its status.' })

/* ─────────────────────────────────────────────────────────────────
 * Two panes: a chat-style "new request" wizard and a "my requests" list.
 * The wizard walks category → (product pick) → describe + photos → done.
 * ───────────────────────────────────────────────────────────────── */
type Pane = 'new' | 'list' | 'detail'
const pane = ref<Pane>('new')

/* ── Category step ─────────────────────────────────────────────── */
interface CategoryDef {
  code: 'product' | 'payment' | 'login' | 'account' | 'refund' | 'other'
  label: string
  hint: string
  icon: any
  needsProduct: boolean
}
const categories: CategoryDef[] = [
  { code: 'product', label: 'Product / Order issue', hint: 'Damaged, wrong, or missing item', icon: Package,    needsProduct: true },
  { code: 'payment', label: 'Payment issue',         hint: 'Charged twice, failed payment',   icon: CreditCard, needsProduct: false },
  { code: 'refund',  label: 'Refund query',          hint: 'Refund not received yet',          icon: RotateCcw,  needsProduct: false },
  { code: 'login',   label: 'Login / Access',        hint: 'Can’t sign in or get OTP',         icon: LogIn,      needsProduct: false },
  { code: 'account', label: 'Account issue',         hint: 'Profile, email or number',         icon: UserCog,    needsProduct: false },
  { code: 'other',   label: 'Something else',        hint: 'Any other help you need',          icon: HelpCircle, needsProduct: false },
]

type Step = 'category' | 'product' | 'describe' | 'done'
const step = ref<Step>('category')
const selectedCategory = ref<CategoryDef | null>(null)

function pickCategory(cat: CategoryDef) {
  selectedCategory.value = cat
  if (cat.needsProduct) {
    step.value = 'product'
    loadOrderedItems()
  } else {
    step.value = 'describe'
  }
}

/* ── Product pick step ─────────────────────────────────────────── */
// Flatten ordered items across the customer's orders into one searchable list.
interface OrderedItem {
  orderItemId: string
  orderNumber: string
  title: string
  image?: string
  variant?: string
  placedAt: string | null
}
const orderedItems = ref<OrderedItem[]>([])
const loadingItems = ref(false)
const productSearch = ref('')
const selectedItem = ref<OrderedItem | null>(null)

async function loadOrderedItems() {
  if (orderedItems.value.length) return
  loadingItems.value = true
  try {
    const res = await customer.orders()
    const orders: Order[] = res.data ?? []
    const flat: OrderedItem[] = []
    for (const o of orders) {
      for (const it of (o.items ?? []) as OrderItem[]) {
        flat.push({
          orderItemId: it.id,
          orderNumber: o.order_number,
          title: it.product_title,
          image: it.image,
          variant: it.variant_label,
          placedAt: o.placed_at || o.created_at,
        })
      }
    }
    orderedItems.value = flat
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    loadingItems.value = false
  }
}

const filteredItems = computed(() => {
  const q = productSearch.value.trim().toLowerCase()
  if (!q) return orderedItems.value
  return orderedItems.value.filter(
    (i) => i.title.toLowerCase().includes(q) || i.orderNumber.toLowerCase().includes(q),
  )
})

function pickItem(item: OrderedItem) {
  selectedItem.value = item
  step.value = 'describe'
}

/* ── Describe + photos step ────────────────────────────────────── */
const description = ref('')
const photoKeys = ref<string[]>([])
const photoPreviews = ref<string[]>([])
const uploading = ref(false)
const submitting = ref(false)

// Chat-style quick suggestions tailored to the chosen category.
const suggestions = computed<string[]>(() => {
  switch (selectedCategory.value?.code) {
    case 'product':
      return ['Item arrived damaged', 'Received wrong product', 'Size/colour is different', 'Part of the order is missing']
    case 'payment':
      return ['I was charged twice', 'Money debited but order failed', 'Payment keeps failing']
    case 'refund':
      return ['Refund not received yet', 'Refund amount is wrong', 'How long will my refund take?']
    case 'login':
      return ['Not receiving OTP', 'Can’t sign in', 'Account seems locked']
    case 'account':
      return ['Update my email', 'Change my phone number', 'Wrong details on profile']
    default:
      return ['I have a general question', 'Need help with my order']
  }
})

function applySuggestion(s: string) {
  description.value = description.value ? description.value + '\n' + s : s
}

async function onPhoto(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (photoKeys.value.length >= 4) {
    toast.error('You can attach up to 4 photos')
    if (input) input.value = ''
    return
  }
  uploading.value = true
  try {
    const res = await customer.uploadImage(file)
    photoKeys.value.push(res.key)
    photoPreviews.value.push(res.url || resolveMediaUrl(MEDIA_BASE, res.key))
  } catch (err) {
    toast.error(extractApiError(err))
  } finally {
    uploading.value = false
    if (input) input.value = ''
  }
}

function removePhoto(i: number) {
  photoKeys.value.splice(i, 1)
  photoPreviews.value.splice(i, 1)
}

const canSubmit = computed(() => description.value.trim().length >= 5 && !submitting.value)

const createdTicketNumber = ref('')

async function submit() {
  if (!selectedCategory.value) return
  if (!canSubmit.value) {
    toast.error('Please describe your issue (at least a few words)')
    return
  }
  submitting.value = true
  try {
    await customer.openSupportTicket({
      category: selectedCategory.value.code,
      description: description.value.trim(),
      order_item_id: selectedItem.value?.orderItemId,
      photo_keys: photoKeys.value,
    })
    step.value = 'done'
    // Refresh the ticket list in the background so switching tabs is instant.
    loadTickets(true)
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    submitting.value = false
  }
}

function resetWizard() {
  step.value = 'category'
  selectedCategory.value = null
  selectedItem.value = null
  productSearch.value = ''
  description.value = ''
  photoKeys.value = []
  photoPreviews.value = []
  createdTicketNumber.value = ''
}

function goBackStep() {
  if (step.value === 'describe') {
    step.value = selectedCategory.value?.needsProduct ? 'product' : 'category'
  } else if (step.value === 'product') {
    step.value = 'category'
  }
}

/* ── My requests (list + detail) ───────────────────────────────── */
const tickets = ref<SupportTicket[]>([])
const loadingTickets = ref(false)
const activeTicket = ref<SupportTicket | null>(null)
const replyBody = ref('')
const replying = ref(false)
const timelineEl = ref<HTMLElement | null>(null)

async function loadTickets(silent = false) {
  if (!silent) loadingTickets.value = true
  try {
    tickets.value = await customer.supportTickets()
    // Keep the open detail in sync if present.
    if (activeTicket.value) {
      const fresh = tickets.value.find((t) => t.id === activeTicket.value!.id)
      if (fresh) activeTicket.value = fresh
    }
  } catch (e) {
    if (!silent) toast.error(extractApiError(e))
  } finally {
    loadingTickets.value = false
  }
}

function openTicket(t: SupportTicket) {
  activeTicket.value = t
  pane.value = 'detail'
  nextTick(scrollTimeline)
}

function scrollTimeline() {
  const el = timelineEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function sendReply() {
  if (!activeTicket.value || !replyBody.value.trim()) return
  replying.value = true
  try {
    await customer.replySupportTicket(activeTicket.value.id, replyBody.value.trim())
    replyBody.value = ''
    await loadTickets(true)
    nextTick(scrollTimeline)
  } catch (e) {
    toast.error(extractApiError(e))
  } finally {
    replying.value = false
  }
}

function switchPane(p: Pane) {
  pane.value = p
  if (p === 'list') loadTickets()
}

/* ── Display helpers ───────────────────────────────────────────── */
const categoryLabelMap: Record<string, string> = {
  product: 'Product / Order',
  payment: 'Payment',
  refund: 'Refund',
  login: 'Login / Access',
  account: 'Account',
  other: 'Other',
}
function catLabel(c: string) { return categoryLabelMap[c] || 'Help' }

function statusBadgeClass(s: string) {
  if (s === 'solved') return 'badge-green'
  if (s === 'in_progress') return 'badge-blue'
  return 'badge-amber'
}
function statusLabel(s: string) {
  if (s === 'solved') return 'Solved'
  if (s === 'in_progress') return 'In progress'
  return 'Open'
}

function authorLabel(role: string, t: SupportTicket) {
  if (role === 'customer') return 'You'
  if (role === 'seller') return t.seller_name || 'Seller'
  if (role === 'admin') return 'Support Team'
  return 'System'
}

function fmtTime(iso: string) {
  const d = new Date(iso)
  return d.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
}

const openCount = computed(() => tickets.value.filter((t) => t.status !== 'solved').length)

onMounted(() => {
  // Prefetch tickets so the "My Requests" tab shows an accurate count.
  loadTickets(true)
})
</script>

<template>
  <div class="mx-auto max-w-2xl pb-24 sm:pb-8">
    <PageBack history />

    <!-- Header -->
    <div class="mb-4">
      <h1 class="text-xl font-semibold text-ink">Help &amp; Support</h1>
      <p class="mt-0.5 text-xs text-stone-500">Raise a request and track it end to end.</p>
    </div>

    <!-- Tab switch -->
    <div class="mb-5 grid grid-cols-2 gap-1 rounded-full bg-stone-100 p-1 text-sm">
      <button
        class="rounded-full py-2 font-medium transition-colors"
        :class="pane === 'new' ? 'bg-white text-ink shadow-sm' : 'text-stone-500'"
        @click="switchPane('new')"
      >
        New Request
      </button>
      <button
        class="flex items-center justify-center gap-1.5 rounded-full py-2 font-medium transition-colors"
        :class="pane !== 'new' ? 'bg-white text-ink shadow-sm' : 'text-stone-500'"
        @click="switchPane('list')"
      >
        My Requests
        <span v-if="openCount" class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-accent px-1 text-[10px] font-semibold text-white">{{ openCount }}</span>
      </button>
    </div>

    <!-- ══════════════ NEW REQUEST WIZARD ══════════════ -->
    <div v-if="pane === 'new'">
      <!-- Progress dots -->
      <div v-if="step !== 'done'" class="mb-4 flex items-center gap-1.5">
        <span class="h-1.5 flex-1 rounded-full" :class="step === 'category' ? 'bg-accent' : 'bg-stone-800'" />
        <span v-if="selectedCategory?.needsProduct" class="h-1.5 flex-1 rounded-full" :class="step === 'product' ? 'bg-accent' : (step === 'describe' ? 'bg-stone-800' : 'bg-stone-200')" />
        <span class="h-1.5 flex-1 rounded-full" :class="step === 'describe' ? 'bg-accent' : 'bg-stone-200'" />
      </div>

      <!-- STEP 1: category -->
      <div v-if="step === 'category'">
        <p class="mb-3 text-sm font-medium text-ink">What do you need help with?</p>
        <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
          <button
            v-for="cat in categories"
            :key="cat.code"
            class="card flex items-start gap-3 p-3.5 text-left transition-all hover:border-ink hover:shadow-sm active:scale-[0.99]"
            @click="pickCategory(cat)"
          >
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent-soft text-accent">
              <component :is="cat.icon" :size="17" />
            </span>
            <span class="min-w-0 flex-1">
              <span class="block text-sm font-semibold text-ink">{{ cat.label }}</span>
              <span class="block text-xs text-stone-500">{{ cat.hint }}</span>
            </span>
            <ChevronRight :size="16" class="mt-1 shrink-0 text-stone-300" />
          </button>
        </div>
      </div>

      <!-- STEP 2: product pick -->
      <div v-else-if="step === 'product'">
        <button class="mb-3 inline-flex items-center gap-1 text-xs text-stone-500 hover:text-ink" @click="goBackStep">
          <ArrowLeft :size="14" /> Back
        </button>
        <p class="mb-1 text-sm font-medium text-ink">Which product is this about?</p>
        <p class="mb-3 text-xs text-stone-500">Pick from the items you've ordered.</p>

        <div class="relative mb-3">
          <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
          <input v-model="productSearch" class="input pl-9" placeholder="Search your orders…" />
        </div>

        <div v-if="loadingItems" class="space-y-2">
          <div v-for="i in 4" :key="i" class="skeleton h-16 w-full rounded-lg" />
        </div>

        <div v-else-if="!orderedItems.length" class="card p-6 text-center">
          <Package :size="26" class="mx-auto mb-2 text-stone-300" />
          <p class="text-sm font-medium text-ink">No orders found</p>
          <p class="mt-1 text-xs text-stone-500">For non-product issues, pick a different category.</p>
        </div>

        <div v-else class="max-h-[22rem] space-y-2 overflow-y-auto pr-1">
          <button
            v-for="item in filteredItems"
            :key="item.orderItemId"
            class="card flex w-full items-center gap-3 p-2.5 text-left transition-colors hover:border-ink hover:bg-stone-50"
            @click="pickItem(item)"
          >
            <div class="h-14 w-12 shrink-0 overflow-hidden rounded-md bg-stone-100">
              <img v-if="item.image" :src="resolveMediaUrl(MEDIA_BASE, item.image)" :alt="item.title" class="h-full w-full object-cover" loading="lazy" />
              <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="16" /></div>
            </div>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium text-ink">{{ item.title }}</p>
              <p v-if="item.variant" class="truncate text-xs text-stone-500">{{ item.variant }}</p>
              <p class="mt-0.5 text-[11px] text-stone-400">Order {{ item.orderNumber }}</p>
            </div>
            <ChevronRight :size="16" class="shrink-0 text-stone-300" />
          </button>
          <p v-if="!filteredItems.length" class="py-6 text-center text-xs text-stone-500">No items match "{{ productSearch }}".</p>
        </div>
      </div>

      <!-- STEP 3: describe + photos -->
      <div v-else-if="step === 'describe'">
        <button class="mb-3 inline-flex items-center gap-1 text-xs text-stone-500 hover:text-ink" @click="goBackStep">
          <ArrowLeft :size="14" /> Back
        </button>

        <!-- Context chips -->
        <div class="mb-3 flex flex-wrap items-center gap-2">
          <span class="badge-accent inline-flex items-center gap-1">
            <component :is="selectedCategory?.icon" :size="12" /> {{ selectedCategory?.label }}
          </span>
          <span v-if="selectedItem" class="badge-gray inline-flex items-center gap-1 max-w-[16rem] truncate">
            <Package :size="12" /> <span class="truncate">{{ selectedItem.title }}</span>
          </span>
        </div>

        <p class="mb-2 text-sm font-medium text-ink">Tell us what happened</p>

        <!-- Auto-suggestion chips -->
        <div class="mb-2.5 flex gap-2 overflow-x-auto scrollbar-hide">
          <button
            v-for="s in suggestions"
            :key="s"
            class="shrink-0 rounded-full border border-stone-200 bg-white px-3 py-1.5 text-xs text-stone-600 transition-colors hover:border-ink hover:text-ink"
            @click="applySuggestion(s)"
          >
            {{ s }}
          </button>
        </div>

        <textarea
          v-model="description"
          class="textarea min-h-[110px] text-sm"
          placeholder="Describe your issue in a few lines…"
          maxlength="2000"
        />

        <!-- Photo proof -->
        <div class="mt-3">
          <p class="mb-1.5 text-xs font-medium text-stone-600">Add photos (optional)</p>
          <div class="flex flex-wrap items-center gap-2">
            <div
              v-for="(p, i) in photoPreviews"
              :key="i"
              class="relative h-16 w-16 overflow-hidden rounded-md border border-stone-200"
            >
              <img :src="p" alt="Proof" class="h-full w-full object-cover" />
              <button
                class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white"
                @click="removePhoto(i)"
              >
                <X :size="12" />
              </button>
            </div>
            <label
              v-if="photoKeys.length < 4"
              class="flex h-16 w-16 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-md border border-dashed border-stone-300 text-stone-400 transition-colors hover:border-ink hover:text-ink"
            >
              <ImagePlus :size="18" />
              <span class="text-[10px]">{{ uploading ? '…' : 'Add' }}</span>
              <input type="file" accept="image/*" class="hidden" :disabled="uploading" @change="onPhoto" />
            </label>
          </div>
        </div>

        <button class="btn-primary mt-5 w-full" :disabled="!canSubmit" @click="submit">
          <Send :size="15" /> {{ submitting ? 'Submitting…' : 'Submit request' }}
        </button>
        <p class="mt-2 text-center text-[11px] text-stone-400">
          We'll email you a confirmation and respond within 24 hours.
        </p>
      </div>

      <!-- STEP 4: done -->
      <div v-else-if="step === 'done'" class="card p-7 text-center">
        <div class="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-full bg-emerald-50">
          <CheckCircle2 :size="30" class="text-emerald-600" />
        </div>
        <h2 class="text-base font-semibold text-ink">Request submitted</h2>
        <p class="mx-auto mt-1.5 max-w-sm text-sm text-stone-500">
          Your request has been sent to
          <span class="font-medium text-ink">{{ selectedCategory?.needsProduct ? 'the seller' : 'our support team' }}</span>.
          A confirmation email is on its way, and we typically respond within 24 hours.
        </p>
        <div class="mt-5 flex flex-col gap-2 sm:flex-row sm:justify-center">
          <button class="btn-primary" @click="switchPane('list')">Track my request</button>
          <button class="btn-ghost" @click="resetWizard">Raise another</button>
        </div>
      </div>
    </div>

    <!-- ══════════════ MY REQUESTS LIST ══════════════ -->
    <div v-else-if="pane === 'list'">
      <div v-if="loadingTickets" class="space-y-2.5">
        <div v-for="i in 4" :key="i" class="skeleton h-20 w-full rounded-lg" />
      </div>

      <div v-else-if="!tickets.length" class="card p-8 text-center">
        <MessageSquare :size="28" class="mx-auto mb-2 text-stone-300" />
        <p class="text-sm font-medium text-ink">No requests yet</p>
        <p class="mt-1 text-xs text-stone-500">When you raise a request, you'll track it here.</p>
        <button class="btn-primary mt-4" @click="switchPane('new')">Raise a request</button>
      </div>

      <div v-else class="space-y-2.5">
        <button
          v-for="t in tickets"
          :key="t.id"
          class="card flex w-full items-start gap-3 p-3.5 text-left transition-colors hover:border-ink hover:bg-stone-50"
          @click="openTicket(t)"
        >
          <div class="h-11 w-10 shrink-0 overflow-hidden rounded-md bg-stone-100">
            <img v-if="t.product_image" :src="resolveMediaUrl(MEDIA_BASE, t.product_image)" :alt="t.product_title" class="h-full w-full object-cover" loading="lazy" />
            <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><HelpCircle :size="16" /></div>
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center justify-between gap-2">
              <p class="truncate text-sm font-semibold text-ink">{{ t.subject }}</p>
              <span :class="statusBadgeClass(t.status)" class="shrink-0">{{ statusLabel(t.status) }}</span>
            </div>
            <p class="mt-0.5 truncate text-xs text-stone-500">
              {{ catLabel(t.category) }}<template v-if="t.product_title"> · {{ t.product_title }}</template>
            </p>
            <p class="mt-1 flex items-center gap-1.5 text-[11px] text-stone-400">
              <span>#{{ t.ticket_number }}</span>
              <span>·</span>
              <span>{{ fmtTime(t.created_at) }}</span>
            </p>
          </div>
        </button>
      </div>
    </div>

    <!-- ══════════════ TICKET DETAIL (chat timeline) ══════════════ -->
    <div v-else-if="pane === 'detail' && activeTicket">
      <button class="mb-3 inline-flex items-center gap-1 text-xs text-stone-500 hover:text-ink" @click="pane = 'list'">
        <ArrowLeft :size="14" /> All requests
      </button>

      <!-- Summary card -->
      <div class="card p-4">
        <div class="flex items-start justify-between gap-2">
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold text-ink">{{ activeTicket.subject }}</p>
            <p class="mt-0.5 text-xs text-stone-500">{{ catLabel(activeTicket.category) }} · #{{ activeTicket.ticket_number }}</p>
          </div>
          <span :class="statusBadgeClass(activeTicket.status)" class="shrink-0">{{ statusLabel(activeTicket.status) }}</span>
        </div>

        <!-- Routed-to line -->
        <div class="mt-3 flex items-center gap-1.5 rounded-lg bg-stone-50 px-3 py-2 text-xs text-stone-600">
          <component :is="activeTicket.assignee === 'seller' ? Store : ShieldCheck" :size="14" class="text-stone-400" />
          <span>Handled by
            <span class="font-medium text-ink">{{ activeTicket.assignee === 'seller' ? (activeTicket.seller_name || 'the seller') : 'CARLITOS Support' }}</span>
          </span>
        </div>

        <!-- Product context -->
        <div v-if="activeTicket.product_title" class="mt-3 flex items-center gap-2.5">
          <div class="h-12 w-11 shrink-0 overflow-hidden rounded-md bg-stone-100">
            <img v-if="activeTicket.product_image" :src="resolveMediaUrl(MEDIA_BASE, activeTicket.product_image)" :alt="activeTicket.product_title" class="h-full w-full object-cover" />
            <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="15" /></div>
          </div>
          <div class="min-w-0">
            <p class="truncate text-xs font-medium text-ink">{{ activeTicket.product_title }}</p>
            <p v-if="activeTicket.order_number" class="text-[11px] text-stone-400">Order {{ activeTicket.order_number }}</p>
          </div>
        </div>

        <!-- Photo proof -->
        <div v-if="activeTicket.photos.length" class="mt-3 flex flex-wrap gap-2">
          <a
            v-for="(ph, i) in activeTicket.photos"
            :key="i"
            :href="resolveMediaUrl(MEDIA_BASE, ph.file_key)"
            target="_blank"
            rel="noopener"
            class="block h-14 w-14 overflow-hidden rounded-md border border-stone-200"
          >
            <img :src="resolveMediaUrl(MEDIA_BASE, ph.file_key)" alt="Proof" class="h-full w-full object-cover" loading="lazy" />
          </a>
        </div>
      </div>

      <!-- Conversation timeline (chat) -->
      <div ref="timelineEl" class="mt-4 max-h-[26rem] space-y-3 overflow-y-auto px-0.5 py-1">
        <div
          v-for="m in activeTicket.messages"
          :key="m.id"
          class="flex"
          :class="m.author_role === 'customer' ? 'justify-end' : 'justify-start'"
        >
          <div class="max-w-[82%]">
            <p
              class="mb-0.5 text-[10px] font-medium text-stone-400"
              :class="m.author_role === 'customer' ? 'text-right' : 'text-left'"
            >
              {{ authorLabel(m.author_role, activeTicket) }} · {{ fmtTime(m.created_at) }}
            </p>
            <div
              class="whitespace-pre-wrap rounded-2xl px-3.5 py-2 text-sm"
              :class="m.author_role === 'customer'
                ? 'rounded-br-sm bg-accent text-white'
                : (m.author_role === 'system'
                    ? 'rounded-bl-sm bg-stone-100 text-stone-500 text-xs italic'
                    : 'rounded-bl-sm bg-stone-100 text-ink')"
            >
              {{ m.body }}
            </div>
          </div>
        </div>
      </div>

      <!-- Resolution banner -->
      <div v-if="activeTicket.status === 'solved'" class="mt-3 flex items-start gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3">
        <CheckCircle2 :size="16" class="mt-0.5 shrink-0 text-emerald-600" />
        <div>
          <p class="text-xs font-semibold text-emerald-800">This request is solved</p>
          <p v-if="activeTicket.resolution" class="mt-0.5 text-xs text-emerald-700">{{ activeTicket.resolution }}</p>
        </div>
      </div>

      <!-- Reply composer -->
      <div v-else class="mt-3 flex items-end gap-2">
        <textarea
          v-model="replyBody"
          class="textarea min-h-[46px] flex-1 py-2.5 text-sm"
          rows="1"
          placeholder="Write a reply…"
          @keydown.enter.exact.prevent="sendReply"
        />
        <button
          class="btn-primary h-[46px] w-[46px] shrink-0 !px-0"
          :disabled="replying || !replyBody.trim()"
          aria-label="Send reply"
          @click="sendReply"
        >
          <Send :size="16" />
        </button>
      </div>
    </div>
  </div>
</template>
