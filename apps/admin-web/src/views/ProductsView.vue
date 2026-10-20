<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Package, Search, Check, X } from 'lucide-vue-next'
import type { Product } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, formatDate, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const router   = useRouter()
const products = ref<Product[]>([])
const total    = ref(0)
const loading  = ref(true)
const status   = ref('submitted')
const query    = ref('')
const error    = ref('')
const page     = ref(1)
const PAGE_SIZE = 20
const busy     = ref('')

const rejectDialog = ref<{ open: boolean; id: string }>({ open: false, id: '' })

const tabs = [
  { value: 'submitted', label: 'Pending Approval' },
  { value: 'approved',  label: 'Approved' },
  { value: 'rejected',  label: 'Rejected' },
  { value: 'suspended', label: 'Suspended' },
  { value: '',          label: 'All' },
]

async function load() {
  loading.value = true
  try {
    const params: Record<string,unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (status.value) params.status = status.value
    if (query.value)  params.q      = query.value
    const res      = await admin.products(params)
    products.value = (res.data as Product[]) || []
    total.value    = res.meta?.total || 0
  } finally { loading.value = false }
}

async function approve(id: string) {
  busy.value = id; error.value = ''
  try { await admin.approveProduct(id); await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}

async function confirmReject(reason: string) {
  busy.value = rejectDialog.value.id; error.value = ''
  try { await admin.rejectProduct(rejectDialog.value.id, reason); await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = ''; rejectDialog.value.open = false }
}

function statusClass(s: string) {
  if (s === 'approved') return 'badge-green'
  if (['rejected','suspended'].includes(s)) return 'badge-red'
  if (s === 'submitted') return 'badge-amber'
  return 'badge-gray'
}

let t: ReturnType<typeof setTimeout>
function onSearch() { clearTimeout(t); t = setTimeout(() => { page.value = 1; load() }, 400) }

onMounted(load)
watch(status, () => { page.value = 1; load() })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Products</h1>
    </div>

    <!-- Tabs -->
    <div class="mb-4 flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="tb in tabs"
        :key="tb.value || 'all'"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', status === tb.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="status = tb.value"
      >{{ tb.label }}</button>
    </div>

    <!-- Search -->
    <div class="mb-5 relative max-w-sm">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
      <input v-model="query" class="input pl-9" placeholder="Search products…" @input="onSearch" aria-label="Search products" />
    </div>

    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 8" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!products.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Package :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No products here</p>
      <p class="mt-1 text-xs text-stone-500">No products match this filter.</p>
    </div>

    <!-- Table -->
    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Product</th>
            <th class="hidden md:table-cell">Brand</th>
            <th class="hidden sm:table-cell">Price</th>
            <th class="hidden lg:table-cell">Created</th>
            <th>Status</th>
            <th class="text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in products" :key="p.id" @click="router.push(`/products/${p.id}`)">
            <td class="font-medium truncate max-w-xs">{{ p.title }}</td>
            <td class="hidden md:table-cell text-stone-500">{{ p.brand_name || '—' }}</td>
            <td class="hidden sm:table-cell">{{ p.variants?.[0] ? formatINR(p.variants[0].selling_price) : '—' }}</td>
            <td class="hidden lg:table-cell text-stone-500 text-xs">{{ formatDate(p.created_at) }}</td>
            <td><span :class="['badge', statusClass(p.status)]">{{ p.status.replace('_',' ') }}</span></td>
            <td @click.stop>
              <div v-if="p.status === 'submitted'" class="flex items-center justify-end gap-2">
                <button class="btn-success btn-xs" :disabled="busy === p.id" @click="approve(p.id)">
                  <Check :size="12" /> Approve
                </button>
                <button class="btn-danger btn-xs" :disabled="busy === p.id" @click="rejectDialog = { open: true, id: p.id }">
                  <X :size="12" /> Reject
                </button>
              </div>
              <span v-else class="text-xs text-stone-400">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="Math.ceil(total/PAGE_SIZE) > 1" class="mt-5 flex justify-center gap-2">
      <button class="btn-ghost btn-sm" :disabled="page<=1" @click="() => { page--; load() }">← Prev</button>
      <span class="flex h-8 items-center px-3 text-sm text-stone-500">{{ page }} / {{ Math.ceil(total/PAGE_SIZE) }}</span>
      <button class="btn-ghost btn-sm" :disabled="page>=Math.ceil(total/PAGE_SIZE)" @click="() => { page++; load() }">Next →</button>
    </div>

    <ConfirmDialog
      :open="rejectDialog.open"
      title="Reject product?"
      variant="danger"
      :require-reason="true"
      confirm-label="Reject"
      message="The seller will be notified with your reason."
      @confirm="confirmReject"
      @cancel="rejectDialog.open = false"
    />
  </div>
</template>
