<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { Ticket, Plus, Pencil, Trash2, History, Power } from 'lucide-vue-next'
import type { Coupon, CouponRedemption } from '@carlitos/types'
import { admin } from '@/api'
import { extractApiError } from '@carlitos/shared'

const coupons      = ref<Coupon[]>([])
const redemptions  = ref<CouponRedemption[]>([])
const loading      = ref(true)
const showForm     = ref(false)
const saving       = ref(false)
const error        = ref('')
const success      = ref('')
const editingId    = ref<string | null>(null)
const tab          = ref<'coupons' | 'history'>('coupons')

const blank = () => ({
  code: '',
  description: '',
  discount_type: 'percent' as 'percent' | 'fixed',
  discount_value: 10,
  max_discount: '' as number | '',
  min_order_amount: 0,
  usage_limit_total: '' as number | '',
  usage_limit_per_user: '' as number | '',
  starts_at: '',
  ends_at: '',
  is_active: true,
})
const form = ref(blank())

async function load() {
  loading.value = true
  try {
    coupons.value = await admin.coupons()
    redemptions.value = await admin.couponRedemptions().catch(() => [])
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.value = blank()
  error.value = ''
  showForm.value = true
}

function openEdit(c: Coupon) {
  editingId.value = c.id
  form.value = {
    code: c.code,
    description: c.description,
    discount_type: c.discount_type,
    discount_value: c.discount_value,
    max_discount: c.max_discount ?? '',
    min_order_amount: c.min_order_amount,
    usage_limit_total: c.usage_limit_total ?? '',
    usage_limit_per_user: c.usage_limit_per_user ?? '',
    starts_at: toLocalInput(c.starts_at),
    ends_at: toLocalInput(c.ends_at),
    is_active: c.is_active,
  }
  error.value = ''
  showForm.value = true
}

function toLocalInput(iso: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  // yyyy-MM-ddThh:mm for datetime-local
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function buildBody() {
  return {
    code: form.value.code.trim().toUpperCase(),
    description: form.value.description,
    discount_type: form.value.discount_type,
    discount_value: Number(form.value.discount_value),
    max_discount: form.value.max_discount === '' ? null : Number(form.value.max_discount),
    min_order_amount: Number(form.value.min_order_amount) || 0,
    usage_limit_total: form.value.usage_limit_total === '' ? null : Number(form.value.usage_limit_total),
    usage_limit_per_user: form.value.usage_limit_per_user === '' ? null : Number(form.value.usage_limit_per_user),
    starts_at: form.value.starts_at || null,
    ends_at: form.value.ends_at || null,
    is_active: form.value.is_active,
  }
}

async function save() {
  error.value = ''; success.value = ''
  if (form.value.code.trim().length < 3) { error.value = 'Code must be at least 3 characters.'; return }
  if (Number(form.value.discount_value) <= 0) { error.value = 'Discount value must be greater than zero.'; return }
  if (form.value.discount_type === 'percent' && Number(form.value.discount_value) > 100) {
    error.value = 'Percent discount cannot exceed 100.'; return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await admin.updateCoupon(editingId.value, buildBody())
      success.value = 'Coupon updated.'
    } else {
      await admin.createCoupon(buildBody())
      success.value = 'Coupon created.'
    }
    showForm.value = false
    await load()
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    saving.value = false
  }
}

async function toggleActive(c: Coupon) {
  try {
    await admin.setCouponActive(c.id, !c.is_active)
    await load()
  } catch (e) { error.value = extractApiError(e) }
}

async function remove(c: Coupon) {
  if (!confirm(`Delete coupon ${c.code}? This cannot be undone.`)) return
  try {
    await admin.deleteCoupon(c.id)
    await load()
  } catch (e) { error.value = extractApiError(e) }
}

function statusOf(c: Coupon): { label: string; cls: string } {
  if (!c.is_active) return { label: 'Inactive', cls: 'badge-gray' }
  const now = Date.now()
  if (c.ends_at && new Date(c.ends_at).getTime() < now) return { label: 'Expired', cls: 'badge-red' }
  if (c.starts_at && new Date(c.starts_at).getTime() > now) return { label: 'Scheduled', cls: 'badge-amber' }
  if (c.usage_limit_total != null && c.used_count >= c.usage_limit_total) return { label: 'Exhausted', cls: 'badge-red' }
  return { label: 'Active', cls: 'badge-green' }
}

function discountText(c: Coupon): string {
  if (c.discount_type === 'percent') {
    return `${c.discount_value}% off` + (c.max_discount ? ` (max ₹${c.max_discount})` : '')
  }
  return `₹${c.discount_value} off`
}

function fmtDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' })
}
function fmtDateTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString('en-IN', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

const activeCount = computed(() => coupons.value.filter(c => statusOf(c).label === 'Active').length)

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Offers &amp; Coupons</h1>
      <button class="btn-primary" @click="openCreate"><Plus :size="15" /> Create Coupon</button>
    </div>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>
    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Summary -->
    <div class="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
      <div class="card p-4"><p class="text-xs text-stone-500">Total Coupons</p><p class="mt-1 text-xl font-semibold">{{ coupons.length }}</p></div>
      <div class="card p-4"><p class="text-xs text-stone-500">Active</p><p class="mt-1 text-xl font-semibold text-emerald-600">{{ activeCount }}</p></div>
      <div class="card p-4"><p class="text-xs text-stone-500">Redemptions</p><p class="mt-1 text-xl font-semibold">{{ redemptions.length }}</p></div>
      <div class="card p-4"><p class="text-xs text-stone-500">Total Discount Given</p><p class="mt-1 text-xl font-semibold">₹{{ redemptions.reduce((s, r) => s + r.discount_amount, 0).toFixed(0) }}</p></div>
    </div>

    <!-- Create / edit form -->
    <div v-if="showForm" class="card p-5 mb-6 space-y-4">
      <h2 class="text-h4">{{ editingId ? 'Edit Coupon' : 'New Coupon' }}</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label">Coupon Code</label>
          <input v-model="form.code" class="input font-mono uppercase" placeholder="SUMMER50" :disabled="!!editingId" />
          <p v-if="editingId" class="field-hint">Code can't be changed after creation.</p>
        </div>
        <div>
          <label class="label">Description</label>
          <input v-model="form.description" class="input" placeholder="Summer sale — 50% off" />
        </div>
        <div>
          <label class="label">Discount Type</label>
          <select v-model="form.discount_type" class="select">
            <option value="percent">Percentage (%)</option>
            <option value="fixed">Fixed amount (₹)</option>
          </select>
        </div>
        <div>
          <label class="label">Discount Value</label>
          <input v-model.number="form.discount_value" type="number" min="1" class="input" :placeholder="form.discount_type === 'percent' ? '50' : '200'" />
        </div>
        <div v-if="form.discount_type === 'percent'">
          <label class="label">Max Discount (₹) <span class="text-stone-400">optional</span></label>
          <input v-model="form.max_discount" type="number" min="0" class="input" placeholder="e.g. 500" />
        </div>
        <div>
          <label class="label">Min Cart Value (₹)</label>
          <input v-model.number="form.min_order_amount" type="number" min="0" class="input" placeholder="0" />
        </div>
        <div>
          <label class="label">Total Usage Limit <span class="text-stone-400">optional</span></label>
          <input v-model="form.usage_limit_total" type="number" min="1" class="input" placeholder="Unlimited" />
          <p class="field-hint">How many times this coupon can be used in total.</p>
        </div>
        <div>
          <label class="label">Per-User Limit <span class="text-stone-400">optional</span></label>
          <input v-model="form.usage_limit_per_user" type="number" min="1" class="input" placeholder="Unlimited" />
          <p class="field-hint">How many times one customer can use it.</p>
        </div>
        <div>
          <label class="label">Starts At <span class="text-stone-400">optional</span></label>
          <input v-model="form.starts_at" type="datetime-local" class="input" />
        </div>
        <div>
          <label class="label">Expires At <span class="text-stone-400">optional</span></label>
          <input v-model="form.ends_at" type="datetime-local" class="input" />
        </div>
        <label class="flex items-center gap-2 text-sm sm:col-span-2">
          <input v-model="form.is_active" type="checkbox" class="accent-ink" /> Active (available to customers)
        </label>
      </div>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="showForm = false">Cancel</button>
        <button class="btn-primary" :disabled="saving" @click="save">{{ saving ? 'Saving…' : (editingId ? 'Update Coupon' : 'Create Coupon') }}</button>
      </div>
    </div>

    <!-- Tabs -->
    <div class="mb-4 flex gap-1 border-b border-stone-200">
      <button :class="['px-4 py-2 text-sm font-medium border-b-2 -mb-px', tab === 'coupons' ? 'border-ink text-ink' : 'border-transparent text-stone-500 hover:text-ink']" @click="tab = 'coupons'">
        <Ticket :size="14" class="inline" /> Coupons
      </button>
      <button :class="['px-4 py-2 text-sm font-medium border-b-2 -mb-px', tab === 'history' ? 'border-ink text-ink' : 'border-transparent text-stone-500 hover:text-ink']" @click="tab = 'history'">
        <History :size="14" class="inline" /> Redemption History
      </button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="skeleton h-16 w-full rounded-lg" />
    </div>

    <!-- Coupons list -->
    <template v-else-if="tab === 'coupons'">
      <div v-if="!coupons.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
        <Ticket :size="40" class="mb-3 text-stone-300" />
        <p class="text-sm font-medium text-stone-700">No coupons yet</p>
        <p class="mt-1 text-xs text-stone-500">Create your first discount coupon.</p>
      </div>

      <div v-else class="table-wrap card">
        <table class="table">
          <thead>
            <tr>
              <th>Code</th><th>Discount</th><th>Min Cart</th><th>Usage</th><th>Validity</th><th>Status</th><th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in coupons" :key="c.id">
              <td>
                <p class="font-mono font-semibold text-ink">{{ c.code }}</p>
                <p v-if="c.description" class="text-xs text-stone-500">{{ c.description }}</p>
              </td>
              <td>{{ discountText(c) }}</td>
              <td>{{ c.min_order_amount > 0 ? '₹' + c.min_order_amount : '—' }}</td>
              <td>
                <span class="text-sm">{{ c.used_count }}<span v-if="c.usage_limit_total != null"> / {{ c.usage_limit_total }}</span></span>
                <p v-if="c.usage_limit_per_user != null" class="text-xs text-stone-400">{{ c.usage_limit_per_user }}/user</p>
              </td>
              <td class="text-xs text-stone-500">
                <span v-if="c.starts_at || c.ends_at">{{ fmtDate(c.starts_at) }} → {{ fmtDate(c.ends_at) }}</span>
                <span v-else>No limit</span>
              </td>
              <td><span :class="['badge', statusOf(c).cls]">{{ statusOf(c).label }}</span></td>
              <td>
                <div class="flex items-center gap-1">
                  <button class="rounded p-1.5 text-stone-400 hover:bg-stone-100 hover:text-ink" :title="c.is_active ? 'Deactivate' : 'Activate'" @click="toggleActive(c)"><Power :size="15" /></button>
                  <button class="rounded p-1.5 text-stone-400 hover:bg-stone-100 hover:text-ink" title="Edit" @click="openEdit(c)"><Pencil :size="15" /></button>
                  <button class="rounded p-1.5 text-stone-400 hover:bg-red-50 hover:text-red-600" title="Delete" @click="remove(c)"><Trash2 :size="15" /></button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- Redemption history -->
    <template v-else>
      <div v-if="!redemptions.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
        <History :size="40" class="mb-3 text-stone-300" />
        <p class="text-sm font-medium text-stone-700">No redemptions yet</p>
        <p class="mt-1 text-xs text-stone-500">Coupon usage will appear here.</p>
      </div>
      <div v-else class="table-wrap card">
        <table class="table">
          <thead><tr><th>Coupon</th><th>Customer</th><th>Order</th><th>Discount</th><th>When</th></tr></thead>
          <tbody>
            <tr v-for="r in redemptions" :key="r.id">
              <td class="font-mono">{{ r.coupon_code }}</td>
              <td>{{ r.customer_name || '—' }}</td>
              <td class="text-xs">{{ r.order_number || '—' }}</td>
              <td>₹{{ r.discount_amount.toFixed(0) }}</td>
              <td class="text-xs text-stone-500">{{ fmtDateTime(r.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
