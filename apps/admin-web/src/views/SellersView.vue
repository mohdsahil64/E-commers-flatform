<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Store, Search } from 'lucide-vue-next'
import type { Seller } from '@carlitos/types'
import { admin } from '@/api'
import { formatDate, SELLER_STATUS_LABELS } from '@carlitos/shared'

const router  = useRouter()
const sellers = ref<Seller[]>([])
const total   = ref(0)
const loading = ref(true)
const status  = ref('')
const query   = ref('')
const page    = ref(1)
const PAGE_SIZE = 20

const statusTabs = [
  { value: '',             label: 'All' },
  { value: 'under_review', label: 'Under Review' },
  { value: 'approved',     label: 'Approved' },
  { value: 'pending',      label: 'Pending' },
  { value: 'rejected',     label: 'Rejected' },
  { value: 'suspended',    label: 'Suspended' },
  { value: 'blocked',      label: 'Blocked' },
]

async function load() {
  loading.value = true
  try {
    const params: Record<string,unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (status.value) params.status = status.value
    if (query.value)  params.q      = query.value
    const res     = await admin.sellers(params)
    sellers.value = (res.data as Seller[]) || []
    total.value   = res.meta?.total || 0
  } finally {
    loading.value = false
  }
}

function statusClass(s: string) {
  if (s === 'approved') return 'badge-green'
  if (['rejected','suspended','blocked'].includes(s)) return 'badge-red'
  return 'badge-amber'
}

let t: ReturnType<typeof setTimeout>
function onSearch() { clearTimeout(t); t = setTimeout(() => { page.value = 1; load() }, 400) }

onMounted(load)
watch(status, () => { page.value = 1; load() })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Sellers</h1>
    </div>

    <!-- Status tabs -->
    <div class="mb-4 flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="tb in statusTabs"
        :key="tb.value || 'all'"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', status === tb.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="status = tb.value"
      >{{ tb.label }}</button>
    </div>

    <!-- Search -->
    <div class="mb-5 relative max-w-sm">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
      <input v-model="query" class="input pl-9" placeholder="Search sellers…" @input="onSearch" aria-label="Search sellers" />
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 8" :key="i" class="skeleton h-12 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!sellers.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Store :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No sellers found</p>
      <p class="mt-1 text-xs text-stone-500">Try a different filter or search term.</p>
    </div>

    <!-- Table -->
    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Store</th>
            <th class="hidden md:table-cell">Owner</th>
            <th class="hidden lg:table-cell">Rating</th>
            <th class="hidden sm:table-cell">Joined</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in sellers" :key="s.id" @click="router.push(`/sellers/${s.id}`)">
            <td>
              <p class="font-medium text-ink">{{ s.store_name }}</p>
              <p class="text-xs text-stone-400">{{ s.store_slug }}</p>
            </td>
            <td class="hidden md:table-cell text-stone-500">{{ s.owner_name }}</td>
            <td class="hidden lg:table-cell text-stone-500">{{ s.rating_avg.toFixed(1) }} ★</td>
            <td class="hidden sm:table-cell text-stone-500 text-xs">{{ formatDate(s.created_at) }}</td>
            <td><span :class="['badge', statusClass(s.status)]">{{ SELLER_STATUS_LABELS[s.status] || s.status }}</span></td>
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
  </div>
</template>
