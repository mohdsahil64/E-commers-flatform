<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { Bell, Landmark, MapPin, Save, KeyRound, Eye, EyeOff, Wallet } from 'lucide-vue-next'
import type { Seller, SellerBankAccount, SellerPickupAddress } from '@carlitos/types'
import { seller } from '@/api'
import { extractApiError } from '@carlitos/shared'

const profile = ref<Seller | null>(null)
const bank = ref<SellerBankAccount | null>(null)
const pickup = ref<SellerPickupAddress | null>(null)
const loading = ref(true)

// ── Notification preferences (persisted server-side) ──────────────
const prefs = ref({
  notify_order_emails: true,
  notify_return_emails: true,
  notify_settlement_emails: true,
  notify_low_stock: true,
  notify_marketing: false,
})
const savingPrefs = ref(false)
const prefsMsg = ref('')
const prefsErr = ref('')

// ── Payment options (Cash on Delivery) ────────────────────────────
// When COD is off, Cash on Delivery is disabled at checkout for every
// product from this store — customers must pay online.
const codEnabled = ref(true)
const savingCod = ref(false)
const codMsg = ref('')
const codErr = ref('')

// ── Change password ───────────────────────────────────────────────
const pw = ref({ current: '', next: '', confirm: '' })
const showPw = ref(false)
const savingPw = ref(false)
const pwMsg = ref('')
const pwErr = ref('')

const pwStrong = (s: string) => {
  if (s.length < 8) return false
  const classes = [/[a-z]/, /[A-Z]/, /[0-9]/, /[^A-Za-z0-9]/].filter(re => re.test(s)).length
  return classes >= 3
}

async function load() {
  loading.value = true
  try {
    // The onboarding bundle carries the seller + saved bank + pickup in one call.
    const bundle = await seller.onboarding().catch(() => null)
    profile.value = bundle?.seller ?? await seller.profile().catch(() => null)
    bank.value = bundle?.bank ?? null
    pickup.value = bundle?.pickup ?? null
    if (profile.value) {
      prefs.value = {
        notify_order_emails: profile.value.notify_order_emails,
        notify_return_emails: profile.value.notify_return_emails,
        notify_settlement_emails: profile.value.notify_settlement_emails,
        notify_low_stock: profile.value.notify_low_stock,
        notify_marketing: profile.value.notify_marketing,
      }
      codEnabled.value = profile.value.cod_enabled ?? true
    }
  } finally {
    loading.value = false
  }
}

async function saveCod() {
  if (!profile.value) return
  codErr.value = ''; codMsg.value = ''; savingCod.value = true
  try {
    const updated = await seller.updateProfile({
      // Preserve store + notification fields; only the COD flag changes here.
      store_name: profile.value.store_name,
      store_description: profile.value.store_description,
      ...prefs.value,
      cod_enabled: codEnabled.value,
    })
    profile.value = updated
    codEnabled.value = updated.cod_enabled ?? codEnabled.value
    codMsg.value = codEnabled.value
      ? 'Cash on Delivery is ON for your products.'
      : 'Cash on Delivery is OFF — customers must pay online.'
    setTimeout(() => { codMsg.value = '' }, 3000)
  } catch (e) {
    codErr.value = extractApiError(e)
  } finally {
    savingCod.value = false
  }
}

// Mask an account number, showing only the last 4 digits.
function maskAccount(n?: string) {
  if (!n) return '—'
  return n.length <= 4 ? n : '•••• ' + n.slice(-4)
}
function pickupLine(a: SellerPickupAddress) {
  return [a.line1, a.line2, a.city, a.state, a.pincode].filter(Boolean).join(', ')
}

async function savePrefs() {
  if (!profile.value) return
  prefsErr.value = ''; prefsMsg.value = ''; savingPrefs.value = true
  try {
    const updated = await seller.updateProfile({
      // Preserve store fields + COD flag; only notification prefs change here.
      store_name: profile.value.store_name,
      store_description: profile.value.store_description,
      ...prefs.value,
      cod_enabled: codEnabled.value,
    })
    profile.value = updated
    prefsMsg.value = 'Preferences saved.'
    setTimeout(() => { prefsMsg.value = '' }, 2500)
  } catch (e) {
    prefsErr.value = extractApiError(e)
  } finally {
    savingPrefs.value = false
  }
}

async function changePassword() {
  pwErr.value = ''; pwMsg.value = ''
  if (!pwStrong(pw.value.next)) {
    pwErr.value = 'New password must be 8+ chars with a mix of upper, lower, number and symbol.'
    return
  }
  if (pw.value.next !== pw.value.confirm) {
    pwErr.value = 'New passwords do not match.'
    return
  }
  savingPw.value = true
  try {
    await seller.changePassword(pw.value.current, pw.value.next)
    pw.value = { current: '', next: '', confirm: '' }
    pwMsg.value = 'Password changed successfully.'
    setTimeout(() => { pwMsg.value = '' }, 3000)
  } catch (e) {
    pwErr.value = extractApiError(e)
  } finally {
    savingPw.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <div class="page-header">
      <h1 class="page-title">Settings</h1>
    </div>

    <div v-if="loading" class="space-y-4">
      <div class="skeleton h-48 w-full rounded-xl" />
      <div class="skeleton h-48 w-full rounded-xl" />
    </div>

    <template v-else>
      <!-- Notifications -->
      <section class="card p-6 mb-5">
        <div class="flex items-center gap-2 mb-4">
          <Bell :size="18" class="text-stone-500" />
          <h2 class="text-h4">Notifications</h2>
        </div>
        <div class="space-y-3">
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm text-stone-700">New order emails</span>
            <input v-model="prefs.notify_order_emails" type="checkbox" class="h-4 w-4 accent-ink" />
          </label>
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm text-stone-700">Return &amp; exchange emails</span>
            <input v-model="prefs.notify_return_emails" type="checkbox" class="h-4 w-4 accent-ink" />
          </label>
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm text-stone-700">Settlement / payout emails</span>
            <input v-model="prefs.notify_settlement_emails" type="checkbox" class="h-4 w-4 accent-ink" />
          </label>
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm text-stone-700">Low stock alerts</span>
            <input v-model="prefs.notify_low_stock" type="checkbox" class="h-4 w-4 accent-ink" />
          </label>
          <label class="flex items-center justify-between cursor-pointer">
            <span class="text-sm text-stone-700">Marketing &amp; tips</span>
            <input v-model="prefs.notify_marketing" type="checkbox" class="h-4 w-4 accent-ink" />
          </label>
        </div>
        <p v-if="prefsMsg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ prefsMsg }}</p>
        <p v-if="prefsErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ prefsErr }}</p>
        <button class="btn-primary mt-4" :disabled="savingPrefs" @click="savePrefs">
          <Save :size="15" /> {{ savingPrefs ? 'Saving…' : 'Save preferences' }}
        </button>
      </section>

      <!-- Payment options: Cash on Delivery -->
      <section class="card p-6 mb-5">
        <div class="flex items-center gap-2 mb-4">
          <Wallet :size="18" class="text-stone-500" />
          <h2 class="text-h4">Payment Options</h2>
        </div>
        <label class="flex items-start justify-between gap-4 cursor-pointer">
          <span>
            <span class="block text-sm font-medium text-stone-800">Cash on Delivery (COD)</span>
            <span class="mt-0.5 block text-xs text-stone-500">
              When off, customers can't choose Cash on Delivery for any of your
              products — they'll pay online instead.
            </span>
          </span>
          <input v-model="codEnabled" type="checkbox" class="mt-1 h-4 w-4 shrink-0 accent-ink" />
        </label>
        <p v-if="codMsg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ codMsg }}</p>
        <p v-if="codErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ codErr }}</p>
        <button class="btn-primary mt-4" :disabled="savingCod" @click="saveCod">
          <Save :size="15" /> {{ savingCod ? 'Saving…' : 'Save payment options' }}
        </button>
      </section>

      <!-- Change password -->
      <section class="card p-6 mb-5">
        <div class="flex items-center gap-2 mb-4">
          <KeyRound :size="18" class="text-stone-500" />
          <h2 class="text-h4">Change Password</h2>
        </div>
        <div class="space-y-4">
          <div>
            <label class="label">Current Password</label>
            <input v-model="pw.current" :type="showPw ? 'text' : 'password'" class="input" autocomplete="current-password" />
          </div>
          <div>
            <label class="label">New Password</label>
            <div class="relative">
              <input v-model="pw.next" :type="showPw ? 'text' : 'password'" class="input pr-10" autocomplete="new-password" />
              <button type="button" class="absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 hover:text-stone-600" @click="showPw = !showPw" :aria-label="showPw ? 'Hide' : 'Show'">
                <EyeOff v-if="showPw" :size="16" /><Eye v-else :size="16" />
              </button>
            </div>
            <p class="field-hint">At least 8 characters, using a mix of upper, lower, number and symbol.</p>
          </div>
          <div>
            <label class="label">Confirm New Password</label>
            <input v-model="pw.confirm" :type="showPw ? 'text' : 'password'" class="input" autocomplete="new-password" />
          </div>
        </div>
        <p v-if="pwMsg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ pwMsg }}</p>
        <p v-if="pwErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ pwErr }}</p>
        <button class="btn-primary mt-4" :disabled="savingPw || !pw.current || !pw.next || !pw.confirm" @click="changePassword">
          <KeyRound :size="15" /> {{ savingPw ? 'Updating…' : 'Update password' }}
        </button>
      </section>

      <!-- Store details: bank + pickup (read-only; locked after submission) -->
      <section class="grid gap-4 sm:grid-cols-2">
        <div class="card p-5">
          <div class="flex items-center gap-2 mb-3">
            <Landmark :size="18" class="text-stone-500" />
            <h3 class="text-sm font-semibold">Bank Account</h3>
          </div>
          <dl v-if="bank" class="space-y-1.5 text-xs">
            <div class="flex justify-between gap-2"><dt class="text-stone-400">Holder</dt><dd class="font-medium text-ink">{{ bank.account_holder }}</dd></div>
            <div class="flex justify-between gap-2"><dt class="text-stone-400">Account</dt><dd class="font-medium text-ink">{{ maskAccount(bank.account_number) }}</dd></div>
            <div class="flex justify-between gap-2"><dt class="text-stone-400">IFSC</dt><dd class="font-medium text-ink">{{ bank.ifsc }}</dd></div>
          </dl>
          <p v-else class="text-xs text-stone-500">Not added yet.</p>
          <p class="mt-3 text-xs text-stone-400">Used for settlement payouts.</p>
        </div>

        <div class="card p-5">
          <div class="flex items-center gap-2 mb-3">
            <MapPin :size="18" class="text-stone-500" />
            <h3 class="text-sm font-semibold">Pickup Address</h3>
          </div>
          <dl v-if="pickup" class="space-y-1.5 text-xs">
            <div class="flex justify-between gap-2"><dt class="text-stone-400">Contact</dt><dd class="font-medium text-ink text-right">{{ pickup.contact_name }}</dd></div>
            <div class="flex justify-between gap-2"><dt class="text-stone-400">Phone</dt><dd class="font-medium text-ink">{{ pickup.contact_phone }}</dd></div>
            <div><dt class="text-stone-400 mb-0.5">Address</dt><dd class="text-ink leading-snug">{{ pickupLine(pickup) }}</dd></div>
          </dl>
          <p v-else class="text-xs text-stone-500">Not added yet.</p>
          <p class="mt-3 text-xs text-stone-400">Where couriers collect your orders.</p>
        </div>
      </section>

      <!-- If still editable (pending/rejected), allow updating via onboarding. -->
      <p v-if="profile && ['pending','rejected'].includes(profile.status)" class="mt-4 text-center text-xs text-stone-500">
        Need to change bank or pickup details?
        <RouterLink to="/onboarding" class="underline">Update in onboarding</RouterLink>.
      </p>
      <p v-else class="mt-4 text-center text-xs text-stone-400">
        Bank &amp; pickup details are locked after your store is submitted. Contact support to change them.
      </p>
    </template>
  </div>
</template>
