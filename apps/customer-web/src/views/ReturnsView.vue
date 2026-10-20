<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { RotateCcw, Package, CheckCircle2, XCircle, Clock, IndianRupee, RefreshCw } from 'lucide-vue-next'
import type { ReturnRecord } from '@carlitos/types'
import { customer, MEDIA_BASE } from '@/api'
import { formatINR, formatDate, returnReasonLabel, returnStatusLabel, resolveMediaUrl } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'

const router = useRouter()
const returns = ref<ReturnRecord[]>([])
const loading = ref(true)

// How many re-tries a customer gets for the same item (mirrors the backend
// MaxReturnAttempts). After this many rejections the item is permanently blocked.
const MAX_ATTEMPTS = 2

async function load() {
  loading.value = true
  try { returns.value = await customer.returns() }
  catch { returns.value = [] }
  finally { loading.value = false }
}
onMounted(load)

// Group returns by the order item so we can count rejections and offer a
// re-submit only while attempts remain.
function itemKey(r: ReturnRecord): string {
  return r.items[0]?.order_item_id || r.id
}
function rejectedCountFor(r: ReturnRecord): number {
  const key = itemKey(r)
  return returns.value.filter(x => itemKey(x) === key && x.status === 'rejected').length
}
// A rejected return can be re-submitted if it's the latest attempt for that
// item AND the attempt cap hasn't been reached.
function canResubmit(r: ReturnRecord): boolean {
  if (r.status !== 'rejected') return false
  const key = itemKey(r)
  const sameItem = returns.value
    .filter(x => itemKey(x) === key)
    .sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at))
  if (sameItem[0]?.id !== r.id) return false         // only the newest rejected one
  return rejectedCountFor(r) < MAX_ATTEMPTS
}
function attemptsLeft(r: ReturnRecord): number {
  return Math.max(0, MAX_ATTEMPTS - rejectedCountFor(r))
}

function resubmit(r: ReturnRecord) {
  const itemId = r.items[0]?.order_item_id
  if (!r.order_id || !itemId) return
  // Open the order detail and auto-launch the return flow for this item.
  router.push(`/orders/${r.order_id}?return=${itemId}`)
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
  return 'bg-accent'
}
function eventIcon(s: string) {
  if (['refunded', 'completed'].includes(s)) return IndianRupee
  if (['received', 'approved'].includes(s)) return CheckCircle2
  if (s === 'rejected') return XCircle
  return Clock
}

const hasAny = computed(() => returns.value.length > 0)
</script>

<template>
  <div class="mx-auto max-w-xl">
    <PageBack history />
    <h1 class="mb-1 text-xl font-semibold text-ink">Returns &amp; Exchanges</h1>
    <p class="mb-5 text-sm text-stone-500">Every return and exchange you've raised, with its live status and refund.</p>

    <!-- Loading -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="skeleton h-40 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!hasAny" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-14 text-center">
      <RotateCcw :size="36" class="mb-3 text-stone-300" />
      <p class="text-sm font-medium text-stone-700">No returns yet</p>
      <p class="mt-1 text-xs text-stone-500">Open a delivered order and choose Return / Exchange to raise one.</p>
      <button class="btn-primary btn-sm mt-5" @click="router.push('/orders')">Go to My Orders</button>
    </div>

    <!-- List -->
    <div v-else class="space-y-4">
      <div v-for="r in returns" :key="r.id" class="card overflow-hidden">
        <!-- Header -->
        <div class="flex items-center justify-between gap-2 border-b border-stone-100 px-4 py-3">
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold text-ink">{{ r.return_number }}</p>
            <p class="mt-0.5 text-xs text-stone-500">
              <span class="capitalize">{{ r.type }}</span>
              <span v-if="r.order_number"> · Order {{ r.order_number }}</span>
              · {{ formatDate(r.created_at) }}
            </p>
          </div>
          <span :class="['shrink-0 rounded-full px-2.5 py-1 text-xs font-medium', statusChip(r.status)]">{{ returnStatusLabel(r.status) }}</span>
        </div>

        <div class="px-4 py-3">
          <!-- Item(s) — full details -->
          <div v-for="it in r.items" :key="it.order_item_id" class="flex items-start gap-3">
            <div class="h-16 w-14 shrink-0 overflow-hidden rounded-md bg-stone-100">
              <img v-if="it.image" :src="resolveMediaUrl(MEDIA_BASE, it.image)" :alt="it.product_title" class="h-full w-full object-cover" loading="lazy" />
              <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="16" /></div>
            </div>
            <div class="min-w-0 flex-1">
              <p class="line-clamp-2 text-sm font-medium leading-snug text-ink">{{ it.product_title }}</p>
              <p class="mt-0.5 text-xs text-stone-500">
                <span v-if="it.variant_label">{{ it.variant_label }} · </span>Qty {{ it.quantity }}
                <span v-if="it.sku"> · SKU {{ it.sku }}</span>
              </p>
              <p class="mt-0.5 text-xs font-semibold text-ink">{{ formatINR(it.refund_amount) }}</p>
            </div>
          </div>

          <!-- Reason + note -->
          <div class="mt-3 rounded-lg border border-stone-100 bg-stone-50 p-3 text-xs">
            <p class="text-stone-500">Reason</p>
            <p class="font-medium text-ink">{{ returnReasonLabel(r.reason_code) || r.reason }}</p>
            <p v-if="r.reason && r.reason !== returnReasonLabel(r.reason_code)" class="mt-1 text-stone-500">{{ r.reason }}</p>
          </div>

          <!-- Photos -->
          <div v-if="r.photos.length" class="mt-3">
            <p class="mb-1 text-xs font-medium text-stone-500">Your photos</p>
            <div class="flex flex-wrap gap-2">
              <a v-for="(p, i) in r.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
                <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Proof" class="h-14 w-14 rounded-md border border-stone-200 object-cover" loading="lazy" />
              </a>
            </div>
          </div>

          <!-- Refund destination + amount (return only) -->
          <div v-if="r.type === 'return'" class="mt-3 rounded-lg border border-stone-100 bg-stone-50 p-3 text-xs">
            <div class="flex items-center justify-between">
              <span class="text-stone-500">Refund {{ r.refund_method === 'upi' ? 'to UPI' : r.refund_method === 'bank' ? 'to bank account' : '' }}</span>
              <span class="font-semibold text-ink">{{ formatINR(r.refund_amount || r.items.reduce((s, i) => s + i.refund_amount, 0)) }}</span>
            </div>
            <p v-if="r.refund_method === 'upi' && r.refund_upi" class="mt-1 text-stone-500">{{ r.refund_upi }}</p>
            <p v-else-if="r.refund_method === 'bank' && r.refund_acc_no" class="mt-1 text-stone-500">A/C {{ r.refund_acc_no }} · {{ r.refund_ifsc }}</p>
            <p v-if="r.refund_utr" class="mt-1 font-medium text-emerald-600">Refund paid · UTR {{ r.refund_utr }}</p>
            <p v-else-if="r.status === 'refund_processing'" class="mt-1 text-blue-600">Refund is being processed</p>
          </div>

          <!-- Rejection + re-submit -->
          <div v-if="r.status === 'rejected'" class="mt-3 rounded-lg border border-red-100 bg-red-50 p-3">
            <p class="text-xs text-red-700"><span class="font-medium">Seller rejected:</span> {{ r.reject_reason || 'No reason provided.' }}</p>
            <div v-if="canResubmit(r)" class="mt-2.5 flex items-center justify-between gap-2">
              <span class="text-[11px] text-stone-500">{{ attemptsLeft(r) }} re-try{{ attemptsLeft(r) > 1 ? 'ies' : '' }} left</span>
              <button class="inline-flex items-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-xs font-semibold text-white hover:bg-accent-dark" @click="resubmit(r)">
                <RefreshCw :size="13" /> Re-submit return
              </button>
            </div>
            <p v-else class="mt-2 text-[11px] font-medium text-stone-500">
              Re-submit limit reached — this item can no longer be returned.
            </p>
          </div>

          <!-- Timeline -->
          <div v-if="r.events.length" class="mt-4">
            <p class="mb-2 text-xs font-semibold text-stone-500">Status timeline</p>
            <ol class="relative ml-1 border-l-2 border-stone-100 pl-5">
              <li v-for="(e, i) in r.events" :key="i" class="relative pb-4 last:pb-0">
                <span :class="['absolute -left-[26px] top-0 flex h-4 w-4 items-center justify-center rounded-full ring-4 ring-white', eventDot(e.status)]">
                  <component :is="eventIcon(e.status)" :size="9" class="text-white" />
                </span>
                <p class="text-xs font-medium text-ink">
                  {{ returnStatusLabel(e.status) }}
                  <span class="font-normal text-stone-400">· {{ e.actor_role }}</span>
                </p>
                <p v-if="e.note" class="text-xs text-stone-500">{{ e.note }}</p>
                <p class="text-[11px] text-stone-400">{{ formatDate(e.created_at) }}</p>
              </li>
            </ol>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
