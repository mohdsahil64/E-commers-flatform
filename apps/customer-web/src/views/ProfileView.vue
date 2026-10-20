<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import {
  User, Mail, Phone, ShieldCheck, Check, Package, MapPin,
  Heart, RotateCcw, Bell, HelpCircle, Pencil, ChevronLeft,
  ChevronRight, Settings as SettingsIcon, Users, X, Plus, Trash2, Star,
} from 'lucide-vue-next'
import type { CustomerProfile, Address } from '@carlitos/types'
import { customer } from '@/api'
import { extractApiError } from '@carlitos/shared'
import { useAuthStore } from '@/stores/auth'
import { useCartStore } from '@/stores/cart'
import { useWishlistStore } from '@/stores/wishlist'
import { useToastStore } from '@/stores/toast'
import { useSeo } from '@/composables/useSeo'

const router = useRouter()
const auth = useAuthStore()
const cart = useCartStore()
const wishlist = useWishlistStore()
const toast = useToastStore()
useSeo({ title: 'My Account', description: 'Manage your CARLITOS profile, orders, addresses and more.' })

const profile  = ref<CustomerProfile | null>(null)
const loading  = ref(true)
const saving   = ref(false)
const error    = ref('')

// Quick counts for the account hub.
const orderCount = ref<number | null>(null)
const wishlistCount = computed(() => wishlist.count)

const editingName = ref(false)
const form = ref({ display_name: '' })

function startNameEdit() {
  form.value.display_name = profile.value?.display_name || ''
  editingName.value = true
}
function cancelNameEdit() {
  form.value.display_name = profile.value?.display_name || ''
  editingName.value = false
}

/* ── Email add/change (OTP-verified) ─────────────────── */
const emailStep   = ref<'idle' | 'code'>('idle')
const emailInput  = ref('')
const emailCode   = ref('')
const emailBusy   = ref(false)
const emailError  = ref('')
const emailNotice = ref('')
const editingEmail = ref(false)

// "Manage Account" opens a modal popup holding name, email and addresses.
const manageOpen = ref(false)

/* ── Addresses (managed inside the popup) ────────────── */
const addresses = ref<Address[]>([])
const addressesLoading = ref(false)
const addrBusy = ref('')
const showAddrForm = ref(false)
const addrForm = ref<Partial<Address>>({
  label: 'Home', contact_name: '', contact_phone: '',
  line1: '', line2: '', city: '', state: '', pincode: '', country: 'India',
})
function resetAddrForm() {
  addrForm.value = {
    label: 'Home', contact_name: '', contact_phone: '',
    line1: '', line2: '', city: '', state: '', pincode: '', country: 'India',
  }
  showAddrForm.value = false
}
async function loadAddresses() {
  addressesLoading.value = true
  try { addresses.value = await customer.addresses() }
  catch { addresses.value = [] }
  finally { addressesLoading.value = false }
}
async function saveAddress() {
  const f = addrForm.value
  if (!f.contact_name || !f.contact_phone || !f.line1 || !f.city || !f.state || !f.pincode) {
    toast.error('Please fill name, phone, address, city, state and pincode.'); return
  }
  addrBusy.value = 'new'
  try {
    await customer.addAddress(f)
    toast.success('Address saved')
    resetAddrForm()
    await loadAddresses()
  } catch (e) { toast.error(extractApiError(e)) } finally { addrBusy.value = '' }
}
async function makeDefault(id: string) {
  addrBusy.value = id
  try { await customer.setDefaultAddress(id); await loadAddresses() }
  catch (e) { toast.error(extractApiError(e)) } finally { addrBusy.value = '' }
}
async function removeAddress(id: string) {
  addrBusy.value = id
  try { await customer.deleteAddress(id); await loadAddresses() }
  catch (e) { toast.error(extractApiError(e)) } finally { addrBusy.value = '' }
}

// Open the manage-account popup (loads addresses lazily the first time).
function openManage() {
  manageOpen.value = true
  if (!addresses.value.length) loadAddresses()
}
function closeManage() {
  manageOpen.value = false
  editingName.value = false
  editingEmail.value = false
  resetAddrForm()
}

const currentEmail = computed(() => profile.value?.email || '')
const hasEmail = computed(() => !!currentEmail.value)
const emailValid = computed(() => /^\S+@\S+\.\S+$/.test(emailInput.value.trim()))
const codeValid  = computed(() => emailCode.value.replace(/\D/g, '').length === 6)

// Main navigation rows (icon + title + subtitle + chevron).
const navRows = computed(() => [
  { to: '/orders',        label: 'Orders',             icon: Package,    hint: orderCount.value != null ? `${orderCount.value} order${orderCount.value === 1 ? '' : 's'}` : 'Track & manage your orders' },
  { to: '/wishlist',      label: 'Wishlist',           icon: Heart,      hint: wishlistCount.value ? `${wishlistCount.value} saved style${wishlistCount.value === 1 ? '' : 's'}` : 'Your most loved styles' },
  { to: '/returns',       label: 'Returns & Exchanges', icon: RotateCcw, hint: 'Requests, exchanges & refund status' },
  { to: '/notifications', label: 'Notifications',      icon: Bell,       hint: 'Updates & alerts' },
  { to: '/support',       label: 'Help Center',        icon: HelpCircle, hint: 'FAQs & contact support' },
  { to: '/team',          label: 'Team & Management',  icon: Users,      hint: 'Meet the people behind CARLITOS' },
])

// Bottom info / legal links.
const legalLinks = [
  { to: '/support',  label: 'FAQs' },
  { to: '/about',    label: 'About Us' },
  { to: '/terms',    label: 'Terms & Conditions' },
  { to: '/privacy',  label: 'Privacy Policy' },
  { to: '/refund',   label: 'Cancellation & Refund Policy' },
  { to: '/shipping', label: 'Shipping Policy' },
  { to: '/cookies',  label: 'Cookie Policy' },
  { to: '/contact',  label: 'Contact Us' },
]

onMounted(async () => {
  try {
    profile.value = await customer.profile()
    form.value.display_name = profile.value?.display_name || ''
    // Cheap count fetches (best-effort).
    customer.orders().then(r => { orderCount.value = r.meta?.total ?? (r.data?.length || 0) }).catch(() => {})
    wishlist.refresh().catch(() => {})
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value  = true
  error.value   = ''
  try {
    await customer.updateProfile({ display_name: form.value.display_name })
    profile.value = await customer.profile()
    editingName.value = false
    toast.success('Profile updated')
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    saving.value = false
  }
}

function startEmailChange() {
  emailStep.value = 'idle'
  emailInput.value = ''
  emailCode.value = ''
  emailError.value = ''
  emailNotice.value = ''
  editingEmail.value = true
}

async function sendEmailCode() {
  emailError.value = ''
  emailNotice.value = ''
  if (!emailValid.value) { emailError.value = 'Enter a valid email address.'; return }
  emailBusy.value = true
  try {
    await customer.requestEmailChange(emailInput.value.trim())
    emailStep.value = 'code'
    emailNotice.value = `We sent a 6-digit code to ${emailInput.value.trim()}.`
  } catch (e) {
    emailError.value = extractApiError(e)
  } finally {
    emailBusy.value = false
  }
}

async function confirmEmailCode() {
  emailError.value = ''
  if (!codeValid.value) { emailError.value = 'Enter the 6-digit code.'; return }
  emailBusy.value = true
  try {
    await customer.confirmEmailChange(emailInput.value.trim(), emailCode.value)
    profile.value = await customer.profile()
    editingEmail.value = false
    emailStep.value = 'idle'
    toast.success('Email verified')
  } catch (e) {
    emailError.value = extractApiError(e)
  } finally {
    emailBusy.value = false
  }
}

function onCodeInput(e: Event) {
  const el = e.target as HTMLInputElement
  emailCode.value = el.value.replace(/\D/g, '').slice(0, 6)
}

async function logout() {
  await auth.logout()
  cart.items = []
  wishlist.server = []
  toast.success('Signed out')
  router.push('/')
}
</script>

<template>
  <div class="mx-auto max-w-2xl">
    <!-- Top bar -->
    <div class="mb-3 flex items-center gap-1.5">
      <button
        type="button"
        class="-ml-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-stone-700 transition-colors hover:bg-stone-100"
        aria-label="Go back"
        @click="router.back()"
      >
        <ChevronLeft :size="18" />
      </button>
      <h1 class="text-base font-semibold text-ink">Profile</h1>
    </div>

    <div v-if="loading" class="space-y-3">
      <div class="skeleton h-16 w-full rounded-xl" />
      <div v-for="i in 6" :key="i" class="skeleton h-16 w-full rounded-lg" />
    </div>

    <div v-else>
      <!-- Header card: name / phone / email -->
      <div class="flex items-center gap-3 rounded-xl border border-stone-200 bg-white p-3.5">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-stone-900">
          <User :size="20" class="text-white" />
        </div>
        <div class="min-w-0 flex-1">
          <div v-if="editingName" class="flex items-center gap-2">
            <input
              v-model="form.display_name"
              class="input h-9 py-1"
              placeholder="Your name"
              @keyup.enter="save"
            />
            <button class="btn-primary btn-sm shrink-0" :disabled="saving" @click="save">
              {{ saving ? '…' : 'Save' }}
            </button>
            <button class="btn-ghost btn-sm shrink-0" @click="cancelNameEdit">Cancel</button>
          </div>
          <div v-else class="flex items-center gap-2">
            <p class="truncate text-base font-semibold text-ink">{{ profile?.display_name || 'Add your name' }}</p>
            <button class="shrink-0 text-stone-400 hover:text-ink" aria-label="Edit name" @click="startNameEdit">
              <Pencil :size="14" />
            </button>
          </div>
          <p class="mt-0.5 flex items-center gap-1.5 text-sm text-stone-500">
            <Phone :size="13" /> {{ profile?.phone }}
            <ShieldCheck :size="13" class="text-emerald-600" />
          </p>
          <p v-if="hasEmail" class="flex items-center gap-1.5 truncate text-sm text-stone-500">
            <Mail :size="13" /> {{ currentEmail }}
          </p>
        </div>
      </div>
      <p v-if="error" class="mt-2 text-sm text-red-600">{{ error }}</p>

      <!-- Main navigation list (large rows) -->
      <div class="mt-4 overflow-hidden rounded-xl border border-stone-200 bg-white divide-y divide-stone-100">
        <!-- Manage Account → opens a popup (name + email + addresses) -->
        <button
          type="button"
          class="flex w-full items-center gap-3 px-3.5 py-3 text-left transition-colors hover:bg-stone-50"
          @click="openManage"
        >
          <component :is="SettingsIcon" :size="19" class="shrink-0 text-ink" />
          <span class="min-w-0 flex-1">
            <span class="block text-sm font-semibold text-ink">Manage Account</span>
            <span class="block text-xs text-stone-500">Name, email &amp; addresses</span>
          </span>
          <ChevronRight :size="17" class="shrink-0 text-stone-400" />
        </button>

        <!-- Feature rows -->
        <RouterLink
          v-for="r in navRows"
          :key="r.to"
          :to="r.to"
          class="flex items-center gap-3 px-3.5 py-3 transition-colors hover:bg-stone-50"
        >
          <component :is="r.icon" :size="19" class="shrink-0 text-ink" />
          <span class="min-w-0 flex-1">
            <span class="block text-sm font-semibold text-ink">{{ r.label }}</span>
            <span class="block truncate text-xs text-stone-500">{{ r.hint }}</span>
          </span>
          <ChevronRight :size="17" class="shrink-0 text-stone-400" />
        </RouterLink>
      </div>

      <!-- Legal / info links (gray section) -->
      <div class="mt-4 rounded-xl bg-stone-50 px-5 py-4">
        <ul class="space-y-3.5">
          <li v-for="l in legalLinks" :key="l.label">
            <RouterLink :to="l.to" class="text-xs font-medium uppercase tracking-wide text-stone-500 transition-colors hover:text-ink">
              {{ l.label }}
            </RouterLink>
          </li>
        </ul>
      </div>

      <!-- Logout -->
      <button
        class="mt-4 mb-6 w-full rounded-lg border border-red-300 py-3.5 text-sm font-semibold uppercase tracking-wide text-accent transition-colors hover:bg-accent-soft"
        @click="logout"
      >
        Log Out
      </button>
    </div>

    <!-- ── Manage Account popup ─────────────────────────────── -->
    <transition name="fade">
      <div v-if="manageOpen" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4" @click.self="closeManage">
        <div class="flex max-h-[92vh] w-full max-w-md flex-col rounded-t-2xl bg-white shadow-lg sm:rounded-2xl">
          <!-- Header -->
          <div class="flex items-center justify-between border-b border-stone-100 px-5 py-4">
            <h3 class="flex items-center gap-2 text-base font-semibold text-ink">
              <SettingsIcon :size="18" class="text-stone-500" /> Manage Account
            </h3>
            <button class="rounded p-1 text-stone-400 hover:bg-stone-50 hover:text-ink" aria-label="Close" @click="closeManage"><X :size="18" /></button>
          </div>

          <div class="flex-1 space-y-5 overflow-y-auto px-5 py-4">
            <!-- Name -->
            <div>
              <p class="mb-1.5 text-xs font-semibold uppercase tracking-wide text-stone-400">Name</p>
              <div v-if="editingName" class="flex items-center gap-2">
                <input v-model="form.display_name" class="input h-9 py-1" placeholder="Your name" @keyup.enter="save" />
                <button class="btn-primary btn-sm shrink-0" :disabled="saving" @click="save">{{ saving ? '…' : 'Save' }}</button>
                <button class="btn-ghost btn-sm shrink-0" @click="cancelNameEdit">Cancel</button>
              </div>
              <div v-else class="flex items-center justify-between rounded-lg border border-stone-200 px-3 py-2.5">
                <span class="flex items-center gap-2 text-sm text-ink"><User :size="15" class="text-stone-400" /> {{ profile?.display_name || 'Add your name' }}</span>
                <button class="shrink-0 text-sm text-stone-500 underline hover:text-ink" @click="startNameEdit">Edit</button>
              </div>
            </div>

            <!-- Email -->
            <div>
              <div class="mb-1.5 flex items-center justify-between">
                <p class="text-xs font-semibold uppercase tracking-wide text-stone-400">Email</p>
                <span v-if="hasEmail" class="inline-flex items-center gap-1 text-xs text-emerald-600"><Check :size="13" /> Verified</span>
              </div>

              <div v-if="hasEmail && !editingEmail" class="flex items-center justify-between rounded-lg border border-stone-200 px-3 py-2.5">
                <span class="flex items-center gap-2 truncate text-sm text-ink"><Mail :size="15" class="shrink-0 text-stone-400" /> {{ currentEmail }}</span>
                <button class="shrink-0 text-sm text-stone-500 underline hover:text-ink" @click="startEmailChange">Change</button>
              </div>
              <button v-if="!hasEmail && !editingEmail" class="btn-ghost btn-sm" @click="startEmailChange"><Mail :size="14" /> Add Email</button>

              <div v-if="editingEmail" class="space-y-3 rounded-lg border border-stone-200 bg-stone-50/60 p-4">
                <template v-if="emailStep === 'idle'">
                  <label class="label">New Email</label>
                  <div class="relative">
                    <Mail class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" :size="15" />
                    <input v-model="emailInput" type="email" class="input pl-9" placeholder="you@example.com" autocomplete="email" />
                  </div>
                  <div class="flex gap-2">
                    <button class="btn-primary btn-sm" :disabled="emailBusy || !emailValid" @click="sendEmailCode">{{ emailBusy ? 'Sending…' : 'Send Code' }}</button>
                    <button class="btn-ghost btn-sm" @click="editingEmail = false">Cancel</button>
                  </div>
                </template>
                <template v-else>
                  <p class="text-xs text-stone-500">{{ emailNotice }}</p>
                  <label class="label">Verification Code</label>
                  <input :value="emailCode" @input="onCodeInput" type="text" inputmode="numeric" maxlength="6" class="input text-center tracking-[0.4em] font-semibold" placeholder="······" />
                  <p class="field-hint">For testing use code <span class="font-medium">123456</span>.</p>
                  <div class="flex gap-2">
                    <button class="btn-primary btn-sm" :disabled="emailBusy || !codeValid" @click="confirmEmailCode">{{ emailBusy ? 'Verifying…' : 'Verify & Save' }}</button>
                    <button class="btn-ghost btn-sm" @click="emailStep = 'idle'">Back</button>
                  </div>
                </template>
                <p v-if="emailError" class="text-sm text-red-600">{{ emailError }}</p>
              </div>
            </div>

            <!-- Addresses -->
            <div>
              <div class="mb-1.5 flex items-center justify-between">
                <p class="text-xs font-semibold uppercase tracking-wide text-stone-400">Addresses</p>
                <button v-if="!showAddrForm" class="inline-flex items-center gap-1 text-sm font-medium text-accent hover:text-accent-dark" @click="showAddrForm = true"><Plus :size="14" /> Add new</button>
              </div>

              <div v-if="addressesLoading" class="space-y-2">
                <div v-for="i in 2" :key="i" class="skeleton h-16 w-full rounded-lg" />
              </div>

              <div v-else class="space-y-2">
                <div v-for="a in addresses" :key="a.id" class="rounded-lg border border-stone-200 p-3">
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p class="flex items-center gap-1.5 text-sm font-medium text-ink">
                        <MapPin :size="13" class="text-stone-400" /> {{ a.label || 'Address' }}
                        <span v-if="a.is_default" class="rounded-full bg-accent-soft px-1.5 py-0.5 text-[10px] font-semibold text-accent">Default</span>
                      </p>
                      <p class="mt-0.5 text-xs text-stone-600">{{ a.contact_name }} · {{ a.contact_phone }}</p>
                      <p class="mt-0.5 text-xs text-stone-500">{{ a.line1 }}<span v-if="a.line2">, {{ a.line2 }}</span>, {{ a.city }}, {{ a.state }} – {{ a.pincode }}</p>
                    </div>
                  </div>
                  <div class="mt-2 flex items-center gap-3">
                    <button v-if="!a.is_default" class="inline-flex items-center gap-1 text-xs font-medium text-stone-500 hover:text-ink" :disabled="addrBusy === a.id" @click="makeDefault(a.id)"><Star :size="12" /> Set default</button>
                    <button class="inline-flex items-center gap-1 text-xs font-medium text-red-500 hover:text-red-600" :disabled="addrBusy === a.id" @click="removeAddress(a.id)"><Trash2 :size="12" /> Remove</button>
                  </div>
                </div>
                <p v-if="!addresses.length && !showAddrForm" class="rounded-lg border border-dashed border-stone-200 py-4 text-center text-xs text-stone-400">No saved addresses yet.</p>
              </div>

              <!-- Add address form -->
              <div v-if="showAddrForm" class="mt-2 space-y-2.5 rounded-lg border border-stone-200 bg-stone-50/60 p-3">
                <div class="flex gap-2">
                  <input v-model="addrForm.label" class="input" placeholder="Label (Home / Work)" />
                </div>
                <div class="flex gap-2">
                  <input v-model="addrForm.contact_name" class="input" placeholder="Full name" />
                  <input v-model="addrForm.contact_phone" class="input" inputmode="numeric" placeholder="Phone" />
                </div>
                <input v-model="addrForm.line1" class="input" placeholder="House no, building, street" />
                <input v-model="addrForm.line2" class="input" placeholder="Area, landmark (optional)" />
                <div class="flex gap-2">
                  <input v-model="addrForm.city" class="input" placeholder="City" />
                  <input v-model="addrForm.state" class="input" placeholder="State" />
                  <input v-model="addrForm.pincode" class="input" inputmode="numeric" placeholder="Pincode" />
                </div>
                <div class="flex gap-2">
                  <button class="btn-ghost btn-sm flex-1" @click="resetAddrForm">Cancel</button>
                  <button class="btn-primary btn-sm flex-1" :disabled="addrBusy === 'new'" @click="saveAddress">{{ addrBusy === 'new' ? 'Saving…' : 'Save address' }}</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
