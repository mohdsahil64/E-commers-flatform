<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Bell, Check } from 'lucide-vue-next'
import type { AppNotification } from '@carlitos/types'
import { customer } from '@/api'
import { formatDate } from '@carlitos/shared'
import PageBack from '@/components/PageBack.vue'

const notifs  = ref<AppNotification[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    notifs.value = await customer.notifications()
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="mx-auto max-w-xl">
    <PageBack history />
    <h1 class="text-h2 mb-6">Notifications</h1>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 5" :key="i" class="skeleton h-16 w-full rounded-lg" />
    </div>

    <div v-else-if="!notifs.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <Bell :size="36" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No notifications</p>
      <p class="mt-1 text-xs text-stone-500">We'll notify you about your orders and account activity.</p>
    </div>

    <div v-else class="space-y-1">
      <div
        v-for="n in notifs"
        :key="n.id"
        :class="['flex items-start gap-3 rounded-lg border p-4 transition-colors', n.is_read ? 'border-stone-200 bg-white' : 'border-stone-300 bg-stone-50']"
      >
        <div :class="['flex h-8 w-8 shrink-0 items-center justify-center rounded-full', n.is_read ? 'bg-stone-100' : 'bg-ink']">
          <Bell :size="14" :class="n.is_read ? 'text-stone-400' : 'text-white'" />
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-medium text-ink">{{ n.title }}</p>
          <p class="mt-0.5 text-xs text-stone-500 leading-relaxed">{{ n.body }}</p>
          <p class="mt-1 text-xs text-stone-400">{{ formatDate(n.created_at) }}</p>
        </div>
        <span v-if="!n.is_read" class="h-2 w-2 shrink-0 rounded-full bg-ink mt-1.5" aria-label="Unread" />
      </div>
    </div>
  </div>
</template>
