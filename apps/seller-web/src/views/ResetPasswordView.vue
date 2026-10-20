<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Lock, Eye, EyeOff, CheckCircle2, ArrowLeft } from 'lucide-vue-next'
import { auth } from '@/api'
import { extractApiError } from '@carlitos/shared'
import AuthShell from '@/components/AuthShell.vue'

const route  = useRoute()
const router = useRouter()

const token    = ref('')
const password = ref('')
const confirm  = ref('')
const showPw   = ref(false)
const loading  = ref(false)
const done     = ref(false)
const error    = ref('')

onMounted(() => {
  // Token arrives via the emailed link: /seller/reset-password?token=...
  token.value = String(route.query.token || '')
  if (!token.value) error.value = 'This reset link is missing its token. Please request a new one.'
})

const pwValid    = computed(() => password.value.length >= 8)
const matches    = computed(() => password.value === confirm.value)
const canSubmit  = computed(() => !!token.value && pwValid.value && matches.value)

async function submit() {
  error.value = ''
  if (!token.value) { error.value = 'Missing reset token. Request a new link.'; return }
  if (!pwValid.value) { error.value = 'Password must be at least 8 characters.'; return }
  if (!matches.value) { error.value = 'Passwords do not match.'; return }
  loading.value = true
  try {
    await auth.resetPassword(token.value, password.value)
    done.value = true
    setTimeout(() => router.replace('/login'), 2200)
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthShell
    :title="done ? 'Password updated' : 'Set a new password'"
    :subtitle="done ? '' : 'Choose a strong password you haven\'t used before.'"
    panel-headline="Secure your store in seconds."
    :panel-points="[
      'Pick a password with at least 8 characters',
      'You\'ll be signed out of other devices',
      'Then sign in with your new password',
      'Your store data stays safe throughout',
    ]"
  >
    <!-- Success -->
    <div v-if="done" class="space-y-5">
      <div class="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-4">
        <CheckCircle2 :size="18" class="mt-0.5 shrink-0 text-emerald-600" />
        <p class="text-sm text-emerald-800">Your password has been reset. Redirecting you to sign in…</p>
      </div>
      <RouterLink to="/login" class="flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink">
        <ArrowLeft :size="15" /> Go to sign in now
      </RouterLink>
    </div>

    <!-- Form -->
    <form v-else class="space-y-4" @submit.prevent="submit" novalidate>
      <div>
        <label class="label" for="rp-pass">New password</label>
        <div class="relative">
          <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
          <input id="rp-pass" v-model="password" :type="showPw ? 'text' : 'password'" class="input pl-9 pr-10" minlength="8" placeholder="Min 8 characters" required autofocus />
          <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showPw = !showPw" :aria-label="showPw ? 'Hide' : 'Show'">
            <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
          </button>
        </div>
      </div>

      <div>
        <label class="label" for="rp-confirm">Confirm password</label>
        <div class="relative">
          <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
          <input id="rp-confirm" v-model="confirm" :type="showPw ? 'text' : 'password'" class="input pl-9" placeholder="Re-enter password" required />
        </div>
        <p v-if="confirm && !matches" class="field-hint text-red-500">Passwords don't match.</p>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</div>

      <button type="submit" class="btn-primary w-full" :disabled="loading || !canSubmit">
        {{ loading ? 'Updating…' : 'Reset password' }}
      </button>

      <RouterLink to="/login" class="flex items-center justify-center gap-1.5 text-sm text-stone-500 hover:text-ink">
        <ArrowLeft :size="15" /> Back to sign in
      </RouterLink>
    </form>
  </AuthShell>
</template>
