<script setup lang="ts">
import { RouterLink } from 'vue-router'
import PageBack from '@/components/PageBack.vue'
</script>

<template>
  <article class="mx-auto max-w-3xl">
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Cookie Policy</span>
    </nav>

    <h1 class="text-h2 mb-2">Cookie Policy</h1>
    <p class="mb-8 text-sm text-stone-500">How CARLITOS uses cookies and local storage.</p>

    <div class="space-y-6 text-sm leading-relaxed text-stone-700">
      <section>
        <h2 class="text-h4 mb-2">What we store</h2>
        <p>CARLITOS uses your browser's local storage to keep you signed in and to remember your cart and wishlist before you log in. These are essential for the store to work.</p>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Types</h2>
        <ul class="list-disc space-y-1 pl-5">
          <li><span class="font-medium">Essential:</span> session tokens, guest cart, guest wishlist.</li>
          <li><span class="font-medium">Preferences:</span> notification settings you choose.</li>
        </ul>
      </section>
      <section>
        <h2 class="text-h4 mb-2">Managing cookies</h2>
        <p>You can clear local storage from your browser settings at any time. Doing so will sign you out and clear a guest cart or wishlist.</p>
      </section>
      <p class="text-xs text-stone-400">Last updated: {{ new Date().getFullYear() }}.</p>
    </div>
  </article>
</template>
