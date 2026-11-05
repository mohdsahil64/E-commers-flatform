<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { Wallet, Plus, Download } from 'lucide-vue-next'
import type { Settlement, SettlementGenerateResult } from '@carlitos/types'
import { admin } from '@/api'
import { formatINR, formatDate, extractApiError } from '@carlitos/shared'

const settlements = ref<Settlement[]>([])
const loading     = ref(true)
const generating  = ref(false)
const sellerID    = ref('')
const error       = ref('')
const message     = ref('')
const showGen     = ref(false)
const generatingAll = ref(false)
const allResults  = ref<SettlementGenerateResult[] | null>(null)

async function load() {
  settlements.value = await admin.settlements().catch(() => [])
}

async function generate() {
  if (!sellerID.value.trim()) return
  error.value = ''; message.value = ''; generating.value = true
  try {
    const res = await admin.generateSettlement(sellerID.value.trim())
    message.value = `Settlement created: ${res.id}`
    sellerID.value = ''
    showGen.value = false
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { generating.value = false }
}

async function generateAll() {
  error.value = ''; message.value = ''; allResults.value = null; generatingAll.value = true
  try {
    allResults.value = (await admin.generateAllSettlements()) ?? []
    const ok = allResults.value.filter(r => r.settlement_id).length
    message.value = allResults.value.length
      ? `Created ${ok} of ${allResults.value.length} payouts`
      : 'No seller has items ready for payout yet (items are paid 7 days after delivery)'
    await load()
  } catch (e) { error.value = extractApiError(e) } finally { generatingAll.value = false }
}

const totalNet = computed(() => settlements.value.reduce((a, s) => a + s.net_payable, 0))
const pendingNet = computed(() => settlements.value.filter(s => s.status === 'pending').reduce((a, s) => a + s.net_payable, 0))

function statusClass(s: string) {
  if (s === 'settled') return 'badge-green'
  if (['failed','reversed'].includes(s)) return 'badge-red'
  if (s === 'processing') return 'badge-blue'
  return 'badge-amber'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Settlements</h1>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" :disabled="generatingAll" @click="generateAll">
          {{ generatingAll ? 'Generating…' : 'Generate payouts for all sellers' }}
        </button>
        <button class="btn-ghost" @click="showGen = !showGen"><Plus :size="15" /> Generate Settlement</button>
      </div>
    </div>

    <!-- Generate-all result -->
    <div v-if="message || error || allResults" class="card p-4 mb-5">
      <p v-if="message" class="text-sm text-emerald-600">{{ message }}</p>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <ul v-if="allResults?.length" class="mt-2 space-y-1 text-sm">
        <li v-for="r in allResults" :key="r.seller_id" class="flex flex-wrap justify-between gap-2">
          <span class="font-medium">{{ r.store_name }}</span>
          <span v-if="r.settlement_id" class="text-emerald-600">{{ formatINR(r.net_payable ?? 0) }} created</span>
          <span v-else class="text-red-600">Skipped: {{ r.error }}</span>
        </li>
      </ul>
    </div>

    <!-- Generate form -->
    <div v-if="showGen" class="card p-5 mb-5">
      <h2 class="text-h4 mb-3">Generate Seller Settlement</h2>
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="generate">
        <div class="flex-1 min-w-64">
          <label class="label">Seller ID</label>
          <input v-model="sellerID" class="input" placeholder="Paste seller UUID…" />
        </div>
        <button class="btn-primary" :disabled="generating || !sellerID">{{ generating ? 'Generating…' : 'Generate' }}</button>
      </form>
    </div>

    <!-- Summary -->
    <div class="mb-6 grid gap-4 sm:grid-cols-3">
      <div class="stat">
        <p class="text-label-sm mb-2">Total Payable</p>
        <div v-if="loading" class="skeleton h-6 w-24 rounded" />
        <p v-else class="text-xl font-semibold">{{ formatINR(totalNet) }}</p>
      </div>
      <div class="stat">
        <p class="text-label-sm mb-2">Pending</p>
        <div v-if="loading" class="skeleton h-6 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-amber-600">{{ formatINR(pendingNet) }}</p>
      </div>
      <div class="stat">
        <p class="text-label-sm mb-2">Total Records</p>
        <div v-if="loading" class="skeleton h-6 w-12 rounded" />
        <p v-else class="text-xl font-semibold">{{ settlements.length }}</p>
      </div>
    </div>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 6" :key="i" class="skeleton h-12 w-full rounded-lg" />
    </div>

    <div v-else-if="!settlements.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Wallet :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No settlements yet</p>
      <p class="mt-1 text-xs text-stone-500">Generate a settlement for a seller above.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Settlement #</th>
            <th class="hidden md:table-cell">Period</th>
            <th>Gross</th>
            <th class="hidden sm:table-cell">Commission</th>
            <th>Net Payable</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in settlements" :key="s.id">
            <td class="font-medium">{{ s.settlement_number }}</td>
            <td class="hidden md:table-cell text-stone-500 text-xs">{{ formatDate(s.period_start) }} – {{ formatDate(s.period_end) }}</td>
            <td>{{ formatINR(s.gross_amount) }}</td>
            <td class="hidden sm:table-cell text-stone-500">{{ formatINR(s.commission_amount) }}</td>
            <td class="font-semibold">{{ formatINR(s.net_payable) }}</td>
            <td><span :class="['badge', statusClass(s.status)]">{{ s.status }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
