<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { RotateCcw, XCircle, IndianRupee, Clock } from 'lucide-vue-next'
import { BUSINESS } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Cancellation &amp; Refund Policy</span>
    </nav>

    <h1 class="text-h2 mb-2">Cancellation &amp; Refund Policy</h1>
    <p class="mb-8 text-sm text-stone-500">How cancellations, returns, and refunds work on {{ BUSINESS.brand }}.</p>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="card p-4">
        <XCircle :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Cancellation</p>
        <p class="mt-0.5 text-sm text-stone-600">Cancel free of charge any time before the order is shipped.</p>
      </div>
      <div class="card p-4">
        <RotateCcw :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Return window</p>
        <p class="mt-0.5 text-sm text-stone-600">Request a return or exchange within {{ BUSINESS.returnWindowDays }} days of delivery.</p>
      </div>
      <div class="card p-4">
        <IndianRupee :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Refund amount</p>
        <p class="mt-0.5 text-sm text-stone-600">Eligible refunds are issued to your original payment method.</p>
      </div>
      <div class="card p-4">
        <Clock :size="20" class="mb-2 text-stone-500" />
        <p class="text-sm font-semibold text-ink">Refund time</p>
        <p class="mt-0.5 text-sm text-stone-600">Refunds are processed within {{ BUSINESS.refundProcessingDays }} business days after approval.</p>
      </div>
    </div>

    <div class="mt-6 space-y-6 text-sm leading-relaxed text-stone-700">
      <section>
        <h2 class="text-h4 mb-2">Order cancellation</h2>
        <p>You can cancel your order free of charge at any time before it is shipped, directly from <RouterLink to="/orders" class="underline hover:text-ink">My Orders</RouterLink>. Once an order has been shipped it cannot be cancelled, but you may raise a return after delivery as described below. If you cancel a prepaid order, the full amount is refunded to your original payment method.</p>
      </section>

      <section>
        <h2 class="text-h4 mb-2">Returns &amp; exchanges</h2>
        <p>Most items can be returned or exchanged within <span class="font-medium text-ink">{{ BUSINESS.returnWindowDays }} days of delivery</span>. To be eligible, the item must be unused, unworn, unwashed, and returned with all original tags and packaging intact. Start a request from your order details on <RouterLink to="/orders" class="underline hover:text-ink">My Orders</RouterLink>.</p>
      </section>

      <section>
        <h2 class="text-h4 mb-2">Non-returnable items</h2>
        <p>For hygiene and safety reasons, certain items may be marked non-returnable on the product page (for example innerwear, personal-care items, or items marked final sale). Items damaged due to misuse are not eligible for return.</p>
      </section>

      <section>
        <h2 class="text-h4 mb-2">Refunds</h2>
        <p>Once a returned item is received and inspected, or a cancellation is confirmed, your refund is approved and processed within <span class="font-medium text-ink">{{ BUSINESS.refundProcessingDays }} business days</span>. The amount is credited back to your original payment method (card, UPI, net banking, or wallet). Depending on your bank or provider, it may take a few additional days to reflect in your account.</p>
        <p class="mt-2">For Cash on Delivery orders, refunds are issued to a bank account or UPI ID you provide when raising the return.</p>
      </section>

      <section>
        <h2 class="text-h4 mb-2">Failed or duplicate payments</h2>
        <p>If an amount is deducted but your order is not confirmed, or in case of a duplicate charge, the amount is automatically reversed by the payment gateway, typically within 5–7 business days. If you do not see the reversal, contact us with your order and payment details.</p>
      </section>

      <section>
        <h2 class="text-h4 mb-2">Need help?</h2>
        <p>For any cancellation, return, or refund query, email us at <a :href="`mailto:${BUSINESS.email}`" class="underline hover:text-ink">{{ BUSINESS.email }}</a> or call {{ BUSINESS.phone }}. You can also visit our <RouterLink to="/contact" class="underline hover:text-ink">Contact</RouterLink> page.</p>
      </section>

      <p class="text-xs text-stone-400">Last updated: {{ new Date().getFullYear() }}.</p>
    </div>
  </article>
</template>
