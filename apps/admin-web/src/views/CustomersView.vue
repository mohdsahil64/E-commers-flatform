<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Users, Search, Ban, RefreshCcw, Phone, Mail } from 'lucide-vue-next'
import type { AdminCustomer, CustomerBlockReason } from '@carlitos/types'
import { admin } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatINR, formatDate, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import BlockCustomerDialog from '@/components/BlockCustomerDialog.vue'

const router = useRouter()
const store = useAuthStore()
const canBlock = computed(() => store.hasPermission('customer.block'))

const customers = ref<AdminCustomer[]>([])
const total = ref(0)
const loading = ref(true)
const status = ref('')
const query = ref('')
const page = ref(1)
const PAGE_SIZE = 20
const error = ref('')
const message = ref('')

const tabs = [
  { value: '', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'blocked', label: 'Blocked' },
]

async function load() {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (status.value) params.status = status.value
    if (query.value.trim()) params.q = query.value.trim()
    const res = await admin.customers(params)
    customers.value = (res.data as AdminCustomer[]) || []
    total.value = res.meta?.total || 0
  } catch (e) {
    error.value = extractApiError(e)
    customers.value = []
  } finally {
    loading.value = false
  }
}

let t: ReturnType<typeof setTimeout>
function onSearch() { clearTimeout(t); t = setTimeout(() => { page.value = 1; load() }, 400) }

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

/* ── Block / unblock from the list ────────────────────────────── */
const reasons = ref<CustomerBlockReason[]>([])
const blockTarget = ref<AdminCustomer | null>(null)
const unblockTarget = ref<AdminCustomer | null>(null)
const busy = ref(false)
const dialogError = ref('')

async function openBlock(c: AdminCustomer) {
  dialogError.value = ''
  if (!reasons.value.length) reasons.value = await admin.customerBlockReasons().catch(() => [])
  blockTarget.value = c
}

async function confirmBlock(p: { reason_code: string; note: string }) {
  if (!blockTarget.value) return
  busy.value = true; dialogError.value = ''
  try {
    await admin.blockCustomer(blockTarget.value.id, p)
    message.value = `${displayName(blockTarget.value)} has been blocked.`
    blockTarget.value = null
    await load()
  } catch (e) { dialogError.value = extractApiError(e) }
  finally { busy.value = false }
}

async function confirmUnblock() {
  if (!unblockTarget.value) return
  busy.value = true; error.value = ''
  try {
    await admin.unblockCustomer(unblockTarget.value.id)
    message.value = `${displayName(unblockTarget.value)} has been unblocked.`
    await load()
  } catch (e) { error.value = extractApiError(e) }
  finally { busy.value = false; unblockTarget.value = null }
}

/* ── Display helpers ──────────────────────────────────────────── */
function displayName(c: AdminCustomer) { return c.name || c.phone || c.email || 'Customer' }
function initial(c: AdminCustomer) { return displayName(c).trim().charAt(0).toUpperCase() || '?' }
function statusClass(s: string) {
  if (s === 'active') return 'badge-green'
  if (s === 'blocked') return 'badge-red'
  return 'badge-amber'
}
function statusLabel(s: string) { return s.charAt(0).toUpperCase() + s.slice(1) }

onMounted(load)
watch(status, () => { page.value = 1; load() })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Customers</h1>
      <p class="text-sm text-stone-500">All registered customers. Open a customer to see their details, or block / unblock their account.</p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Status tabs -->
    <div class="mb-4 flex gap-2 overflow-x-auto pb-1">
      <button
        v-for="tb in tabs" :key="tb.value || 'all'"
        :class="['shrink-0 rounded-full border px-3.5 py-1.5 text-sm transition-colors', status === tb.value ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="status = tb.value"
      >{{ tb.label }}</button>
    </div>

    <!-- Search -->
    <div class="relative mb-5 max-w-sm">
      <Search class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
      <input v-model="query" class="input pl-9" placeholder="Search name, phone or email…" aria-label="Search customers" @input="onSearch" />
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 8" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <div v-else-if="!customers.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-12 text-center">
      <Users :size="40" class="mb-3 text-stone-300" />
      <p class="text-sm font-medium text-stone-700">No customers found</p>
      <p class="mt-1 text-xs text-stone-500">Try a different filter or search term.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Customer</th>
            <th class="hidden md:table-cell">Contact</th>
            <th class="hidden sm:table-cell text-right">Orders</th>
            <th class="hidden lg:table-cell text-right">Spent</th>
            <th class="hidden xl:table-cell">Last login</th>
            <th class="hidden lg:table-cell">Joined</th>
            <th>Status</th>
            <th v-if="canBlock" class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in customers" :key="c.id" class="cursor-pointer" @click="router.push(`/customers/${c.id}`)">
            <td>
              <div class="flex items-center gap-2.5">
                <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold" :class="c.status === 'blocked' ? 'bg-red-50 text-red-600' : 'bg-stone-100 text-stone-600'">
                  {{ initial(c) }}
                </div>
                <div class="min-w-0">
                  <p class="truncate font-medium text-ink">{{ displayName(c) }}</p>
                  <p class="truncate text-xs text-stone-400 md:hidden">{{ c.phone || c.email || '—' }}</p>
                  <p v-if="c.status === 'blocked' && c.block_reason" class="line-clamp-1 max-w-[16rem] text-xs text-red-500" :title="c.block_reason">{{ c.block_reason }}</p>
                </div>
              </div>
            </td>
            <td class="hidden md:table-cell">
              <p v-if="c.phone" class="flex items-center gap-1 text-xs text-stone-600"><Phone :size="11" class="text-stone-400" /> {{ c.phone }}</p>
              <p v-if="c.email" class="flex items-center gap-1 truncate text-xs text-stone-500"><Mail :size="11" class="text-stone-400" /> {{ c.email }}</p>
              <span v-if="!c.phone && !c.email" class="text-stone-400">—</span>
            </td>
            <td class="hidden sm:table-cell text-right text-stone-600">{{ c.orders_count }}</td>
            <td class="hidden lg:table-cell text-right text-stone-600">{{ formatINR(c.total_spent) }}</td>
            <td class="hidden xl:table-cell text-xs text-stone-500">{{ c.last_login_at ? formatDate(c.last_login_at) : '—' }}</td>
            <td class="hidden lg:table-cell text-xs text-stone-500">{{ formatDate(c.created_at) }}</td>
            <td><span :class="['badge', statusClass(c.status)]">{{ statusLabel(c.status) }}</span></td>
            <td v-if="canBlock" class="text-right" @click.stop>
              <button v-if="c.status === 'blocked'" class="btn-primary btn-xs" @click="unblockTarget = c"><RefreshCcw :size="12" /> Unblock</button>
              <button v-else-if="c.status === 'active'" class="btn-danger btn-xs" @click="openBlock(c)"><Ban :size="12" /> Block</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="totalPages > 1" class="mt-5 flex items-center justify-center gap-2">
      <button class="btn-ghost btn-sm" :disabled="page <= 1" @click="() => { page--; load() }">← Prev</button>
      <span class="flex h-8 items-center px-3 text-sm text-stone-500">{{ page }} / {{ totalPages }}</span>
      <button class="btn-ghost btn-sm" :disabled="page >= totalPages" @click="() => { page++; load() }">Next →</button>
    </div>

    <BlockCustomerDialog
      :open="!!blockTarget"
      :customer-name="blockTarget ? displayName(blockTarget) : ''"
      :reasons="reasons"
      :busy="busy"
      :error="dialogError"
      @confirm="confirmBlock"
      @cancel="blockTarget = null"
    />
    <ConfirmDialog
      :open="!!unblockTarget"
      title="Unblock customer?"
      :message="`${unblockTarget ? displayName(unblockTarget) : 'This customer'} will be able to log in again.`"
      confirm-label="Unblock"
      variant="primary"
      @confirm="confirmUnblock"
      @cancel="unblockTarget = null"
    />
  </div>
</template>
