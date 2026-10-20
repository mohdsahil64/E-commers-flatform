<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Truck, RotateCcw, Package, Clock } from 'lucide-vue-next'
import { BUSINESS } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Shipping Policy</span>
    </nav>

    <h1 class="text-h2 mb-2">Shipping Policy</h1>
    <p class="mb-8 text-sm text-stone-500">Delivery timelines, charges, and tracking.</p>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="card p-4">
        <Clock :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Dispatch time</p>
        <p class="mt-0.5 text-sm text-stone-600">Orders are usually dispatched within 1–2 business days of confirmation.</p>
      </div>
      <div class="card p-4">
        <Truck :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Delivery time</p>
        <p class="mt-0.5 text-sm text-stone-600">Most orders are delivered within 3–7 business days, depending on your location and serviceability.</p>
      </div>
      <div class="card p-4">
        <Package :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Order tracking</p>
        <p class="mt-0.5 text-sm text-stone-600">Track every order from <RouterLink to="/orders" class="underline hover:text-ink">My Orders</RouterLink> once the seller ships it.</p>
      </div>
      <div class="card p-4">
        <RotateCcw :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Easy returns</p>
        <p class="mt-0.5 text-sm text-stone-600">Most items can be returned or exchanged within {{ BUSINESS.returnWindowDays }} days of delivery. See our <RouterLink to="/refund" class="underline hover:text-ink">Refund Policy</RouterLink>.</p>
      </div>
    </div>

    <div class="mt-6 space-y-4 text-sm leading-relaxed text-stone-700">
      <section>
        <h2 class="text-h4 mb-2">Shipping charges</h2>
        <p>Delivery charges (if any) and taxes are always calculated and shown on the order summary at checkout. The amount you see before placing the order is the final amount — there are no hidden charges. Free standard delivery is offered on eligible orders, and any applicable charge is clearly displayed before payment.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Delivery area</h2>
        <p>We deliver across serviceable pin codes in India. If your location is not serviceable, it will be indicated at checkout.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Delays</h2>
        <p>Delivery timelines are estimates and may vary due to weather, courier delays, or circumstances beyond our control. For any delay beyond the expected window, contact us at {{ BUSINESS.email }} or {{ BUSINESS.phone }}.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Cash on Delivery</h2>
        <p>COD is available at checkout on eligible orders. Pay in cash when your order is delivered.</p>
      </section>
    </div>
  </article>
</template>
