<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AdminLayout from '@/components/AdminLayout.vue'
import RouteLoader from '@/components/RouteLoader.vue'

const route = useRoute()
const bare = computed(() => route.name === 'login')
</script>

<template>
  <RouteLoader />
  <RouterView v-if="bare" />
  <AdminLayout v-else>
    <RouterView />
  </AdminLayout>
</template>
