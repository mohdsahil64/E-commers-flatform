<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Mail, Lock, Eye, EyeOff, ShieldCheck, KeyRound, Check, Circle, ArrowLeft } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { extractApiError, BRAND } from '@carlitos/shared'
import { REDIRECT_KEY } from '@/router'

const store    = useAuthStore()
const router   = useRouter()
const route    = useRoute()
const email    = ref('')
const password = ref('')
const showPw   = ref(false)
const error    = ref('')
const notice   = ref(
  route.query.reason === 'account_inactive' ? 'This account has been disabled. Contact your administrator.'
    : route.query.reason === 'password_change_required' ? 'Finish setting up your account to continue.'
    : '',
)

// ── Step 2: first sign-in of an invited employee ───────────────────
const code      = ref('')
const newPw     = ref('')
const confirmPw = ref('')
const showNewPw = ref(false)
const info      = ref('')
const cooldown  = ref(0)
let timer: ReturnType<typeof setInterval> | undefined

const inSetup = computed(() => !!store.setup)

const rules = computed(() => [
  { ok: newPw.value.length >= 10,           label: 'At least 10 characters' },
  { ok: /[a-z]/.test(newPw.value),          label: 'A lowercase letter' },
  { ok: /[A-Z]/.test(newPw.value),          label: 'An uppercase letter' },
  { ok: /[0-9]/.test(newPw.value),          label: 'A number' },
  { ok: /[^A-Za-z0-9]/.test(newPw.value),   label: 'A symbol (e.g. @ # $ !)' },
])
const pwOk      = computed(() => rules.value.every((r) => r.ok))
const matches   = computed(() => !!confirmPw.value && newPw.value === confirmPw.value)
const codeOk    = computed(() => /^\d{6}$/.test(code.value.trim()))
const canVerify = computed(() => codeOk.value && pwOk.value && matches.value && !store.loading)

function startCooldown(seconds = 60) {
  cooldown.value = seconds
  clearInterval(timer)
  timer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0) clearInterval(timer)
  }, 1000)
}
onBeforeUnmount(() => clearInterval(timer))

function goHome() {
  const redirect = sessionStorage.getItem(REDIRECT_KEY) || '/'
  sessionStorage.removeItem(REDIRECT_KEY)
  router.replace(redirect)
}

async function submit() {
  error.value = ''; notice.value = ''
  try {
    const result = await store.login(email.value, password.value)
    if (result === 'setup') {
      password.value = ''
      startCooldown()
      return
    }
    goHome()
  } catch (e) {
    error.value = extractApiError(e)
  }
}

async function verify() {
  if (!canVerify.value) return
  error.value = ''; info.value = ''
  try {
    await store.completeSetup(code.value, newPw.value)
    goHome()
  } catch (e) {
    error.value = extractApiError(e)
    const c = (e as { response?: { data?: { error?: { code?: string } } } })?.response?.data?.error?.code
    // The setup session is gone — start again from the sign-in form.
    if (c === 'setup_expired' || c === 'too_many_attempts' || c === 'invitation_expired') {
      store.cancelSetup()
      notice.value = error.value
      error.value = ''
      code.value = newPw.value = confirmPw.value = ''
    }
  }
}

async function resend() {
  if (cooldown.value > 0) return
  error.value = ''; info.value = ''
  try {
    await store.resendSetupCode()
    info.value = 'A new code is on its way to your email.'
    code.value = ''
    startCooldown()
  } catch (e) {
    error.value = extractApiError(e)
  }
}

function backToSignIn() {
  store.cancelSetup()
  error.value = ''; info.value = ''
  code.value = newPw.value = confirmPw.value = ''
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-stone-900 px-4 py-8">
    <div class="w-full max-w-sm">
      <!-- Logo -->
      <div class="mb-8 text-center">
        <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-white/10">
          <ShieldCheck :size="24" class="text-white" />
        </div>
        <p class="text-2xl font-semibold tracking-tight text-white">{{ BRAND }}</p>
        <p class="text-sm text-stone-400">Admin Control Center</p>
      </div>

      <!-- Step 1: sign in -->
      <div v-if="!inSetup" class="rounded-xl bg-white p-6 shadow-lg">
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
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showPw = !showPw" :aria-label="showPw ? 'Hide password' : 'Show password'">
                <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
              </button>
            </div>
            <p class="field-hint">New employee? Use the temporary password from your invitation email.</p>
          </div>

          <div v-if="notice" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800" role="status">{{ notice }}</div>
          <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ error }}</div>

          <button type="submit" class="btn-primary w-full" :disabled="store.loading || !email || !password">
            {{ store.loading ? 'Signing in…' : 'Sign in' }}
          </button>

          <div class="flex items-center justify-between text-xs">
            <a href="#" class="text-stone-500 hover:text-ink underline">Forgot password?</a>
            <span class="text-stone-400">Secure sign-in</span>
          </div>

          <p class="border-t border-stone-100 pt-4 text-xs text-stone-400 leading-relaxed">
            Admin accounts are provisioned or invited by an administrator. Public registration is not available.
          </p>
        </form>
      </div>

      <!-- Step 2: verify code + choose a new password -->
      <div v-else class="rounded-xl bg-white p-6 shadow-lg">
        <button type="button" class="mb-3 inline-flex items-center gap-1 text-xs text-stone-500 hover:text-ink" @click="backToSignIn">
          <ArrowLeft :size="13" /> Back to sign in
        </button>
        <h1 class="text-lg font-semibold text-ink">Finish setting up your account</h1>
        <p class="mt-1 text-sm text-stone-500">
          Welcome<span v-if="store.setup?.name">, {{ store.setup.name }}</span>. For your security, verify
          your email and replace the temporary password.
        </p>

        <form class="mt-5 space-y-4" @submit.prevent="verify" novalidate>
          <div>
            <label class="label" for="s-code">Verification code</label>
            <div class="relative">
              <KeyRound class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input
                id="s-code" v-model="code" class="input pl-9 tracking-[0.35em]" inputmode="numeric"
                maxlength="6" autocomplete="one-time-code" placeholder="••••••" autofocus
              />
            </div>
            <p class="field-hint">
              We emailed a 6-digit code to <strong>{{ store.setup?.email_masked }}</strong>.
              <button type="button" class="ml-1 underline disabled:text-stone-400 disabled:no-underline" :disabled="cooldown > 0" @click="resend">
                {{ cooldown > 0 ? `Resend in ${cooldown}s` : 'Resend code' }}
              </button>
            </p>
          </div>

          <div>
            <label class="label" for="s-new">New password</label>
            <div class="relative">
              <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input id="s-new" v-model="newPw" :type="showNewPw ? 'text' : 'password'" class="input pl-9 pr-10" autocomplete="new-password" />
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showNewPw = !showNewPw" :aria-label="showNewPw ? 'Hide password' : 'Show password'">
                <EyeOff v-if="showNewPw" :size="16" /><Eye v-else :size="16" />
              </button>
            </div>
            <ul class="mt-2 grid grid-cols-1 gap-x-3 gap-y-1 text-xs sm:grid-cols-2" aria-label="Password requirements">
              <li v-for="r in rules" :key="r.label" :class="['flex items-center gap-1.5', r.ok ? 'text-emerald-600' : 'text-stone-400']">
                <Check v-if="r.ok" :size="12" /><Circle v-else :size="12" /> {{ r.label }}
              </li>
            </ul>
          </div>

          <div>
            <label class="label" for="s-confirm">Confirm new password</label>
            <input id="s-confirm" v-model="confirmPw" :type="showNewPw ? 'text' : 'password'" class="input" autocomplete="new-password" />
            <p v-if="confirmPw && !matches" class="field-error">Passwords don't match.</p>
          </div>

          <div v-if="info" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ info }}</div>
          <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ error }}</div>

          <button type="submit" class="btn-primary w-full" :disabled="!canVerify">
            {{ store.loading ? 'Verifying…' : 'Verify & activate account' }}
          </button>
        </form>
      </div>

      <p class="mt-6 text-center text-xs text-stone-500">
        © {{ new Date().getFullYear() }} {{ BRAND }} · Secure administrative access
      </p>
    </div>
  </div>
</template>
