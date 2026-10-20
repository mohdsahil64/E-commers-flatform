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
      <span class="font-medium text-stone-700">Privacy Policy</span>
    </nav>

    <h1 class="text-h2 mb-2">Privacy Policy</h1>
    <p class="mb-8 text-sm text-stone-500">How {{ BUSINESS.brand }} collects, uses, and protects your information.</p>

    <div class="space-y-6 text-sm leading-relaxed text-stone-700">
      <section>
        <p>
          This Privacy Policy explains how <span class="font-medium text-ink">{{ BUSINESS.legalName }}</span>
          ({{ BUSINESS.entityType }}), which operates the {{ BUSINESS.brand }} marketplace at
          {{ BUSINESS.domain }} ("we", "us", "our"), collects and handles your personal
          information when you use our website and services.
        </p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Information we collect</h2>
        <p>We collect the details you provide when you create an account and shop with us — your mobile number, name, email (if added), delivery addresses, and order history. During checkout, payment is processed securely by our payment partner; we do not store your full card, UPI, or bank details on our servers. We also collect basic device and usage data to keep the service secure and working well.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">How we use your information</h2>
        <ul class="list-disc space-y-1 pl-5">
          <li>To process orders, payments, deliveries, returns, and refunds.</li>
          <li>To verify your identity via one-time passwords (OTP).</li>
          <li>To send order updates and, with your consent, occasional offers.</li>
          <li>To prevent fraud and keep your account secure.</li>
          <li>To comply with applicable legal and tax obligations.</li>
        </ul>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Payments</h2>
        <p>Online payments are handled by our PCI-DSS compliant payment gateway. Your sensitive payment credentials are entered on the gateway's secure environment and are never stored by us. We only receive confirmation of the payment status for your order.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Sharing</h2>
        <p>We share only what is needed to fulfil your order — for example, your delivery address with the seller and courier, and the order amount with our payment partner. We do not sell your personal data to any third party.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Data retention &amp; security</h2>
        <p>We retain your information for as long as your account is active or as needed to provide services and meet legal requirements. We use reasonable technical and organisational measures to protect your data against unauthorised access.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Your choices</h2>
        <p>You can update your profile, manage addresses, and control notification preferences from your account. To request access to, correction of, or deletion of your data, contact us using the details below.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Contact</h2>
        <p>Questions about privacy? Reach us at <a :href="`mailto:${BUSINESS.email}`" class="underline hover:text-ink">{{ BUSINESS.email }}</a>, call {{ BUSINESS.phone }}, or write to {{ BUSINESS.addressLine }}. See our <RouterLink to="/contact" class="underline hover:text-ink">Contact</RouterLink> page.</p>
      </section>
      <p class="text-xs text-stone-400">Last updated: {{ new Date().getFullYear() }}.</p>
    </div>
  </article>
</template>
