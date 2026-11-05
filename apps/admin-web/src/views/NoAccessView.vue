<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { ShieldOff } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { LANDING_ORDER } from '@/access'

const store  = useAuthStore()
const router = useRouter()

// The first page this person is allowed to open (dashboard is open to everyone).
const home = computed(() => LANDING_ORDER.find((n) => store.canAccess(n)) || 'dashboard')
</script>

<template>
  <div class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-20 text-center">
    <div class="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-stone-100">
      <ShieldOff :size="26" class="text-stone-400" />
    </div>
    <h1 class="text-base font-semibold text-stone-800">You don't have access to this page</h1>
    <p class="mt-1.5 max-w-md text-sm text-stone-500">
      Your role doesn't include this section. If you think you should have access,
      ask an administrator to update your role.
    </p>
    <button class="btn-primary mt-6" @click="router.replace({ name: home })">Go to my dashboard</button>
  </div>
</template>
