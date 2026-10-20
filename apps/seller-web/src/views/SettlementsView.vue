<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { Wallet, TrendingUp, Clock, Download } from 'lucide-vue-next'
import type { Settlement } from '@carlitos/types'
import { seller } from '@/api'
import { formatINR, formatDate } from '@carlitos/shared'

const settlements = ref<Settlement[]>([])
const loading     = ref(true)

onMounted(async () => {
  try { settlements.value = (await seller.settlements()) ?? [] }
  catch { settlements.value = [] }
  finally { loading.value = false }
})

const totalGross = computed(() => settlements.value.reduce((a, s) => a + s.gross_amount, 0))
const totalCommission = computed(() => settlements.value.reduce((a, s) => a + s.commission_amount, 0))
const pendingNet = computed(() => settlements.value.filter(s => s.status === 'pending').reduce((a, s) => a + s.net_payable, 0))
const settledNet = computed(() => settlements.value.filter(s => s.status === 'settled').reduce((a, s) => a + s.net_payable, 0))

function statusClass(s: string) {
  if (s === 'settled')    return 'badge-green'
  if (s === 'failed' || s === 'reversed') return 'badge-red'
  if (s === 'processing') return 'badge-blue'
  return 'badge-amber'
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Settlements</h1>
    </div>

    <!-- Summary cards: 2 columns on mobile, 4 on wide screens -->
    <div class="mb-6 grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4">
      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3"><p class="text-label-sm">Total Sales</p><TrendingUp :size="18" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-lg font-semibold sm:text-xl">{{ formatINR(totalGross) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3"><p class="text-label-sm">Commission</p><Wallet :size="18" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-lg font-semibold sm:text-xl">{{ formatINR(totalCommission) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3"><p class="text-label-sm">Pending</p><Clock :size="18" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-lg font-semibold text-amber-600 sm:text-xl">{{ formatINR(pendingNet) }}</p>
      </div>
      <div class="stat">
        <div class="flex items-center justify-between mb-2 sm:mb-3"><p class="text-label-sm">Settled</p><TrendingUp :size="18" class="shrink-0 text-stone-400" /></div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-lg font-semibold text-emerald-600 sm:text-xl">{{ formatINR(settledNet) }}</p>
      </div>
    </div>

    <!-- History -->
    <h2 class="text-h4 mb-3">Settlement History</h2>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 4" :key="i" class="skeleton h-14 w-full rounded-lg" />
    </div>

    <div v-else-if="!settlements.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Wallet :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No settlements yet</p>
      <p class="mt-1 text-xs text-stone-500">Settlements are generated after your orders are delivered.</p>
    </div>

    <div v-else class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Settlement</th>
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
