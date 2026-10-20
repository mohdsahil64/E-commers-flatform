<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { Mail, Lock, Eye, EyeOff } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { extractApiError } from '@carlitos/shared'
import AuthShell from '@/components/AuthShell.vue'
import { REDIRECT_KEY } from '@/router'

const store    = useAuthStore()
const router   = useRouter()
const email    = ref('')
const password = ref('')
const showPw   = ref(false)
const remember = ref(false)
const error    = ref('')

async function submit() {
  error.value = ''
  try {
    await store.login(email.value, password.value)
    const redirect = sessionStorage.getItem(REDIRECT_KEY) || '/'
    sessionStorage.removeItem(REDIRECT_KEY)
    router.replace(redirect)
  } catch (e) {
    error.value = extractApiError(e)
  }
}
</script>

<template>
  <AuthShell
    title="Sign in to your store"
    subtitle="Welcome back. Manage your products, orders, and payouts in one place."
    panel-headline="Grow your fashion business on CARLITOS."
    :panel-points="[
      'Reach thousands of buyers across India',
      'Track orders, shipments & settlements live',
      'Get paid on time, every time',
      'Manage everything from one dashboard',
    ]"
  >
    <form class="space-y-4" @submit.prevent="submit" novalidate>
      <div>
        <label class="label" for="s-email">Email</label>
        <div class="relative">
          <Mail class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
          <input id="s-email" v-model="email" type="email" class="input pl-9" placeholder="seller@example.com" autocomplete="email" required autofocus />
        </div>
      </div>

      <div>
        <div class="mb-1.5 flex items-center justify-between">
          <label class="label mb-0" for="s-password">Password</label>
          <RouterLink to="/forgot-password" class="text-xs text-stone-500 underline hover:text-ink">Forgot password?</RouterLink>
        </div>
        <div class="relative">
          <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
          <input id="s-password" v-model="password" :type="showPw ? 'text' : 'password'" class="input pl-9 pr-10" autocomplete="current-password" required />
          <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" :aria-label="showPw ? 'Hide password' : 'Show password'" @click="showPw = !showPw">
            <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
          </button>
        </div>
      </div>

      <label class="flex cursor-pointer items-center gap-2 text-sm text-stone-600">
        <input v-model="remember" type="checkbox" class="accent-ink" /> Remember this device
      </label>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</div>

      <button type="submit" class="btn-primary w-full" :disabled="store.loading">
        {{ store.loading ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>

    <p class="mt-6 text-center text-sm text-stone-500">
      New seller?
      <RouterLink to="/register" class="font-medium text-ink underline hover:opacity-75">Register your store</RouterLink>
    </p>
  </AuthShell>
</template>
