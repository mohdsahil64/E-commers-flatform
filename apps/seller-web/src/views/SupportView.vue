<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, computed, nextTick } from 'vue'
import { LifeBuoy, Package, Send, CheckCircle2, X } from 'lucide-vue-next'
import type { SupportTicket, SupportMessage, SupportSocketEvent } from '@carlitos/types'
import { seller, MEDIA_BASE, api, tokenStore } from '@/api'
import { formatDate, resolveMediaUrl, extractApiError, connectSupportSocket } from '@carlitos/shared'

const tickets = ref<SupportTicket[]>([])
const loading = ref(true)
const message = ref('')
const error   = ref('')
const filter  = ref<'all' | 'open' | 'in_progress' | 'solved'>('all')

// Per-ticket reply drafts + the resolve modal target.
const drafts = ref<Record<string, string>>({})
const busy = ref('')
const resolveTarget = ref<SupportTicket | null>(null)
const resolution = ref('')

async function load(silent = false) {
  if (!silent) loading.value = true
  try { tickets.value = await seller.supportTickets() }
  catch { if (!silent) tickets.value = [] }
  finally { loading.value = false }
}

/* ── Live updates (WebSocket) ──────────────────────────────────── */
let socket: { close: () => void } | null = null

function applyMessage(ticketId: string, msg: SupportMessage, status?: SupportTicket['status']) {
  const t = tickets.value.find(x => x.id === ticketId)
  if (!t) return
  if (!t.messages.some(m => m.id === msg.id)) t.messages.push(msg)
  if (status) t.status = status
  nextTick(() => scrollTimeline(ticketId))
}

function handleSocketEvent(ev: SupportSocketEvent) {
  if (ev.scope) return // seller → admin ticket events belong to the Admin Support page
  if (ev.type === 'ticket_changed') { load(true); return }
  if (!ev.message) return
  if (!tickets.value.some(t => t.id === ev.ticket_id)) { load(true); return }
  applyMessage(ev.ticket_id, ev.message, ev.status)
}

onMounted(() => {
  load()
  socket = connectSupportSocket<SupportSocketEvent>({
    baseURL: api.http.defaults.baseURL || '',
    getToken: () => tokenStore.getAccess(),
    onEvent: handleSocketEvent,
    onOpen: (reconnected) => { if (reconnected) load(true) },
    beforeReconnect: async () => { await seller.supportTickets() },
  })
})
onBeforeUnmount(() => socket?.close())



const filtered = computed(() =>
  filter.value === 'all' ? tickets.value : tickets.value.filter(t => t.status === filter.value),
)
const counts = computed(() => ({
  open: tickets.value.filter(t => t.status !== 'solved').length,
}))

async function sendReply(t: SupportTicket) {
  const body = (drafts.value[t.id] || '').trim()
  if (!body) return
  busy.value = t.id; error.value = ''; message.value = ''
  try {
    const msg = await seller.replySupportTicket(t.id, body)
    drafts.value[t.id] = ''
    if (msg?.id) applyMessage(t.id, msg)
  } catch (e) { error.value = extractApiError(e) }
  finally { busy.value = '' }
}

function openResolve(t: SupportTicket) { resolveTarget.value = t; resolution.value = '' }
async function submitResolve() {
  if (!resolveTarget.value) return
  if (!resolution.value.trim()) { error.value = 'Add a short resolution note.'; return }
  busy.value = resolveTarget.value.id; error.value = ''; message.value = ''
  try {
    await seller.resolveSupportTicket(resolveTarget.value.id, resolution.value.trim())
    message.value = `Ticket ${resolveTarget.value.ticket_number} marked solved.`
    resolveTarget.value = null
    await load()
  } catch (e) { error.value = extractApiError(e) }
  finally { busy.value = '' }
}

function scrollTimeline(id: string) {
  const el = document.getElementById('tl-' + id)
  if (el) el.scrollTop = el.scrollHeight
}

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
function authorLabel(role: string, t: SupportTicket) {
  if (role === 'customer') return t.customer_name || 'Customer'
  if (role === 'seller') return 'You'
  if (role === 'admin') return 'CARLITOS'
  return 'System'
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Customer Issues</h1>
      <p class="text-sm text-stone-500">
        {{ counts.open }} open · Customer issues about your products. Reply and mark solved when done.
      </p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Filters -->
    <div class="mb-4 flex flex-wrap gap-2">
      <button
        v-for="f in (['all','open','in_progress','solved'] as const)" :key="f"
        :class="['rounded-full border px-3 py-1.5 text-xs font-medium', filter === f ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="filter = f"
      >
        {{ f === 'all' ? 'All' : statusLabel(f) }}
      </button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 3" :key="i" class="skeleton h-48 w-full rounded-xl" />
    </div>

    <div v-else-if="!filtered.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <LifeBuoy :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No support requests</p>
      <p class="mt-1 text-xs text-stone-500">Customer issues about your products will appear here.</p>
    </div>

    <div v-else class="space-y-4">
      <div v-for="t in filtered" :key="t.id" class="card overflow-hidden">
        <!-- Header -->
        <div class="flex flex-wrap items-center justify-between gap-2 border-b border-stone-100 px-4 py-3">
          <div class="flex items-center gap-2">
            <p class="text-sm font-semibold">{{ t.ticket_number }}</p>
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(t.status)]">{{ statusLabel(t.status) }}</span>
          </div>
          <p class="text-xs text-stone-500">{{ t.customer_name }} · {{ formatDate(t.created_at) }}</p>
        </div>

        <div class="grid gap-4 p-4 md:grid-cols-2">
          <!-- Left: product + photos -->
          <div>
            <div v-if="t.product_title" class="flex items-center gap-3">
              <div class="h-16 w-14 shrink-0 overflow-hidden rounded-md bg-stone-100">
                <img v-if="t.product_image" :src="resolveMediaUrl(MEDIA_BASE, t.product_image)" :alt="t.product_title" class="h-full w-full object-cover" loading="lazy" />
                <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="18" /></div>
              </div>
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-ink">{{ t.product_title }}</p>
                <p v-if="t.variant_label" class="truncate text-xs text-stone-500">{{ t.variant_label }}</p>
                <p v-if="t.order_number" class="mt-0.5 text-[11px] text-stone-400">Order {{ t.order_number }}</p>
              </div>
            </div>

            <div v-if="t.photos.length" class="mt-3">
              <p class="mb-1 text-xs font-medium text-stone-500">Customer photos</p>
              <div class="flex flex-wrap gap-2">
                <a v-for="(p, i) in t.photos" :key="i" :href="resolveMediaUrl(MEDIA_BASE, p.file_key)" target="_blank" rel="noopener">
                  <img :src="resolveMediaUrl(MEDIA_BASE, p.file_key)" alt="Proof" class="h-16 w-16 rounded-md border border-stone-200 object-cover" loading="lazy" />
                </a>
              </div>
            </div>

            <div v-if="t.status === 'solved' && t.resolution" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2.5">
              <p class="flex items-center gap-1 text-xs font-semibold text-emerald-800"><CheckCircle2 :size="13" /> Resolved</p>
              <p class="mt-0.5 text-xs text-emerald-700">{{ t.resolution }}</p>
            </div>
          </div>

          <!-- Right: conversation + composer -->
          <div class="flex flex-col">
            <p class="mb-2 text-xs font-semibold text-stone-500">Conversation</p>
            <div :id="'tl-' + t.id" class="max-h-56 space-y-2.5 overflow-y-auto pr-1">
              <div v-for="m in t.messages" :key="m.id" class="flex" :class="m.author_role === 'seller' ? 'justify-end' : 'justify-start'">
                <div class="max-w-[85%]">
                  <p class="mb-0.5 text-[10px] font-medium text-stone-400" :class="m.author_role === 'seller' ? 'text-right' : 'text-left'">
                    {{ authorLabel(m.author_role, t) }} · {{ formatDate(m.created_at) }}
                  </p>
                  <div
                    class="whitespace-pre-wrap rounded-2xl px-3 py-1.5 text-sm"
                    :class="m.author_role === 'seller'
                      ? 'rounded-br-sm bg-ink text-white'
                      : (m.author_role === 'system'
                          ? 'rounded-bl-sm bg-stone-100 text-stone-500 text-xs italic'
                          : 'rounded-bl-sm bg-stone-100 text-ink')"
                  >
                    <a v-if="m.image_key" :href="resolveMediaUrl(MEDIA_BASE, m.image_key)" target="_blank" rel="noopener" class="block">
                      <img
                        :src="resolveMediaUrl(MEDIA_BASE, m.image_key)"
                        alt="Shared photo"
                        class="max-h-48 w-full rounded-lg object-cover"
                        :class="m.body ? 'mb-1.5' : ''"
                        loading="lazy"
                      />
                    </a>
                    <template v-if="m.body">{{ m.body }}</template>
                  </div>
                </div>
              </div>
            </div>

            <!-- Composer (unsolved only) -->
            <div v-if="t.status !== 'solved'" class="mt-3">
              <div class="flex items-end gap-2">
                <textarea
                  v-model="drafts[t.id]"
                  rows="1"
                  class="textarea min-h-[42px] flex-1 py-2.5 text-sm"
                  placeholder="Reply to the customer…"
                  @keydown.enter.exact.prevent="sendReply(t)"
                />
                <button class="btn-primary h-[42px] w-[42px] shrink-0 !px-0" :disabled="busy === t.id || !(drafts[t.id] || '').trim()" aria-label="Send" @click="sendReply(t)">
                  <Send :size="15" />
                </button>
              </div>
              <button class="btn-success mt-2 w-full btn-sm" :disabled="busy === t.id" @click="openResolve(t)">
                <CheckCircle2 :size="14" /> Mark as solved
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Resolve modal -->
    <div v-if="resolveTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="resolveTarget = null">
      <div class="w-full max-w-md rounded-2xl bg-white p-5 shadow-lg">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-h4">Mark solved</h3>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="resolveTarget = null"><X :size="18" /></button>
        </div>
        <p class="mb-3 text-sm text-stone-600">{{ resolveTarget.ticket_number }} — this note is emailed to the customer.</p>
        <label class="label">Resolution note</label>
        <textarea v-model="resolution" class="textarea" rows="4" maxlength="1000" placeholder="Explain what happened and how it was resolved…" />
        <div class="mt-4 flex gap-3">
          <button class="btn-ghost flex-1" @click="resolveTarget = null">Cancel</button>
          <button class="btn-success flex-1" :disabled="!!busy" @click="submitResolve">Confirm solved</button>
        </div>
      </div>
    </div>
  </div>
</template>
