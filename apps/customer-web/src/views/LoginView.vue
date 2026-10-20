<script setup lang="ts">
import { ref, computed, nextTick, onBeforeUnmount } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { Smartphone, ChevronLeft, ArrowRight, ShieldCheck, MessageSquareText } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { extractApiError, BRAND } from '@carlitos/shared'
import BrandLogo from '@/components/BrandLogo.vue'
import { REDIRECT_KEY } from '@/router'

const store  = useAuthStore()
const router = useRouter()

const step   = ref<'phone' | 'otp'>('phone')
const phone  = ref('')
const error  = ref('')
const accepted = ref(false)          // privacy / terms acceptance
const resendIn = ref(0)
let resendTimer: ReturnType<typeof setInterval> | undefined

/* ── OTP: 6 separate boxes for a clean, animated entry ─────────── */
const OTP_LEN = 6
const otp = ref<string[]>(Array(OTP_LEN).fill(''))
const otpInputs = ref<HTMLInputElement[]>([])
const code = computed(() => otp.value.join(''))

const phoneDigits = computed(() => phone.value.replace(/\D/g, ''))
const phoneValid = computed(() => phoneDigits.value.length >= 10 && phoneDigits.value.length <= 15)
const codeValid = computed(() => code.value.length === OTP_LEN)
const canContinue = computed(() => phoneValid.value && accepted.value)

// Keep the phone field numeric-only regardless of how input arrives.
function onPhoneInput(e: Event) {
  const el = e.target as HTMLInputElement
  phone.value = el.value.replace(/\D/g, '').slice(0, 15)
}

function startResendCountdown() {
  resendIn.value = 30
  clearInterval(resendTimer)
  resendTimer = setInterval(() => {
    resendIn.value--
    if (resendIn.value <= 0) clearInterval(resendTimer)
  }, 1000)
}
onBeforeUnmount(() => clearInterval(resendTimer))

async function sendOtp() {
  error.value = ''
  if (!phoneValid.value) { error.value = 'Enter a valid mobile number.'; return }
  if (!accepted.value) { error.value = 'Please accept the Privacy Policy to continue.'; return }
  try {
    await store.requestOtp(phone.value)
    step.value = 'otp'
    otp.value = Array(OTP_LEN).fill('')
    startResendCountdown()
    await nextTick()
    otpInputs.value[0]?.focus()
  } catch (e) {
    error.value = extractApiError(e)
  }
}

async function resend() {
  if (resendIn.value > 0) return
  await sendOtp()
}

async function verify() {
  error.value = ''
  if (!codeValid.value) { error.value = 'Enter the 6-digit code.'; return }
  try {
    await store.verifyOtp(phone.value, code.value)
    const redirect = sessionStorage.getItem(REDIRECT_KEY) || '/'
    sessionStorage.removeItem(REDIRECT_KEY)
    router.replace(redirect)
  } catch (e) {
    error.value = extractApiError(e)
    // Clear the boxes on a wrong code so the user can retype cleanly.
    otp.value = Array(OTP_LEN).fill('')
    await nextTick()
    otpInputs.value[0]?.focus()
  }
}

function backToPhone() {
  step.value = 'phone'
  error.value = ''
}

/* ── OTP box interactions ──────────────────────────────────────── */
function onOtpInput(e: Event, i: number) {
  const el = e.target as HTMLInputElement
  const digit = el.value.replace(/\D/g, '').slice(-1)   // keep last digit only
  otp.value[i] = digit
  el.value = digit
  if (digit && i < OTP_LEN - 1) otpInputs.value[i + 1]?.focus()
  // Auto-submit once all boxes are filled.
  if (code.value.length === OTP_LEN && !store.loading) verify()
}

function onOtpKeydown(e: KeyboardEvent, i: number) {
  if (e.key === 'Backspace' && !otp.value[i] && i > 0) {
    otp.value[i - 1] = ''
    otpInputs.value[i - 1]?.focus()
  } else if (e.key === 'ArrowLeft' && i > 0) {
    otpInputs.value[i - 1]?.focus()
  } else if (e.key === 'ArrowRight' && i < OTP_LEN - 1) {
    otpInputs.value[i + 1]?.focus()
  }
}

function onOtpPaste(e: ClipboardEvent) {
  e.preventDefault()
  const digits = (e.clipboardData?.getData('text') || '').replace(/\D/g, '').slice(0, OTP_LEN)
  if (!digits) return
  const next = Array(OTP_LEN).fill('')
  for (let i = 0; i < digits.length; i++) next[i] = digits[i]
  otp.value = next
  const target = Math.min(digits.length, OTP_LEN - 1)
  nextTick(() => {
    otpInputs.value[target]?.focus()
    if (code.value.length === OTP_LEN && !store.loading) verify()
  })
}
</script>

<template>
  <div class="flex min-h-screen">
    <!-- Left brand panel (desktop) -->
    <div class="relative hidden flex-1 items-center justify-center overflow-hidden bg-gradient-to-br from-accent to-accent-dark lg:flex">
      <div class="max-w-sm px-8 text-center">
        <p class="text-4xl font-semibold tracking-tight text-white">{{ BRAND }}</p>
        <p class="mt-4 text-sm leading-relaxed text-white/80">
          A premium multi-vendor fashion &amp; lifestyle marketplace. Sign in with your mobile number to start shopping.
        </p>
        <div class="mt-8 flex items-center justify-center gap-2 text-xs text-white/70">
          <ShieldCheck :size="15" /> Secure OTP login · No password needed
        </div>
      </div>
    </div>

    <!-- Right form panel -->
    <div class="flex flex-1 flex-col items-center justify-center bg-white px-6 py-12">
      <div class="w-full max-w-sm">
        <BrandLogo size="md" class="mb-8 lg:hidden" />

        <!-- STEP 1: phone -->
        <template v-if="step === 'phone'">
          <h1 class="text-2xl font-semibold text-ink">Login or Sign up</h1>
          <p class="mt-1 text-sm text-stone-500">We'll send a one-time code to verify your number.</p>

          <form class="mt-8 space-y-5" @submit.prevent="sendOtp" novalidate>
            <div>
              <label class="label" for="phone">Mobile Number</label>
              <div class="flex items-stretch overflow-hidden rounded-md border border-stone-300 transition-colors focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
                <span class="flex items-center gap-1.5 border-r border-stone-200 bg-stone-50 px-3 text-sm font-medium text-stone-600">
                  <Smartphone :size="15" class="text-stone-400" /> +91
                </span>
                <input
                  id="phone"
                  :value="phone"
                  @input="onPhoneInput"
                  type="tel"
                  inputmode="numeric"
                  pattern="[0-9]*"
                  autocomplete="tel"
                  maxlength="15"
                  class="min-w-0 flex-1 px-3 py-2.5 text-sm text-ink outline-none placeholder:text-stone-400"
                  placeholder="10-digit mobile number"
                  required
                  autofocus
                />
              </div>
            </div>

            <!-- Privacy acceptance -->
            <label class="flex cursor-pointer items-start gap-2.5 select-none">
              <input
                v-model="accepted"
                type="checkbox"
                class="mt-0.5 h-4 w-4 shrink-0 rounded border-stone-300 text-accent accent-accent focus:ring-accent"
              />
              <span class="text-xs leading-relaxed text-stone-500">
                I agree to the
                <RouterLink to="/terms" class="font-medium text-accent hover:underline">Terms of Use</RouterLink>
                and
                <RouterLink to="/privacy" class="font-medium text-accent hover:underline">Privacy Policy</RouterLink>.
              </span>
            </label>

            <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">
              {{ error }}
            </div>

            <button type="submit" class="btn-primary w-full" :disabled="store.loading || !canContinue">
              {{ store.loading ? 'Sending code…' : 'Continue' }}
              <ArrowRight v-if="!store.loading" :size="15" />
            </button>
          </form>
        </template>

        <!-- STEP 2: OTP -->
        <template v-else>
          <button class="mb-4 flex items-center gap-1 text-sm text-stone-500 hover:text-ink" @click="backToPhone">
            <ChevronLeft :size="16" /> Change number
          </button>

          <div class="mb-6 flex h-12 w-12 items-center justify-center rounded-full bg-accent-soft text-accent">
            <MessageSquareText :size="22" />
          </div>
          <h1 class="text-2xl font-semibold text-ink">Verify your number</h1>
          <p class="mt-1 text-sm text-stone-500">
            Enter the 6-digit code sent to <span class="font-medium text-ink">+91 {{ phoneDigits }}</span>.
          </p>

          <form class="mt-8 space-y-5" @submit.prevent="verify" novalidate>
            <!-- Segmented OTP boxes -->
            <div class="flex justify-between gap-2" @paste="onOtpPaste">
              <input
                v-for="(_, i) in otp"
                :key="i"
                :ref="el => { if (el) otpInputs[i] = el as HTMLInputElement }"
                :value="otp[i]"
                @input="onOtpInput($event, i)"
                @keydown="onOtpKeydown($event, i)"
                @focus="($event.target as HTMLInputElement).select()"
                type="tel"
                inputmode="numeric"
                pattern="[0-9]*"
                maxlength="1"
                autocomplete="one-time-code"
                :class="[
                  'otp-box h-12 w-full rounded-lg border text-center text-lg font-semibold text-ink outline-none transition-all',
                  otp[i] ? 'otp-filled border-accent bg-accent-soft/40' : 'border-stone-300',
                  'focus:border-accent focus:ring-2 focus:ring-accent/30',
                ]"
                aria-label="OTP digit"
              />
            </div>
            <p class="field-hint">For testing, use code <span class="font-medium">123456</span>.</p>

            <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">
              {{ error }}
            </div>

            <button type="submit" class="btn-primary w-full" :disabled="store.loading || !codeValid">
              {{ store.loading ? 'Verifying…' : 'Verify & Continue' }}
            </button>

            <div class="text-center text-sm text-stone-500">
              Didn't receive it?
              <button
                type="button"
                class="font-medium text-accent underline disabled:no-underline disabled:text-stone-400"
                :disabled="resendIn > 0"
                @click="resend"
              >
                {{ resendIn > 0 ? `Resend in ${resendIn}s` : 'Resend code' }}
              </button>
            </div>
          </form>
        </template>

        <p class="mt-8 text-center text-xs text-stone-400">
          <RouterLink to="/" class="hover:text-ink underline">← Back to store</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Subtle lift on the focused OTP box, and a pop when a digit lands. */
.otp-box:focus { transform: translateY(-1px); }
@keyframes otp-pop {
  0%   { transform: scale(0.88); }
  60%  { transform: scale(1.07); }
  100% { transform: scale(1); }
}
.otp-filled { animation: otp-pop 160ms ease; }
</style>
