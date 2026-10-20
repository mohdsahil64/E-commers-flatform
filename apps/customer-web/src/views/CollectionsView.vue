<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Layers } from 'lucide-vue-next'
import type { Collection } from '@carlitos/types'
import { pub, MEDIA_BASE } from '@/api'
import { resolveMediaUrl } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'

const collections = ref<Collection[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    collections.value = await pub.collections()
  } catch {
    collections.value = []
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <PageBack />
    <nav class="mb-5 flex items-center gap-1.5 text-xs text-stone-500" aria-label="Breadcrumb">
      <RouterLink to="/" class="hover:text-ink">Home</RouterLink>
      <span class="text-stone-300">/</span>
      <span class="font-medium text-stone-700">Collections</span>
    </nav>

    <h1 class="text-h2 mb-6">Collections</h1>

    <!-- Loading -->
    <div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="i in 6" :key="i" class="skeleton h-40 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!collections.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-16">
      <Layers :size="44" class="mb-4 text-stone-300" />
      <p class="text-base font-medium text-stone-700">No collections yet</p>
      <p class="mt-1 text-sm text-stone-500">New curated collections will appear here.</p>
      <RouterLink to="/" class="btn-primary btn-sm mt-5">Browse products</RouterLink>
    </div>

    <!-- Grid -->
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <RouterLink
        v-for="c in collections"
        :key="c.id"
        :to="`/collections/${c.slug}`"
        class="group relative flex h-40 items-end overflow-hidden rounded-xl bg-stone-800 p-5"
      >
        <img
          v-if="c.image_key"
          :src="resolveMediaUrl(MEDIA_BASE, c.image_key)"
          :alt="c.name"
          class="absolute inset-0 h-full w-full object-cover opacity-70 transition-transform duration-300 group-hover:scale-105"
          loading="lazy"
        />
        <div class="absolute inset-0 bg-gradient-to-t from-black/70 via-black/20 to-transparent" />
        <div class="relative flex w-full items-end justify-between">
          <div class="min-w-0">
            <p class="text-lg font-semibold text-white">{{ c.name }}</p>
            <p v-if="c.description" class="mt-0.5 line-clamp-1 text-xs text-white/70">{{ c.description }}</p>
          </div>
          <ArrowRight :size="18" class="shrink-0 text-white/80 transition-transform group-hover:translate-x-1" />
        </div>
      </RouterLink>
    </div>
  </div>
</template>
