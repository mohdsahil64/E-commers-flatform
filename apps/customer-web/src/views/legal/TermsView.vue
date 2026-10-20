<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { BUSINESS } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Terms &amp; Conditions</span>
    </nav>

    <h1 class="text-h2 mb-2">Terms &amp; Conditions</h1>
    <p class="mb-8 text-sm text-stone-500">The rules for using the {{ BUSINESS.brand }} marketplace.</p>

    <div class="space-y-6 text-sm leading-relaxed text-stone-700">
      <section>
        <p>
          These Terms &amp; Conditions govern your use of the {{ BUSINESS.brand }} marketplace at
          {{ BUSINESS.domain }}, operated by <span class="font-medium text-ink">{{ BUSINESS.legalName }}</span>
          ({{ BUSINESS.entityType }}). By browsing or purchasing on {{ BUSINESS.brand }} you agree to these terms.
        </p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Your account</h2>
        <p>You must provide accurate information and keep your account secure. You are responsible for all activity under your account. You must be at least 18 years old, or use the platform under the supervision of a parent or guardian.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Orders &amp; pricing</h2>
        <p>All prices are shown in Indian Rupees (INR) and include applicable taxes at checkout. An order is confirmed once payment is authorised (or accepted for Cash on Delivery). We may cancel an order if an item is unavailable, a pricing error occurs, or the order is flagged for fraud, and any amount paid will be refunded in full.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Payments</h2>
        <p>Online payments are processed through a secure, PCI-DSS compliant payment gateway. By placing an order you authorise us to charge the applicable amount through your chosen payment method.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Sellers &amp; products</h2>
        <p>Products are sold by independent sellers on the marketplace. Product descriptions and stock are provided by the seller. {{ BUSINESS.brand }} facilitates the transaction and support, but the contract of sale is between you and the seller.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Cancellations, returns &amp; refunds</h2>
        <p>Cancellations, returns and refunds are governed by our <RouterLink to="/refund" class="underline hover:text-ink">Cancellation &amp; Refund Policy</RouterLink>. Delivery timelines and charges are explained in our <RouterLink to="/shipping" class="underline hover:text-ink">Shipping Policy</RouterLink>.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Prohibited use</h2>
        <p>You may not misuse the platform, attempt to disrupt the service, or infringe others' rights. We may suspend accounts that violate these terms.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Governing law</h2>
        <p>These terms are governed by the laws of India. Any disputes are subject to the jurisdiction of the courts at Mewat, Haryana.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Questions</h2>
        <p>See our <RouterLink to="/privacy" class="underline hover:text-ink">Privacy Policy</RouterLink> and <RouterLink to="/shipping" class="underline hover:text-ink">Shipping Policy</RouterLink>, or <RouterLink to="/contact" class="underline hover:text-ink">contact us</RouterLink> at {{ BUSINESS.email }}.</p>
      </section>
      <p class="text-xs text-stone-400">Last updated: {{ new Date().getFullYear() }}.</p>
    </div>
  </article>
</template>
