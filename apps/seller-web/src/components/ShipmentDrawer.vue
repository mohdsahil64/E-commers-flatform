<script setup lang="ts">
// Slide-over with everything about ONE shipment: progress, address, items,
// tracking history and the actions a seller can take (dispatch, deliver, cancel,
// label, refresh). Full screen on phones, a side panel on larger screens.
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import {
  X, Truck, Package, FileText, CheckCircle2, Copy, Check, ExternalLink, RefreshCw, Ban,
  MapPin, Phone, AlertTriangle, Banknote, CreditCard, Loader2,
} from 'lucide-vue-next'
import type { ShipmentDetail } from '@carlitos/types'
import { seller, api } from '@/api'
import {
  formatINR, extractApiError, shipmentStatusLabel, shipmentBadgeClass, shipmentSteps,
  shipmentStepIndex, formatDateTime,
} from '@carlitos/shared'

const props = defineProps<{ id: string | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'changed'): void }>()

const d       = ref<ShipmentDetail | null>(null)
const loading = ref(false)
const busy    = ref('')
const error   = ref('')
const notice  = ref('')
const copied  = ref(false)
const confirm = ref<'deliver' | 'cancel' | null>(null)
const closeBtn = ref<HTMLButtonElement | null>(null)

async function load(silent = false) {
  if (!props.id) return
  if (!silent) { loading.value = true; d.value = null }
  error.value = ''
  try { d.value = await seller.shipment(props.id) }
  catch (e) { error.value = extractApiError(e) }
  finally { loading.value = false }
}

watch(() => props.id, async (id) => {
  notice.value = ''; confirm.value = null; copied.value = false
  document.body.style.overflow = id ? 'hidden' : ''
  if (id) {
    await load()
    await nextTick()
    closeBtn.value?.focus()
  } else {
    d.value = null
  }
}, { immediate: true })

onBeforeUnmount(() => { document.body.style.overflow = '' })

function onKey(e: KeyboardEvent) {
  if (e.key !== 'Escape' || !props.id) return
  if (confirm.value) confirm.value = null
  else emit('close')
}
window.addEventListener('keydown', onKey)
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

/* ── derived state ───────────────────────────────────────────── */
const isSelf    = computed(() => d.value?.ship_type === 'self')
const status    = computed(() => d.value?.status || '')
const terminal  = computed(() => ['delivered', 'cancelled', 'rto'].includes(status.value))
const waiting   = computed(() => ['created', 'label_generated', 'pickup_scheduled'].includes(status.value))
const moving    = computed(() => ['picked_up', 'in_transit', 'out_for_delivery'].includes(status.value))
const steps     = computed(() => shipmentSteps(d.value?.ship_type || 'courier'))
const stepIdx   = computed(() => shipmentStepIndex(d.value?.ship_type || 'courier', status.value))

const canDispatch = computed(() => waiting.value)
const canDeliver  = computed(() => !terminal.value && (isSelf.value || moving.value))
const canCancel   = computed(() => waiting.value)
const canRefresh  = computed(() => !isSelf.value && !terminal.value)

const timeline = computed(() => d.value?.events || [])
const addressLine = computed(() => {
  const x = d.value
  if (!x) return ''
  return [x.ship_line1, x.ship_line2].filter(Boolean).join(', ')
})

/* ── actions ─────────────────────────────────────────────────── */
async function run(key: string, fn: () => Promise<unknown>, ok: string) {
  busy.value = key; error.value = ''; notice.value = ''
  try {
    await fn()
    notice.value = ok
    await load(true)
    emit('changed')
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}

function dispatch() {
  if (!d.value) return
  const id = d.value.id
  return run('dispatch', () => seller.markShipped(id),
    isSelf.value ? 'Marked as dispatched. The customer has been told it is on the way.'
                 : 'Marked as handed over. The customer has been told it is on the way.')
}
function deliver() {
  if (!d.value) return
  const id = d.value.id
  confirm.value = null
  return run('deliver', () => seller.markDelivered(id), 'Marked as delivered. Nice work!')
}
function cancelShipment() {
  if (!d.value) return
  const id = d.value.id
  confirm.value = null
  return run('cancel', () => seller.cancelShipment(id), 'Shipment cancelled. The order is back in Ready to ship.')
}
async function refresh() {
  if (!d.value) return
  busy.value = 'refresh'; error.value = ''; notice.value = ''
  try {
    d.value = await seller.refreshShipment(d.value.id)
    notice.value = 'Tracking updated.'
    emit('changed')
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
async function downloadLabel() {
  if (!d.value) return
  busy.value = 'label'; error.value = ''
  try {
    const blob = await api.getBlob(seller.labelURL(d.value.id))
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = `carlitos-label-${d.value.order_number || d.value.id}.pdf`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
async function copyAwb() {
  if (!d.value?.awb_number) return
  try { await navigator.clipboard?.writeText(d.value.awb_number); copied.value = true; setTimeout(() => { copied.value = false }, 1500) }
  catch { /* clipboard blocked — ignore */ }
}
</script>

<template>
  <teleport to="body">
    <transition name="drawer">
      <div v-if="id" class="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-labelledby="ship-title">
        <div class="absolute inset-0 bg-black/40" @click="emit('close')" />

        <aside class="drawer-panel absolute inset-y-0 right-0 flex w-full max-w-lg flex-col bg-white shadow-xl">
          <!-- Header -->
          <header class="flex items-start justify-between gap-3 border-b border-stone-200 px-4 py-3.5 md:px-5">
            <div class="min-w-0">
              <p class="text-label-sm">Shipment</p>
              <h2 id="ship-title" class="truncate text-base font-semibold text-ink">
                <RouterLink v-if="d" :to="`/orders/${d.order_id}`" class="hover:underline" @click="emit('close')">{{ d.order_number }}</RouterLink>
                <span v-else class="skeleton inline-block h-5 w-28 align-middle" />
              </h2>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <span v-if="d" :class="['badge', shipmentBadgeClass(d.status)]">{{ shipmentStatusLabel(d.status, d.ship_type) }}</span>
              <button ref="closeBtn" type="button" class="rounded-md p-1.5 text-stone-500 hover:bg-stone-100" aria-label="Close" @click="emit('close')">
                <X :size="18" />
              </button>
            </div>
          </header>

          <!-- Body -->
          <div class="flex-1 space-y-5 overflow-y-auto px-4 py-4 md:px-5">
            <div v-if="loading" class="space-y-3">
              <div class="skeleton h-16 w-full rounded-lg" /><div class="skeleton h-28 w-full rounded-lg" /><div class="skeleton h-40 w-full rounded-lg" />
            </div>

            <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700" role="alert">{{ error }}</p>
            <p v-if="notice" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ notice }}</p>

            <template v-if="d">
              <!-- Attention -->
              <div v-if="d.attention && !terminal" class="flex items-start gap-2.5 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                <AlertTriangle :size="16" class="mt-0.5 shrink-0" />
                <div>
                  <p class="font-medium">{{ d.attention }}</p>
                  <p class="mt-0.5 text-xs">
                    <template v-if="waiting && !isSelf">Contact the courier or cancel and book it again.</template>
                    <template v-else-if="waiting && isSelf">Dispatch it or cancel the shipment.</template>
                    <template v-else>Check the tracking history below or refresh tracking.</template>
                  </p>
                </div>
              </div>
              <div v-else-if="status === 'rto'" class="flex items-start gap-2.5 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                <AlertTriangle :size="16" class="mt-0.5 shrink-0" />
                <p>This parcel is coming back to you (RTO). Contact support if you need help with the refund.</p>
              </div>
              <div v-else-if="status === 'cancelled'" class="rounded-lg border border-stone-200 bg-stone-50 p-3 text-sm text-stone-600">
                This shipment was cancelled. If the order is still unshipped it appears again under <strong>Ready to ship</strong>.
              </div>

              <!-- Progress -->
              <section v-if="stepIdx >= 0" aria-label="Shipment progress">
                <ol class="flex items-start">
                  <li v-for="(s, i) in steps" :key="s.key" class="relative flex flex-1 flex-col items-center text-center">
                    <span v-if="i > 0" :class="['absolute right-1/2 top-3 h-0.5 w-full', i <= stepIdx ? 'bg-emerald-500' : 'bg-stone-200']" aria-hidden="true" />
                    <span :class="['relative z-10 flex h-6 w-6 items-center justify-center rounded-full border-2 text-[10px] font-semibold',
                      i < stepIdx ? 'border-emerald-500 bg-emerald-500 text-white' : i === stepIdx ? 'border-ink bg-ink text-white' : 'border-stone-300 bg-white text-stone-400']">
                      <Check v-if="i < stepIdx || (i === stepIdx && status === 'delivered')" :size="12" /><span v-else>{{ i + 1 }}</span>
                    </span>
                    <span :class="['mt-1.5 px-0.5 text-[10px] leading-tight sm:text-[11px]', i === stepIdx ? 'font-semibold text-ink' : 'text-stone-500']">{{ s.label }}</span>
                  </li>
                </ol>
              </section>

              <!-- Shipping details -->
              <section>
                <h3 class="mb-2 text-sm font-semibold text-ink">Shipping details</h3>
                <dl class="grid grid-cols-2 gap-x-4 gap-y-3 rounded-lg border border-stone-200 p-3 text-sm">
                  <div>
                    <dt class="text-label-sm">Shipped via</dt>
                    <dd class="mt-0.5 flex items-center gap-1.5 font-medium text-ink">
                      <component :is="isSelf ? Package : Truck" :size="14" class="text-stone-400" />{{ isSelf ? 'Self-ship' : (d.courier_name || 'Courier') }}
                    </dd>
                  </div>
                  <div>
                    <dt class="text-label-sm">{{ isSelf ? 'Reference' : 'AWB' }}</dt>
                    <dd class="mt-0.5 flex items-center gap-1 font-medium text-ink">
                      <span class="break-all">{{ d.awb_number || '—' }}</span>
                      <button v-if="d.awb_number" type="button" class="shrink-0 text-stone-400 hover:text-ink" :aria-label="copied ? 'Copied' : 'Copy'" @click="copyAwb">
                        <Check v-if="copied" :size="13" class="text-emerald-600" /><Copy v-else :size="13" />
                      </button>
                    </dd>
                  </div>
                  <div>
                    <dt class="text-label-sm">Payment</dt>
                    <dd class="mt-0.5 flex items-center gap-1.5 font-medium text-ink">
                      <component :is="d.payment_method === 'cod' ? Banknote : CreditCard" :size="14" class="text-stone-400" />
                      <span v-if="d.payment_method === 'cod'" class="text-amber-700">Collect {{ formatINR(d.cod_amount) }}</span>
                      <span v-else>Prepaid</span>
                    </dd>
                  </div>
                  <div v-if="!isSelf && d.freight_charge > 0">
                    <dt class="text-label-sm">Courier charge</dt>
                    <dd class="mt-0.5 font-medium text-ink">{{ formatINR(d.freight_charge) }}<span class="block text-[11px] font-normal text-stone-500">deducted from payout</span></dd>
                  </div>
                  <div>
                    <dt class="text-label-sm">Booked</dt>
                    <dd class="mt-0.5 text-ink">{{ formatDateTime(d.created_at) }}</dd>
                  </div>
                  <div>
                    <dt class="text-label-sm">{{ d.delivered_at ? 'Delivered' : 'Dispatched' }}</dt>
                    <dd class="mt-0.5 text-ink">{{ formatDateTime(d.delivered_at || d.shipped_at) }}</dd>
                  </div>
                </dl>
                <a v-if="d.tracking_url" :href="d.tracking_url" target="_blank" rel="noopener noreferrer" class="mt-2 inline-flex items-center gap-1.5 text-sm text-accent hover:underline">
                  <ExternalLink :size="14" /> Open courier tracking page
                </a>
              </section>

              <!-- Deliver to -->
              <section>
                <h3 class="mb-2 flex items-center gap-1.5 text-sm font-semibold text-ink"><MapPin :size="14" class="text-stone-400" /> Deliver to</h3>
                <div class="rounded-lg border border-stone-200 p-3 text-sm">
                  <p class="font-medium text-ink">{{ d.customer_name }}</p>
                  <a v-if="d.contact_phone" :href="`tel:${d.contact_phone}`" class="mt-0.5 inline-flex items-center gap-1 text-stone-600 hover:underline"><Phone :size="12" /> {{ d.contact_phone }}</a>
                  <p class="mt-1 text-stone-600">{{ addressLine }}</p>
                  <p class="text-stone-600">{{ d.ship_city }}, {{ d.ship_state }} – {{ d.ship_pincode }}</p>
                </div>
              </section>

              <!-- Items -->
              <section v-if="d.items.length">
                <h3 class="mb-2 text-sm font-semibold text-ink">Items in this parcel</h3>
                <ul class="divide-y divide-stone-100 rounded-lg border border-stone-200">
                  <li v-for="(it, i) in d.items" :key="i" class="flex items-start justify-between gap-3 p-3 text-sm">
                    <div class="min-w-0">
                      <p class="font-medium text-ink">{{ it.title }}</p>
                      <p class="text-xs text-stone-500">{{ [it.variant, it.sku].filter(Boolean).join(' · ') }}</p>
                    </div>
                    <span class="shrink-0 text-stone-600">× {{ it.quantity }}</span>
                  </li>
                </ul>
              </section>

              <!-- Timeline -->
              <section>
                <h3 class="mb-2 text-sm font-semibold text-ink">Tracking history</h3>
                <p v-if="!timeline.length" class="rounded-lg bg-stone-50 p-3 text-sm text-stone-500">No updates yet.</p>
                <ol v-else class="relative ml-2 space-y-4 border-l border-stone-200 pl-5">
                  <li v-for="(ev, i) in timeline" :key="i" class="relative">
                    <span :class="['absolute -left-[26px] top-1 h-2.5 w-2.5 rounded-full ring-4 ring-white', i === 0 ? 'bg-ink' : 'bg-stone-300']" aria-hidden="true" />
                    <p :class="['text-sm', i === 0 ? 'font-medium text-ink' : 'text-stone-700']">{{ ev.description || shipmentStatusLabel(ev.status) }}</p>
                    <p class="text-xs text-stone-500">{{ formatDateTime(ev.event_time) }}<span v-if="ev.location"> · {{ ev.location }}</span></p>
                  </li>
                </ol>
              </section>
            </template>
          </div>

          <!-- Actions -->
          <footer v-if="d && (canDispatch || canDeliver || canCancel || canRefresh || isSelf)" class="border-t border-stone-200 bg-white px-4 py-3 md:px-5">
            <div class="flex flex-wrap gap-2">
              <button v-if="canDispatch" class="btn-primary flex-1" :disabled="!!busy" @click="dispatch">
                <Loader2 v-if="busy === 'dispatch'" :size="15" class="animate-spin" /><Truck v-else :size="15" />
                {{ isSelf ? 'Mark dispatched' : 'Handed to courier' }}
              </button>
              <button v-if="canDeliver" :class="[canDispatch ? 'btn-ghost' : 'btn-primary', 'flex-1']" :disabled="!!busy" @click="confirm = 'deliver'">
                <Loader2 v-if="busy === 'deliver'" :size="15" class="animate-spin" /><CheckCircle2 v-else :size="15" /> Mark delivered
              </button>
            </div>
            <div v-if="isSelf || canRefresh || canCancel" class="mt-2 flex flex-wrap gap-2">
              <button v-if="isSelf" class="btn-ghost btn-sm" :disabled="!!busy" @click="downloadLabel">
                <Loader2 v-if="busy === 'label'" :size="13" class="animate-spin" /><FileText v-else :size="13" /> Print label
              </button>
              <button v-if="canRefresh" class="btn-ghost btn-sm" :disabled="!!busy" @click="refresh">
                <RefreshCw :size="13" :class="{ 'animate-spin': busy === 'refresh' }" /> Refresh tracking
              </button>
              <button v-if="canCancel" class="btn-ghost btn-sm ml-auto text-red-600" :disabled="!!busy" @click="confirm = 'cancel'">
                <Ban :size="13" /> Cancel shipment
              </button>
            </div>
          </footer>

          <!-- Confirmations -->
          <div v-if="confirm" class="absolute inset-0 z-10 flex items-end justify-center bg-black/40 p-4 md:items-center" role="alertdialog" aria-modal="true" aria-labelledby="ship-confirm-title" @click.self="confirm = null">
            <div class="w-full max-w-sm rounded-xl bg-white p-5 shadow-lg">
              <h3 id="ship-confirm-title" class="text-base font-semibold text-ink">
                {{ confirm === 'deliver' ? 'Mark this order as delivered?' : 'Cancel this shipment?' }}
              </h3>
              <p class="mt-1.5 text-sm text-stone-600">
                <template v-if="confirm === 'deliver'">The customer is told their order was delivered and the return window starts. This can't be undone.</template>
                <template v-else-if="isSelf">The self-ship label becomes invalid. The order goes back to Ready to ship.</template>
                <template v-else>We'll ask the courier to cancel the pickup and release the courier charge. The order goes back to Ready to ship so you can book it again.</template>
              </p>
              <div class="mt-5 flex justify-end gap-2">
                <button class="btn-ghost" @click="confirm = null">Not now</button>
                <button :class="confirm === 'deliver' ? 'btn-primary' : 'btn-danger'" @click="confirm === 'deliver' ? deliver() : cancelShipment()">
                  {{ confirm === 'deliver' ? 'Yes, delivered' : 'Yes, cancel' }}
                </button>
              </div>
            </div>
          </div>
        </aside>
      </div>
    </transition>
  </teleport>
</template>

<style scoped>
.drawer-enter-active, .drawer-leave-active { transition: opacity 180ms ease; }
.drawer-enter-active .drawer-panel, .drawer-leave-active .drawer-panel { transition: transform 220ms cubic-bezier(.2,.8,.2,1); }
.drawer-enter-from, .drawer-leave-to { opacity: 0; }
.drawer-enter-from .drawer-panel, .drawer-leave-to .drawer-panel { transform: translateX(100%); }
@media (prefers-reduced-motion: reduce) {
  .drawer-enter-active, .drawer-leave-active,
  .drawer-enter-active .drawer-panel, .drawer-leave-active .drawer-panel { transition: none; }
}
</style>
