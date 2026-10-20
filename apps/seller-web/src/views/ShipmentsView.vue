<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Truck, Package, FileText, CheckCircle, Copy, ExternalLink } from 'lucide-vue-next'
import { seller, api } from '@/api'
import { formatDate, extractApiError } from '@carlitos/shared'

interface Shipment {
  id: string; order_id: string; order_number?: string; ship_type: string
  awb_number: string; courier_name: string; status: string; tracking_url: string
  provider: string
}

const shipments = ref<Shipment[]>([])
const loading   = ref(true)
const busy      = ref('')
const message   = ref('')
const error     = ref('')
const tab       = ref<'all' | 'active' | 'delivered'>('all')

const DELIVERED = ['delivered']
const ACTIVE    = ['created','label_generated','pickup_scheduled','in_transit','shipped','out_for_delivery']

async function load() {
  loading.value = true
  try { shipments.value = (await seller.shipments() as Shipment[]) ?? [] }
  catch { shipments.value = [] }
  finally { loading.value = false }
}

const filtered = computed(() => shipments.value.filter(s => {
  if (tab.value === 'delivered') return DELIVERED.includes(s.status)
  if (tab.value === 'active')    return !DELIVERED.includes(s.status)
  return true
}))

const activeCount    = computed(() => shipments.value.filter(s => !DELIVERED.includes(s.status)).length)
const deliveredCount = computed(() => shipments.value.filter(s => DELIVERED.includes(s.status)).length)

async function run(id: string, fn: () => Promise<unknown>, ok: string) {
  busy.value = id; error.value = ''; message.value = ''
  try { await fn(); message.value = ok; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
function markShipped(id: string)   { run(id, () => seller.markShipped(id), 'Shipment marked as shipped.') }
function markDelivered(id: string) { run(id, () => seller.markDelivered(id), 'Shipment marked as delivered.') }

async function downloadLabel(s: Shipment) {
  busy.value = s.id; error.value = ''
  try {
    const blob = await api.getBlob(seller.labelURL(s.id))
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = `carlitos-label-${s.order_number || s.id}.pdf`
    document.body.appendChild(a); a.click(); a.remove()
    URL.revokeObjectURL(url)
  } catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}
function copyAwb(awb: string) { if (awb) navigator.clipboard?.writeText(awb) }

function statusClass(s: string) {
  if (DELIVERED.includes(s)) return 'badge-green'
  if (['cancelled','failed','returned'].includes(s)) return 'badge-red'
  return 'badge-blue'
}
function canMarkShipped(s: Shipment) {
  return ['created','label_generated','pickup_scheduled','packed','ready_to_ship'].includes(s.status)
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Shipments</h1>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Tabs -->
    <div class="mb-4 flex gap-2">
      <button
        v-for="t in [{v:'all',l:'All'},{v:'active',l:`Active (${activeCount})`},{v:'delivered',l:`Delivered (${deliveredCount})`}]"
        :key="t.v"
        :class="['flex-1 rounded-full border px-3 py-1.5 text-sm transition-colors sm:flex-none', tab === t.v ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600 hover:border-stone-400']"
        @click="tab = t.v as any"
      >{{ t.l }}</button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-24 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!filtered.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Truck :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No shipments here</p>
      <p class="mt-1 text-xs text-stone-500">Ship an order from its detail page to generate an AWB and label.</p>
    </div>

    <!-- Shipment cards -->
    <div v-else class="space-y-2.5">
      <div v-for="s in filtered" :key="s.id" class="rounded-lg border border-stone-200 bg-white p-3.5">
        <!-- Top: order # + status -->
        <div class="flex items-center justify-between gap-2">
          <RouterLink :to="`/orders/${s.order_id}`" class="truncate font-semibold text-ink hover:underline">
            {{ s.order_number || 'Order' }}
          </RouterLink>
          <span :class="['badge shrink-0 capitalize', statusClass(s.status)]">{{ (s.status || '').replace(/_/g,' ') }}</span>
        </div>

        <!-- Meta: type + AWB -->
        <div class="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-stone-500">
          <span class="inline-flex items-center gap-1">
            <component :is="s.ship_type === 'self' ? Package : Truck" :size="13" />
            {{ s.ship_type === 'self' ? 'Self-Ship' : (s.courier_name || 'Courier') }}
          </span>
          <span v-if="s.awb_number" class="inline-flex items-center gap-1">
            AWB: <span class="font-medium text-ink">{{ s.awb_number }}</span>
            <button class="text-stone-400 hover:text-ink" aria-label="Copy AWB" @click="copyAwb(s.awb_number)"><Copy :size="12" /></button>
          </span>
        </div>

        <!-- Actions -->
        <div class="mt-3 flex flex-wrap gap-2 border-t border-stone-100 pt-3">
          <button v-if="s.ship_type === 'self'" class="btn-ghost btn-sm" :disabled="busy === s.id" @click="downloadLabel(s)">
            <FileText :size="13" /> Label
          </button>
          <a v-if="s.tracking_url" :href="s.tracking_url" target="_blank" rel="noopener" class="btn-ghost btn-sm">
            <ExternalLink :size="13" /> Track
          </a>
          <button v-if="canMarkShipped(s)" class="btn-ghost btn-sm" :disabled="busy === s.id" @click="markShipped(s.id)">
            <Truck :size="13" /> Mark Shipped
          </button>
          <button v-if="!DELIVERED.includes(s.status)" class="btn-primary btn-sm" :disabled="busy === s.id" @click="markDelivered(s.id)">
            <CheckCircle :size="13" /> Mark Delivered
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
