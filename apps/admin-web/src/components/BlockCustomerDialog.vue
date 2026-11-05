<script setup lang="ts">
// "Block customer" dialog: the admin picks a reason from a dropdown (and adds a
// note when the reason needs one). The chosen reason is shown to the customer
// when they try to log in.
import { computed, ref, watch } from 'vue'
import { Ban, X } from 'lucide-vue-next'
import type { CustomerBlockReason } from '@carlitos/types'

const props = defineProps<{
  open: boolean
  customerName: string
  reasons: CustomerBlockReason[]
  busy?: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'confirm', payload: { reason_code: string; note: string }): void
  (e: 'cancel'): void
}>()

const code = ref('')
const note = ref('')

const selected = computed(() => props.reasons.find((r) => r.code === code.value) || null)
const noteRequired = computed(() => !!selected.value?.note_required)
const canSubmit = computed(
  () => !!selected.value && (!noteRequired.value || note.value.trim().length >= 3) && !props.busy,
)

// Start fresh every time the dialog opens.
watch(() => props.open, (o) => { if (o) { code.value = ''; note.value = '' } })

function submit() {
  if (!canSubmit.value) return
  emit('confirm', { reason_code: code.value, note: note.value.trim() })
}
</script>

<template>
  <transition name="dialog">
    <div v-if="open" class="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4" role="dialog" aria-modal="true" aria-labelledby="block-title">
      <div class="absolute inset-0 bg-black/40" @click="emit('cancel')" />
      <div class="relative max-h-[92vh] w-full overflow-y-auto rounded-t-2xl bg-white p-5 shadow-lg sm:max-w-md sm:rounded-xl sm:p-6">
        <div class="flex items-start gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-red-50">
            <Ban :size="20" class="text-red-500" />
          </div>
          <div class="min-w-0 flex-1">
            <h2 id="block-title" class="text-h4 text-ink">Block customer?</h2>
            <p class="mt-1 text-sm text-stone-600">
              <span class="font-medium text-ink">{{ customerName || 'This customer' }}</span>
              will be signed out and will not be able to log in. They will see the reason you choose below.
            </p>
          </div>
          <button class="shrink-0 text-stone-400 hover:text-stone-600" aria-label="Close" @click="emit('cancel')"><X :size="18" /></button>
        </div>

        <label class="label mt-5" for="block-reason">Reason for blocking</label>
        <select id="block-reason" v-model="code" class="select w-full">
          <option value="" disabled>Select a reason…</option>
          <option v-for="r in reasons" :key="r.code" :value="r.code">{{ r.label }}</option>
        </select>

        <label class="label mt-4" for="block-note">
          Note <span class="font-normal text-stone-400">{{ noteRequired ? '(required)' : '(optional)' }}</span>
        </label>
        <textarea
          id="block-note"
          v-model="note"
          class="textarea"
          rows="3"
          maxlength="300"
          :placeholder="noteRequired ? 'Explain the reason (the customer will see this)…' : 'Add more detail (the customer will see this)…'"
        />

        <p v-if="error" class="mt-3 rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-600">{{ error }}</p>

        <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button class="btn-ghost" @click="emit('cancel')">Cancel</button>
          <button class="btn-danger" :disabled="!canSubmit" @click="submit">
            <Ban :size="14" /> {{ busy ? 'Blocking…' : 'Block customer' }}
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>
