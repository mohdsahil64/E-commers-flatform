<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Store, Save, Lock, Upload, Loader2, Trash2, Landmark, MapPin, Plus, BadgeCheck,
} from 'lucide-vue-next'
import type { Seller, SellerBankAccount, SellerPickupAddress } from '@carlitos/types'
import { seller, MEDIA_BASE } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { SELLER_STATUS_LABELS, formatDate, resolveMediaUrl, extractApiError } from '@carlitos/shared'

const auth    = useAuthStore()
const profile = ref<Seller | null>(null)
// Saved bank account + pickup address (shown under Business Details).
const bank    = ref<SellerBankAccount | null>(null)
const pickup  = ref<SellerPickupAddress | null>(null)
const loading = ref(true)
const saving  = ref(false)
const message = ref('')
const error   = ref('')

// Editable working copy (description + logo + name-when-allowed are editable).
const form = ref({ store_name: '', store_description: '' })

// Store logo: the storage key the seller has chosen (uploaded or existing).
const logoKey   = ref('')
const uploading = ref(false)
const logoInput = ref<HTMLInputElement | null>(null)

const logoUrl = computed(() => (logoKey.value ? resolveMediaUrl(MEDIA_BASE, logoKey.value) : ''))

function pickLogo() { logoInput.value?.click() }

async function onLogoChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  error.value = ''
  if (!/^image\//.test(file.type)) { error.value = 'Please choose an image file.'; return }
  if (file.size > 5 * 1024 * 1024) { error.value = 'Logo must be 5 MB or smaller.'; return }
  uploading.value = true
  try {
    const res = await seller.uploadImage(file)
    logoKey.value = res.key
  } catch (err) {
    error.value = extractApiError(err)
  } finally {
    uploading.value = false
    if (logoInput.value) logoInput.value.value = ''
  }
}

function removeLogo() { logoKey.value = '' }

// The store name is customer-facing and tied to the public slug — once the store
// is submitted/approved it's locked. Editable only while pending or rejected.
const nameEditable = computed(() =>
  !!profile.value && ['pending', 'rejected'].includes(profile.value.status)
)

async function load() {
  loading.value = true
  try {
    // The onboarding bundle carries the seller + saved bank + pickup in one call.
    const bundle = await seller.onboarding().catch(() => null)
    profile.value = bundle?.seller ?? await seller.profile().catch(() => null)
    bank.value    = bundle?.bank ?? null
    pickup.value  = bundle?.pickup ?? null
    if (profile.value) {
      form.value.store_name = profile.value.store_name
      form.value.store_description = profile.value.store_description || ''
      logoKey.value = profile.value.store_logo_key || ''
    }
  } finally {
    loading.value = false
  }
}

/* ── Business details ───────────────────────────────────────────────
   Rule: a detail that is already on file is shown read-only (locked). A detail
   that is still MISSING shows an input so the seller can add it — even after
   approval. The server never overwrites a value that already exists. */
type BizKey =
  | 'owner_name' | 'legal_business_name' | 'business_type'
  | 'contact_email' | 'contact_phone' | 'gst_number' | 'pan_number'

const BUSINESS_TYPES = [
  { value: 'individual',     label: 'Individual' },
  { value: 'proprietorship', label: 'Proprietorship' },
  { value: 'partnership',    label: 'Partnership' },
  { value: 'llp',            label: 'LLP' },
  { value: 'pvt_ltd',        label: 'Private Limited' },
]
const BIZ_FIELDS: { key: BizKey; label: string; placeholder: string; kind?: 'select' | 'email' | 'tel'; maxlength?: number; upper?: boolean; hint?: string }[] = [
  { key: 'owner_name',          label: 'Owner name',          placeholder: 'Full name of the owner' },
  { key: 'legal_business_name', label: 'Legal business name', placeholder: 'Name as on your business documents' },
  { key: 'business_type',       label: 'Business type',       placeholder: 'Select type', kind: 'select' },
  { key: 'contact_email',       label: 'Contact email',       placeholder: 'you@example.com', kind: 'email' },
  { key: 'contact_phone',       label: 'Contact phone',       placeholder: '10-digit mobile', kind: 'tel', maxlength: 14 },
  { key: 'gst_number',          label: 'GSTIN',               placeholder: '22ABCDE1234F1Z5', maxlength: 15, upper: true, hint: '15-character GST number' },
  { key: 'pan_number',          label: 'PAN',                 placeholder: 'ABCDE1234F', maxlength: 10, upper: true, hint: '10-character PAN' },
]
const biz = ref<Record<BizKey, string>>({
  owner_name: '', legal_business_name: '', business_type: '',
  contact_email: '', contact_phone: '', gst_number: '', pan_number: '',
})
const bizSaving = ref(false)
const bizMsg    = ref('')
const bizErr    = ref('')

function bizValue(k: BizKey): string {
  return String((profile.value as unknown as Record<string, unknown>)?.[k] ?? '').trim()
}
function isMissing(k: BizKey) { return !bizValue(k) }
function bizDisplay(k: BizKey): string {
  const v = bizValue(k)
  if (k === 'business_type') return BUSINESS_TYPES.find(t => t.value === v)?.label || v.replace('_', ' ')
  return v
}
// Show the "Save details" button only when the seller typed into a missing field.
const canSaveBiz = computed(() => BIZ_FIELDS.some(f => isMissing(f.key) && biz.value[f.key].trim()))
const missingCount = computed(() => BIZ_FIELDS.filter(f => isMissing(f.key)).length)

async function saveBusiness() {
  if (!profile.value) return
  bizErr.value = ''; bizMsg.value = ''; bizSaving.value = true
  try {
    const body: Record<string, string> = {}
    for (const f of BIZ_FIELDS) {
      const v = biz.value[f.key].trim()
      if (isMissing(f.key) && v) body[f.key] = f.upper ? v.toUpperCase() : v
    }
    profile.value = await seller.fillBusiness(body)
    for (const f of BIZ_FIELDS) biz.value[f.key] = ''
    bizMsg.value = 'Details saved. They are now locked.'
    setTimeout(() => { bizMsg.value = '' }, 3000)
  } catch (e) {
    bizErr.value = extractApiError(e)
  } finally {
    bizSaving.value = false
  }
}

/* ── Pickup address + bank account (add only when missing) ───────── */
const addingPickup = ref(false)
const pickupForm = ref({ contact_name: '', contact_phone: '', line1: '', line2: '', city: '', state: '', pincode: '', country: 'India' })
const pickupSaving = ref(false)
const pickupErr = ref('')
const pickupValid = computed(() =>
  pickupForm.value.contact_name.trim().length > 1 &&
  pickupForm.value.contact_phone.replace(/\D/g, '').length >= 10 &&
  pickupForm.value.line1.trim().length > 2 &&
  !!pickupForm.value.city.trim() && !!pickupForm.value.state.trim() &&
  /^\d{6}$/.test(pickupForm.value.pincode.trim())
)
async function savePickup() {
  pickupErr.value = ''; pickupSaving.value = true
  try {
    const f = pickupForm.value
    await seller.savePickup({
      label: 'Primary', contact_name: f.contact_name.trim(), contact_phone: f.contact_phone.trim(),
      line1: f.line1.trim(), line2: f.line2.trim(), city: f.city.trim(), state: f.state.trim(),
      pincode: f.pincode.trim(), country: f.country.trim() || 'India',
    })
    addingPickup.value = false
    await load()
  } catch (e) { pickupErr.value = extractApiError(e) } finally { pickupSaving.value = false }
}

const addingBank = ref(false)
const bankForm = ref({ account_holder: '', account_number: '', ifsc: '', bank_name: '', branch: '' })
const bankSaving = ref(false)
const bankErr = ref('')
const bankValid = computed(() =>
  bankForm.value.account_holder.trim().length > 1 &&
  bankForm.value.account_number.replace(/\D/g, '').length >= 8 &&
  /^[A-Z]{4}0[A-Z0-9]{6}$/.test(bankForm.value.ifsc.trim().toUpperCase())
)
async function saveBank() {
  bankErr.value = ''; bankSaving.value = true
  try {
    const f = bankForm.value
    await seller.saveBank({
      account_holder: f.account_holder.trim(),
      account_number: f.account_number.replace(/\D/g, ''),
      ifsc: f.ifsc.trim().toUpperCase(),
      bank_name: f.bank_name.trim(), branch: f.branch.trim(),
    })
    addingBank.value = false
    await load()
  } catch (e) { bankErr.value = extractApiError(e) } finally { bankSaving.value = false }
}

function maskAccount(n?: string) {
  if (!n) return '—'
  return n.length <= 4 ? n : '•••• ' + n.slice(-4)
}
function pickupLine(a: SellerPickupAddress) {
  return [a.line1, a.line2, a.city, a.state, a.pincode].filter(Boolean).join(', ')
}
// Commission actually applied to this seller (override → signed contract → default).
const commissionLabel = computed(() => {
  const p = profile.value
  const v = p?.commission_percent ?? p?.contract_commission
  return v == null ? 'Marketplace default' : `${v}%`
})
const canEditViaOnboarding = computed(() => ['pending', 'rejected'].includes(profile.value?.status || ''))

async function save() {
  if (!profile.value) return
  error.value = ''; message.value = ''; saving.value = true
  try {
    const updated = await seller.updateProfile({
      // Name is only sent when editable; the server also enforces this.
      store_name: nameEditable.value ? form.value.store_name.trim() : profile.value.store_name,
      store_description: form.value.store_description.trim(),
      // Store logo key: '' clears it, a key sets it (server treats null as
      // "unchanged", so we always send the resolved current value here).
      store_logo_key: logoKey.value,
      // Preserve existing notification prefs (managed on the Settings page).
      notify_order_emails: profile.value.notify_order_emails,
      notify_return_emails: profile.value.notify_return_emails,
      notify_settlement_emails: profile.value.notify_settlement_emails,
      notify_low_stock: profile.value.notify_low_stock,
      notify_marketing: profile.value.notify_marketing,
    })
    profile.value = updated
    form.value.store_name = updated.store_name
    form.value.store_description = updated.store_description || ''
    logoKey.value = updated.store_logo_key || ''
    await auth.refreshSellerStatus()
    message.value = 'Profile updated.'
    setTimeout(() => { message.value = '' }, 2500)
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    saving.value = false
  }
}

function statusClass(s?: string) {
  if (s === 'approved') return 'badge-green'
  if (['rejected','suspended','blocked'].includes(s || '')) return 'badge-red'
  return 'badge-amber'
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <div class="page-header">
      <h1 class="page-title">Store Profile</h1>
    </div>

    <div v-if="loading" class="space-y-4">
      <div class="skeleton h-24 w-full rounded-xl" />
      <div class="skeleton h-40 w-full rounded-xl" />
    </div>

    <template v-else-if="profile">
      <!-- Header card -->
      <div class="rounded-xl bg-stone-900 p-6 mb-5">
        <div class="flex items-center gap-4">
          <div class="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-full bg-white/10">
            <img v-if="logoUrl" :src="logoUrl" :alt="profile.store_name" class="h-full w-full object-cover" />
            <Store v-else :size="24" class="text-white" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-xl font-semibold text-white">{{ profile.store_name }}</h2>
              <span :class="['badge', statusClass(profile.status)]">{{ SELLER_STATUS_LABELS[profile.status] || profile.status }}</span>
            </div>
            <p class="mt-0.5 text-sm text-stone-400">{{ profile.store_slug }}</p>
          </div>
        </div>
      </div>

      <!-- Editable: store name (locked once approved) + description -->
      <div class="card p-6 mb-5">
        <h3 class="text-h4 mb-4">Edit Store</h3>
        <div class="space-y-4">
          <!-- Store logo -->
          <div>
            <label class="label">Store Logo</label>
            <div class="flex items-center gap-4">
              <div class="flex h-20 w-20 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-stone-200 bg-stone-50">
                <img v-if="logoUrl" :src="logoUrl" :alt="form.store_name || 'Store logo'" class="h-full w-full object-cover" />
                <Store v-else :size="28" class="text-stone-300" />
              </div>
              <div class="flex flex-col gap-2">
                <div class="flex flex-wrap gap-2">
                  <button type="button" class="btn-ghost btn-sm" :disabled="uploading" @click="pickLogo">
                    <Loader2 v-if="uploading" :size="14" class="animate-spin" />
                    <Upload v-else :size="14" />
                    {{ uploading ? 'Uploading…' : (logoUrl ? 'Replace' : 'Upload logo') }}
                  </button>
                  <button v-if="logoUrl" type="button" class="btn-ghost btn-sm text-red-600" :disabled="uploading" @click="removeLogo">
                    <Trash2 :size="14" /> Remove
                  </button>
                </div>
                <p class="field-hint">Square image works best. PNG or JPG, up to 5 MB. Shown on your storefront.</p>
              </div>
              <input ref="logoInput" type="file" accept="image/*" class="hidden" @change="onLogoChange" />
            </div>
          </div>
          <div>
            <label class="label flex items-center gap-1.5">
              Store Name
              <Lock v-if="!nameEditable" :size="13" class="text-stone-400" />
            </label>
            <input
              v-model="form.store_name"
              class="input"
              :disabled="!nameEditable"
              :class="{ 'cursor-not-allowed bg-stone-50 text-stone-500': !nameEditable }"
            />
            <p v-if="!nameEditable" class="field-hint">Your store name is locked once your store is submitted for review — it's shown to customers and can't be changed.</p>
            <p v-else class="field-hint">You can change this until your store is approved.</p>
          </div>
          <div>
            <label class="label">Store Description</label>
            <textarea v-model="form.store_description" class="textarea" rows="4" placeholder="Tell customers about your brand…" />
            <p class="field-hint">Shown on your storefront page.</p>
          </div>
        </div>

        <p v-if="message" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
        <p v-if="error" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

        <button class="btn-primary mt-4" :disabled="saving" @click="save">
          <Save :size="15" /> {{ saving ? 'Saving…' : 'Save changes' }}
        </button>
      </div>

      <!-- Business details: filled details are locked, missing ones can be added -->
      <div class="card p-6 mb-5">
        <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-h4">Business Details</h3>
          <span v-if="missingCount" class="badge badge-amber">{{ missingCount }} to add</span>
          <span v-else class="badge badge-green">Complete</span>
        </div>
        <p class="mb-4 text-xs text-stone-500">
          Details already on file are locked. Anything missing can be added below
          <template v-if="missingCount">(once saved, it is locked too)</template>.
        </p>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
          <div v-for="f in BIZ_FIELDS" :key="f.key" class="min-w-0">
            <dt class="mb-1 flex items-center gap-1.5 text-xs uppercase tracking-wide text-stone-400">
              {{ f.label }}
              <Lock v-if="!isMissing(f.key)" :size="11" class="text-stone-300" aria-label="Locked" />
            </dt>
            <dd v-if="!isMissing(f.key)" class="break-words font-medium text-ink">{{ bizDisplay(f.key) }}</dd>
            <dd v-else>
              <select v-if="f.kind === 'select'" v-model="biz[f.key]" class="select" :aria-label="f.label">
                <option value="">{{ f.placeholder }}</option>
                <option v-for="t in BUSINESS_TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
              </select>
              <input
                v-else
                v-model="biz[f.key]"
                class="input"
                :class="{ uppercase: f.upper }"
                :type="f.kind || 'text'"
                :placeholder="f.placeholder"
                :maxlength="f.maxlength"
                :aria-label="f.label"
                :autocomplete="f.kind === 'email' ? 'email' : f.kind === 'tel' ? 'tel' : 'off'"
              />
              <p class="field-hint">{{ f.hint || 'Not added yet' }}</p>
            </dd>
          </div>
        </dl>

        <p v-if="bizMsg" class="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700" role="status">{{ bizMsg }}</p>
        <p v-if="bizErr" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600" role="alert">{{ bizErr }}</p>
        <button v-if="missingCount" class="btn-primary mt-4" :disabled="bizSaving || !canSaveBiz" @click="saveBusiness">
          <Save :size="15" /> {{ bizSaving ? 'Saving…' : 'Save details' }}
        </button>
      </div>

      <!-- Pickup address -->
      <div class="card p-6 mb-5">
        <div class="mb-3 flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <MapPin :size="18" class="text-stone-500" />
            <h3 class="text-h4">Pickup Address</h3>
          </div>
          <Lock v-if="pickup" :size="13" class="text-stone-300" aria-label="Locked" />
        </div>

        <dl v-if="pickup" class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Contact</dt><dd class="mt-0.5 font-medium">{{ pickup.contact_name }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Phone</dt><dd class="mt-0.5">{{ pickup.contact_phone }}</dd></div>
          <div class="sm:col-span-2"><dt class="text-xs uppercase tracking-wide text-stone-400">Address</dt><dd class="mt-0.5 leading-snug">{{ pickupLine(pickup) }}</dd></div>
        </dl>

        <template v-else>
          <p v-if="!addingPickup" class="text-sm text-stone-500">No pickup address added yet. Couriers need this to collect your orders.</p>
          <button v-if="!addingPickup" class="btn-ghost btn-sm mt-3" @click="addingPickup = true"><Plus :size="14" /> Add pickup address</button>
          <div v-else class="grid gap-3 sm:grid-cols-2">
            <div><label class="label" for="pk-name">Contact name</label><input id="pk-name" v-model="pickupForm.contact_name" class="input" /></div>
            <div><label class="label" for="pk-phone">Contact phone</label><input id="pk-phone" v-model="pickupForm.contact_phone" inputmode="numeric" class="input" placeholder="10-digit mobile" /></div>
            <div class="sm:col-span-2"><label class="label" for="pk-l1">Address line 1</label><input id="pk-l1" v-model="pickupForm.line1" class="input" placeholder="Building, street" /></div>
            <div class="sm:col-span-2"><label class="label" for="pk-l2">Address line 2</label><input id="pk-l2" v-model="pickupForm.line2" class="input" placeholder="Area, landmark (optional)" /></div>
            <div><label class="label" for="pk-city">City</label><input id="pk-city" v-model="pickupForm.city" class="input" /></div>
            <div><label class="label" for="pk-state">State</label><input id="pk-state" v-model="pickupForm.state" class="input" /></div>
            <div><label class="label" for="pk-pin">Pincode</label><input id="pk-pin" v-model="pickupForm.pincode" inputmode="numeric" maxlength="6" class="input" /></div>
            <div><label class="label" for="pk-country">Country</label><input id="pk-country" v-model="pickupForm.country" class="input" /></div>
            <p v-if="pickupErr" class="text-sm text-red-600 sm:col-span-2" role="alert">{{ pickupErr }}</p>
            <div class="flex gap-2 sm:col-span-2">
              <button class="btn-primary" :disabled="pickupSaving || !pickupValid" @click="savePickup"><Save :size="15" /> {{ pickupSaving ? 'Saving…' : 'Save address' }}</button>
              <button class="btn-ghost" :disabled="pickupSaving" @click="addingPickup = false">Cancel</button>
            </div>
          </div>
        </template>
      </div>

      <!-- Bank account -->
      <div class="card p-6 mb-5">
        <div class="mb-3 flex items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <Landmark :size="18" class="text-stone-500" />
            <h3 class="text-h4">Bank Account</h3>
          </div>
          <Lock v-if="bank" :size="13" class="text-stone-300" aria-label="Locked" />
        </div>

        <dl v-if="bank" class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Account holder</dt><dd class="mt-0.5 font-medium">{{ bank.account_holder }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Account number</dt><dd class="mt-0.5">{{ maskAccount(bank.account_number) }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">IFSC</dt><dd class="mt-0.5">{{ bank.ifsc }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Bank</dt><dd class="mt-0.5">{{ [bank.bank_name, bank.branch].filter(Boolean).join(' · ') || '—' }}</dd></div>
        </dl>
        <p v-if="bank" class="mt-3 text-xs text-stone-400">Settlement payouts are sent to this account.</p>

        <template v-else>
          <p v-if="!addingBank" class="text-sm text-stone-500">No bank account added yet. It is needed to receive your settlement payouts.</p>
          <button v-if="!addingBank" class="btn-ghost btn-sm mt-3" @click="addingBank = true"><Plus :size="14" /> Add bank account</button>
          <div v-else class="grid gap-3 sm:grid-cols-2">
            <div><label class="label" for="bk-holder">Account holder</label><input id="bk-holder" v-model="bankForm.account_holder" class="input" /></div>
            <div><label class="label" for="bk-no">Account number</label><input id="bk-no" v-model="bankForm.account_number" inputmode="numeric" class="input" /></div>
            <div><label class="label" for="bk-ifsc">IFSC</label><input id="bk-ifsc" v-model="bankForm.ifsc" class="input uppercase" maxlength="11" placeholder="HDFC0001234" /></div>
            <div><label class="label" for="bk-bank">Bank name</label><input id="bk-bank" v-model="bankForm.bank_name" class="input" placeholder="Optional" /></div>
            <div class="sm:col-span-2"><label class="label" for="bk-branch">Branch</label><input id="bk-branch" v-model="bankForm.branch" class="input" placeholder="Optional" /></div>
            <p v-if="bankErr" class="text-sm text-red-600 sm:col-span-2" role="alert">{{ bankErr }}</p>
            <div class="flex gap-2 sm:col-span-2">
              <button class="btn-primary" :disabled="bankSaving || !bankValid" @click="saveBank"><Save :size="15" /> {{ bankSaving ? 'Saving…' : 'Save bank account' }}</button>
              <button class="btn-ghost" :disabled="bankSaving" @click="addingBank = false">Cancel</button>
            </div>
          </div>
        </template>
      </div>

      <!-- Account & agreement (read-only) -->
      <div class="card p-6">
        <div class="mb-3 flex items-center gap-2">
          <BadgeCheck :size="18" class="text-stone-500" />
          <h3 class="text-h4">Account</h3>
        </div>
        <dl class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Store link</dt><dd class="mt-0.5 break-all">/seller/{{ profile.store_slug }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Commission</dt><dd class="mt-0.5 font-medium">{{ commissionLabel }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Seller agreement</dt><dd class="mt-0.5">{{ profile.contract_signed ? 'Signed' : 'Not signed' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Rating</dt><dd class="mt-0.5">{{ profile.rating_count ? `${profile.rating_avg.toFixed(1)} ★ (${profile.rating_count})` : 'No ratings yet' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Member since</dt><dd class="mt-0.5">{{ formatDate(profile.created_at) }}</dd></div>
        </dl>

        <p v-if="canEditViaOnboarding" class="mt-4 border-t border-stone-100 pt-4 text-xs text-stone-500">
          Your application isn't approved yet, so you can still change everything in
          <RouterLink to="/onboarding" class="underline">onboarding</RouterLink>.
        </p>
        <p v-else class="mt-4 border-t border-stone-100 pt-4 text-xs text-stone-500">
          Details on file are locked after submission. To change one, please contact seller support.
        </p>
      </div>
    </template>

    <div v-else class="rounded-xl border border-stone-200 bg-stone-50 p-8 text-center">
      <p class="text-sm text-stone-500">Could not load profile.</p>
    </div>
  </div>
</template>
