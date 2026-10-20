<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import DeskLayout from '@/components/DeskLayout.vue'
import RouteLoader from '@/components/RouteLoader.vue'

const route = useRoute()
// Auth pages render without the desk chrome (full-screen split layout).
const AUTH_PAGES = new Set(['login', 'register', 'forgot-password', 'reset-password'])
const bare = computed(() => AUTH_PAGES.has(String(route.name)))
</script>

<template>
  <RouteLoader />
  <RouterView v-if="bare" />
  <DeskLayout v-else>
    <RouterView />
  </DeskLayout>
</template>
