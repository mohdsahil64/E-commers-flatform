<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import RouteLoader from '@/components/RouteLoader.vue'

const route = useRoute()
// These routes render without AppLayout (full-page standalone screens)
const bareRoutes = ['login', 'not-found']
const isBare = computed(() => bareRoutes.includes(route.name as string))
</script>

<template>
  <!-- Centered circular loader — reflects real page-load speed, shown only on
       route switches (not on add-to-cart / wishlist actions). -->
  <RouteLoader />

  <template v-if="isBare">
    <RouterView />
  </template>
  <AppLayout v-else>
    <RouterView />
  </AppLayout>
</template>
