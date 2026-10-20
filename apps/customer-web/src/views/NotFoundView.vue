<script setup lang="ts">
import { RouterLink } from 'vue-router'
import BrandLogo from '@/components/BrandLogo.vue'
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center bg-white px-6 text-center">
    <BrandLogo size="md" />
    <p class="mt-6 text-7xl font-semibold text-stone-200">404</p>
    <h1 class="mt-4 text-xl font-semibold text-ink">Page not found</h1>
    <p class="mt-2 text-sm text-stone-500">The page you're looking for doesn't exist or has been moved.</p>
    <RouterLink to="/" class="btn-primary btn-sm mt-6">Back to Home</RouterLink>
  </div>
</template>
