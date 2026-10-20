<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { User } from 'lucide-vue-next'
import type { TeamMember } from '@carlitos/types'
import { pub, MEDIA_BASE } from '@/api'
import { resolveMediaUrl } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'
import { useSeo } from '@/composables/useSeo'

useSeo({ title: 'Team & Management', description: 'Meet the people leading CARLITOS.' })

const members = ref<TeamMember[]>([])
const loading = ref(true)

onMounted(async () => {
  members.value = await pub.team().catch(() => [])
  loading.value = false
})
</script>

<template>
  <div class="mx-auto max-w-4xl">
    <PageBack history />
    <h1 class="text-h2 mb-1">Team &amp; Management</h1>
    <p class="mb-6 text-sm text-stone-500">Meet the people leading CARLITOS.</p>

    <!-- Loading -->
    <div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <div v-for="i in 6" :key="i" class="skeleton h-64 w-full rounded-xl" />
    </div>

    <!-- Empty -->
    <div v-else-if="!members.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12 text-center">
      <User :size="36" class="mb-3 text-stone-300" />
      <p class="text-sm font-medium text-stone-700">Team details coming soon.</p>
    </div>

    <!-- Grid -->
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <article v-for="m in members" :key="m.id" class="card overflow-hidden">
        <div class="aspect-square w-full bg-stone-100">
          <img
            v-if="m.photo_key"
            :src="resolveMediaUrl(MEDIA_BASE, m.photo_key)"
            :alt="m.name"
            class="h-full w-full object-cover"
            loading="lazy"
          />
          <div v-else class="flex h-full w-full items-center justify-center">
            <div class="flex h-16 w-16 items-center justify-center rounded-full bg-stone-900">
              <User :size="28" class="text-white" />
            </div>
          </div>
        </div>
        <div class="p-4">
          <p class="font-semibold text-ink break-words">{{ m.name }}</p>
          <p class="mt-0.5 text-sm font-medium" style="color: #ff3f6c">{{ m.role }}</p>
          <p v-if="m.bio" class="mt-2 text-sm leading-relaxed text-stone-500 break-words">{{ m.bio }}</p>
        </div>
      </article>
    </div>
  </div>
</template>
