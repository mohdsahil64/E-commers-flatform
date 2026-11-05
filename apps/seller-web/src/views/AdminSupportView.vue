<script setup lang="ts">
// Seller → Admin support: the seller raises a request to the CARLITOS team and
// chats with them live. (Customer issues about the seller's products live on
// the separate "Customer Issues" page.)
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  Headset, Plus, X, ImagePlus, Send, ArrowLeft, CheckCircle2, MessageSquare,
  Wallet, Truck, Package, UserCog, Wrench, HelpCircle, ShieldCheck,
} from 'lucide-vue-next'
import type { SellerSupportTicket, SellerSupportCategory, SupportMessage, SupportSocketEvent } from '@carlitos/types'
import { seller, MEDIA_BASE, api, tokenStore } from '@/api'
import { formatDate, resolveMediaUrl, extractApiError, connectSupportSocket } from '@carlitos/shared'

const tickets = ref<SellerSupportTicket[]>([])
const loading = ref(true)
const error = ref('')
const message = ref('')
const filter = ref<'all' | 'open' | 'in_progress' | 'solved'>('all')

// Which ticket's chat is open (null = list only; on mobile this swaps screens).
const activeId = ref<string | null>(null)
const active = computed(() => tickets.value.find((t) => t.id === activeId.value) || null)

const categories: { code: SellerSupportCategory; label: string; hint: string; icon: any }[] = [
  { code: 'payments',  label: 'Payments & Settlements', hint: 'Payout, commission, deductions', icon: Wallet },
  { code: 'orders',    label: 'Orders & Shipping',      hint: 'Order, courier, return problems', icon: Truck },
  { code: 'products',  label: 'Products & Listings',    hint: 'Approval, catalog, stock',        icon: Package },
  { code: 'account',   label: 'Account & KYC',          hint: 'Profile, bank, documents',        icon: UserCog },
  { code: 'technical', label: 'Technical problem',      hint: 'Something is not working',        icon: Wrench },
  { code: 'other',     label: 'Something else',         hint: 'Any other help you need',         icon: HelpCircle },
]
const catMap = Object.fromEntries(categories.map((c) => [c.code, c])) as Record<string, (typeof categories)[number]>
const catLabel = (c: string) => catMap[c]?.label || 'Help'

/* ── Load ─────────────────────────────────────────────────────── */
async function load(silent = false) {
  if (!silent) loading.value = true
  try { tickets.value = await seller.adminSupportTickets() }
  catch (e) { if (!silent) { error.value = extractApiError(e); tickets.value = [] } }
  finally { loading.value = false }
}

const filtered = computed(() =>
  filter.value === 'all' ? tickets.value : tickets.value.filter((t) => t.status === filter.value),
)
const openCount = computed(() => tickets.value.filter((t) => t.status !== 'solved').length)

/* ── New request form ─────────────────────────────────────────── */
const showForm = ref(false)
const form = ref({ category: '' as SellerSupportCategory | '', subject: '', description: '' })
const photoKeys = ref<string[]>([])
const photoPreviews = ref<string[]>([])
const uploading = ref(false)
const submitting = ref(false)

function openForm() {
  showForm.value = true
  activeId.value = null
  error.value = ''
  message.value = ''
}
function closeForm() {
  showForm.value = false
  form.value = { category: '', subject: '', description: '' }
  photoKeys.value = []
  photoPreviews.value = []
}

async function onPhoto(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (photoKeys.value.length >= 4) { error.value = 'You can attach up to 4 photos.'; input.value = ''; return }
  uploading.value = true; error.value = ''
  try {
    const res = await seller.uploadImage(file)
    photoKeys.value.push(res.key)
    photoPreviews.value.push(res.url || resolveMediaUrl(MEDIA_BASE, res.key))
  } catch (err) { error.value = extractApiError(err) }
  finally { uploading.value = false; input.value = '' }
}
function removePhoto(i: number) { photoKeys.value.splice(i, 1); photoPreviews.value.splice(i, 1) }

const canSubmit = computed(
  () => !!form.value.category && form.value.description.trim().length >= 5 && !submitting.value && !uploading.value,
)

async function submit() {
  if (!canSubmit.value) return
  submitting.value = true; error.value = ''
  try {
    const res = await seller.openAdminSupportTicket({
      category: form.value.category,
      subject: form.value.subject.trim() || undefined,
      description: form.value.description.trim(),
      photo_keys: photoKeys.value,
    })
    closeForm()
    message.value = 'Request sent to the CARLITOS team. We will reply here.'
    await load(true)
    activeId.value = res.id
    nextTick(scrollTimeline)
  } catch (e) { error.value = extractApiError(e) }
  finally { submitting.value = false }
}

/* ── Chat ─────────────────────────────────────────────────────── */
const timelineEl = ref<HTMLElement | null>(null)
const draft = ref('')
const draftImage = ref<{ key: string; preview: string } | null>(null)
const uploadingReply = ref(false)
const sending = ref(false)

function openTicket(t: SellerSupportTicket) {
  activeId.value = t.id
  showForm.value = false
  message.value = ''
  error.value = ''
  draft.value = ''
  draftImage.value = null
  nextTick(scrollTimeline)
}
function scrollTimeline() {
  const el = timelineEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function onReplyImage(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  uploadingReply.value = true; error.value = ''
  try {
    const res = await seller.uploadImage(file)
    draftImage.value = { key: res.key, preview: res.url || resolveMediaUrl(MEDIA_BASE, res.key) }
  } catch (err) { error.value = extractApiError(err) }
  finally { uploadingReply.value = false; input.value = '' }
}

const canSend = computed(
  () => !sending.value && !uploadingReply.value && (!!draft.value.trim() || !!draftImage.value),
)

function applyMessage(ticketId: string, msg: SupportMessage, status?: SellerSupportTicket['status']) {
  const t = tickets.value.find((x) => x.id === ticketId)
  if (!t) return
  if (!t.messages.some((m) => m.id === msg.id)) t.messages.push(msg)
  if (status) t.status = status
  if (activeId.value === ticketId) nextTick(scrollTimeline)
}

async function send() {
  if (!active.value || !canSend.value) return
  sending.value = true; error.value = ''
  const id = active.value.id
  try {
    const msg = await seller.replyAdminSupportTicket(id, draft.value.trim(), draftImage.value?.key)
    draft.value = ''
    draftImage.value = null
    if (msg?.id) applyMessage(id, msg)
  } catch (e) { error.value = extractApiError(e) }
  finally { sending.value = false }
}

/* ── Live updates (WebSocket) ─────────────────────────────────── */
let socket: { close: () => void } | null = null

function handleSocketEvent(ev: SupportSocketEvent) {
  if (ev.scope !== 'seller_support') return // customer-ticket events belong to "Customer Issues"
  if (ev.type === 'ticket_changed') { load(true); return }
  if (!ev.message) return
  if (!tickets.value.some((t) => t.id === ev.ticket_id)) { load(true); return }
  applyMessage(ev.ticket_id, ev.message, ev.status)
}

onMounted(() => {
  load()
  socket = connectSupportSocket<SupportSocketEvent>({
    baseURL: api.http.defaults.baseURL || '',
    getToken: () => tokenStore.getAccess(),
    onEvent: handleSocketEvent,
    onOpen: (reconnected) => { if (reconnected) load(true) },
    beforeReconnect: async () => { await seller.adminSupportTickets() },
  })
})
onBeforeUnmount(() => socket?.close())

/* ── Display helpers ──────────────────────────────────────────── */
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
function authorLabel(role: string) {
  if (role === 'seller') return 'You'
  if (role === 'admin') return 'CARLITOS Team'
  return 'System'
}
</script>

<template>
  <div>
    <div class="page-header flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 class="page-title">Admin Support</h1>
        <p class="text-sm text-stone-500">
          Need help from the CARLITOS team? Raise a request and chat with us live.
          <span v-if="openCount"> · {{ openCount }} open</span>
        </p>
      </div>
      <button class="btn-primary shrink-0" @click="openForm"><Plus :size="15" /> New request</button>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error" class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Two panes on large screens; one at a time on phones/tablets. -->
    <div class="grid gap-4 lg:grid-cols-[minmax(280px,360px)_1fr]">

      <!-- ═════ Left: filters + request list ═════ -->
      <div :class="(activeId || showForm) ? 'hidden lg:block' : 'block'" class="min-w-0">
        <div class="mb-3 flex flex-wrap gap-2">
          <button
            v-for="f in (['all','open','in_progress','solved'] as const)" :key="f"
            :class="['rounded-full border px-3 py-1.5 text-xs font-medium', filter === f ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
            @click="filter = f"
          >
            {{ f === 'all' ? 'All' : statusLabel(f) }}
          </button>
        </div>

        <div v-if="loading" class="space-y-3">
          <div v-for="i in 3" :key="i" class="skeleton h-20 w-full rounded-xl" />
        </div>

        <div v-else-if="!filtered.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-12 text-center">
          <Headset :size="38" class="mb-3 text-stone-300" />
          <p class="text-sm font-medium text-stone-700">No requests yet</p>
          <p class="mt-1 text-xs text-stone-500">Raise a request and the CARLITOS team will reply here.</p>
          <button class="btn-primary mt-4" @click="openForm"><Plus :size="15" /> New request</button>
        </div>

        <div v-else class="space-y-2.5">
          <button
            v-for="t in filtered" :key="t.id"
            class="card w-full p-3.5 text-left transition-colors hover:border-ink"
            :class="activeId === t.id ? 'border-ink bg-stone-50' : ''"
            @click="openTicket(t)"
          >
            <div class="flex items-start justify-between gap-2">
              <p class="truncate text-sm font-semibold text-ink">{{ t.subject }}</p>
              <span :class="['shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(t.status)]">{{ statusLabel(t.status) }}</span>
            </div>
            <p class="mt-0.5 truncate text-xs text-stone-500">{{ catLabel(t.category) }}</p>
            <p class="mt-1 text-[11px] text-stone-400">#{{ t.ticket_number }} · {{ formatDate(t.created_at) }}</p>
          </button>
        </div>
      </div>

      <!-- ═════ Right: new request form / chat / empty state ═════ -->
      <div :class="(activeId || showForm) ? 'block' : 'hidden lg:block'" class="min-w-0">

        <!-- New request form -->
        <div v-if="showForm" class="card p-4 sm:p-5">
          <div class="mb-4 flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <button class="rounded p-1 text-stone-500 hover:bg-stone-50 lg:hidden" aria-label="Back" @click="closeForm"><ArrowLeft :size="18" /></button>
              <h2 class="text-h4">New request to CARLITOS</h2>
            </div>
            <button class="hidden rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink lg:block" aria-label="Close" @click="closeForm"><X :size="18" /></button>
          </div>

          <p class="label">What do you need help with?</p>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            <button
              v-for="c in categories" :key="c.code" type="button"
              class="flex items-start gap-2.5 rounded-lg border p-2.5 text-left transition-colors"
              :class="form.category === c.code ? 'border-ink bg-stone-50 ring-1 ring-ink' : 'border-stone-200 hover:border-stone-400'"
              @click="form.category = c.code"
            >
              <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" :class="form.category === c.code ? 'bg-ink text-white' : 'bg-stone-100 text-stone-500'">
                <component :is="c.icon" :size="16" />
              </span>
              <span class="min-w-0">
                <span class="block text-sm font-medium text-ink">{{ c.label }}</span>
                <span class="block text-xs text-stone-500">{{ c.hint }}</span>
              </span>
            </button>
          </div>

          <label class="label mt-4">Subject (optional)</label>
          <input v-model="form.subject" class="input" maxlength="120" placeholder="Short title, e.g. Settlement not received" />

          <label class="label mt-3">Describe your issue</label>
          <textarea v-model="form.description" class="textarea" rows="5" maxlength="2000" placeholder="Tell us what happened, with order numbers or dates if you have them…" />

          <p class="label mt-3">Photos (optional, up to 4)</p>
          <div class="flex flex-wrap items-center gap-2">
            <div v-for="(p, i) in photoPreviews" :key="i" class="relative h-16 w-16 overflow-hidden rounded-md border border-stone-200">
              <img :src="p" alt="Proof" class="h-full w-full object-cover" />
              <button class="absolute right-0.5 top-0.5 flex h-5 w-5 items-center justify-center rounded-full bg-black/60 text-white" aria-label="Remove photo" @click="removePhoto(i)"><X :size="12" /></button>
            </div>
            <label v-if="photoKeys.length < 4" class="flex h-16 w-16 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-md border border-dashed border-stone-300 text-stone-400 hover:border-ink hover:text-ink">
              <ImagePlus :size="18" />
              <span class="text-[10px]">{{ uploading ? '…' : 'Add' }}</span>
              <input type="file" accept="image/*" class="hidden" :disabled="uploading" @change="onPhoto" />
            </label>
          </div>

          <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button class="btn-ghost" @click="closeForm">Cancel</button>
            <button class="btn-primary" :disabled="!canSubmit" @click="submit">
              <Send :size="15" /> {{ submitting ? 'Sending…' : 'Send request' }}
            </button>
          </div>
        </div>

        <!-- Chat -->
        <div v-else-if="active" class="card flex flex-col overflow-hidden">
          <div class="flex items-start gap-2 border-b border-stone-100 px-3 py-3 sm:px-4">
            <button class="mt-0.5 rounded p-1 text-stone-500 hover:bg-stone-50 lg:hidden" aria-label="Back to requests" @click="activeId = null"><ArrowLeft :size="18" /></button>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-semibold text-ink">{{ active.subject }}</p>
              <p class="truncate text-xs text-stone-500">#{{ active.ticket_number }} · {{ catLabel(active.category) }} · {{ formatDate(active.created_at) }}</p>
            </div>
            <span :class="['shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(active.status)]">{{ statusLabel(active.status) }}</span>
          </div>

          <div v-if="active.photos.length" class="flex flex-wrap gap-2 border-b border-stone-100 px-3 py-2.5 sm:px-4">
            <a v-for="(p, i) in active.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
              <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Attachment" class="h-14 w-14 rounded-md border border-stone-200 object-cover" loading="lazy" />
            </a>
          </div>

          <div ref="timelineEl" class="max-h-[55vh] min-h-[14rem] space-y-3 overflow-y-auto px-3 py-3 sm:px-4">
            <div v-for="m in active.messages" :key="m.id" class="flex" :class="m.author_role === 'seller' ? 'justify-end' : 'justify-start'">
              <div class="max-w-[88%] sm:max-w-[80%]">
                <p class="mb-0.5 text-[10px] font-medium text-stone-400" :class="m.author_role === 'seller' ? 'text-right' : 'text-left'">
                  {{ authorLabel(m.author_role) }} · {{ formatDate(m.created_at) }}
                </p>
                <div
                  class="whitespace-pre-wrap break-words rounded-2xl px-3.5 py-2 text-sm"
                  :class="m.author_role === 'seller' ? 'rounded-br-sm bg-ink text-white' : 'rounded-bl-sm bg-stone-100 text-ink'"
                >
                  <a v-if="m.image_key" :href="resolveMediaUrl(MEDIA_BASE, m.image_key)" target="_blank" rel="noopener" class="block">
                    <img :src="resolveMediaUrl(MEDIA_BASE, m.image_key)" alt="Shared photo" class="max-h-56 w-full rounded-lg object-cover" :class="m.body ? 'mb-1.5' : ''" loading="lazy" />
                  </a>
                  <template v-if="m.body">{{ m.body }}</template>
                </div>
              </div>
            </div>
          </div>

          <div v-if="active.status === 'solved'" class="m-3 flex items-start gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3 sm:m-4">
            <CheckCircle2 :size="16" class="mt-0.5 shrink-0 text-emerald-600" />
            <div class="min-w-0">
              <p class="text-xs font-semibold text-emerald-800">This request is solved</p>
              <p v-if="active.resolution" class="mt-0.5 break-words text-xs text-emerald-700">{{ active.resolution }}</p>
            </div>
          </div>

          <div v-else class="border-t border-stone-100 p-3 sm:p-4">
            <div v-if="draftImage" class="mb-2 inline-block">
              <div class="relative h-14 w-14 overflow-hidden rounded-md border border-stone-200">
                <img :src="draftImage.preview" alt="Photo to send" class="h-full w-full object-cover" />
                <button class="absolute right-0.5 top-0.5 flex h-4 w-4 items-center justify-center rounded-full bg-black/60 text-white" aria-label="Remove photo" @click="draftImage = null"><X :size="10" /></button>
              </div>
            </div>
            <div class="flex items-end gap-2">
              <label class="flex h-[42px] w-[42px] shrink-0 cursor-pointer items-center justify-center rounded-lg border border-stone-200 text-stone-500 hover:border-ink hover:text-ink" :class="uploadingReply ? 'pointer-events-none opacity-60' : ''" aria-label="Attach photo">
                <ImagePlus :size="16" />
                <input type="file" accept="image/*" class="hidden" :disabled="uploadingReply" @change="onReplyImage" />
              </label>
              <textarea v-model="draft" rows="1" class="textarea min-h-[42px] min-w-0 flex-1 py-2.5 text-sm" placeholder="Write a message…" @keydown.enter.exact.prevent="send" />
              <button class="btn-primary h-[42px] w-[42px] shrink-0 !px-0" :disabled="!canSend" aria-label="Send message" @click="send"><Send :size="15" /></button>
            </div>
            <p class="mt-2 flex items-center gap-1 text-[11px] text-stone-400"><ShieldCheck :size="11" /> Only you and the CARLITOS team can see this chat.</p>
          </div>
        </div>

        <!-- Empty state (desktop) -->
        <div v-else class="hidden flex-col items-center justify-center rounded-xl border border-dashed border-stone-200 bg-stone-50 py-20 text-center lg:flex">
          <MessageSquare :size="36" class="mb-3 text-stone-300" />
          <p class="text-sm font-medium text-stone-700">Select a request to open the chat</p>
          <p class="mt-1 text-xs text-stone-500">Or raise a new one with “New request”.</p>
        </div>
      </div>
    </div>
  </div>
</template>
