<script setup lang="ts">
// Admin slide-over for ONE shipment: who ships it, where it is, what's in it and
// the full tracking history. Read-only except "Refresh tracking" (pulls the
// courier's latest status) — the seller owns the physical hand-over.
import { ref, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { RouterLink } from 'vue-router'
import {
  X, Truck, Package, Copy, Check, ExternalLink, RefreshCw, MapPin, Phone, AlertTriangle,
  Banknote, CreditCard, Store, Loader2,
} from 'lucide-vue-next'
import type { ShipmentDetail } from '@carlitos/types'
import { admin } from '@/api'
import { useAuthStore } from '@/stores/auth'
import {
  formatINR, extractApiError, shipmentStatusLabel, shipmentBadgeClass, shipmentSteps,
  shipmentStepIndex, formatDateTime,
} from '@carlitos/shared'

const props = defineProps<{ id: string | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'changed'): void }>()

const store   = useAuthStore()
const d       = ref<ShipmentDetail | null>(null)
const loading = ref(false)
const busy    = ref(false)
const error   = ref('')
const notice  = ref('')
const copied  = ref(false)
const closeBtn = ref<HTMLButtonElement | null>(null)

async function load() {
  if (!props.id) return
  loading.value = true; d.value = null; error.value = ''
  try { d.value = await admin.shipment(props.id) }
  catch (e) { error.value = extractApiError(e) }
  finally { loading.value = false }
}

watch(() => props.id, async (id) => {
  notice.value = ''; copied.value = false
  document.body.style.overflow = id ? 'hidden' : ''
  if (id) { await load(); await nextTick(); closeBtn.value?.focus() } else { d.value = null }
}, { immediate: true })

onBeforeUnmount(() => { document.body.style.overflow = '' })
function onKey(e: KeyboardEvent) { if (e.key === 'Escape' && props.id) emit('close') }
window.addEventListener('keydown', onKey)
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

const isSelf   = computed(() => d.value?.ship_type === 'self')
const status   = computed(() => d.value?.status || '')
const terminal = computed(() => ['delivered', 'cancelled', 'rto'].includes(status.value))
const steps    = computed(() => shipmentSteps(d.value?.ship_type || 'courier'))
const stepIdx  = computed(() => shipmentStepIndex(d.value?.ship_type || 'courier', status.value))
// Needs order.manage on the server; hide the button for people who lack it.
const canRefresh = computed(() => !!d.value && !isSelf.value && !terminal.value && store.hasPermission('order.manage'))
const timeline   = computed(() => d.value?.events || [])
const addressLine = computed(() => [d.value?.ship_line1, d.value?.ship_line2].filter(Boolean).join(', '))

async function refresh() {
  if (!d.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try {
    d.value = await admin.refreshShipment(d.value.id)
    notice.value = 'Tracking updated from the courier.'
    emit('changed')
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = false }
}
async function copyAwb() {
  if (!d.value?.awb_number) return
  try { await navigator.clipboard?.writeText(d.value.awb_number); copied.value = true; setTimeout(() => { copied.value = false }, 1500) }
  catch { /* clipboard blocked */ }
}
</script>

<template>
  <teleport to="body">
    <transition name="drawer">
      <div v-if="id" class="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-labelledby="adm-ship-title">
        <div class="absolute inset-0 bg-black/40" @click="emit('close')" />

        <aside class="drawer-panel absolute inset-y-0 right-0 flex w-full max-w-lg flex-col bg-white shadow-xl">
          <header class="flex items-start justify-between gap-3 border-b border-stone-200 px-4 py-3.5 md:px-5">
            <div class="min-w-0">
              <p class="text-label-sm">Shipment</p>
              <h2 id="adm-ship-title" class="truncate text-base font-semibold text-ink">
                <RouterLink v-if="d" :to="`/orders/${d.order_id}`" class="hover:underline" @click="emit('close')">{{ d.order_number }}</RouterLink>
                <span v-else class="skeleton inline-block h-5 w-28 align-middle" />
              </h2>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <span v-if="d" :class="['badge', shipmentBadgeClass(d.status)]">{{ shipmentStatusLabel(d.status, d.ship_type) }}</span>
              <button ref="closeBtn" type="button" class="rounded-md p-1.5 text-stone-500 hover:bg-stone-100" aria-label="Close" @click="emit('close')"><X :size="18" /></button>
            </div>
          </header>

          <div class="flex-1 space-y-5 overflow-y-auto px-4 py-4 md:px-5">
            <div v-if="loading" class="space-y-3">
              <div class="skeleton h-16 w-full rounded-lg" /><div class="skeleton h-28 w-full rounded-lg" /><div class="skeleton h-40 w-full rounded-lg" />
            </div>
            <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700" role="alert">{{ error }}</p>
            <p v-if="notice" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ notice }}</p>

            <template v-if="d">
              <div v-if="d.attention && status !== 'delivered' && status !== 'cancelled'" class="flex items-start gap-2.5 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
                <AlertTriangle :size="16" class="mt-0.5 shrink-0" />
                <div>
                  <p class="font-medium">{{ d.attention }}</p>
                  <p class="mt-0.5 text-xs">Contact {{ d.store_name || 'the seller' }}<template v-if="!isSelf"> or the courier</template> to resolve it.</p>
                </div>
              </div>

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

              <section>
                <h3 class="mb-2 text-sm font-semibold text-ink">Shipping details</h3>
                <dl class="grid grid-cols-2 gap-x-4 gap-y-3 rounded-lg border border-stone-200 p-3 text-sm">
                  <div class="col-span-2">
                    <dt class="text-label-sm">Seller</dt>
                    <dd class="mt-0.5 flex items-center gap-1.5 font-medium text-ink"><Store :size="14" class="text-stone-400" />{{ d.store_name || '—' }}</dd>
                  </div>
                  <div>
                    <dt class="text-label-sm">Shipped via</dt>
                    <dd class="mt-0.5 flex items-center gap-1.5 font-medium text-ink"><component :is="isSelf ? Package : Truck" :size="14" class="text-stone-400" />{{ isSelf ? 'Self-ship' : (d.courier_name || 'Courier') }}</dd>
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
                      <span v-if="d.payment_method === 'cod'" class="text-amber-700">COD {{ formatINR(d.cod_amount) }}</span><span v-else>Prepaid</span>
                    </dd>
                  </div>
                  <div v-if="!isSelf">
                    <dt class="text-label-sm">Courier charge</dt>
                    <dd class="mt-0.5 font-medium text-ink">{{ d.freight_charge > 0 ? formatINR(d.freight_charge) : '—' }}</dd>
                  </div>
                  <div><dt class="text-label-sm">Booked</dt><dd class="mt-0.5 text-ink">{{ formatDateTime(d.created_at) }}</dd></div>
                  <div><dt class="text-label-sm">{{ d.delivered_at ? 'Delivered' : 'Dispatched' }}</dt><dd class="mt-0.5 text-ink">{{ formatDateTime(d.delivered_at || d.shipped_at) }}</dd></div>
                </dl>
                <a v-if="d.tracking_url" :href="d.tracking_url" target="_blank" rel="noopener noreferrer" class="mt-2 inline-flex items-center gap-1.5 text-sm text-accent hover:underline">
                  <ExternalLink :size="14" /> Open courier tracking page
                </a>
              </section>

              <section>
                <h3 class="mb-2 flex items-center gap-1.5 text-sm font-semibold text-ink"><MapPin :size="14" class="text-stone-400" /> Deliver to</h3>
                <div class="rounded-lg border border-stone-200 p-3 text-sm">
                  <p class="font-medium text-ink">{{ d.customer_name }}</p>
                  <a v-if="d.contact_phone" :href="`tel:${d.contact_phone}`" class="mt-0.5 inline-flex items-center gap-1 text-stone-600 hover:underline"><Phone :size="12" /> {{ d.contact_phone }}</a>
                  <p class="mt-1 text-stone-600">{{ addressLine }}</p>
                  <p class="text-stone-600">{{ d.ship_city }}, {{ d.ship_state }} – {{ d.ship_pincode }}</p>
                </div>
              </section>

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

          <footer v-if="canRefresh" class="border-t border-stone-200 bg-white px-4 py-3 md:px-5">
            <button class="btn-ghost w-full" :disabled="busy" @click="refresh">
              <Loader2 v-if="busy" :size="15" class="animate-spin" /><RefreshCw v-else :size="15" /> Refresh tracking from courier
            </button>
          </footer>
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
