<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft, Store, Check, X, Ban, RefreshCcw, Pause } from 'lucide-vue-next'
import type { Seller } from '@carlitos/types'
import { admin } from '@/api'
import { formatDate, SELLER_STATUS_LABELS, extractApiError } from '@carlitos/shared'
import ConfirmDialog from '@/components/ConfirmDialog.vue'

const route   = useRoute()
const router  = useRouter()
const seller  = ref<Seller | null>(null)
const loading = ref(true)
const busy    = ref(false)
const message = ref('')
const error   = ref('')

// Dialog state
const dialog = ref<{ open: boolean; action: string; title: string; variant: 'danger'|'primary'; requireReason: boolean; confirmLabel: string }>({
  open: false, action: '', title: '', variant: 'primary', requireReason: false, confirmLabel: 'Confirm',
})

async function load() {
  seller.value = await admin.seller(route.params.id as string).catch(() => null)
}

function openDialog(action: string) {
  const map: Record<string, any> = {
    approve:   { title: 'Approve seller?',    variant: 'primary', requireReason: false, confirmLabel: 'Approve' },
    reject:    { title: 'Reject seller?',     variant: 'danger',  requireReason: true,  confirmLabel: 'Reject' },
    suspend:   { title: 'Suspend seller?',    variant: 'danger',  requireReason: true,  confirmLabel: 'Suspend' },
    unsuspend: { title: 'Lift suspension?',   variant: 'primary', requireReason: false, confirmLabel: 'Unsuspend' },
    block:     { title: 'Block seller?',      variant: 'danger',  requireReason: true,  confirmLabel: 'Block' },
    unblock:   { title: 'Unblock seller?',    variant: 'primary', requireReason: false, confirmLabel: 'Unblock' },
    activate:  { title: 'Reactivate seller?', variant: 'primary', requireReason: false, confirmLabel: 'Reactivate' },
  }
  dialog.value = { open: true, action, ...map[action] }
}

async function handleConfirm(reason: string) {
  busy.value = true; error.value = ''; message.value = ''
  const id = seller.value!.id
  try {
    const a = dialog.value.action
    if (a === 'approve')   await admin.approveSeller(id)
    if (a === 'reject')    await admin.rejectSeller(id, reason)
    if (a === 'suspend')   await admin.suspendSeller(id, reason)
    if (a === 'unsuspend') await admin.unsuspendSeller(id)
    if (a === 'block')     await admin.blockSeller(id, reason)
    if (a === 'unblock')   await admin.unblockSeller(id)
    if (a === 'activate')  await admin.activateSeller(id)
    message.value = 'Seller updated successfully.'
    await load()
  } catch (e) {
    error.value = extractApiError(e)
  } finally {
    busy.value = false
    dialog.value.open = false
  }
}

const s = computed(() => seller.value)
function statusClass(st?: string) {
  if (st === 'approved') return 'badge-green'
  if (['rejected','suspended','blocked'].includes(st || '')) return 'badge-red'
  return 'badge-amber'
}

onMounted(async () => { await load(); loading.value = false })
</script>

<template>
  <div v-if="loading" class="space-y-4">
    <div class="skeleton h-8 w-1/3 rounded" />
    <div class="skeleton h-32 w-full rounded-xl" />
  </div>

  <div v-else-if="!s" class="py-12 text-center">
    <p class="text-stone-600">Seller not found.</p>
    <button class="btn-ghost btn-sm mt-4" @click="router.push('/sellers')">← All sellers</button>
  </div>

  <div v-else class="max-w-4xl">
    <button class="mb-5 flex items-center gap-1.5 text-sm text-stone-500 hover:text-ink" @click="router.push('/sellers')">
      <ChevronLeft :size="16" /> All sellers
    </button>

    <!-- Header -->
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div class="flex items-center gap-4">
        <div class="flex h-14 w-14 items-center justify-center rounded-full bg-stone-900">
          <Store :size="24" class="text-white" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h1 class="page-title">{{ s.store_name }}</h1>
            <span :class="['badge', statusClass(s.status)]">{{ SELLER_STATUS_LABELS[s.status] || s.status }}</span>
          </div>
          <p class="mt-0.5 text-sm text-stone-500">{{ s.store_slug }} · Joined {{ formatDate(s.created_at) }}</p>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex flex-wrap gap-2">
        <button v-if="['under_review','pending'].includes(s.status)" class="btn-success btn-sm" :disabled="busy" @click="openDialog('approve')">
          <Check :size="14" /> Approve
        </button>
        <button v-if="['under_review','pending'].includes(s.status)" class="btn-danger btn-sm" :disabled="busy" @click="openDialog('reject')">
          <X :size="14" /> Reject
        </button>
        <button v-if="s.status === 'approved'" class="btn-warning btn-sm" :disabled="busy" @click="openDialog('suspend')">
          <Pause :size="14" /> Suspend
        </button>
        <button v-if="s.status === 'suspended'" class="btn-primary btn-sm" :disabled="busy" @click="openDialog('unsuspend')">
          <RefreshCcw :size="14" /> Unsuspend
        </button>
        <button v-if="s.status === 'rejected'" class="btn-primary btn-sm" :disabled="busy" @click="openDialog('activate')">
          <RefreshCcw :size="14" /> Reactivate
        </button>
        <button v-if="s.status === 'blocked'" class="btn-primary btn-sm" :disabled="busy" @click="openDialog('unblock')">
          <RefreshCcw :size="14" /> Unblock
        </button>
        <button v-if="!['blocked','suspended'].includes(s.status)" class="btn-danger btn-sm" :disabled="busy" @click="openDialog('block')">
          <Ban :size="14" /> Block
        </button>
      </div>
    </div>

    <p v-if="message" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ message }}</p>
    <p v-if="error"   class="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-600">{{ error }}</p>

    <!-- Details -->
    <div class="grid gap-5 lg:grid-cols-2">
      <div class="card p-6">
        <h2 class="text-h4 mb-4">Business Details</h2>
        <dl class="grid grid-cols-2 gap-4 text-sm">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Owner</dt><dd class="mt-0.5 font-medium">{{ s.owner_name }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Legal Name</dt><dd class="mt-0.5">{{ s.legal_business_name || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Type</dt><dd class="mt-0.5 capitalize">{{ (s.business_type||'').replace('_',' ') || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Commission</dt><dd class="mt-0.5">{{ s.commission_percent ?? 'Default' }}%</dd></div>
        </dl>
      </div>

      <div class="card p-6">
        <h2 class="text-h4 mb-4">KYC & Contact</h2>
        <dl class="grid grid-cols-2 gap-4 text-sm">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Email</dt><dd class="mt-0.5">{{ s.contact_email || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Phone</dt><dd class="mt-0.5">{{ s.contact_phone || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">GST</dt><dd class="mt-0.5">{{ s.gst_number || '—' }}</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">PAN</dt><dd class="mt-0.5">{{ s.pan_number || '—' }}</dd></div>
        </dl>
      </div>

      <div class="card p-6">
        <h2 class="text-h4 mb-4">Performance</h2>
        <dl class="grid grid-cols-2 gap-4 text-sm">
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Rating</dt><dd class="mt-0.5 font-medium">{{ s.rating_avg.toFixed(1) }} ★</dd></div>
          <div><dt class="text-xs uppercase tracking-wide text-stone-400">Reviews</dt><dd class="mt-0.5">{{ s.rating_count }}</dd></div>
        </dl>
      </div>

      <div class="card p-6">
        <h2 class="text-h4 mb-4">Application Status</h2>
        <dl class="grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt class="text-xs uppercase tracking-wide text-stone-400">Onboarding</dt>
            <dd class="mt-0.5">{{ s.onboarding_completed ? 'Completed' : 'Incomplete' }}</dd>
          </div>
          <div>
            <dt class="text-xs uppercase tracking-wide text-stone-400">Contract</dt>
            <dd class="mt-0.5">{{ s.contract_signed ? `Signed · ${s.contract_commission ?? s.commission_percent ?? '—'}%` : 'Not signed' }}</dd>
          </div>
          <div>
            <dt class="text-xs uppercase tracking-wide text-stone-400">Resubmissions</dt>
            <dd class="mt-0.5">{{ s.resubmit_count }} / 3</dd>
          </div>
        </dl>
      </div>

      <div v-if="s.rejection_reason" class="card p-6">
        <h2 class="text-h4 mb-2">Rejection Reason</h2>
        <p class="text-sm text-stone-600">{{ s.rejection_reason }}</p>
      </div>

      <div v-if="s.suspended_reason && ['suspended','blocked'].includes(s.status)" class="card p-6">
        <h2 class="text-h4 mb-2">{{ s.status === 'blocked' ? 'Block' : 'Suspension' }} Reason</h2>
        <p class="text-sm text-stone-600">{{ s.suspended_reason }}</p>
      </div>
    </div>

    <div v-if="s.store_description" class="card p-6 mt-5">
      <h2 class="text-h4 mb-2">Store Description</h2>
      <p class="text-sm text-stone-600">{{ s.store_description }}</p>
    </div>

    <!-- Confirm dialog -->
    <ConfirmDialog
      :open="dialog.open"
      :title="dialog.title"
      :variant="dialog.variant"
      :require-reason="dialog.requireReason"
      :confirm-label="dialog.confirmLabel"
      message="This action will notify the seller and update their store status."
      @confirm="handleConfirm"
      @cancel="dialog.open = false"
    />
  </div>
</template>
