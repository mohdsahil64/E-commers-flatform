<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Bell, Save, KeyRound, Eye, EyeOff, Wallet } from 'lucide-vue-next'
import type { Seller } from '@carlitos/types'
import { seller } from '@/api'
import { extractApiError } from '@carlitos/shared'

const profile = ref<Seller | null>(null)
const loading = ref(true)

// ── Notification preferences (persisted server-side, applied instantly) ──
type PrefKey =
  | 'notify_order_emails' | 'notify_return_emails' | 'notify_settlement_emails'
  | 'notify_low_stock' | 'notify_marketing'

const prefs = ref<Record<PrefKey, boolean>>({
  notify_order_emails: true,
  notify_return_emails: true,
  notify_settlement_emails: true,
  notify_low_stock: true,
  notify_marketing: false,
})
const PREF_ROWS: { key: PrefKey; label: string; hint: string }[] = [
  { key: 'notify_order_emails',      label: 'New order emails',          hint: 'Get an email when a customer places an order with you.' },
  { key: 'notify_return_emails',     label: 'Return & exchange emails',  hint: 'Get an email when a customer raises a return or exchange.' },
  { key: 'notify_settlement_emails', label: 'Settlement / payout emails', hint: 'Get an email when a payout is sent to your bank account.' },
  { key: 'notify_low_stock',         label: 'Low stock alerts',          hint: 'Get an email when an order leaves an item at or below its low-stock level.' },
  { key: 'notify_marketing',         label: 'Marketing & tips',          hint: 'Occasional selling tips and product updates.' },
]
// Which preference is currently being saved (disables its switch).
const savingKey = ref<PrefKey | ''>('')
const prefsMsg = ref('')
const prefsErr = ref('')
let prefsMsgTimer: ReturnType<typeof setTimeout> | undefined

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
    profile.value = await seller.profile().catch(() => null)
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

// Flip one notification switch and save it immediately. If the save fails the
// switch snaps back, so what the seller sees is always what the server has.
async function togglePref(key: PrefKey) {
  if (!profile.value || savingKey.value) return
  const next = !prefs.value[key]
  const previous = prefs.value[key]
  prefs.value[key] = next
  savingKey.value = key
  prefsErr.value = ''; prefsMsg.value = ''
  clearTimeout(prefsMsgTimer)
  try {
    const updated = await seller.updateProfile({
      // Preserve store fields + COD flag; only the notification prefs change.
      store_name: profile.value.store_name,
      store_description: profile.value.store_description,
      ...prefs.value,
      cod_enabled: codEnabled.value,
    })
    profile.value = updated
    const row = PREF_ROWS.find(r => r.key === key)
    prefsMsg.value = `${row?.label ?? 'Preference'} turned ${next ? 'ON' : 'OFF'}.`
    prefsMsgTimer = setTimeout(() => { prefsMsg.value = '' }, 2500)
  } catch (e) {
    prefs.value[key] = previous
    prefsErr.value = extractApiError(e)
  } finally {
    savingKey.value = ''
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
      <!-- Notifications: each switch saves instantly -->
      <section class="card p-6 mb-5">
        <div class="flex items-center gap-2 mb-1">
          <Bell :size="18" class="text-stone-500" />
          <h2 class="text-h4">Notifications</h2>
        </div>
        <p class="mb-4 text-xs text-stone-500">Emails go to your store's contact email. Changes apply immediately.</p>

        <ul class="divide-y divide-stone-100">
          <li v-for="row in PREF_ROWS" :key="row.key" class="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0">
            <div class="min-w-0">
              <p :id="`pref-${row.key}`" class="text-sm font-medium text-stone-800">{{ row.label }}</p>
              <p class="mt-0.5 text-xs text-stone-500">{{ row.hint }}</p>
            </div>
            <button
              type="button"
              role="switch"
              :aria-checked="prefs[row.key]"
              :aria-labelledby="`pref-${row.key}`"
              :disabled="!!savingKey"
              :class="[
                'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ink focus-visible:ring-offset-2 disabled:cursor-wait disabled:opacity-60',
                prefs[row.key] ? 'bg-ink' : 'bg-stone-300',
              ]"
              @click="togglePref(row.key)"
            >
              <span
                :class="[
                  'inline-block h-5 w-5 transform rounded-full bg-white shadow transition-transform',
                  prefs[row.key] ? 'translate-x-5' : 'translate-x-0.5',
                ]"
              />
            </button>
          </li>
        </ul>

        <p v-if="prefsMsg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ prefsMsg }}</p>
        <p v-if="prefsErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ prefsErr }}</p>
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
    </template>
  </div>
</template>
