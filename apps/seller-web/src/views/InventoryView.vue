<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Boxes, Search, X, Loader2, Check } from 'lucide-vue-next'
import type { Product, Variant } from '@carlitos/types'
import { seller, MEDIA_BASE } from '@/api'
import { formatINR, resolveMediaUrl, extractApiError } from '@carlitos/shared'

const LOW_STOCK = 10   // below this = low (shown in red)

const products = ref<Product[]>([])
const loading  = ref(true)
const query    = ref('')
const stockFilter = ref<'all' | 'in' | 'out'>('all')

// ── Stock popup state ──────────────────────────────────────────
const editing    = ref<Product | null>(null)   // list row being edited
const detail     = ref<Product | null>(null)    // fully hydrated (with variants)
const detailLoading = ref(false)
const sizeInputs = ref<Record<string, number>>({})   // seller_listing_id -> qty
const savingId   = ref('')                       // listing id being saved
const popupErr   = ref('')
const popupOk    = ref('')

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res = await seller.products({ status: 'approved', page_size: 100 })
    products.value = (res.data as Product[]) || []
  } finally { loading.value = false }
}

function stockOf(p: Product): number { return p.available_quantity ?? 0 }
function priceOf(p: Product): number { return p.selling_price ?? p.variants?.[0]?.selling_price ?? 0 }
function firstImage(p: Product | null): string {
  const key = p?.images?.[0]?.file_key
  return key ? resolveMediaUrl(MEDIA_BASE, key) : ''
}
function sizeLabel(v: Variant): string {
  if (v.size) return v.size
  const code = (v.variant_code || '').trim()
  if (!code || code.toUpperCase() === 'DEFAULT') return 'One size'
  // Seed products use codes like "M-DEFAULT" — show just the size part.
  return code.replace(/-DEFAULT$/i, '') || 'One size'
}

const STOCK_TABS = [
  { value: 'all', label: 'All' },
  { value: 'in',  label: 'In stock' },
  { value: 'out', label: 'Out of stock' },
] as const

const filtered = computed(() => products.value.filter(p => {
  if (query.value && !p.title.toLowerCase().includes(query.value.toLowerCase())) return false
  const s = stockOf(p)
  if (stockFilter.value === 'in')  return s > 0
  if (stockFilter.value === 'out') return s <= 0
  return true
}))

const lowCount = computed(() => products.value.filter(p => { const s = stockOf(p); return s > 0 && s < LOW_STOCK }).length)
const outCount = computed(() => products.value.filter(p => stockOf(p) <= 0).length)

function stockClass(s: number): string { return s < LOW_STOCK ? 'text-red-600' : 'text-emerald-600' }

// ── Popup ──────────────────────────────────────────────────────
async function openStock(p: Product) {
  editing.value = p
  detail.value = null
  popupErr.value = ''
  popupOk.value = ''
  sizeInputs.value = {}
  detailLoading.value = true
  try {
    const full = await seller.product(p.id)
    detail.value = full
    // Key inputs by variant id (works even if seller_listing_id isn't present).
    const map: Record<string, number> = {}
    for (const v of (full.variants || [])) {
      map[v.id] = v.available_quantity ?? 0
    }
    sizeInputs.value = map
  } catch (e) {
    popupErr.value = extractApiError(e)
  } finally {
    detailLoading.value = false
  }
}
function closeStock() {
  editing.value = null
  detail.value = null
  savingId.value = ''
  popupErr.value = ''
  popupOk.value = ''
}

async function saveSize(v: Variant) {
  if (!editing.value) return
  const qty = Number(sizeInputs.value[v.id])
  if (!Number.isFinite(qty) || qty < 0) { popupErr.value = 'Enter a valid quantity (0 or more).'; return }
  savingId.value = v.id
  popupErr.value = ''
  popupOk.value = ''
  try {
    // Pass the size's listing id when known; empty falls back to the primary listing.
    await seller.setProductStock(editing.value.id, Math.floor(qty), v.seller_listing_id || undefined)
    // Reflect locally: update the variant + the list row total.
    v.available_quantity = Math.floor(qty)
    if (detail.value) {
      const total = (detail.value.variants || []).reduce((a, x) => a + (x.available_quantity ?? 0), 0)
      const row = products.value.find(p => p.id === editing.value!.id)
      if (row) row.available_quantity = total
    }
    popupOk.value = `${sizeLabel(v)} updated.`
    setTimeout(() => { popupOk.value = '' }, 1800)
  } catch (e) {
    popupErr.value = extractApiError(e)
    savingId.value = ''
    return
  }
  savingId.value = ''
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1 class="page-title">Inventory</h1>
        <p class="mt-0.5 text-sm text-stone-500">Live stock across your approved listings.</p>
      </div>
    </div>

    <!-- Low / out of stock — small note style -->
    <p v-if="!loading && (lowCount || outCount)" class="mb-3 text-xs text-stone-500">
      <span class="font-medium text-amber-600">Note:</span>
      <template v-if="outCount">{{ outCount }} out of stock</template>
      <template v-if="outCount && lowCount"> · </template>
      <template v-if="lowCount">{{ lowCount }} running low (under {{ LOW_STOCK }})</template>.
    </p>

    <!-- Search -->
    <div class="mb-3">
      <div class="relative">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
        <input v-model="query" class="input pl-9" placeholder="Search products…" aria-label="Search inventory" />
      </div>
    </div>

    <!-- 3 stock tabs -->
    <div class="mb-4 flex gap-2">
      <button
        v-for="t in STOCK_TABS" :key="t.value"
        :class="['flex-1 rounded-full border px-3 py-1.5 text-sm transition-colors sm:flex-none', stockFilter === t.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="stockFilter = t.value"
      >{{ t.label }}</button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!filtered.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Boxes :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">
        {{ products.length ? 'No products match this filter' : 'No approved products' }}
      </p>
      <p class="mt-1 text-xs text-stone-500">
        {{ products.length ? 'Try another tab or search.' : 'Once products are approved, their stock will show here.' }}
      </p>
      <RouterLink v-if="!products.length" to="/products" class="btn-ghost btn-sm mt-4">Go to Products</RouterLink>
    </div>

    <!-- Slidable table: full product name + stock + update -->
    <div v-else class="table-wrap">
      <table class="table min-w-[560px]">
        <thead>
          <tr>
            <th class="whitespace-nowrap">Product</th>
            <th class="whitespace-nowrap">Price</th>
            <th class="whitespace-nowrap text-center">Total Stock</th>
            <th class="whitespace-nowrap text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in filtered" :key="p.id">
            <td>
              <div class="flex items-center gap-2.5">
                <div class="h-10 w-10 shrink-0 overflow-hidden rounded-md border border-stone-100 bg-stone-50">
                  <img v-if="firstImage(p)" :src="firstImage(p)" :alt="p.title" class="h-full w-full object-cover" />
                  <div v-else class="flex h-full w-full items-center justify-center"><Boxes :size="14" class="text-stone-300" /></div>
                </div>
                <div class="whitespace-nowrap">
                  <p class="font-medium text-ink">{{ p.title }}</p>
                  <p class="text-xs text-stone-400">{{ p.brand_name || '—' }}</p>
                </div>
              </div>
            </td>
            <td class="whitespace-nowrap font-medium">{{ priceOf(p) > 0 ? formatINR(priceOf(p)) : '—' }}</td>
            <td class="whitespace-nowrap text-center">
              <span v-if="stockOf(p) <= 0" class="inline-flex items-center rounded-full bg-red-50 px-2 py-0.5 text-xs font-semibold text-red-600">Out of stock</span>
              <template v-else>
                <span :class="['text-base font-semibold', stockClass(stockOf(p))]">{{ stockOf(p) }}</span>
                <span class="text-xs text-stone-400"> pcs</span>
              </template>
            </td>
            <td class="whitespace-nowrap text-right">
              <button class="btn-ghost btn-sm" @click="openStock(p)">Update stock</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Stock update popup (per size) ──────────────────────── -->
    <transition name="fade">
      <div v-if="editing" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="closeStock">
        <div class="max-h-[85vh] w-full max-w-sm overflow-y-auto rounded-t-2xl bg-white p-5 shadow-lg sm:rounded-2xl">
          <div class="mb-4 flex items-start justify-between gap-3">
            <h2 class="text-base font-semibold text-ink">Update Stock</h2>
            <button class="rounded p-1 text-stone-400 hover:bg-stone-50" aria-label="Close" @click="closeStock">
              <X :size="18" />
            </button>
          </div>

          <!-- Product identity -->
          <div class="mb-4 flex items-center gap-3">
            <div class="h-16 w-16 shrink-0 overflow-hidden rounded-lg border border-stone-200 bg-stone-50">
              <img v-if="firstImage(detail || editing)" :src="firstImage(detail || editing)" :alt="editing.title" class="h-full w-full object-cover" />
              <div v-else class="flex h-full w-full items-center justify-center"><Boxes :size="20" class="text-stone-300" /></div>
            </div>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-ink">{{ editing.title }}</p>
              <p class="text-xs text-stone-400">Set stock per size below</p>
            </div>
          </div>

          <div v-if="detailLoading" class="space-y-2">
            <div v-for="i in 3" :key="i" class="skeleton h-10 w-full rounded" />
          </div>

          <div v-else-if="detail && (detail.variants || []).length" class="space-y-2">
            <p class="mb-1 text-xs text-stone-400">Choose a size and set its stock:</p>
            <div v-for="v in detail.variants" :key="v.id" class="flex items-center gap-2">
              <span class="w-16 shrink-0 rounded-md border border-stone-200 bg-stone-50 px-2 py-1 text-center text-sm font-medium text-ink">{{ sizeLabel(v) }}</span>
              <input
                v-model.number="sizeInputs[v.id]"
                type="number" min="0" inputmode="numeric"
                class="input flex-1"
                :placeholder="`Qty for ${sizeLabel(v)}`"
                @keyup.enter="saveSize(v)"
              />
              <button
                class="btn-primary btn-sm shrink-0"
                :disabled="savingId === v.id"
                @click="saveSize(v)"
              >
                <Loader2 v-if="savingId === v.id" :size="14" class="animate-spin" />
                <Check v-else :size="14" /> OK
              </button>
            </div>
          </div>

          <p v-else-if="detail" class="text-sm text-stone-500">No sizes found for this product.</p>

          <p v-if="popupErr" class="mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-600">{{ popupErr }}</p>
          <p v-if="popupOk" class="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">{{ popupOk }}</p>

          <button class="btn-ghost mt-4 w-full" @click="closeStock">Done</button>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 180ms ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
