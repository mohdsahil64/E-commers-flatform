<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Mail, Lock, Eye, EyeOff, ShieldCheck } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { extractApiError, BRAND } from '@carlitos/shared'
import { REDIRECT_KEY } from '@/router'

const store    = useAuthStore()
const router   = useRouter()
const email    = ref('')
const password = ref('')
const showPw   = ref(false)
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
  <div class="flex min-h-screen items-center justify-center bg-stone-900 px-4">
    <div class="w-full max-w-sm">
      <!-- Logo -->
      <div class="mb-8 text-center">
        <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-white/10">
          <ShieldCheck :size="24" class="text-white" />
        </div>
        <p class="text-2xl font-semibold tracking-tight text-white">{{ BRAND }}</p>
        <p class="text-sm text-stone-400">Admin Control Center</p>
      </div>

      <div class="rounded-xl bg-white p-6 shadow-lg">
        <h1 class="text-lg font-semibold text-ink">Administrator Sign In</h1>
        <p class="mt-1 text-sm text-stone-500">Authorized personnel only.</p>

        <form class="mt-6 space-y-4" @submit.prevent="submit" novalidate>
          <div>
            <label class="label" for="a-email">Email</label>
            <div class="relative">
              <Mail class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input id="a-email" v-model="email" type="email" class="input pl-9" autocomplete="email" required autofocus />
            </div>
          </div>
          <div>
            <label class="label" for="a-password">Password</label>
            <div class="relative">
              <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input id="a-password" v-model="password" :type="showPw ? 'text' : 'password'" class="input pl-9 pr-10" autocomplete="current-password" required />
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showPw = !showPw" :aria-label="showPw ? 'Hide' : 'Show'">
                <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
              </button>
            </div>
          </div>

          <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</div>

          <button type="submit" class="btn-primary w-full" :disabled="store.loading">
            {{ store.loading ? 'Signing in…' : 'Sign in' }}
          </button>

          <div class="flex items-center justify-between text-xs">
            <a href="#" class="text-stone-500 hover:text-ink underline">Forgot password?</a>
            <span class="text-stone-400">2FA ready</span>
          </div>

          <p class="border-t border-stone-100 pt-4 text-xs text-stone-400 leading-relaxed">
            Admin accounts are provisioned by the platform. Public registration is not available.
          </p>
        </form>
      </div>

      <p class="mt-6 text-center text-xs text-stone-500">
        © {{ new Date().getFullYear() }} {{ BRAND }} · Secure administrative access
      </p>
    </div>
  </div>
</template>
