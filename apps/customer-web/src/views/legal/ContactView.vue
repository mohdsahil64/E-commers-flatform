<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Mail, LifeBuoy, Phone, MapPin, Clock } from 'lucide-vue-next'
import { BUSINESS } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Contact</span>
    </nav>

    <h1 class="text-h2 mb-2">Contact Us</h1>
    <p class="mb-8 text-sm text-stone-500">We're here to help with orders, returns, payments, and anything else.</p>

    <div class="grid gap-3 sm:grid-cols-2">
      <a :href="`mailto:${BUSINESS.email}`" class="card card-hover flex items-start gap-3 p-4">
        <Mail :size="20" class="mt-0.5 text-stone-500" />
        <span>
          <span class="block text-sm font-semibold text-ink">Email support</span>
          <span class="block text-sm text-stone-600">{{ BUSINESS.email }}</span>
        </span>
      </a>
      <a :href="`tel:${BUSINESS.phoneHref}`" class="card card-hover flex items-start gap-3 p-4">
        <Phone :size="20" class="mt-0.5 text-stone-500" />
        <span>
          <span class="block text-sm font-semibold text-ink">Phone support</span>
          <span class="block text-sm text-stone-600">{{ BUSINESS.phone }}</span>
        </span>
      </a>
      <RouterLink to="/support" class="card card-hover flex items-start gap-3 p-4">
        <LifeBuoy :size="20" class="mt-0.5 text-stone-500" />
        <span>
          <span class="block text-sm font-semibold text-ink">Help Centre</span>
          <span class="block text-sm text-stone-600">Browse FAQs and self-service help.</span>
        </span>
      </RouterLink>
      <div class="card flex items-start gap-3 p-4">
        <MapPin :size="20" class="mt-0.5 text-stone-500" />
        <span>
          <span class="block text-sm font-semibold text-ink">Business address</span>
          <span class="block text-sm text-stone-600">{{ BUSINESS.addressLine }}</span>
        </span>
      </div>
    </div>

    <div class="mt-6 space-y-4 text-sm leading-relaxed text-stone-700">
      <section>
        <h2 class="text-h4 mb-2">Business details</h2>
        <p>
          {{ BUSINESS.brand }} is operated by <span class="font-medium text-ink">{{ BUSINESS.legalName }}</span>
          ({{ BUSINESS.entityType }}).
        </p>
        <ul class="mt-2 space-y-1">
          <li><span class="font-medium text-ink">Email:</span> {{ BUSINESS.email }}</li>
          <li><span class="font-medium text-ink">Phone:</span> {{ BUSINESS.phone }}</li>
          <li><span class="font-medium text-ink">Address:</span> {{ BUSINESS.addressLine }}</li>
        </ul>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Order help</h2>
        <p>For questions about a specific order, open it from <RouterLink to="/orders" class="underline hover:text-ink">My Orders</RouterLink> — you can track, cancel, return, or review items right there.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2 flex items-center gap-1.5"><Clock :size="16" class="text-stone-400" /> Response time</h2>
        <p>We typically respond to emails and calls within {{ BUSINESS.supportResponseHours }} hours on business days (Monday to Saturday).</p>
      </section>
    </div>
  </article>
</template>
