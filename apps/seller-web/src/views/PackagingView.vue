<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Box, Plus, Package } from 'lucide-vue-next'
import { seller } from '@/api'
import { formatDate, extractApiError } from '@carlitos/shared'

const items    = ref<any[]>([])
const balances = ref<any[]>([])
const requests = ref<any[]>([])
const loading  = ref(true)
const error    = ref('')
const success  = ref('')
const showForm = ref(false)
const saving   = ref(false)
const req      = ref({ packaging_item_id: '', quantity: 1 })

async function load() {
  const [it, ba, rq] = await Promise.all([
    seller.packagingItems().catch(() => []) as Promise<any[]>,
    seller.packagingBalances().catch(() => []) as Promise<any[]>,
    seller.packagingRequests().catch(() => []) as Promise<any[]>,
  ])
  // Backend may return null (no rows) on success — coalesce to [] so the
  // template's .length / v-for never sees null.
  items.value    = it ?? []
  balances.value = ba ?? []
  requests.value = rq ?? []
}

async function submitRequest() {
  error.value = ''; success.value = ''; saving.value = true
  try {
    await seller.requestPackaging(req.value.packaging_item_id, req.value.quantity)
    success.value = 'Packaging request submitted.'
    req.value = { packaging_item_id: '', quantity: 1 }
    showForm.value = false
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

function statusClass(s: string) {
  if (s === 'approved' || s === 'issued') return 'badge-green'
  if (s === 'rejected') return 'badge-red'
  return 'badge-amber'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Packaging</h1>
      <button class="btn-primary w-full sm:w-auto" @click="showForm = !showForm">
        <Plus :size="15" /> Request Packaging
      </button>
    </div>

    <!-- How packaging works -->
    <p class="mb-4 text-xs text-stone-500">
      <span class="font-medium text-stone-600">How it works:</span>
      request packaging material → admin approves &amp; issues it → your balance goes up → each order you pack uses from this balance.
    </p>

    <p v-if="success" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ success }}</p>

    <!-- Request form -->
    <div v-if="showForm" class="card p-5 mb-6 space-y-4">
      <h2 class="text-h4">New Packaging Request</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label">Packaging Item</label>
          <select v-model="req.packaging_item_id" class="select" required>
            <option value="">Select item…</option>
            <option v-for="i in items" :key="i.id" :value="i.id">
              {{ i.name }} <template v-if="i.available_quantity != null">({{ i.available_quantity }} available)</template>
            </option>
          </select>
        </div>
        <div>
          <label class="label">Quantity</label>
          <input v-model.number="req.quantity" type="number" min="1" class="input" />
        </div>
      </div>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="showForm = false">Cancel</button>
        <button class="btn-primary" :disabled="saving || !req.packaging_item_id" @click="submitRequest">
          {{ saving ? 'Submitting…' : 'Submit Request' }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-4">
      <div class="skeleton h-32 w-full rounded-lg" />
      <div class="skeleton h-32 w-full rounded-lg" />
    </div>

    <template v-else>
      <!-- Balances -->
      <section class="mb-8">
        <h2 class="text-h4 mb-3">Available Packaging</h2>
        <div v-if="!balances.length" class="rounded-xl border border-stone-200 bg-stone-50 p-8 text-center">
          <Box :size="36" class="text-stone-300 mx-auto mb-2" />
          <p class="text-sm text-stone-500">No packaging issued yet. Request packaging above.</p>
        </div>
        <div v-else class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="b in balances" :key="b.packaging_item_id" class="stat">
            <p class="text-sm font-medium">{{ b.item_name }}</p>
            <div class="mt-2 flex items-baseline gap-4 text-sm">
              <div><span class="text-stone-400 text-xs">Issued</span><p class="font-semibold">{{ b.issued ?? '—' }}</p></div>
              <div><span class="text-stone-400 text-xs">Used</span><p class="font-semibold">{{ b.used ?? '—' }}</p></div>
              <div><span class="text-stone-400 text-xs">Remaining</span><p class="font-semibold text-emerald-600">{{ b.remaining ?? '—' }}</p></div>
            </div>
          </div>
        </div>
      </section>

      <!-- Request history -->
      <section>
        <h2 class="text-h4 mb-3">Request History</h2>
        <div v-if="!requests.length" class="rounded-xl border border-stone-200 bg-stone-50 p-8 text-center">
          <Package :size="36" class="text-stone-300 mx-auto mb-2" />
          <p class="text-sm text-stone-500">No packaging requests yet.</p>
        </div>
        <div v-else class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Item</th>
                <th>Requested Qty</th>
                <th class="hidden sm:table-cell">Date</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in requests" :key="r.id">
                <td class="font-medium">{{ r.item_name || r.packaging_item_name || '—' }}</td>
                <td>{{ r.requested_quantity }}</td>
                <td class="hidden sm:table-cell text-stone-500 text-xs">{{ formatDate(r.created_at) }}</td>
                <td><span :class="['badge', statusClass(r.status)]">{{ r.status }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>
