<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RotateCcw, Package, X, CheckCircle2, XCircle, Clock, IndianRupee, Eye } from 'lucide-vue-next'
import type { ReturnRecord } from '@carlitos/types'
import { admin, MEDIA_BASE } from '@/api'
import { formatINR, formatDate, returnReasonLabel, returnStatusLabel, resolveMediaUrl, extractApiError } from '@carlitos/shared'

const returns = ref<ReturnRecord[]>([])
const loading = ref(true)
const error   = ref('')
const message = ref('')
const filter  = ref<'all' | 'refund_processing' | 'refunded' | 'requested'>('all')

// Detail drawer + refund modal.
const detail = ref<ReturnRecord | null>(null)
const refundTarget = ref<ReturnRecord | null>(null)
const utr = ref('')
const paying = ref(false)

async function load() {
  loading.value = true
  try { returns.value = await admin.returns() }
  catch { returns.value = [] }
  finally { loading.value = false }
}
onMounted(load)

const filtered = computed(() =>
  filter.value === 'all' ? returns.value : returns.value.filter(r => r.status === filter.value),
)
const pendingPayouts = computed(() => returns.value.filter(r => r.status === 'refund_processing').length)

function openRefund(r: ReturnRecord) { refundTarget.value = r; utr.value = ''; error.value = ''; message.value = '' }
async function submitRefund() {
  if (!refundTarget.value) return
  if (!utr.value.trim()) { error.value = 'Enter the bank UTR / reference number.'; return }
  paying.value = true; error.value = ''
  try {
    await admin.payRefund(refundTarget.value.id, utr.value.trim())
    message.value = `Refund for ${refundTarget.value.return_number} marked paid.`
    refundTarget.value = null
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { paying.value = false }
}

function statusChip(s: string) {
  if (['refunded', 'completed', 'received'].includes(s)) return 'bg-emerald-50 text-emerald-600'
  if (s === 'rejected') return 'bg-red-50 text-red-500'
  if (['approved', 'refund_processing'].includes(s)) return 'bg-blue-50 text-blue-600'
  return 'bg-amber-50 text-amber-600'
}
function eventDot(s: string) {
  if (['refunded', 'completed', 'received', 'approved'].includes(s)) return 'bg-emerald-500'
  if (s === 'rejected') return 'bg-red-500'
  if (s === 'refund_processing') return 'bg-blue-500'
  return 'bg-amber-500'
}
function eventIcon(s: string) {
  if (['refunded', 'completed'].includes(s)) return IndianRupee
  if (['received', 'approved'].includes(s)) return CheckCircle2
  if (s === 'rejected') return XCircle
  return Clock
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Returns &amp; Refunds</h1>
      <p class="text-sm text-stone-500">Read-only view of all returns. Pay refunds for items the seller has received.</p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Filter tabs -->
    <div class="mb-4 flex flex-wrap gap-2">
      <button v-for="f in (['all','requested','refund_processing','refunded'] as const)" :key="f"
        :class="['rounded-full border px-3 py-1.5 text-xs font-medium capitalize', filter === f ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="filter = f">
        {{ f === 'all' ? 'All' : f === 'refund_processing' ? `Refund pending${pendingPayouts ? ' ('+pendingPayouts+')' : ''}` : returnStatusLabel(f) }}
      </button>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-16 w-full rounded-lg" />
    </div>

    <div v-else-if="!filtered.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <RotateCcw :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">Nothing here</p>
      <p class="mt-1 text-xs text-stone-500">Return and exchange requests will appear here.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Return #</th>
            <th class="hidden sm:table-cell">Order</th>
            <th class="hidden md:table-cell">Customer</th>
            <th class="hidden lg:table-cell">Type</th>
            <th>Status</th>
            <th class="hidden sm:table-cell text-right">Refund</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in filtered" :key="r.id">
            <td class="font-medium">{{ r.return_number }}</td>
            <td class="hidden sm:table-cell text-stone-500 text-xs">{{ r.order_number }}</td>
            <td class="hidden md:table-cell text-stone-500">{{ r.customer_name }}</td>
            <td class="hidden lg:table-cell capitalize text-stone-500">{{ r.type }}</td>
            <td><span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(r.status)]">{{ returnStatusLabel(r.status) }}</span></td>
            <td class="hidden sm:table-cell text-right">
              <span v-if="r.refund_utr" class="text-xs text-emerald-600">Paid</span>
              <span v-else>{{ formatINR(r.refund_amount) }}</span>
            </td>
            <td>
              <div class="flex items-center justify-end gap-2">
                <button class="btn-ghost btn-xs" @click="detail = r"><Eye :size="12" /> View</button>
                <!-- Refund unlocked only once the seller has approved + received -->
                <button
                  v-if="r.status === 'refund_processing'"
                  class="btn-success btn-xs"
                  @click="openRefund(r)"
                ><IndianRupee :size="12" /> Pay refund</button>
                <span v-else-if="r.refund_utr" class="text-xs text-stone-400">UTR {{ r.refund_utr }}</span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Detail drawer ─────────────────────────────────── -->
    <div v-if="detail" class="fixed inset-0 z-50 flex justify-end bg-black/40" @click.self="detail = null">
      <div class="flex h-full w-full max-w-lg flex-col bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-stone-200 px-5 py-4">
          <div>
            <h3 class="text-h4">{{ detail.return_number }}</h3>
            <p class="text-xs text-stone-500">{{ detail.order_number }} · {{ detail.customer_name }} · {{ detail.seller_name }}</p>
          </div>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="detail = null"><X :size="18" /></button>
        </div>

        <div class="flex-1 space-y-5 overflow-y-auto p-5">
          <div class="flex items-center gap-2">
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(detail.status)]">{{ returnStatusLabel(detail.status) }}</span>
            <span class="rounded-full bg-stone-100 px-2.5 py-0.5 text-xs capitalize text-stone-600">{{ detail.type }}</span>
          </div>

          <!-- Items -->
          <div class="space-y-3">
            <div v-for="it in detail.items" :key="it.order_item_id" class="flex items-center gap-3">
              <div class="h-16 w-14 shrink-0 overflow-hidden rounded-md bg-stone-100">
                <img v-if="it.image" :src="resolveMediaUrl(MEDIA_BASE, it.image)" :alt="it.product_title" class="h-full w-full object-cover" />
                <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="18" /></div>
              </div>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-ink">{{ it.product_title }}</p>
                <p class="mt-0.5 text-xs text-stone-500"><span v-if="it.variant_label">{{ it.variant_label }} · </span>Qty {{ it.quantity }} · {{ formatINR(it.refund_amount) }}</p>
              </div>
            </div>
          </div>

          <!-- Reason -->
          <div class="rounded-lg border border-stone-100 bg-stone-50 p-3 text-sm">
            <p class="text-stone-500">Reason</p>
            <p class="font-medium text-ink">{{ returnReasonLabel(detail.reason_code) || detail.reason }}</p>
            <p v-if="detail.reason && detail.reason !== returnReasonLabel(detail.reason_code)" class="mt-1 text-xs text-stone-500">{{ detail.reason }}</p>
            <p v-if="detail.reject_reason" class="mt-2 text-xs text-red-600">Rejected by seller: {{ detail.reject_reason }}</p>
          </div>

          <!-- Photos -->
          <div v-if="detail.photos.length">
            <p class="mb-1 text-xs font-medium text-stone-500">Customer photos</p>
            <div class="flex flex-wrap gap-2">
              <a v-for="(p, i) in detail.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
                <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Proof" class="h-20 w-20 rounded-md border border-stone-200 object-cover" />
              </a>
            </div>
          </div>

          <!-- Refund destination -->
          <div v-if="detail.type === 'return'" class="rounded-lg border border-stone-100 bg-stone-50 p-3 text-sm">
            <p class="text-stone-500">Refund destination</p>
            <p v-if="detail.refund_method === 'upi'" class="font-medium text-ink">UPI · {{ detail.refund_upi }}</p>
            <template v-else-if="detail.refund_method === 'bank'">
              <p class="font-medium text-ink">{{ detail.refund_acc_name }}</p>
              <p class="text-xs text-stone-600">A/C {{ detail.refund_acc_no }} · {{ detail.refund_ifsc }}</p>
            </template>
            <p class="mt-2 flex items-center justify-between">
              <span class="text-stone-500">Amount</span>
              <span class="font-semibold text-ink">{{ formatINR(detail.refund_amount) }}</span>
            </p>
            <p v-if="detail.refund_utr" class="mt-1 text-xs text-emerald-600">Paid · UTR {{ detail.refund_utr }}</p>
          </div>

          <!-- Timeline -->
          <div v-if="detail.events.length">
            <p class="mb-2 text-xs font-semibold text-stone-500">Status timeline</p>
            <ol class="relative ml-1 border-l-2 border-stone-100 pl-5">
              <li v-for="(e, i) in detail.events" :key="i" class="relative pb-4 last:pb-0">
                <span :class="['absolute -left-[26px] top-0 flex h-4 w-4 items-center justify-center rounded-full ring-4 ring-white', eventDot(e.status)]">
                  <component :is="eventIcon(e.status)" :size="9" class="text-white" />
                </span>
                <p class="text-xs font-medium text-ink">{{ returnStatusLabel(e.status) }} <span class="font-normal text-stone-400">· {{ e.actor_role }}</span></p>
                <p v-if="e.note" class="text-xs text-stone-500">{{ e.note }}</p>
                <p class="text-[11px] text-stone-400">{{ formatDate(e.created_at) }}</p>
              </li>
            </ol>
          </div>
        </div>

        <!-- Footer action: pay refund if unlocked -->
        <div v-if="detail.status === 'refund_processing'" class="border-t border-stone-200 p-4">
          <button class="btn-success w-full" @click="openRefund(detail)"><IndianRupee :size="14" /> Pay refund &amp; record UTR</button>
        </div>
      </div>
    </div>

    <!-- ── Refund payout modal ───────────────────────────── -->
    <div v-if="refundTarget" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/40 p-4" @click.self="refundTarget = null">
      <div class="w-full max-w-md rounded-2xl bg-white p-5 shadow-lg">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-h4">Pay refund</h3>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="refundTarget = null"><X :size="18" /></button>
        </div>
        <div class="mb-4 rounded-lg border border-stone-100 bg-stone-50 p-3 text-sm">
          <div class="flex items-center justify-between"><span class="text-stone-500">Return</span><span class="font-medium">{{ refundTarget.return_number }}</span></div>
          <div class="mt-1 flex items-center justify-between"><span class="text-stone-500">Amount</span><span class="font-semibold text-ink">{{ formatINR(refundTarget.refund_amount) }}</span></div>
          <div class="mt-1 text-stone-500">
            Pay to:
            <span v-if="refundTarget.refund_method === 'upi'" class="font-medium text-ink">{{ refundTarget.refund_upi }}</span>
            <span v-else class="font-medium text-ink">{{ refundTarget.refund_acc_name }} · {{ refundTarget.refund_acc_no }} · {{ refundTarget.refund_ifsc }}</span>
          </div>
        </div>
        <label class="label">Bank UTR / reference number</label>
        <input v-model="utr" class="input" placeholder="e.g. 1234567890 (shown to seller & customer)" />
        <p class="field-hint">Pay the customer out-of-band first, then record the UTR here to close the refund.</p>
        <div class="mt-4 flex gap-3">
          <button class="btn-ghost flex-1" @click="refundTarget = null">Cancel</button>
          <button class="btn-success flex-1" :disabled="paying" @click="submitRefund">{{ paying ? 'Saving…' : 'Mark paid' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
