<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ScrollText } from 'lucide-vue-next'
import { admin } from '@/api'
import { formatDate } from '@carlitos/shared'

const logs    = ref<any[]>([])
const loading = ref(true)

onMounted(async () => {
  try { logs.value = (await admin.auditLogs()) as any[] }
  finally { loading.value = false }
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Audit Logs</h1>
    </div>
    <p class="mb-5 text-sm text-stone-500">Immutable record of administrative actions.</p>

    <div v-if="loading" class="space-y-2">
      <div v-for="i in 10" :key="i" class="skeleton h-11 w-full rounded-lg" />
    </div>

    <div v-else-if="!logs.length" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <ScrollText :size="40" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No audit entries</p>
      <p class="mt-1 text-xs text-stone-500">Administrative actions will be logged here.</p>
    </div>

    <div v-else class="table-wrap rounded-lg border border-stone-200">
      <table class="table">
        <thead>
          <tr>
            <th>Time</th>
            <th class="hidden sm:table-cell">Actor</th>
            <th>Action</th>
            <th class="hidden md:table-cell">Entity</th>
            <th class="hidden lg:table-cell">IP</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in logs" :key="log.id">
            <td class="text-xs text-stone-400 whitespace-nowrap">{{ formatDate(log.created_at) }}</td>
            <td class="hidden sm:table-cell text-stone-600 capitalize">{{ log.actor_role || '—' }}</td>
            <td class="font-medium">{{ log.action }}</td>
            <td class="hidden md:table-cell text-xs text-stone-500">{{ log.entity_type }} <span class="text-stone-400">{{ log.entity_id?.slice(0,8) }}</span></td>
            <td class="hidden lg:table-cell text-xs text-stone-400">{{ log.ip || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
