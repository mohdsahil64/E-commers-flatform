<script setup lang="ts">
import { onMounted, ref, computed, nextTick } from 'vue'
import {
  LifeBuoy, Package, X, CheckCircle2, Clock, Eye, Send, Store, ShieldCheck, Lock,
} from 'lucide-vue-next'
import type { SupportTicket } from '@carlitos/types'
import { admin, MEDIA_BASE } from '@/api'
import { formatDate, resolveMediaUrl, extractApiError } from '@carlitos/shared'

const tickets = ref<SupportTicket[]>([])
const loading = ref(true)
const error   = ref('')
const message = ref('')

// Scope: 'mine' = admin-owned (non-product) tickets; 'all' additionally shows
// the read-only product tickets handled by sellers (for auditing).
const scope  = ref<'mine' | 'all'>('mine')
const status = ref<'all' | 'open' | 'in_progress' | 'solved'>('all')

const detail   = ref<SupportTicket | null>(null)
const replyBody = ref('')
const replying  = ref(false)
const resolveOpen = ref(false)
const resolution  = ref('')
const resolving   = ref(false)
const timelineEl  = ref<HTMLElement | null>(null)

async function load() {
  loading.value = true
  try {
    const params: { scope?: 'all'; status?: string } = {}
    if (scope.value === 'all') params.scope = 'all'
    if (status.value !== 'all') params.status = status.value
    tickets.value = await admin.supportTickets(params)
    if (detail.value) {
      const fresh = tickets.value.find(t => t.id === detail.value!.id)
      detail.value = fresh ?? null
    }
  } catch (e) {
    error.value = extractApiError(e)
    tickets.value = []
  } finally {
    loading.value = false
  }
}
onMounted(load)

function setScope(s: 'mine' | 'all') { scope.value = s; load() }
function setStatus(s: typeof status.value) { status.value = s; load() }

const openCount = computed(() => tickets.value.filter(t => t.status !== 'solved').length)

// Admin may only act on admin-assigned tickets. Product (seller) tickets are
// read-only here — the server enforces this too.
function canAct(t: SupportTicket) { return t.assignee === 'admin' }

function openDetail(t: SupportTicket) {
  detail.value = t
  replyBody.value = ''
  resolveOpen.value = false
  resolution.value = ''
  nextTick(scrollTimeline)
}
function scrollTimeline() {
  const el = timelineEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function sendReply() {
  if (!detail.value || !replyBody.value.trim()) return
  replying.value = true; error.value = ''
  try {
    await admin.replySupportTicket(detail.value.id, replyBody.value.trim())
    replyBody.value = ''
    await load()
    nextTick(scrollTimeline)
  } catch (e) { error.value = extractApiError(e) }
  finally { replying.value = false }
}

async function submitResolve() {
  if (!detail.value) return
  if (!resolution.value.trim()) { error.value = 'Add a short resolution note.'; return }
  resolving.value = true; error.value = ''
  try {
    await admin.resolveSupportTicket(detail.value.id, resolution.value.trim())
    message.value = `Ticket ${detail.value.ticket_number} marked solved.`
    resolveOpen.value = false
    resolution.value = ''
    await load()
  } catch (e) { error.value = extractApiError(e) }
  finally { resolving.value = false }
}

const categoryLabelMap: Record<string, string> = {
  product: 'Product / Order', payment: 'Payment', refund: 'Refund',
  login: 'Login / Access', account: 'Account', other: 'Other',
}
function catLabel(c: string) { return categoryLabelMap[c] || 'Help' }

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
  if (role === 'seller') return t.seller_name || 'Seller'
  if (role === 'admin') return 'Support Team'
  return 'System'
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Help &amp; Support</h1>
      <p class="text-sm text-stone-500">
        Handle customer requests for payment, login, account and other issues.
        Product issues are handled by sellers — shown here read-only for auditing.
      </p>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Scope + status filters -->
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <div class="flex gap-2">
        <button
          v-for="s in (['mine','all'] as const)" :key="s"
          :class="['rounded-full border px-3 py-1.5 text-xs font-medium', scope === s ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
          @click="setScope(s)"
        >
          {{ s === 'mine' ? `My queue${openCount && scope === 'mine' ? ' ('+openCount+')' : ''}` : 'All (incl. seller)' }}
        </button>
      </div>
      <span class="mx-1 hidden h-4 w-px bg-stone-200 sm:block" />
      <div class="flex flex-wrap gap-2">
        <button
          v-for="f in (['all','open','in_progress','solved'] as const)" :key="f"
          :class="['rounded-full border px-3 py-1.5 text-xs font-medium', status === f ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
          @click="setStatus(f)"
        >
          {{ f === 'all' ? 'All' : statusLabel(f) }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-16 w-full rounded-lg" />
    </div>

    <div v-else-if="!tickets.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <LifeBuoy :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No tickets</p>
      <p class="mt-1 text-xs text-stone-500">Customer help requests will appear here.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Ticket #</th>
            <th class="hidden md:table-cell">Customer</th>
            <th>Category</th>
            <th class="hidden lg:table-cell">Handled by</th>
            <th>Status</th>
            <th class="hidden sm:table-cell">Raised</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="t in tickets" :key="t.id">
            <td class="font-medium">{{ t.ticket_number }}</td>
            <td class="hidden md:table-cell text-stone-500">{{ t.customer_name }}</td>
            <td class="text-stone-600">{{ catLabel(t.category) }}</td>
            <td class="hidden lg:table-cell">
              <span class="inline-flex items-center gap-1 text-xs text-stone-500">
                <component :is="t.assignee === 'seller' ? Store : ShieldCheck" :size="12" />
                {{ t.assignee === 'seller' ? (t.seller_name || 'Seller') : 'Admin' }}
              </span>
            </td>
            <td><span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(t.status)]">{{ statusLabel(t.status) }}</span></td>
            <td class="hidden sm:table-cell text-xs text-stone-500">{{ formatDate(t.created_at) }}</td>
            <td class="text-right">
              <button class="btn-ghost btn-xs" @click="openDetail(t)">
                <component :is="canAct(t) ? Eye : Lock" :size="12" /> {{ canAct(t) ? 'Open' : 'View' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- ── Detail drawer ─────────────────────────────────── -->
    <div v-if="detail" class="fixed inset-0 z-50 flex justify-end bg-black/40" @click.self="detail = null">
      <div class="flex h-full w-full max-w-lg flex-col bg-white shadow-xl">
        <div class="flex items-center justify-between border-b border-stone-200 px-5 py-4">
          <div class="min-w-0">
            <h3 class="text-h4 truncate">{{ detail.subject }}</h3>
            <p class="truncate text-xs text-stone-500">#{{ detail.ticket_number }} · {{ detail.customer_name }} · {{ catLabel(detail.category) }}</p>
          </div>
          <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="detail = null"><X :size="18" /></button>
        </div>

        <div class="flex-1 space-y-4 overflow-y-auto p-5">
          <div class="flex flex-wrap items-center gap-2">
            <span :class="['rounded-full px-2.5 py-0.5 text-xs font-medium', statusChip(detail.status)]">{{ statusLabel(detail.status) }}</span>
            <span class="inline-flex items-center gap-1 rounded-full bg-stone-100 px-2.5 py-0.5 text-xs text-stone-600">
              <component :is="detail.assignee === 'seller' ? Store : ShieldCheck" :size="11" />
              {{ detail.assignee === 'seller' ? (detail.seller_name || 'Seller') : 'Admin' }}
            </span>
          </div>

          <!-- Read-only notice for seller tickets -->
          <div v-if="!canAct(detail)" class="flex items-start gap-2 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2.5 text-xs text-stone-600">
            <Lock :size="14" class="mt-0.5 shrink-0 text-stone-400" />
            <span>This is a product ticket handled by the seller. You can view the full conversation but can't reply or resolve it.</span>
          </div>

          <!-- Product context -->
          <div v-if="detail.product_title" class="flex items-center gap-3">
            <div class="h-14 w-12 shrink-0 overflow-hidden rounded-md bg-stone-100">
              <img v-if="detail.product_image" :src="resolveMediaUrl(MEDIA_BASE, detail.product_image)" :alt="detail.product_title" class="h-full w-full object-cover" />
              <div v-else class="flex h-full w-full items-center justify-center text-stone-300"><Package :size="16" /></div>
            </div>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-ink">{{ detail.product_title }}</p>
              <p v-if="detail.order_number" class="text-xs text-stone-400">Order {{ detail.order_number }}</p>
            </div>
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

          <!-- Conversation timeline -->
          <div>
            <p class="mb-2 text-xs font-semibold text-stone-500">Conversation</p>
            <div ref="timelineEl" class="max-h-[22rem] space-y-3 overflow-y-auto pr-1">
              <div v-for="m in detail.messages" :key="m.id" class="flex" :class="m.author_role === 'customer' ? 'justify-start' : 'justify-end'">
                <div class="max-w-[85%]">
                  <p class="mb-0.5 text-[10px] font-medium text-stone-400" :class="m.author_role === 'customer' ? 'text-left' : 'text-right'">
                    {{ authorLabel(m.author_role, detail) }} · {{ formatDate(m.created_at) }}
                  </p>
                  <div
                    class="whitespace-pre-wrap rounded-2xl px-3.5 py-2 text-sm"
                    :class="m.author_role === 'customer'
                      ? 'rounded-bl-sm bg-stone-100 text-ink'
                      : (m.author_role === 'system'
                          ? 'rounded-bl-sm bg-stone-100 text-stone-500 text-xs italic'
                          : 'rounded-br-sm bg-ink text-white')"
                  >
                    {{ m.body }}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Resolution banner -->
          <div v-if="detail.status === 'solved' && detail.resolution" class="flex items-start gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3">
            <CheckCircle2 :size="16" class="mt-0.5 shrink-0 text-emerald-600" />
            <div>
              <p class="text-xs font-semibold text-emerald-800">Resolved</p>
              <p class="mt-0.5 text-xs text-emerald-700">{{ detail.resolution }}</p>
            </div>
          </div>
        </div>

        <!-- Footer: reply + resolve (admin-owned, unsolved only) -->
        <div v-if="canAct(detail) && detail.status !== 'solved'" class="border-t border-stone-200 p-4">
          <div v-if="!resolveOpen">
            <div class="flex items-end gap-2">
              <textarea
                v-model="replyBody"
                rows="1"
                class="textarea min-h-[42px] flex-1 py-2.5 text-sm"
                placeholder="Reply to the customer…"
                @keydown.enter.exact.prevent="sendReply"
              />
              <button class="btn-primary h-[42px] w-[42px] shrink-0 !px-0" :disabled="replying || !replyBody.trim()" aria-label="Send" @click="sendReply">
                <Send :size="15" />
              </button>
            </div>
            <button class="btn-success mt-2 w-full btn-sm" @click="resolveOpen = true">
              <CheckCircle2 :size="14" /> Mark as solved
            </button>
          </div>

          <div v-else>
            <label class="label">Resolution note</label>
            <textarea v-model="resolution" rows="3" class="textarea text-sm" placeholder="Explain what happened and how it was resolved (sent to the customer by email)." />
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
