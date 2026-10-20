<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Box, Plus, Check } from 'lucide-vue-next'
import { admin } from '@/api'
import { extractApiError } from '@carlitos/shared'

const items    = ref<any[]>([])
const requests = ref<any[]>([])
const loading  = ref(true)
const error    = ref('')
const success  = ref('')
const busy     = ref('')
const showForm = ref(false)
const saving   = ref(false)
const tab      = ref<'inventory'|'requests'>('inventory')

const newItem = ref({ sku: '', name: '', type: 'polybag', unit: 'piece' })

async function load() {
  [items.value, requests.value] = await Promise.all([
    admin.packagingItems().catch(() => []) as Promise<any[]>,
    admin.packagingRequests().catch(() => []) as Promise<any[]>,
  ])
}

async function createItem() {
  saving.value = true; error.value = ''; success.value = ''
  try {
    await admin.createPackagingItem(newItem.value)
    newItem.value = { sku: '', name: '', type: 'polybag', unit: 'piece' }
    success.value = 'Packaging item created.'
    showForm.value = false
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

async function approve(id: string) {
  busy.value = id; error.value = ''; success.value = ''
  try { await admin.approvePackaging(id); success.value = 'Request approved.'; await load() }
  catch (e) { error.value = extractApiError(e) } finally { busy.value = '' }
}

function statusClass(s: string) {
  if (['approved','issued'].includes(s)) return 'badge-green'
  if (s === 'rejected') return 'badge-red'
  return 'badge-amber'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Packaging</h1>
      <button v-if="tab === 'inventory'" class="btn-primary" @click="showForm = !showForm"><Plus :size="15" /> Add Item</button>
    </div>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Tabs -->
    <div class="mb-5 flex gap-2">
      <button :class="['rounded-full border px-3.5 py-1.5 text-sm', tab === 'inventory' ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600']" @click="tab='inventory'">Inventory</button>
      <button :class="['rounded-full border px-3.5 py-1.5 text-sm', tab === 'requests' ? 'border-ink bg-ink text-white' : 'border-stone-300 text-stone-600']" @click="tab='requests'">
        Requests <span v-if="requests.filter(r=>r.status==='pending').length" class="ml-1 rounded-full bg-amber-100 px-1.5 text-xs text-amber-700">{{ requests.filter(r=>r.status==='pending').length }}</span>
      </button>
    </div>

    <!-- Create form -->
    <div v-if="showForm && tab === 'inventory'" class="card p-5 mb-5 space-y-4">
      <h2 class="text-h4">New Packaging Item</h2>
      <div class="grid gap-4 sm:grid-cols-4">
        <div><label class="label">SKU</label><input v-model="newItem.sku" class="input" required /></div>
        <div><label class="label">Name</label><input v-model="newItem.name" class="input" required /></div>
        <div>
          <label class="label">Type</label>
          <select v-model="newItem.type" class="select">
            <option value="polybag">Polybag</option>
            <option value="courier_bag">Courier Bag</option>
            <option value="box">Box</option>
            <option value="tape">Tape</option>
            <option value="sticker">Sticker</option>
            <option value="shipping_label">Shipping Label</option>
            <option value="packing_slip">Packing Slip</option>
          </select>
        </div>
        <div><label class="label">Unit</label><input v-model="newItem.unit" class="input" /></div>
      </div>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="showForm = false">Cancel</button>
        <button class="btn-primary" :disabled="saving || !newItem.sku || !newItem.name" @click="createItem">{{ saving ? 'Creating…' : 'Create' }}</button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-2">
      <div v-for="i in 5" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <!-- Inventory tab -->
    <template v-else-if="tab === 'inventory'">
      <div v-if="!items.length" class="flex flex-col items-center rounded-xl border border-stone-200 bg-stone-50 py-12">
        <Box :size="40" class="text-stone-300 mb-3" />
        <p class="text-sm font-medium text-stone-700">No packaging items</p>
      </div>
      <div v-else class="table-wrap rounded-lg border border-stone-200">
        <table class="table">
          <thead><tr><th>Item</th><th class="hidden sm:table-cell">Type</th><th>Available</th></tr></thead>
          <tbody>
            <tr v-for="it in items" :key="it.id">
              <td><p class="font-medium">{{ it.name }}</p><p class="text-xs text-stone-400">{{ it.sku }}</p></td>
              <td class="hidden sm:table-cell text-stone-500 capitalize">{{ (it.type||'').replace('_',' ') }}</td>
              <td class="font-medium">{{ it.available_quantity ?? '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>

    <!-- Requests tab -->
    <template v-else>
      <div v-if="!requests.length" class="flex flex-col items-center rounded-xl border border-stone-200 bg-stone-50 py-12">
        <Box :size="40" class="text-stone-300 mb-3" />
        <p class="text-sm font-medium text-stone-700">No packaging requests</p>
      </div>
      <div v-else class="table-wrap rounded-lg border border-stone-200">
        <table class="table">
          <thead><tr><th>Item</th><th>Qty</th><th>Status</th><th class="text-right">Actions</th></tr></thead>
          <tbody>
            <tr v-for="r in requests" :key="r.id">
              <td class="font-medium">{{ r.item_name || '—' }}</td>
              <td>{{ r.requested_quantity }}</td>
              <td><span :class="['badge', statusClass(r.status)]">{{ r.status }}</span></td>
              <td class="text-right">
                <button v-if="r.status === 'pending'" class="btn-success btn-xs" :disabled="busy === r.id" @click="approve(r.id)"><Check :size="12" /> Approve</button>
                <span v-else class="text-xs text-stone-400">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
