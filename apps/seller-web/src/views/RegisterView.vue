<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import {
  Store, User, Mail, Phone, Lock, Eye, EyeOff, FileText,
  Landmark, ChevronRight, ChevronLeft, CheckCircle2,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { extractApiError } from '@carlitos/shared'
import { seller } from '@/api'
import AuthShell from '@/components/AuthShell.vue'

const store  = useAuthStore()
const router = useRouter()

// Registration flow (each step is gated by the previous being valid):
//   1) Contact  -> email + mobile, each OTP-verified before continuing
//   2) Security -> store name + a STRONG password
//   3) Owner    -> owner identity / KYC
//   4) Bank     -> payout account
// On submit we create the account (step-1/2 essentials), then persist owner &
// bank server-side so onboarding only has to collect pickup + the contract.
const step  = ref(1)
const TOTAL = 4
const STEP_META = [
  { n: 1, label: 'Contact' },
  { n: 2, label: 'Security' },
  { n: 3, label: 'Owner' },
  { n: 4, label: 'Bank' },
]

const showPw = ref(false)
const error  = ref('')
const busy   = ref(false)

const form = ref({
  // Step 1 — contact
  email: '',
  phone: '',
  // Step 2 — security
  store_name: '',
  password: '',
  // Step 3 — owner / KYC
  owner_name: '',
  pan_number: '',
  aadhaar_number: '',
  gst_number: '',
  // Step 4 — bank
  bank_holder: '',
  bank_account: '',
  bank_ifsc: '',
})

// ── Verification state ────────────────────────────────────────
const emailSent     = ref(false)
const emailVerified = ref(false)
const emailCode     = ref('')
const emailBusy     = ref(false)

const phoneSent     = ref(false)
const phoneVerified = ref(false)
const phoneCode     = ref('')
const phoneBusy     = ref(false)

// ── Validation ────────────────────────────────────────────────
const emailValid  = computed(() => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(form.value.email.trim()))
const phoneDigits = computed(() => form.value.phone.replace(/\D/g, ''))
const phoneValid  = computed(() => phoneDigits.value.length >= 10 && phoneDigits.value.length <= 15)
const panValid    = computed(() => /^[A-Z]{5}[0-9]{4}[A-Z]$/.test(form.value.pan_number.trim().toUpperCase()))
const aadhaarValid = computed(() => form.value.aadhaar_number.replace(/\D/g, '').length === 12)
const ifscValid   = computed(() => /^[A-Z]{4}0[A-Z0-9]{6}$/.test(form.value.bank_ifsc.trim().toUpperCase()))

// Strong password: 8+ chars and at least 3 of {lower, upper, digit, symbol}.
const pwChecks = computed(() => {
  const p = form.value.password
  return {
    length: p.length >= 8,
    lower:  /[a-z]/.test(p),
    upper:  /[A-Z]/.test(p),
    digit:  /[0-9]/.test(p),
    symbol: /[^A-Za-z0-9]/.test(p),
  }
})
const pwClasses  = computed(() => [pwChecks.value.lower, pwChecks.value.upper, pwChecks.value.digit, pwChecks.value.symbol].filter(Boolean).length)
const pwStrong   = computed(() => pwChecks.value.length && pwClasses.value >= 3)
const pwStrength = computed(() => {
  if (!form.value.password) return { pct: 0, label: '', color: '' }
  const score = (pwChecks.value.length ? 1 : 0) + pwClasses.value
  if (score <= 2) return { pct: 33, label: 'Weak',   color: 'bg-red-500' }
  if (score === 3) return { pct: 66, label: 'Fair',   color: 'bg-amber-500' }
  return { pct: 100, label: 'Strong', color: 'bg-emerald-500' }
})

const step1Valid = computed(() => emailVerified.value && phoneVerified.value)
const step2Valid = computed(() => form.value.store_name.trim().length > 1 && pwStrong.value)
const step3Valid = computed(() => form.value.owner_name.trim().length > 1 && panValid.value && aadhaarValid.value)
const step4Valid = computed(() =>
  form.value.bank_holder.trim().length > 1 &&
  form.value.bank_account.replace(/\D/g, '').length >= 8 && ifscValid.value
)

// ── Verification actions ──────────────────────────────────────
async function sendEmailCode() {
  error.value = ''
  if (!emailValid.value) { error.value = 'Enter a valid email first.'; return }
  emailBusy.value = true
  try {
    await seller.requestEmailCode(form.value.email.trim())
    emailSent.value = true
  } catch (e) { error.value = extractApiError(e) } finally { emailBusy.value = false }
}
async function verifyEmailCode() {
  error.value = ''
  emailBusy.value = true
  try {
    await seller.confirmEmailCode(form.value.email.trim(), emailCode.value.trim())
    emailVerified.value = true
  } catch (e) { error.value = extractApiError(e) } finally { emailBusy.value = false }
}
async function sendPhoneCode() {
  error.value = ''
  if (!phoneValid.value) { error.value = 'Enter a valid mobile number first.'; return }
  phoneBusy.value = true
  try {
    await seller.requestPhoneCode(form.value.phone.trim())
    phoneSent.value = true
  } catch (e) { error.value = extractApiError(e) } finally { phoneBusy.value = false }
}
async function verifyPhoneCode() {
  error.value = ''
  phoneBusy.value = true
  try {
    await seller.confirmPhoneCode(form.value.phone.trim(), phoneCode.value.trim())
    phoneVerified.value = true
  } catch (e) { error.value = extractApiError(e) } finally { phoneBusy.value = false }
}

// Re-lock verification if the seller edits a verified email/phone.
function onEmailInput() { if (emailVerified.value || emailSent.value) { emailVerified.value = false; emailSent.value = false; emailCode.value = '' } }
function onPhoneInput() { if (phoneVerified.value || phoneSent.value) { phoneVerified.value = false; phoneSent.value = false; phoneCode.value = '' } }

// ── Navigation ────────────────────────────────────────────────
function next() {
  error.value = ''
  if (step.value === 1 && !step1Valid.value) { error.value = 'Please verify both your email and mobile number.'; return }
  if (step.value === 2 && !step2Valid.value) { error.value = 'Choose a store name and a strong password.'; return }
  if (step.value === 3 && !step3Valid.value) { error.value = 'Enter a valid PAN and 12-digit Aadhaar.'; return }
  if (step.value < TOTAL) step.value++
}
function back() { error.value = ''; if (step.value > 1) step.value-- }

async function submit() {
  error.value = ''
  if (!step4Valid.value) { error.value = 'Please complete your bank details correctly.'; return }
  busy.value = true
  try {
    // 1) Create the account (email + phone were OTP-verified above).
    await store.register({
      owner_name: form.value.owner_name.trim(),
      store_name: form.value.store_name.trim(),
      email: form.value.email.trim(),
      phone: form.value.phone.trim(),
      password: form.value.password,
    })
    // 2) Persist owner/KYC + bank so onboarding never re-asks them.
    await seller.saveOwner({
      legal_business_name: form.value.store_name.trim(),
      owner_name: form.value.owner_name.trim(),
      contact_email: form.value.email.trim(),
      contact_phone: form.value.phone.trim(),
      pan_number: form.value.pan_number.trim().toUpperCase(),
      gst_number: form.value.gst_number.trim().toUpperCase(),
    }).catch(() => {})
    await seller.saveBank({
      account_holder: form.value.bank_holder.trim(),
      account_number: form.value.bank_account.replace(/\D/g, ''),
      ifsc: form.value.bank_ifsc.trim().toUpperCase(),
    }).catch(() => {})
    await store.refreshSellerStatus()
    // 3) Straight to the dashboard — the onboarding banner takes it from here.
    router.push('/')
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    busy.value = false
  }
}

const headline = computed(() => 'Start selling on CARLITOS today.')
const points = [
  'Zero cost to register — pay only when you sell',
  'Simple product & inventory management',
  'Fast, transparent settlements to your bank',
  'Logistics & packaging support built in',
  'Grow with millions of CARLITOS shoppers',
]
const subtitle = computed(() => {
  if (step.value === 1) return 'Verify your email & mobile to begin.'
  if (step.value === 2) return 'Pick your store name & a strong password.'
  if (step.value === 3) return 'Owner identity details for KYC.'
  return 'Where we should send your payouts.'
})
</script>

<template>
  <AuthShell
    title="Create your seller account"
    :subtitle="subtitle"
    :panel-headline="headline"
    :panel-points="points"
  >
    <!-- Step indicator — full dotted stepper on ≥sm; on mobile a compact
         "Step X of 4 · Label" line + a thin progress bar (counter hidden). -->
    <div class="mb-6 hidden items-center sm:flex">
      <template v-for="(s, i) in STEP_META" :key="s.n">
        <div class="flex items-center gap-2">
          <span
            :class="['flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold transition-colors',
              step === s.n ? 'bg-ink text-white' : step > s.n ? 'bg-emerald-100 text-emerald-700' : 'bg-stone-100 text-stone-400']"
          >
            <CheckCircle2 v-if="step > s.n" :size="15" /><span v-else>{{ s.n }}</span>
          </span>
          <span :class="['text-xs font-medium', step >= s.n ? 'text-ink' : 'text-stone-400']">{{ s.label }}</span>
        </div>
        <div v-if="i < STEP_META.length - 1" class="mx-3 h-px flex-1 bg-stone-200" />
      </template>
    </div>

    <!-- Mobile-only compact progress -->
    <div class="mb-6 sm:hidden">
      <div class="mb-1.5 flex items-center justify-between">
        <span class="text-sm font-semibold text-ink">{{ STEP_META[step - 1].label }}</span>
        <span class="text-xs font-medium text-stone-400">Step {{ step }} of {{ TOTAL }}</span>
      </div>
      <div class="h-1.5 w-full overflow-hidden rounded-full bg-stone-100">
        <div class="h-full rounded-full bg-ink transition-all duration-300" :style="{ width: (step / TOTAL * 100) + '%' }" />
      </div>
    </div>

    <form class="space-y-4" @submit.prevent="step < TOTAL ? next() : submit()" novalidate>
      <!-- ── STEP 1: Contact + verification ──────────────── -->
      <template v-if="step === 1">
        <!-- Email -->
        <div>
          <div class="flex items-center justify-between">
            <label class="label" for="r-email">Email <span class="text-red-500">*</span></label>
            <span v-if="emailVerified" class="flex items-center gap-1 text-xs font-medium text-emerald-600">
              <CheckCircle2 :size="14" /> Verified
            </span>
          </div>
          <div class="flex flex-col gap-2 xs:flex-row">
            <div class="relative flex-1">
              <Mail class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input id="r-email" v-model="form.email" type="email" class="input pl-9" placeholder="seller@example.com"
                     :disabled="emailVerified" @input="onEmailInput" required />
            </div>
            <button v-if="!emailVerified" type="button" class="btn-ghost shrink-0 whitespace-nowrap" :disabled="!emailValid || emailBusy" @click="sendEmailCode">
              {{ emailSent ? 'Resend' : 'Send code' }}
            </button>
          </div>
          <div v-if="emailSent && !emailVerified" class="mt-2 flex gap-2">
            <input v-model="emailCode" inputmode="numeric" maxlength="6" class="input flex-1" placeholder="6-digit email code" />
            <button type="button" class="btn-primary shrink-0 whitespace-nowrap" :disabled="emailCode.length < 4 || emailBusy" @click="verifyEmailCode">Verify</button>
          </div>
          <p v-if="emailSent && !emailVerified" class="field-hint mt-1">We sent a code to your email. Enter it to verify.</p>
        </div>

        <!-- Mobile -->
        <div>
          <div class="flex items-center justify-between">
            <label class="label" for="r-phone">Mobile Number <span class="text-red-500">*</span></label>
            <span v-if="phoneVerified" class="flex items-center gap-1 text-xs font-medium text-emerald-600">
              <CheckCircle2 :size="14" /> Verified
            </span>
          </div>
          <div class="flex flex-col gap-2 xs:flex-row">
            <div class="relative flex-1">
              <Phone class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
              <input id="r-phone" v-model="form.phone" type="tel" inputmode="numeric" class="input pl-9" placeholder="10-digit mobile number"
                     :disabled="phoneVerified" @input="onPhoneInput" required />
            </div>
            <button v-if="!phoneVerified" type="button" class="btn-ghost shrink-0 whitespace-nowrap" :disabled="!phoneValid || phoneBusy" @click="sendPhoneCode">
              {{ phoneSent ? 'Resend' : 'Send code' }}
            </button>
          </div>
          <div v-if="phoneSent && !phoneVerified" class="mt-2 flex gap-2">
            <input v-model="phoneCode" inputmode="numeric" maxlength="6" class="input flex-1" placeholder="6-digit SMS code" />
            <button type="button" class="btn-primary shrink-0 whitespace-nowrap" :disabled="phoneCode.length < 4 || phoneBusy" @click="verifyPhoneCode">Verify</button>
          </div>
          <p v-if="phoneSent && !phoneVerified" class="field-hint mt-1">We sent an SMS code to your mobile. Enter it to verify.</p>
        </div>
      </template>

      <!-- ── STEP 2: Store + strong password ──────────────── -->
      <template v-else-if="step === 2">
        <div>
          <label class="label" for="r-store">Store Name <span class="text-red-500">*</span></label>
          <div class="relative">
            <Store class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-store" v-model="form.store_name" class="input pl-9" placeholder="e.g. Trendy Threads" required autofocus />
          </div>
        </div>
        <div>
          <label class="label" for="r-pass">Password <span class="text-red-500">*</span></label>
          <div class="relative">
            <Lock class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-pass" v-model="form.password" :type="showPw ? 'text' : 'password'" class="input pl-9 pr-10" placeholder="Create a strong password" required />
            <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showPw = !showPw" :aria-label="showPw ? 'Hide' : 'Show'">
              <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
            </button>
          </div>
          <!-- Strength meter -->
          <div v-if="form.password" class="mt-2">
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-stone-100">
              <div :class="['h-full rounded-full transition-all', pwStrength.color]" :style="{ width: pwStrength.pct + '%' }" />
            </div>
            <p class="mt-1 text-xs font-medium" :class="pwStrong ? 'text-emerald-600' : 'text-stone-500'">{{ pwStrength.label }}</p>
          </div>
          <!-- Instructions -->
          <ul class="mt-2 space-y-0.5 text-xs">
            <li :class="pwChecks.length ? 'text-emerald-600' : 'text-stone-400'">• At least 8 characters</li>
            <li :class="pwClasses >= 3 ? 'text-emerald-600' : 'text-stone-400'">• Use 3 of: lowercase, UPPERCASE, number, symbol</li>
          </ul>
        </div>
      </template>

      <!-- ── STEP 3: Owner / KYC ──────────────────────────── -->
      <template v-else-if="step === 3">
        <div>
          <label class="label" for="r-owner">Owner Full Name <span class="text-red-500">*</span></label>
          <div class="relative">
            <User class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-owner" v-model="form.owner_name" class="input pl-9" placeholder="As per PAN card" required autofocus />
          </div>
        </div>
        <div>
          <label class="label" for="r-pan">PAN Number <span class="text-red-500">*</span></label>
          <div class="relative">
            <FileText class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-pan" v-model="form.pan_number" class="input pl-9 uppercase" maxlength="10" placeholder="ABCDE1234F" required />
          </div>
          <p v-if="form.pan_number && !panValid" class="field-hint text-red-500">Enter a valid 10-character PAN.</p>
        </div>
        <div>
          <label class="label" for="r-aadhaar">Aadhaar Number <span class="text-red-500">*</span></label>
          <div class="relative">
            <FileText class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-aadhaar" v-model="form.aadhaar_number" inputmode="numeric" maxlength="14" class="input pl-9" placeholder="12-digit Aadhaar" required />
          </div>
          <p v-if="form.aadhaar_number && !aadhaarValid" class="field-hint text-red-500">Aadhaar must be 12 digits.</p>
        </div>
        <div>
          <label class="label" for="r-gst">GSTIN <span class="font-normal text-stone-400">(optional)</span></label>
          <div class="relative">
            <FileText class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-gst" v-model="form.gst_number" class="input pl-9 uppercase" maxlength="15" placeholder="15-digit GSTIN (add later if you don't have one)" />
          </div>
          <p class="field-hint">Not required to register. You can add it anytime.</p>
        </div>
      </template>

      <!-- ── STEP 4: Bank ─────────────────────────────────── -->
      <template v-else>
        <div>
          <label class="label" for="r-holder">Account Holder Name <span class="text-red-500">*</span></label>
          <div class="relative">
            <User class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-holder" v-model="form.bank_holder" class="input pl-9" placeholder="As per bank records" required autofocus />
          </div>
        </div>
        <div>
          <label class="label" for="r-acct">Account Number <span class="text-red-500">*</span></label>
          <div class="relative">
            <Landmark class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-acct" v-model="form.bank_account" inputmode="numeric" class="input pl-9" placeholder="Bank account number" required />
          </div>
        </div>
        <div>
          <label class="label" for="r-ifsc">IFSC Code <span class="text-red-500">*</span></label>
          <div class="relative">
            <Landmark class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="16" />
            <input id="r-ifsc" v-model="form.bank_ifsc" class="input pl-9 uppercase" maxlength="11" placeholder="e.g. HDFC0001234" required />
          </div>
          <p v-if="form.bank_ifsc && !ifscValid" class="field-hint text-red-500">Enter a valid 11-character IFSC.</p>
        </div>
      </template>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</div>

      <!-- Navigation -->
      <div class="flex items-center gap-2.5 pt-1 sm:gap-3">
        <button v-if="step > 1" type="button" class="btn-ghost shrink-0" @click="back">
          <ChevronLeft :size="15" /> <span class="hidden xs:inline">Back</span>
        </button>
        <button v-if="step < TOTAL" type="submit" class="btn-primary flex-1">
          Continue <ChevronRight :size="15" />
        </button>
        <button v-else type="submit" class="btn-primary flex-1" :disabled="busy || !step4Valid">
          <template v-if="busy">Creating…</template>
          <template v-else>
            <span class="xs:hidden">Create account</span>
            <span class="hidden xs:inline">Create seller account</span>
          </template>
        </button>
      </div>

      <p v-if="step === TOTAL" class="rounded-lg bg-stone-50 p-3 text-xs leading-relaxed text-stone-500">
        After registering you'll finish a short onboarding (pickup location &amp; commission agreement).
        Your store goes live only after an admin approves it — until then you can set things up but can't list products.
      </p>
    </form>

    <p class="mt-6 text-center text-sm text-stone-500">
      Already a seller?
      <RouterLink to="/login" class="font-medium text-ink underline hover:opacity-75">Sign in</RouterLink>
    </p>
  </AuthShell>
</template>
