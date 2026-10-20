<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  CheckCircle, ChevronRight, ChevronLeft, MapPin, ShieldCheck, Send,
  User, Landmark, FileText,
} from 'lucide-vue-next'
import type { SellerOnboardingBundle } from '@carlitos/types'
import { seller } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { extractApiError, BRAND } from '@carlitos/shared'

const router  = useRouter()
const auth    = useAuthStore()
const bundle  = ref<SellerOnboardingBundle | null>(null)
const step    = ref(1)
const message = ref('')
const error   = ref('')
const saving  = ref(false)
const loading = ref(true)

// Onboarding after registration is just the two things we DON'T already have:
//   1) Pickup / store location (where couriers collect orders)
//   2) The commission agreement (contract) the seller signs
const STEPS = [
  { n: 1, label: 'Pickup',   icon: MapPin },
  { n: 2, label: 'Contract', icon: ShieldCheck },
  { n: 3, label: 'Submit',   icon: Send },
]

const pickup = ref({
  contact_name: '', contact_phone: '', line1: '', line2: '',
  city: '', state: '', pincode: '', country: 'India',
})
const signedName = ref('')
const agreed     = ref(false)

const defaultCommission = computed(() => bundle.value?.default_commission ?? 12)
const pickupDone   = computed(() => !!bundle.value?.onboarding?.pickup_completed)
const contractDone = computed(() => !!bundle.value?.onboarding?.contract_completed || !!bundle.value?.seller?.contract_signed)
const status       = computed(() => bundle.value?.seller?.status ?? 'pending')
const alreadySubmitted = computed(() => status.value === 'under_review')

const pickupValid = computed(() =>
  pickup.value.contact_name.trim().length > 1 &&
  pickup.value.contact_phone.replace(/\D/g, '').length >= 10 &&
  pickup.value.line1.trim().length > 2 &&
  pickup.value.city.trim() && pickup.value.state.trim() &&
  /^\d{6}$/.test(pickup.value.pincode.trim())
)

async function load() {
  loading.value = true
  try {
    bundle.value = await seller.onboarding()
    // Prefill pickup with anything already saved so we never re-ask.
    const p = bundle.value?.pickup
    if (p) {
      pickup.value.contact_name  = p.contact_name || ''
      pickup.value.contact_phone = p.contact_phone || ''
      pickup.value.line1 = p.line1 || ''
      pickup.value.line2 = p.line2 || ''
      pickup.value.city  = p.city || ''
      pickup.value.state = p.state || ''
      pickup.value.pincode = p.pincode || ''
      pickup.value.country = p.country || 'India'
    } else if (bundle.value?.seller) {
      // Fall back to the owner's name/phone for the pickup contact.
      pickup.value.contact_name  = bundle.value.seller.owner_name || ''
      pickup.value.contact_phone = bundle.value.seller.contact_phone || ''
    }
    if (bundle.value?.seller) signedName.value = bundle.value.seller.owner_name || ''
    // Jump to the first incomplete step.
    if (pickupDone.value && !contractDone.value) step.value = 2
    else if (pickupDone.value && contractDone.value) step.value = 3
    else step.value = 1
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    loading.value = false
  }
}

async function savePickup() {
  error.value = ''; message.value = ''; saving.value = true
  try {
    await seller.savePickup({
      label: 'Primary',
      contact_name: pickup.value.contact_name.trim(),
      contact_phone: pickup.value.contact_phone.trim(),
      line1: pickup.value.line1.trim(),
      line2: pickup.value.line2.trim(),
      city: pickup.value.city.trim(),
      state: pickup.value.state.trim(),
      pincode: pickup.value.pincode.trim(),
      country: pickup.value.country.trim() || 'India',
    })
    await load()
    step.value = 2
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

async function signContract() {
  error.value = ''; message.value = ''; saving.value = true
  try {
    await seller.signContract(signedName.value.trim())
    await load()
    step.value = 3
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

async function submitApplication() {
  error.value = ''; message.value = ''; saving.value = true
  try {
    await seller.submitOnboarding()
    await auth.refreshSellerStatus()
    message.value = 'Application submitted for admin review. You will be notified once approved.'
    await load()
    setTimeout(() => router.push('/'), 1800)
  } catch (e) { error.value = extractApiError(e) } finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <div class="mb-6">
      <h1 class="page-title">Seller Onboarding</h1>
      <p class="mt-0.5 text-sm text-stone-500">Two quick steps to activate your {{ BRAND }} store.</p>
    </div>

    <!-- Already under review -->
    <div v-if="alreadySubmitted" class="card p-6 text-center">
      <div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-amber-100">
        <ShieldCheck :size="22" class="text-amber-600" />
      </div>
      <h2 class="text-h4">Application under review</h2>
      <p class="mt-1 text-sm text-stone-500">Your store is being reviewed by our team. We'll notify you once it's approved.</p>
      <button class="btn-primary mt-4" @click="router.push('/')">Back to dashboard</button>
    </div>

    <template v-else>
      <!-- Step indicator -->
      <div class="mb-8 flex items-center overflow-x-auto pb-1">
        <template v-for="(s, i) in STEPS" :key="s.n">
          <button
            :class="['flex shrink-0 items-center gap-1.5 text-xs transition-colors', step === s.n ? 'font-semibold text-ink' : step > s.n ? 'text-emerald-600' : 'text-stone-400']"
            @click="step > s.n && (step = s.n)"
          >
            <span :class="['flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold', step === s.n ? 'bg-ink text-white' : step > s.n ? 'bg-emerald-100 text-emerald-700' : 'bg-stone-100 text-stone-400']">
              <CheckCircle v-if="step > s.n" :size="14" /><span v-else>{{ s.n }}</span>
            </span>
            <span class="hidden sm:block">{{ s.label }}</span>
          </button>
          <div v-if="i < STEPS.length - 1" class="mx-2 h-px w-6 shrink-0 bg-stone-200" />
        </template>
      </div>

      <div v-if="loading" class="card p-6"><div class="skeleton h-40 w-full rounded" /></div>

      <div v-else class="card p-6">
        <!-- STEP 1: Pickup -->
        <div v-show="step === 1">
          <h2 class="text-h4 mb-1">Pickup / Store Location</h2>
          <p class="mb-4 text-sm text-stone-500">Where should couriers collect your orders? We only ask this once.</p>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="label">Contact Name <span class="text-red-500">*</span></label>
              <div class="relative">
                <User class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
                <input v-model="pickup.contact_name" class="input pl-9" />
              </div>
            </div>
            <div>
              <label class="label">Contact Phone <span class="text-red-500">*</span></label>
              <input v-model="pickup.contact_phone" inputmode="numeric" class="input" placeholder="10-digit mobile" />
            </div>
            <div class="sm:col-span-2">
              <label class="label">Address Line 1 <span class="text-red-500">*</span></label>
              <input v-model="pickup.line1" class="input" placeholder="Building, street" />
            </div>
            <div class="sm:col-span-2">
              <label class="label">Address Line 2</label>
              <input v-model="pickup.line2" class="input" placeholder="Area, landmark (optional)" />
            </div>
            <div><label class="label">City <span class="text-red-500">*</span></label><input v-model="pickup.city" class="input" /></div>
            <div><label class="label">State <span class="text-red-500">*</span></label><input v-model="pickup.state" class="input" /></div>
            <div><label class="label">Pincode <span class="text-red-500">*</span></label><input v-model="pickup.pincode" inputmode="numeric" maxlength="6" class="input" /></div>
            <div><label class="label">Country</label><input v-model="pickup.country" class="input" /></div>
          </div>
          <div class="mt-5 flex justify-end">
            <button class="btn-primary" :disabled="saving || !pickupValid" @click="savePickup">Save & Continue <ChevronRight :size="15" /></button>
          </div>
        </div>

        <!-- STEP 2: Contract -->
        <div v-show="step === 2">
          <h2 class="text-h4 mb-1">Seller Commission Agreement</h2>
          <p class="mb-4 text-sm text-stone-500">Please read and sign the agreement to continue.</p>

          <div class="rounded-lg border border-stone-200 bg-stone-50 p-4 text-sm text-stone-600 max-h-60 overflow-y-auto space-y-2">
            <p class="font-semibold text-stone-800">CARLITOS Marketplace Seller Agreement</p>
            <p>This Agreement is entered into between CARLITOS ("the Marketplace") and the undersigned seller ("You"). By signing, you agree to the following terms:</p>
            <p><strong>1. Commission.</strong> The Marketplace will deduct a commission of
              <strong>{{ defaultCommission }}%</strong> on the sale value of each successfully delivered order.
              Commission is calculated on the item price excluding shipping.</p>
            <p><strong>2. Settlements.</strong> Net proceeds (sale value less commission, applicable fees and refunds)
              are settled to your registered bank account per the settlement cycle after successful delivery.</p>
            <p><strong>3. Obligations.</strong> You agree to list authentic products, honour pricing and stock
              commitments, dispatch orders within stated timelines, and comply with returns and quality policies.</p>
            <p><strong>4. Conduct.</strong> Violation of these terms, counterfeit listings, or repeated policy
              breaches may lead to suspension or permanent removal from the Marketplace.</p>
            <p><strong>5. Term.</strong> This Agreement remains in effect while your seller account is active.</p>
          </div>

          <div class="mt-4">
            <label class="label">Type your full legal name to sign <span class="text-red-500">*</span></label>
            <div class="relative">
              <FileText class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
              <input v-model="signedName" class="input pl-9" placeholder="Full legal name" />
            </div>
          </div>
          <label class="mt-3 flex items-start gap-2 text-sm cursor-pointer">
            <input v-model="agreed" type="checkbox" class="mt-0.5 accent-ink" />
            I have read and agree to the CARLITOS Seller Commission Agreement at {{ defaultCommission }}% commission.
          </label>

          <div class="mt-5 flex justify-between">
            <button class="btn-ghost" @click="step = 1"><ChevronLeft :size="15" /> Back</button>
            <button class="btn-primary" :disabled="saving || !agreed || signedName.trim().length < 2" @click="signContract">
              Sign & Continue <ChevronRight :size="15" />
            </button>
          </div>
        </div>

        <!-- STEP 3: Submit -->
        <div v-show="step === 3">
          <h2 class="text-h4 mb-1">Review & Submit</h2>
          <p class="mb-4 text-sm text-stone-500">Everything's ready. Submit your application for admin review.</p>

          <ul class="space-y-2 text-sm">
            <li class="flex items-center gap-2 text-emerald-700"><CheckCircle :size="16" /> Owner &amp; KYC details</li>
            <li class="flex items-center gap-2" :class="bundle?.bank ? 'text-emerald-700' : 'text-stone-400'"><Landmark :size="16" /> Bank account for payouts</li>
            <li class="flex items-center gap-2" :class="pickupDone ? 'text-emerald-700' : 'text-stone-400'"><MapPin :size="16" /> Pickup / store location</li>
            <li class="flex items-center gap-2" :class="contractDone ? 'text-emerald-700' : 'text-stone-400'"><ShieldCheck :size="16" /> Commission agreement signed</li>
          </ul>

          <p v-if="message" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
          <p v-if="error" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

          <div class="mt-5 flex justify-between">
            <button class="btn-ghost" @click="step = 2"><ChevronLeft :size="15" /> Back</button>
            <button class="btn-primary" :disabled="saving || !pickupDone || !contractDone" @click="submitApplication">
              <Send :size="15" /> {{ saving ? 'Submitting…' : 'Submit for review' }}
            </button>
          </div>
        </div>
      </div>

      <p v-if="error && step !== 3" class="mt-4 text-sm text-red-600">{{ error }}</p>
    </template>
  </div>
</template>
