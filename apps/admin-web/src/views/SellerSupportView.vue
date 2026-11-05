<script setup lang="ts">
// Requests raised by SELLERS to the admin team (seller → admin). Customer
// help requests live on the separate "Customer Support" page.
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Headset, X, CheckCircle2, Eye, Send, Store } from 'lucide-vue-next'
import type { SellerSupportTicket, SupportMessage, SupportSocketEvent } from '@carlitos/types'
import { admin, MEDIA_BASE, api, tokenStore } from '@/api'
import { formatDate, resolveMediaUrl, extractApiError, connectSupportSocket } from '@carlitos/shared'

const tickets = ref<SellerSupportTicket[]>([])
const loading = ref(true)
const error = ref('')
const message = ref('')
const status = ref<'all' | 'open' | 'in_progress' | 'solved'>('all')

const detail = ref<SellerSupportTicket | null>(null)
const replyBody = ref('')
const replying = ref(false)
const resolveOpen = ref(false)
const resolution = ref('')
const resolving = ref(false)
const timelineEl = ref<HTMLElement | null>(null)

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    tickets.value = await admin.sellerSupportTickets(status.value === 'all' ? undefined : { status: status.value })
    if (detail.value) detail.value = tickets.value.find((t) => t.id === detail.value!.id) ?? null
  } catch (e) {
    if (!silent) { error.value = extractApiError(e); tickets.value = [] }
  } finally {
    loading.value = false
  }
}
function setStatus(s: typeof status.value) { status.value = s; load() }

const openCount = computed(() => tickets.value.filter((t) => t.status !== 'solved').length)

function openDetail(t: SellerSupportTicket) {
  detail.value = t
  replyBody.value = ''
  resolveOpen.value = false
  resolution.value = ''
  message.value = ''
  nextTick(scrollTimeline)
}
function scrollTimeline() {
  const el = timelineEl.value
  if (el) el.scrollTop = el.scrollHeight
}

function applyMessage(ticketId: string, msg: SupportMessage, st?: SellerSupportTicket['status']) {
  const copies = new Set<SellerSupportTicket>()
  const inList = tickets.value.find((t) => t.id === ticketId)
  if (inList) copies.add(inList)
  if (detail.value?.id === ticketId) copies.add(detail.value)
  for (const t of copies) {
    if (!t.messages.some((m) => m.id === msg.id)) t.messages.push(msg)
    if (st) t.status = st
  }
  if (detail.value?.id === ticketId) nextTick(scrollTimeline)
}

async function sendReply() {
  if (!detail.value || !replyBody.value.trim()) return
  replying.value = true; error.value = ''
  const id = detail.value.id
  try {
    const msg = await admin.replySellerSupportTicket(id, replyBody.value.trim())
    replyBody.value = ''
    if (msg?.id) applyMessage(id, msg)
  } catch (e) { error.value = extractApiError(e) }
  finally { replying.value = false }
}

async function submitResolve() {
  if (!detail.value) return
  if (!resolution.value.trim()) { error.value = 'Add a short resolution note.'; return }
  resolving.value = true; error.value = ''
  try {
    await admin.resolveSellerSupportTicket(detail.value.id, resolution.value.trim())
    message.value = `Request ${detail.value.ticket_number} marked solved.`
    resolveOpen.value = false
    resolution.value = ''
    await load(true)
  } catch (e) { error.value = extractApiError(e) }
  finally { resolving.value = false }
}

/* ── Live updates (WebSocket) ─────────────────────────────────── */
let socket: { close: () => void } | null = null

function handleSocketEvent(ev: SupportSocketEvent) {
  if (ev.scope !== 'seller_support') return // customer-ticket events belong to "Customer Support"
  if (ev.type === 'ticket_changed') { load(true); return }
  if (ev.message) applyMessage(ev.ticket_id, ev.message, ev.status)
}

onMounted(() => {
  load()
  socket = connectSupportSocket<SupportSocketEvent>({
    baseURL: api.http.defaults.baseURL || '',
    getToken: () => tokenStore.getAccess(),
    onEvent: handleSocketEvent,
    onOpen: (reconnected) => { if (reconnected) load(true) },
    beforeReconnect: async () => { await admin.sellerSupportTickets() },
  })
})
onBeforeUnmount(() => socket?.close())

/* ── Display helpers ──────────────────────────────────────────── */
const categoryLabelMap: Record<string, string> = {
  payments: 'Payments & Settlements', orders: 'Orders & Shipping', products: 'Products & Listings',
  account: 'Account & KYC', technical: 'Technical problem', other: 'Other',
}
const catLabel = (c: string) => categoryLabelMap[c] || 'Help'

function statusChip(s: string) {
  if (s === 'solved') return 'bg-emerald-50 text-emerald-600'
  if (s === 'in_progress') return 'bg-blue-50 text-blue-600'
  return 'bg-amber-50 text-amber-600'
}
function statusLabel(s: string) {
  if (s === 'solved') return 'Solved'
  if (s === 'in_progress') return 'In progress'
  return 'Open'
}
function authorLabel(role: string, t: SellerSupportTicket) {
  if (role === 'seller') return t.seller_name || 'Seller'
  if (role === 'admin') return 'Support Team'
  return 'System'
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Seller Support</h1>
      <p class="text-sm text-stone-500">
        Help requests raised by sellers (payments, settlements, account, technical).
        Customer requests are on the Customer Support page.
        <span v-if="openCount"> · {{ openCount }} open</span>
      </p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <div class="mb-4 flex flex-wrap gap-2">
      <button
        v-for="s in (['all','open','in_progress','solved'] as const)" :key="s"
        :class="['rounded-full border px-3 py-1.5 text-xs font-medium', status === s ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="setStatus(s)"
      >
        {{ s === 'all' ? 'All' : statusLabel(s) }}
      </button>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-12 w-full rounded-lg" />
    </div>

    <div v-else-if="!tickets.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-12 text-center">
      <Headset :size="40" class="mb-3 text-stone-300" />
      <p class="text-sm font-medium text-stone-700">No seller requests</p>
      <p class="mt-1 text-xs text-stone-500">Requests raised by sellers will appear here.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Request #</th>
            <th>Seller</th>
            <th class="hidden md:table-cell">Category</th>
            <th>Status</th>
            <th class="hidden sm:table-cell">Raised</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tickets" :key="t.id">
            <td class="font-medium">{{ t.ticket_number }}</td>
            <td class="max-w-[10rem] truncate text-stone-600">{{ t.seller_name || 'Seller' }}</td>
            <td class="hidden md:table-cell text-stone-600">{{ catLabel(t.category) }}</td>
            <td><span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(t.status)]">{{ statusLabel(t.status) }}</span></td>
            <td class="hidden sm:table-cell text-xs text-stone-500">{{ formatDate(t.created_at) }}</td>
            <td class="text-right">
              <button class="btn-ghost btn-xs" @click="openDetail(t)"><Eye :size="12" /> Open</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Detail drawer (full width on phones, side panel on larger screens) -->
    <div v-if="detail" class="fixed inset-0 z-50 flex justify-end bg-black/40" @click.self="detail = null">
      <div class="flex h-full w-full max-w-lg flex-col bg-white shadow-xl">
        <div class="flex items-center justify-between gap-2 border-b border-stone-200 px-4 py-4 sm:px-5">
          <div class="min-w-0">
            <h3 class="text-h4 truncate">{{ detail.subject }}</h3>
            <p class="truncate text-xs text-stone-500">#{{ detail.ticket_number }} · {{ detail.seller_name || 'Seller' }} · {{ catLabel(detail.category) }}</p>
          </div>
          <button class="shrink-0 rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="detail = null"><X :size="18" /></button>
        </div>

        <div class="flex-1 space-y-4 overflow-y-auto p-4 sm:p-5">
          <div class="flex flex-wrap items-center gap-2">
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(detail.status)]">{{ statusLabel(detail.status) }}</span>
            <span class="inline-flex items-center gap-1 rounded-full bg-stone-100 px-2.5 py-0.5 text-xs text-stone-600">
              <Store :size="11" /> {{ detail.seller_name || 'Seller' }}
            </span>
          </div>

          <div v-if="detail.photos.length">
            <p class="mb-1 text-xs font-semibold text-stone-500">Seller photos</p>
            <div class="flex flex-wrap gap-2">
              <a v-for="(p, i) in detail.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
                <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Attachment" class="h-16 w-16 rounded-md border border-stone-200 object-cover" loading="lazy" />
              </a>
            </div>
          </div>

          <div>
            <p class="mb-2 text-xs font-semibold text-stone-500">Conversation</p>
            <div ref="timelineEl" class="max-h-[22rem] space-y-3 overflow-y-auto pr-1">
              <div v-for="m in detail.messages" :key="m.id" class="flex" :class="m.author_role === 'seller' ? 'justify-start' : 'justify-end'">
                <div class="max-w-[88%]">
                  <p class="mb-0.5 text-[10px] font-medium text-stone-400" :class="m.author_role === 'seller' ? 'text-left' : 'text-right'">
                    {{ authorLabel(m.author_role, detail) }} · {{ formatDate(m.created_at) }}
                  </p>
                  <div
                    class="whitespace-pre-wrap break-words rounded-2xl px-3.5 py-2 text-sm"
                    :class="m.author_role === 'seller' ? 'rounded-bl-sm bg-stone-100 text-ink' : 'rounded-br-sm bg-ink text-white'"
                  >
                    <a v-if="m.image_key" :href="resolveMediaUrl(MEDIA_BASE, m.image_key)" target="_blank" rel="noopener" class="block">
                      <img :src="resolveMediaUrl(MEDIA_BASE, m.image_key)" alt="Shared photo" class="max-h-56 w-full rounded-lg object-cover" :class="m.body ? 'mb-1.5' : ''" loading="lazy" />
                    </a>
                    <template v-if="m.body">{{ m.body }}</template>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="detail.status === 'solved' && detail.resolution" class="flex items-start gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3">
            <CheckCircle2 :size="16" class="mt-0.5 shrink-0 text-emerald-600" />
            <div class="min-w-0">
              <p class="text-xs font-semibold text-emerald-800">Resolved</p>
              <p class="mt-0.5 break-words text-xs text-emerald-700">{{ detail.resolution }}</p>
            </div>
          </div>
        </div>

        <div v-if="detail.status !== 'solved'" class="border-t border-stone-200 p-4">
          <div v-if="!resolveOpen">
            <div class="flex items-end gap-2">
              <textarea v-model="replyBody" rows="1" class="textarea min-h-[42px] min-w-0 flex-1 py-2.5 text-sm" placeholder="Reply to the seller…" @keydown.enter.exact.prevent="sendReply" />
              <button class="btn-primary h-[42px] w-[42px] shrink-0 !px-0" :disabled="replying || !replyBody.trim()" aria-label="Send" @click="sendReply"><Send :size="15" /></button>
            </div>
            <button class="btn-success btn-sm mt-2 w-full" @click="resolveOpen = true"><CheckCircle2 :size="14" /> Mark as solved</button>
          </div>
          <div v-else>
            <label class="label">Resolution note</label>
            <textarea v-model="resolution" rows="3" class="textarea text-sm" placeholder="Explain what happened and how it was resolved (the seller sees this)." />
            <div class="mt-3 flex gap-2">
              <button class="btn-ghost flex-1" @click="resolveOpen = false">Cancel</button>
              <button class="btn-success flex-1" :disabled="resolving" @click="submitResolve">{{ resolving ? 'Saving…' : 'Confirm solved' }}</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
