<script setup lang="ts">
import { Truck } from 'lucide-vue-next'
import { formatINR } from '@carlitos/shared'

defineProps<{
  subtotal: number
  total: number
  coupon?: { code: string; discount: number } | null
}>()
</script>

<template>
  <section class="card p-3">
    <h2 class="mb-2 text-xs font-semibold text-stone-700">Price summary</h2>
    <div class="space-y-1.5 text-xs">
      <div class="flex justify-between">
        <span class="text-stone-500">Subtotal</span>
        <span class="font-medium">{{ formatINR(subtotal) }}</span>
      </div>
      <div v-if="coupon" class="flex justify-between">
        <span class="text-stone-500">Coupon discount</span>
        <span class="font-medium text-emerald-600">− {{ formatINR(coupon.discount) }}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-stone-500">Shipping</span>
        <span class="font-medium text-emerald-600">FREE</span>
      </div>
      <div class="flex justify-between border-t border-stone-100 pt-1.5 text-sm font-semibold">
        <span>Total</span>
        <span>{{ formatINR(total) }}</span>
      </div>
      <p class="flex items-start gap-1 pt-0.5 text-[11px] text-stone-400">
        <Truck :size="12" class="mt-0.5 shrink-0" />
        Taxes are calculated at order confirmation.
      </p>
    </div>
  </section>
</template>
