<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Mail, ArrowLeft, MailCheck } from 'lucide-vue-next'
import { auth } from '@/api'
import { extractApiError } from '@carlitos/shared'
import AuthShell from '@/components/AuthShell.vue'

const email   = ref('')
const sent     = ref(false)
const loading  = ref(false)
const error    = ref('')

const emailValid = computed(() => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email.value.trim()))

async function submit() {
  error.value = ''
  if (!emailValid.value) { error.value = 'Enter a valid email address.'; return }
  loading.value = true
  try {
    // The API always resolves — it never reveals whether the email exists.
    await auth.forgotPasswordSeller(email.value.trim())
    sent.value = true
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthShell
    :title="sent ? 'Check your inbox' : 'Forgot your password?'"
    :subtitle="sent ? '' : 'Enter the email tied to your seller account and we\'ll send a reset link.'"
    panel-headline="Back to business in a moment."
    :panel-points="[
      'A secure, single-use reset link',
      'The link expires in 30 minutes',
      'Your account stays protected',
      'No password? Reset it in seconds',
    ]"
  >
    <!-- Success state -->
    <div v-if="sent" class="space-y-5">
      <div class="flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-4">
        <MailCheck :size="18" class="mt-0.5 shrink-0 text-emerald-600" />
        <p class="text-sm text-emerald-800">
          If an account exists for <span class="font-medium">{{ email }}</span>, a password reset link is on its way.
          Check your inbox (and spam folder).
        </p>
      </div>
      <p class="text-sm text-stone-500">
        Didn't get it?
        <button class="font-medium text-ink underline hover:opacity-75" @click="sent = false">Try again</button>
      </p>
      <RouterLink to="/login" class="flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink">
        <ArrowLeft :size="15" /> Back to sign in
      </RouterLink>
    </div>

    <!-- Form state -->
    <form v-else class="space-y-4" @submit.prevent="submit" novalidate>
      <div>
        <label class="label" for="f-email">Email</label>
        <div class="relative">
          <Mail class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
          <input id="f-email" v-model="email" type="email" class="input pl-9" placeholder="seller@example.com" autocomplete="email" required autofocus />
        </div>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</div>

      <button type="submit" class="btn-primary w-full" :disabled="loading">
        {{ loading ? 'Sending link…' : 'Send reset link' }}
      </button>

      <RouterLink to="/login" class="flex items-center justify-center gap-1.5 text-sm text-stone-500 hover:text-ink">
        <ArrowLeft :size="15" /> Back to sign in
      </RouterLink>
    </form>
  </AuthShell>
</template>
