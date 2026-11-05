<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Hourglass, CalendarClock, CircleCheck } from 'lucide-vue-next'
import type { SettlementSummary } from '@carlitos/types'
import { seller } from '@/api'
import { formatINR, formatDate } from '@carlitos/shared'

// "When will I get paid" boxes. `compact` is used on the dashboard.
defineProps<{ compact?: boolean }>()

const summary = ref<SettlementSummary | null>(null)
const loading = ref(true)

onMounted(async () => {
  try { summary.value = await seller.settlementSummary() }
  catch { summary.value = null }
  finally { loading.value = false }
})

function plural(n: number) { return `${n} item${n === 1 ? '' : 's'}` }
</script>

<template>
  <div>
    <div class="mb-2 flex items-center justify-between gap-2">
      <p class="text-xs font-semibold uppercase tracking-wide text-stone-400">Your payouts</p>
      <RouterLink v-if="compact" to="/settlements" class="shrink-0 text-xs text-stone-500 hover:text-ink">Details</RouterLink>
    </div>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
      <div class="stat min-w-0">
        <div class="mb-2 flex items-center justify-between sm:mb-3">
          <p class="text-label-sm truncate">Processing</p>
          <Hourglass :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-ink">{{ formatINR(summary?.processing_amount ?? 0) }}</p>
        <p class="mt-1 text-xs text-stone-500">
          {{ plural(summary?.processing_count ?? 0) }} not delivered yet
        </p>
      </div>

      <div class="stat min-w-0">
        <div class="mb-2 flex items-center justify-between sm:mb-3">
          <p class="text-label-sm truncate">Upcoming payout</p>
          <CalendarClock :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-amber-600">{{ formatINR(summary?.upcoming_amount ?? 0) }}</p>
        <p class="mt-1 text-xs text-stone-500">
          <template v-if="summary?.upcoming_count">
            {{ plural(summary.upcoming_count) }} delivered<template v-if="summary.next_payout_date"> · next payout from {{ formatDate(summary.next_payout_date) }}</template>
          </template>
          <template v-else>No delivered items waiting</template>
        </p>
      </div>

      <div class="stat min-w-0">
        <div class="mb-2 flex items-center justify-between sm:mb-3">
          <p class="text-label-sm truncate">Paid</p>
          <CircleCheck :size="18" class="shrink-0 text-stone-400" />
        </div>
        <div v-if="loading" class="skeleton h-7 w-24 rounded" />
        <p v-else class="text-xl font-semibold text-emerald-600">{{ formatINR(summary?.paid_amount ?? 0) }}</p>
        <p class="mt-1 break-words text-xs text-stone-500">
          <template v-if="summary?.last_paid_at">
            Last paid {{ formatDate(summary.last_paid_at) }}<template v-if="summary.last_utr"> · UTR {{ summary.last_utr }}</template>
          </template>
          <template v-else>No payouts yet</template>
        </p>
      </div>
    </div>
    <p class="mt-2 text-xs text-stone-500">
      Paid {{ summary?.hold_days ?? 7 }} days after delivery, after commission and delivery charge.
    </p>
  </div>
</template>
