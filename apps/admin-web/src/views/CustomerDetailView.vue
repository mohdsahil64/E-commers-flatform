<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ChevronLeft, Ban, RefreshCcw, Phone, Mail, MapPin, ShoppingBag, ShieldAlert,
  BadgeCheck, CircleSlash,
} from 'lucide-vue-next'
import type { AdminCustomerDetail, CustomerBlockReason } from '@carlitos/types'
import { admin } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { formatINR, formatDate, orderStatusLabel, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import BlockCustomerDialog from '@/components/BlockCustomerDialog.vue'

const route = useRoute()
const router = useRouter()
const store = useAuthStore()
const canBlock = computed(() => store.hasPermission('customer.block'))

const c = ref<AdminCustomerDetail | null>(null)
const loading = ref(true)
const busy = ref(false)
const message = ref('')
const error = ref('')
const dialogError = ref('')

const reasons = ref<CustomerBlockReason[]>([])
const showBlock = ref(false)
const showUnblock = ref(false)

async function load() {
  c.value = await admin.customer(route.params.id as string).catch(() => null)
}

async function openBlock() {
  dialogError.value = ''
  if (!reasons.value.length) reasons.value = await admin.customerBlockReasons().catch(() => [])
  showBlock.value = true
}

async function confirmBlock(p: { reason_code: string; note: string }) {
  if (!c.value) return
  busy.value = true; dialogError.value = ''; message.value = ''
  try {
    await admin.blockCustomer(c.value.id, p)
    showBlock.value = false
    message.value = 'Customer blocked. They are signed out and will see the reason when they try to log in.'
    await load()
  } catch (e) { dialogError.value = extractApiError(e) }
  finally { busy.value = false }
}

async function confirmUnblock() {
  if (!c.value) return
  busy.value = true; error.value = ''; message.value = ''
  try {
    await admin.unblockCustomer(c.value.id)
    message.value = 'Customer unblocked. They can log in again.'
    await load()
  } catch (e) { error.value = extractApiError(e) }
  finally { busy.value = false; showUnblock.value = false }
}

const name = computed(() => c.value?.name || c.value?.phone || c.value?.email || 'Customer')
function statusClass(s?: string) {
  if (s === 'active') return 'badge-green'
  if (s === 'blocked') return 'badge-red'
  return 'badge-amber'
}
function statusLabel(s?: string) { return s ? s.charAt(0).toUpperCase() + s.slice(1) : '' }
function orderClass(s: string) {
  if (s === 'delivered') return 'badge-green'
  if (s === 'cancelled' || s.startsWith('refund')) return 'badge-red'
  return 'badge-amber'
}
function addrLine(a: AdminCustomerDetail['addresses'][number]) {
  return [a.line1, a.line2, a.landmark, a.city, a.state].filter(Boolean).join(', ') + ' – ' + a.pincode
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="skeleton h-8 w-1/3 rounded" />
    <div class="skeleton h-32 w-full rounded-xl" />
  </div>

  <div v-else-if="!c" class="py-12 text-center">
    <p class="text-stone-600">Customer not found.</p>
    <button class="btn-ghost btn-sm mt-4" @click="router.push('/customers')">← All customers</button>
  </div>

  <div v-else class="max-w-5xl">
    <button class="mb-5 flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink" @click="router.push('/customers')">
      <ChevronLeft :size="16" /> All customers
    </button>

    <!-- Header -->
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div class="flex min-w-0 items-center gap-4">
        <div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full text-xl font-semibold" :class="c.status === 'blocked' ? 'bg-red-50 text-red-600' : 'bg-stone-900 text-white'">
          {{ name.trim().charAt(0).toUpperCase() }}
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h1 class="page-title truncate">{{ name }}</h1>
            <span :class="['badge', statusClass(c.status)]">{{ statusLabel(c.status) }}</span>
          </div>
          <p class="mt-0.5 text-sm text-stone-500">Customer since {{ formatDate(c.created_at) }}</p>
        </div>
      </div>

      <div v-if="canBlock" class="flex flex-wrap gap-2">
        <button v-if="c.status === 'blocked'" class="btn-primary btn-sm" :disabled="busy" @click="showUnblock = true">
          <RefreshCcw :size="14" /> Unblock
        </button>
        <button v-else-if="c.status === 'active'" class="btn-danger btn-sm" :disabled="busy" @click="openBlock">
          <Ban :size="14" /> Block customer
        </button>
      </div>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Blocked banner -->
    <div v-if="c.status === 'blocked'" class="mb-5 flex items-start gap-3 rounded-xl border border-red-200 bg-red-50 p-4">
      <ShieldAlert :size="20" class="mt-0.5 shrink-0 text-red-500" />
      <div class="min-w-0">
        <p class="text-sm font-semibold text-red-800">This account is blocked</p>
        <p class="mt-0.5 break-words text-sm text-red-700">Reason shown to the customer: {{ c.block_reason || 'No reason recorded' }}</p>
        <p class="mt-1 text-xs text-red-600">
          <template v-if="c.blocked_at">Blocked on {{ formatDate(c.blocked_at) }}</template>
          <template v-if="c.blocked_by_name"> by {{ c.blocked_by_name }}</template>
        </p>
      </div>
    </div>

    <!-- Summary numbers -->
    <div class="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <div class="card p-4"><p class="text-xs uppercase tracking-wide text-stone-400">Orders</p><p class="mt-1 text-xl font-semibold text-ink">{{ c.orders_count }}</p></div>
      <div class="card p-4"><p class="text-xs uppercase tracking-wide text-stone-400">Total spent</p><p class="mt-1 text-xl font-semibold text-ink">{{ formatINR(c.total_spent) }}</p></div>
      <div class="card p-4"><p class="text-xs uppercase tracking-wide text-stone-400">Cancelled</p><p class="mt-1 text-xl font-semibold text-ink">{{ c.cancelled_count }}</p></div>
      <div class="card p-4"><p class="text-xs uppercase tracking-wide text-stone-400">Support tickets</p><p class="mt-1 text-xl font-semibold text-ink">{{ c.tickets_count }}</p></div>
    </div>

    <div class="grid gap-5 lg:grid-cols-2">
      <!-- Contact -->
      <div class="card p-5 sm:p-6">
        <h2 class="text-h4 mb-4">Contact & account</h2>
        <dl class="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
          <div class="min-w-0">
            <dt class="text-xs uppercase tracking-wide text-stone-400">Phone</dt>
            <dd class="mt-0.5 flex items-center gap-1.5">
              <Phone :size="13" class="shrink-0 text-stone-400" /> {{ c.phone || '—' }}
              <BadgeCheck v-if="c.phone && c.phone_verified" :size="14" class="shrink-0 text-emerald-500" title="Verified" />
            </dd>
          </div>
          <div class="min-w-0">
            <dt class="text-xs uppercase tracking-wide text-stone-400">Email</dt>
            <dd class="mt-0.5 flex items-center gap-1.5 break-all">
              <Mail :size="13" class="shrink-0 text-stone-400" /> {{ c.email || '—' }}
              <BadgeCheck v-if="c.email && c.email_verified" :size="14" class="shrink-0 text-emerald-500" title="Verified" />
            </dd>
          </div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Gender</dt><dd class="mt-0.5 capitalize">{{ c.gender || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Marketing</dt><dd class="mt-0.5">{{ c.marketing_opt_in ? 'Opted in' : 'Not opted in' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Last login</dt><dd class="mt-0.5">{{ c.last_login_at ? formatDate(c.last_login_at) : 'Never' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Joined</dt><dd class="mt-0.5">{{ formatDate(c.created_at) }}</dd></div>
        </dl>
      </div>

      <!-- Addresses -->
      <div class="card p-5 sm:p-6">
        <h2 class="text-h4 mb-4">Addresses</h2>
        <p v-if="!c.addresses.length" class="flex items-center gap-2 text-sm text-stone-500"><CircleSlash :size="14" /> No saved addresses.</p>
        <ul v-else class="space-y-3">
          <li v-for="a in c.addresses" :key="a.id" class="rounded-lg border border-stone-100 bg-stone-50 p-3 text-sm">
            <div class="flex items-center justify-between gap-2">
              <p class="flex items-center gap-1.5 font-medium text-ink"><MapPin :size="13" class="text-stone-400" /> {{ a.label || a.address_type }}</p>
              <span v-if="a.is_default" class="badge-gray">Default</span>
            </div>
            <p class="mt-1 text-xs text-stone-600">{{ a.contact_name }} · {{ a.contact_phone }}</p>
            <p class="mt-0.5 break-words text-xs text-stone-500">{{ addrLine(a) }}</p>
          </li>
        </ul>
      </div>
    </div>

    <!-- Recent orders -->
    <div class="card mt-5 overflow-hidden">
      <div class="flex items-center gap-2 border-b border-stone-100 px-5 py-4 sm:px-6">
        <ShoppingBag :size="16" class="text-stone-400" />
        <h2 class="text-h4">Recent orders</h2>
      </div>
      <p v-if="!c.recent_orders.length" class="px-5 py-8 text-center text-sm text-stone-500 sm:px-6">No orders yet.</p>
      <div v-else class="table-wrap">
        <table class="table">
          <thead>
            <tr><th>Order</th><th class="hidden sm:table-cell">Placed</th><th class="hidden sm:table-cell">Payment</th><th>Status</th><th class="text-right">Total</th></tr>
          </thead>
          <tbody>
            <tr v-for="o in c.recent_orders" :key="o.id" class="cursor-pointer" @click="router.push(`/orders/${o.id}`)">
              <td class="font-medium">{{ o.order_number }}</td>
              <td class="hidden sm:table-cell text-xs text-stone-500">{{ formatDate(o.created_at) }}</td>
              <td class="hidden sm:table-cell text-xs uppercase text-stone-500">{{ o.payment_method }}</td>
              <td><span :class="['badge', orderClass(o.status)]">{{ orderStatusLabel(o.status) }}</span></td>
              <td class="text-right">{{ formatINR(o.grand_total) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <BlockCustomerDialog
      :open="showBlock"
      :customer-name="name"
      :reasons="reasons"
      :busy="busy"
      :error="dialogError"
      @confirm="confirmBlock"
      @cancel="showBlock = false"
    />
    <ConfirmDialog
      :open="showUnblock"
      title="Unblock customer?"
      :message="`${name} will be able to log in again.`"
      confirm-label="Unblock"
      variant="primary"
      @confirm="confirmUnblock"
      @cancel="showUnblock = false"
    />
  </div>
</template>
