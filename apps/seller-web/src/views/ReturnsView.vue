<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RotateCcw, Check, X, Package, CheckCircle2, XCircle, Clock, IndianRupee } from 'lucide-vue-next'
import type { ReturnRecord } from '@carlitos/types'
import { seller, MEDIA_BASE } from '@/api'
import { formatINR, formatDate, returnReasonLabel, returnStatusLabel, resolveMediaUrl, extractApiError } from '@carlitos/shared'

const returns = ref<ReturnRecord[]>([])
const loading = ref(true)
const busy    = ref('')
const message = ref('')
const error   = ref('')

// Reject reason dropdown (seller side).
const SELLER_REJECT_REASONS = [
  'Return window has passed',
  'Item shows signs of use / wash',
  'Tags or packaging missing',
  'Not the item we shipped',
  'No defect found on inspection',
  'Reason not valid for return policy',
  'Other',
]
const rejectTarget = ref<ReturnRecord | null>(null)
const rejectReason = ref('')
const rejectNote = ref('')

async function load() {
  loading.value = true
  try { returns.value = await seller.returns() }
  catch { returns.value = [] }
  finally { loading.value = false }
}
onMounted(load)

async function approve(id: string) {
  busy.value = id; error.value = ''; message.value = ''
  try { await seller.approveReturn(id); message.value = 'Return approved.'; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
async function received(id: string) {
  busy.value = id; error.value = ''; message.value = ''
  try { await seller.receiveReturn(id); message.value = 'Marked received; refund is now pending admin payout.'; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
function openReject(r: ReturnRecord) { rejectTarget.value = r; rejectReason.value = ''; rejectNote.value = '' }
async function submitReject() {
  if (!rejectTarget.value) return
  if (!rejectReason.value) { error.value = 'Please choose a rejection reason.'; return }
  const reason = rejectReason.value === 'Other' && rejectNote.value ? rejectNote.value : rejectReason.value
  busy.value = rejectTarget.value.id; error.value = ''; message.value = ''
  try {
    await seller.rejectReturn(rejectTarget.value.id, reason)
    message.value = 'Return rejected.'
    rejectTarget.value = null
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}

const counts = computed(() => ({
  pending: returns.value.filter(r => r.status === 'requested').length,
  approved: returns.value.filter(r => r.status === 'approved').length,
}))

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
  return 'bg-accent'
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
      <h1 class="page-title">Return Requests</h1>
      <p class="text-sm text-stone-500">
        {{ counts.pending }} awaiting your review · {{ counts.approved }} approved
      </p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Loading -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="skeleton h-40 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!returns.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <RotateCcw :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No return requests</p>
      <p class="mt-1 text-xs text-stone-500">Return and exchange requests from customers will appear here.</p>
    </div>

    <!-- Cards -->
    <div v-else class="space-y-4">
      <div v-for="r in returns" :key="r.id" class="card overflow-hidden">
        <!-- Header -->
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-stone-100 px-4 py-3">
          <div class="flex items-center gap-2">
            <p class="text-sm font-semibold">{{ r.return_number }}</p>
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(r.status)]">{{ returnStatusLabel(r.status) }}</span>
            <span class="rounded-full bg-stone-100 px-2.5 py-0.5 text-xs capitalize text-stone-600">{{ r.type }}</span>
          </div>
          <p class="text-xs text-stone-500">{{ r.order_number }} · {{ formatDate(r.created_at) }}</p>
        </div>

        <div class="grid gap-4 p-4 md:grid-cols-2">
          <!-- Left: item + reason + photos -->
          <div>
            <div v-for="it in r.items" :key="it.order_item_id" class="flex items-center gap-3">
              <div class="h-16 w-14 shrink-0 overflow-hidden rounded-md bg-stone-100">
                <img v-if="it.image" :src="resolveMediaUrl(MEDIA_BASE, it.image)" :alt="it.product_title" class="h-full w-full object-cover" loading="lazy" />
                <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="18" /></div>
              </div>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-ink">{{ it.product_title }}</p>
                <p class="mt-0.5 text-xs text-stone-500"><span v-if="it.variant_label">{{ it.variant_label }} · </span>Qty {{ it.quantity }} · {{ formatINR(it.refund_amount) }}</p>
              </div>
            </div>

            <p class="mt-3 text-xs text-stone-500">Reason: <span class="text-stone-700">{{ returnReasonLabel(r.reason_code) || r.reason }}</span></p>
            <p v-if="r.reason && r.reason !== returnReasonLabel(r.reason_code)" class="mt-0.5 text-xs text-stone-500">Note: {{ r.reason }}</p>

            <!-- Photo proof -->
            <div v-if="r.photos.length" class="mt-2">
              <p class="mb-1 text-xs font-medium text-stone-500">Customer photos</p>
              <div class="flex flex-wrap gap-2">
                <a v-for="(p, i) in r.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
                  <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Proof" class="h-16 w-16 rounded-md border border-stone-200 object-cover" loading="lazy" />
                </a>
              </div>
            </div>

            <!-- Refund destination (so seller knows how customer will be paid) -->
            <div v-if="r.type === 'return'" class="mt-3 rounded-lg border border-stone-100 bg-stone-50 p-3 text-xs text-stone-600">
              <p class="font-medium text-stone-700">Customer refund destination</p>
              <p v-if="r.refund_method === 'upi'">UPI: {{ r.refund_upi }}</p>
              <template v-else-if="r.refund_method === 'bank'">
                <p>{{ r.refund_acc_name }}</p>
                <p>A/C {{ r.refund_acc_no }} · {{ r.refund_ifsc }}</p>
              </template>
              <p v-if="r.refund_utr" class="mt-1 text-emerald-600">Refund paid · UTR {{ r.refund_utr }}</p>
            </div>
          </div>

          <!-- Right: actions + timeline -->
          <div class="flex flex-col">
            <!-- Actions -->
            <div class="mb-4 flex flex-wrap gap-2">
              <template v-if="r.status === 'requested'">
                <button class="btn-success btn-sm" :disabled="busy === r.id" @click="approve(r.id)"><Check :size="13" /> Approve</button>
                <button class="btn-danger btn-sm" :disabled="busy === r.id" @click="openReject(r)"><X :size="13" /> Reject</button>
              </template>
              <button v-else-if="r.status === 'approved' || r.status === 'picked'" class="btn-primary btn-sm" :disabled="busy === r.id" @click="received(r.id)">
                <Package :size="13" /> Mark Received
              </button>
              <p v-else-if="r.status === 'rejected'" class="text-xs text-red-500">Rejected: {{ r.reject_reason }}</p>
              <p v-else-if="r.status === 'refund_processing'" class="text-xs text-blue-600">Awaiting admin refund payout</p>
              <p v-else-if="r.status === 'refunded'" class="text-xs text-emerald-600">Refund completed</p>
            </div>

            <!-- Timeline -->
            <div v-if="r.events.length" class="mt-auto">
              <p class="mb-2 text-xs font-semibold text-stone-500">Timeline</p>
              <ol class="relative ml-1 border-l-2 border-stone-100 pl-5">
                <li v-for="(e, i) in r.events" :key="i" class="relative pb-3 last:pb-0">
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
        </div>
      </div>
    </div>

    <!-- Reject modal -->
    <div v-if="rejectTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="rejectTarget = null">
      <div class="w-full max-w-md rounded-2xl bg-white p-5 shadow-lg">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-h4">Reject return</h3>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="rejectTarget = null"><X :size="18" /></button>
        </div>
        <p class="mb-3 text-sm text-stone-600">{{ rejectTarget.return_number }} — the customer will see this reason.</p>
        <label class="label">Reason</label>
        <select v-model="rejectReason" class="select w-full">
          <option value="" disabled>Select a reason…</option>
          <option v-for="r in SELLER_REJECT_REASONS" :key="r" :value="r">{{ r }}</option>
        </select>
        <div v-if="rejectReason === 'Other'" class="mt-3">
          <label class="label">Details</label>
          <textarea v-model="rejectNote" class="textarea" rows="3" maxlength="500" placeholder="Explain why this return is being rejected…"></textarea>
        </div>
        <div class="mt-4 flex gap-3">
          <button class="btn-ghost flex-1" @click="rejectTarget = null">Cancel</button>
          <button class="btn-danger flex-1" :disabled="!!busy" @click="submitReject">Reject return</button>
        </div>
      </div>
    </div>
  </div>
</template>
