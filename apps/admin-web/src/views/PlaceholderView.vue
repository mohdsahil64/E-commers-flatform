<script setup lang="ts">
import { Construction } from 'lucide-vue-next'
defineProps<{ title: string }>()
</script>

<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ title }}</h1>
    </div>

    <div class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-20 text-center">
      <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-stone-100">
        <Construction :size="26" class="text-stone-400" />
      </div>
      <p class="text-base font-medium text-stone-700">{{ title }} module</p>
      <p class="mt-1.5 max-w-md text-sm text-stone-500">
        This module's backend endpoints and data are live. The dedicated management interface
        for <strong>{{ title }}</strong> is on the roadmap and will connect to the existing API.
      </p>
    </div>
  </div>
</template>
