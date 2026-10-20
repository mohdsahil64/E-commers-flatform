<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Store, Save, Lock } from 'lucide-vue-next'
import type { Seller } from '@carlitos/types'
import { seller } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { SELLER_STATUS_LABELS, formatDate, extractApiError } from '@carlitos/shared'

const auth    = useAuthStore()
const profile = ref<Seller | null>(null)
const loading = ref(true)
const saving  = ref(false)
const message = ref('')
const error   = ref('')

// Editable working copy (only description + name-when-allowed are editable).
const form = ref({ store_name: '', store_description: '' })

// The store name is customer-facing and tied to the public slug — once the store
// is submitted/approved it's locked. Editable only while pending or rejected.
const nameEditable = computed(() =>
  !!profile.value && ['pending', 'rejected'].includes(profile.value.status)
)

async function load() {
  loading.value = true
  try {
    profile.value = await seller.profile().catch(() => null)
    if (profile.value) {
      form.value.store_name = profile.value.store_name
      form.value.store_description = profile.value.store_description || ''
    }
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!profile.value) return
  error.value = ''; message.value = ''; saving.value = true
  try {
    const updated = await seller.updateProfile({
      // Name is only sent when editable; the server also enforces this.
      store_name: nameEditable.value ? form.value.store_name.trim() : profile.value.store_name,
      store_description: form.value.store_description.trim(),
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
          <div class="flex h-14 w-14 items-center justify-center rounded-full bg-white/10">
            <Store :size="24" class="text-white" />
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

      <!-- Read-only business details -->
      <div class="card p-6">
        <h3 class="text-h4 mb-4">Business Details</h3>
        <dl class="grid grid-cols-1 gap-4 sm:grid-cols-2 text-sm">
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Owner</dt><dd class="mt-0.5 font-medium">{{ profile.owner_name }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Legal Name</dt><dd class="mt-0.5">{{ profile.legal_business_name || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Business Type</dt><dd class="mt-0.5 capitalize">{{ (profile.business_type || '').replace('_',' ') || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Contact Email</dt><dd class="mt-0.5">{{ profile.contact_email || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Contact Phone</dt><dd class="mt-0.5">{{ profile.contact_phone || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">GST</dt><dd class="mt-0.5">{{ profile.gst_number || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">PAN</dt><dd class="mt-0.5">{{ profile.pan_number || '—' }}</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Commission</dt><dd class="mt-0.5">{{ profile.commission_percent ?? 'Default' }}%</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Rating</dt><dd class="mt-0.5">{{ profile.rating_avg.toFixed(1) }} ★ ({{ profile.rating_count }})</dd></div>
          <div><dt class="text-stone-400 text-xs uppercase tracking-wide">Member Since</dt><dd class="mt-0.5">{{ formatDate(profile.created_at) }}</dd></div>
        </dl>

        <p class="mt-4 border-t border-stone-100 pt-4 text-xs text-stone-500">
          To change owner, bank or pickup details, use
          <RouterLink to="/onboarding" class="underline">onboarding</RouterLink>.
        </p>
      </div>
    </template>

    <div v-else class="rounded-xl border border-stone-200 bg-stone-50 p-8 text-center">
      <p class="text-sm text-stone-500">Could not load profile.</p>
    </div>
  </div>
</template>
