<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { BRAND, BUSINESS } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">About Us</span>
    </nav>

    <h1 class="text-h2 mb-2">About {{ BRAND }}</h1>
    <p class="mb-8 text-sm text-stone-500">Style lives here.</p>

    <div class="space-y-6 text-sm leading-relaxed text-stone-700">
      <p>{{ BRAND }} is a premium multi-vendor fashion &amp; lifestyle marketplace bringing together trusted sellers under one clean, modern storefront. We focus on a simple, honest shopping experience — real products, transparent pricing, and dependable delivery.</p>
      <section>
        <h2 class="text-h4 mb-2">What we stand for</h2>
        <ul class="list-disc space-y-1 pl-5">
          <li>Curated fashion for men, women, boys and girls.</li>
          <li>Fair, transparent pricing with taxes shown up front.</li>
          <li>Easy returns and responsive support.</li>
          <li>A level playing field for independent sellers.</li>
        </ul>
      </section>
      <p>Explore our <RouterLink to="/collections" class="underline hover:text-ink">collections</RouterLink> or start with <RouterLink to="/collections/new-arrivals" class="underline hover:text-ink">new arrivals</RouterLink>.</p>
      <section>
        <h2 class="text-h4 mb-2">Business information</h2>
        <p>{{ BRAND }} is operated by <span class="font-medium text-ink">{{ BUSINESS.legalName }}</span> ({{ BUSINESS.entityType }}).</p>
        <ul class="mt-2 space-y-1">
          <li><span class="font-medium text-ink">Email:</span> {{ BUSINESS.email }}</li>
          <li><span class="font-medium text-ink">Phone:</span> {{ BUSINESS.phone }}</li>
          <li><span class="font-medium text-ink">Address:</span> {{ BUSINESS.addressLine }}</li>
        </ul>
        <p class="mt-2">Have a question? Visit our <RouterLink to="/contact" class="underline hover:text-ink">Contact</RouterLink> page.</p>
      </section>
    </div>
  </article>
</template>
