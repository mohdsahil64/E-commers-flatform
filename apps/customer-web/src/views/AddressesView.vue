<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { MapPin, Plus, Trash2, Star, Pencil } from 'lucide-vue-next'
import type { Address } from '@carlitos/types'
import { customer } from '@/api'
import { extractApiError } from '@carlitos/shared'
import { useToastStore } from '@/stores/toast'
import { INDIA_STATES, DEFAULT_COUNTRY } from '@/constants/india'
import PageBack from '@/components/PageBack.vue'

const toast = useToastStore()
const addresses = ref<Address[]>([])
const loading   = ref(true)
const showForm  = ref(false)
const saving    = ref(false)
const error     = ref('')
// When set, the form is editing an existing address (id of the old record to
// replace). The backend has no PUT, so edit = add-new + delete-old.
const editingId = ref<string | null>(null)

// Label & type are no longer user-editable; we keep sensible fixed values so
// the backend still receives the fields it expects. Country is fixed to India.
const form = ref({
  contact_name:'', contact_phone:'', line1:'', line2:'',
  landmark:'', city:'', state:'', pincode:'', country: DEFAULT_COUNTRY, is_default: false,
})

function resetForm() {
  form.value = { contact_name:'', contact_phone:'', line1:'', line2:'', landmark:'', city:'', state:'', pincode:'', country: DEFAULT_COUNTRY, is_default:false }
  editingId.value = null
}

function openAdd() {
  resetForm()
  showForm.value = true
}

function openEdit(a: Address) {
  form.value = {
    contact_name: a.contact_name || '',
    contact_phone: a.contact_phone || '',
    line1: a.line1 || '',
    line2: a.line2 || '',
    landmark: a.landmark || '',
    city: a.city || '',
    state: a.state || '',
    pincode: a.pincode || '',
    country: a.country || DEFAULT_COUNTRY,
    is_default: a.is_default || false,
  }
  editingId.value = a.id
  showForm.value = true
}

function cancelForm() {
  showForm.value = false
  resetForm()
  error.value = ''
}

async function load() {
  loading.value = true
  try { addresses.value = (await customer.addresses()) || [] }
  catch (e) { toast.error(extractApiError(e)) }
  finally { loading.value = false }
}

function validate(): string | null {
  if (!form.value.contact_name.trim()) return 'Please enter the full name.'
  if (!form.value.line1.trim()) return 'Please enter address line 1.'
  if (!form.value.city.trim()) return 'Please enter the city.'
  if (!/^\d{6}$/.test(form.value.pincode.trim())) return 'Please enter a valid 6-digit pincode.'
  return null
}

async function save() {
  const v = validate()
  if (v) { error.value = v; return }
  saving.value = true
  error.value  = ''
  try {
    // label/address_type are fixed defaults (no longer shown in the form).
    await customer.addAddress({ ...form.value, label: 'Home', address_type: 'home' })
    // Editing: remove the previous record after the replacement is created.
    if (editingId.value) {
      try { await customer.deleteAddress(editingId.value) } catch { /* ignore */ }
    }
    await load()
    toast.success(editingId.value ? 'Address updated' : 'Address added')
    showForm.value = false
    resetForm()
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    saving.value = false
  }
}

async function setDefault(id: string) {
  try {
    await customer.setDefaultAddress(id)
    await load()
    toast.success('Default address updated')
  } catch (e) {
    toast.error(extractApiError(e))
  }
}

async function remove(id: string) {
  if (!confirm('Delete this address?')) return
  try {
    await customer.deleteAddress(id)
    await load()
    toast.success('Address removed')
  } catch (e) {
    toast.error(extractApiError(e))
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-xl">
    <PageBack history />
    <div class="mb-6 flex items-center justify-between">
      <h1 class="text-h2">Addresses</h1>
      <button class="btn-ghost btn-sm" @click="openAdd">
        <Plus :size="15" /> Add address
      </button>
    </div>

    <!-- Add / edit form -->
    <div v-if="showForm" class="card p-5 mb-5 space-y-4">
      <h2 class="text-h4">{{ editingId ? 'Edit Address' : 'New Address' }}</h2>
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="label">Full Name</label>
          <input v-model="form.contact_name" class="input" placeholder="e.g. Rahul Sharma" required />
        </div>
        <div>
          <label class="label">Phone</label>
          <input v-model="form.contact_phone" class="input" type="tel" inputmode="numeric" maxlength="10" placeholder="10-digit mobile" required />
        </div>
        <div class="sm:col-span-2">
          <label class="label">Flat / House no, Building, Area</label>
          <input v-model="form.line1" class="input" placeholder="House/flat no, building, street" required />
        </div>
        <div class="sm:col-span-2">
          <label class="label">Area / Locality <span class="text-stone-400">(optional)</span></label>
          <input v-model="form.line2" class="input" placeholder="Colony, sector, locality" />
        </div>
        <div class="sm:col-span-2">
          <label class="label">Landmark <span class="text-stone-400">(optional)</span></label>
          <input v-model="form.landmark" class="input" placeholder="Near school, temple, landmark…" />
        </div>
        <div>
          <label class="label">Pincode</label>
          <input v-model="form.pincode" class="input" inputmode="numeric" maxlength="6" placeholder="6-digit pincode" required />
        </div>
        <div>
          <label class="label">City</label>
          <input v-model="form.city" class="input" placeholder="City / town" required />
        </div>
        <div>
          <label class="label">State</label>
          <select v-model="form.state" class="select" required>
            <option value="" disabled>Select state</option>
            <option v-for="s in INDIA_STATES" :key="s" :value="s">{{ s }}</option>
          </select>
        </div>
        <div>
          <label class="label">Country</label>
          <input :value="form.country" class="input bg-stone-50 text-stone-500" disabled />
        </div>
      </div>
      <label class="flex items-center gap-2 text-sm cursor-pointer">
        <input v-model="form.is_default" type="checkbox" class="accent-ink" />
        Set as default
      </label>
      <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex gap-3">
        <button class="btn-ghost" @click="cancelForm">Cancel</button>
        <button class="btn-primary" :disabled="saving" @click="save">
          {{ saving ? 'Saving…' : (editingId ? 'Update address' : 'Save address') }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 2" :key="i" class="skeleton h-24 w-full rounded-lg" />
    </div>

    <!-- Empty -->
    <div v-else-if="!addresses.length && !showForm" class="flex flex-col items-center justify-center rounded-xl border border-stone-200 bg-stone-50 py-12">
      <MapPin :size="36" class="text-stone-300 mb-3" />
      <p class="text-sm font-medium text-stone-700">No saved addresses</p>
      <button class="btn-ghost btn-sm mt-3" @click="openAdd">Add your first address</button>
    </div>

    <!-- Address cards -->
    <div v-else class="space-y-3">
      <div
        v-for="a in addresses"
        :key="a.id"
        class="rounded-lg border border-stone-200 bg-white p-4"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="flex-1">
            <div class="flex flex-wrap items-center gap-2 mb-1">
              <span class="text-sm font-semibold">{{ a.contact_name }}</span>
              <span class="badge-gray">{{ a.label || a.address_type }}</span>
              <span v-if="a.is_default" class="badge-green">Default</span>
            </div>
            <p class="text-sm text-stone-600">{{ a.contact_phone }}</p>
            <p class="text-sm text-stone-600 mt-0.5">{{ a.line1 }}<span v-if="a.line2">, {{ a.line2 }}</span></p>
            <p class="text-sm text-stone-600">{{ a.city }}, {{ a.state }} – {{ a.pincode }}</p>
          </div>

          <div class="flex items-center gap-1 shrink-0">
            <button
              v-if="!a.is_default"
              class="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-stone-500 hover:bg-stone-50 hover:text-ink"
              @click="setDefault(a.id)"
              aria-label="Set as default"
            >
              <Star :size="13" /> Default
            </button>
            <button
              class="flex h-8 w-8 items-center justify-center rounded-md text-stone-400 hover:bg-stone-50 hover:text-ink"
              aria-label="Edit address"
              @click="openEdit(a)"
            >
              <Pencil :size="14" />
            </button>
            <button
              class="flex h-8 w-8 items-center justify-center rounded-md text-stone-400 hover:bg-red-50 hover:text-red-600"
              aria-label="Delete address"
              @click="remove(a.id)"
            >
              <Trash2 :size="15" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
